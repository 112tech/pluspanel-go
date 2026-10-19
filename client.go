// Copyright (c) 2026 2Lines Sistem SRL. Licensed under the MIT License (see LICENSE).

package pluspanel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Version is the version of this client, reported in the User-Agent header.
const Version = "1.0.0-beta.0"

// UserAgent is the User-Agent header value sent by clients made with NewWithAPIKey.
const UserAgent = "pluspanel-go/" + Version

// BasePath is the path of the API under a panel's origin.
const BasePath = "/api/v1"

// ErrInvalidToken is returned by NewWithAPIKey for an empty token or one that cannot
// be sent in an HTTP header.
var ErrInvalidToken = errors.New("pluspanel: invalid API token")

// ErrInsecureBaseURL is returned by NewWithAPIKey when the base URL would send the
// token in clear text to a host other than the local machine.
var ErrInsecureBaseURL = errors.New("pluspanel: base URL must use https (http is allowed for loopback hosts only)")

// NewWithAPIKey returns a client that authenticates every request with an API key
// (Authorization: Bearer <token>) and identifies itself as UserAgent.
//
// baseURL is the panel's address, e.g. "https://panel.example.com". When it has no
// path, BasePath (/api/v1) is appended; a URL that already has a path is used as is.
// Plain http is refused except for loopback hosts, so a token never crosses a network
// in clear text. opts are applied after the client's own settings, so a
// WithRequestEditorFn passed here runs after (and may override) the defaults.
func NewWithAPIKey(baseURL, token string, opts ...ClientOption) (*ClientWithResponses, error) {
	if !validToken(token) {
		return nil, ErrInvalidToken
	}
	server, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	auth := "Bearer " + token
	all := make([]ClientOption, 0, len(opts)+1)
	all = append(all, WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", auth)
		req.Header.Set("User-Agent", UserAgent)
		return nil
	}))
	all = append(all, opts...)
	return NewClientWithResponses(server, all...)
}

// validToken accepts visible ASCII only: anything else could split or corrupt the
// Authorization header.
func validToken(token string) bool {
	if token == "" {
		return false
	}
	for i := range len(token) {
		if c := token[i]; c <= ' ' || c > '~' {
			return false
		}
	}
	return true
}

func normalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("pluspanel: parse base URL: %w", err)
	}
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("pluspanel: base URL %q must be scheme://host[:port][/path]", raw)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopback(u.Hostname()) {
			return "", ErrInsecureBaseURL
		}
	default:
		return "", ErrInsecureBaseURL
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = BasePath
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u.String(), nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
