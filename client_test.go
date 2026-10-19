// Copyright (c) 2026 2Lines Sistem SRL. Licensed under the MIT License (see LICENSE).

package pluspanel_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/112tech/pluspanel-go"
)

const token = "ppk_abcdefghijkl_c2VjcmV0LXNlY3JldC1zZWNyZXQtc2VjcmV0LXNlY3I"

// fakePanel answers like the control plane: bearer auth on every route, a page of
// servers, a problem+json document for an unknown server.
func fakePanel(t *testing.T) *httptest.Server {
	t.Helper()
	serverID := uuid.MustParse("6f1c1f9e-8f8a-4a7e-9d43-3c3f2f5c9b10")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/servers", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "2" {
			t.Errorf("limit = %q", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"items":[{"id":%q,"name":"web-1","status":"online","channel":"stable",
			"channel_explicit":false,"lab":false,"update_ring_internal":false,"created_at":"2026-10-01T10:00:00Z",
			"agent_version":null,"enrolled_at":null,"os_image_digest":null,"os_version":null,
			"public_ipv4":null,"public_ipv6":null}],"next_cursor":"c2"}`, serverID)
	})
	mux.HandleFunc("POST /api/v1/servers", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var in map[string]any
		if err := json.Unmarshal(body, &in); err != nil || in["name"] != "web-2" {
			t.Errorf("create body = %s", body)
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"type":"https://pluspanel.net/problems/edition_limit_reached","title":"Unprocessable Entity",
			"status":409,"detail":"The Free edition allows 1 server.","instance":"/api/v1/servers",
			"code":"edition_limit_reached","request_id":"req-1",
			"error":{"code":"edition_limit_reached","message":"The Free edition allows 1 server.","request_id":"req-1"}}`)
	})
	mux.HandleFunc("GET /api/v1/servers/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"type":"https://pluspanel.net/problems/not_found","title":"Not Found","status":404,
			"detail":"server not found","instance":"/api/v1/servers/x","code":"not_found","request_id":"req-2"}`)
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("%s %s: Authorization = %q", r.Method, r.URL.Path, got)
		}
		if got := r.Header.Get("User-Agent"); got != pluspanel.UserAgent {
			t.Errorf("User-Agent = %q", got)
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestListServersWithAPIKey(t *testing.T) {
	t.Parallel()
	srv := fakePanel(t)
	c, err := pluspanel.NewWithAPIKey(srv.URL, token)
	if err != nil {
		t.Fatalf("NewWithAPIKey: %v", err)
	}
	limit := int64(2)
	res, err := c.ListServersWithResponse(context.Background(), &pluspanel.ListServersParams{Limit: &limit})
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if err := pluspanel.CheckResponse(res.HTTPResponse, res.Body); err != nil {
		t.Fatalf("CheckResponse: %v", err)
	}
	if res.JSON200 == nil || res.JSON200.Items == nil || len(*res.JSON200.Items) != 1 {
		t.Fatalf("page = %+v", res.JSON200)
	}
	s := (*res.JSON200.Items)[0]
	if s.Name != "web-1" || s.Channel != "stable" || s.Id.String() != "6f1c1f9e-8f8a-4a7e-9d43-3c3f2f5c9b10" {
		t.Errorf("server = %+v", s)
	}
	if res.JSON200.NextCursor == nil || *res.JSON200.NextCursor != "c2" {
		t.Errorf("next_cursor = %v", res.JSON200.NextCursor)
	}
}

func TestProblemFromOperations(t *testing.T) {
	t.Parallel()
	srv := fakePanel(t)
	// A caller's own editor runs after the defaults (here: adds a header, keeps auth).
	c, err := pluspanel.NewWithAPIKey(srv.URL+"/api/v1/", token, pluspanel.WithRequestEditorFn(
		func(_ context.Context, r *http.Request) error { r.Header.Set("X-Request-ID", "mine"); return nil }))
	if err != nil {
		t.Fatalf("NewWithAPIKey: %v", err)
	}
	ctx := context.Background()

	got, err := c.GetServerWithResponse(ctx, uuid.New(), nil)
	if err != nil {
		t.Fatalf("GetServer: %v", err)
	}
	// The generated response decodes the problem document into the package's Problem.
	if got.ApplicationproblemJSON404 == nil || got.ApplicationproblemJSON404.Code != "not_found" {
		t.Errorf("ApplicationproblemJSON404 = %+v", got.ApplicationproblemJSON404)
	}
	err = pluspanel.CheckResponse(got.HTTPResponse, got.Body)
	var p *pluspanel.Problem
	if !errors.As(err, &p) {
		t.Fatalf("err = %v, want *Problem", err)
	}
	want := pluspanel.Problem{
		Type: "https://pluspanel.net/problems/not_found", Title: "Not Found", Status: 404,
		Detail: "server not found", Instance: "/api/v1/servers/x", Code: "not_found", RequestID: "req-2",
	}
	if *p != want {
		t.Errorf("problem = %+v", *p)
	}
	if !pluspanel.IsProblem(err, "not_found") || pluspanel.IsProblem(err, "forbidden") {
		t.Error("IsProblem")
	}

	created, err := c.CreateServerWithResponse(ctx, nil, pluspanel.CreateServerJSONRequestBody{Name: "web-2"})
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	err = pluspanel.CheckResponse(created.HTTPResponse, created.Body)
	// The body's status wins over the transport status (they agree in practice).
	if !errors.As(err, &p) || p.Status != 409 || p.Code != "edition_limit_reached" || p.RequestID != "req-1" {
		t.Errorf("create problem = %v", err)
	}
}

func TestCheckResponse(t *testing.T) {
	t.Parallel()
	hdr := func(id string) http.Header {
		h := http.Header{}
		if id != "" {
			h.Set("X-Request-ID", id)
		}
		return h
	}
	for _, tc := range []struct {
		name   string
		resp   *http.Response
		body   string
		want   *pluspanel.Problem
		errMsg string
	}{
		{name: "nil response", errMsg: "pluspanel: no response"},
		{name: "success", resp: &http.Response{StatusCode: 204, Header: hdr("")}},
		{name: "redirect is not an error", resp: &http.Response{StatusCode: 304, Header: hdr("")}},
		{
			name: "legacy envelope", resp: &http.Response{StatusCode: 403, Header: hdr("")},
			body:   `{"error":{"code":"forbidden","message":"missing permission","request_id":"r9"}}`,
			want:   &pluspanel.Problem{Title: "Forbidden", Status: 403, Detail: "missing permission", Code: "forbidden", RequestID: "r9"},
			errMsg: "pluspanel: 403 forbidden: missing permission (request r9)",
		},
		{
			name: "problem wins over legacy members", resp: &http.Response{StatusCode: 409, Header: hdr("")},
			body: `{"title":"Conflict","status":409,"detail":"new","code":"a","request_id":"r1","error":{"code":"b","message":"old","request_id":"r2"}}`,
			want: &pluspanel.Problem{Title: "Conflict", Status: 409, Detail: "new", Code: "a", RequestID: "r1"},
		},
		{
			name: "not json", resp: &http.Response{StatusCode: 502, Header: hdr("hdr-id")}, body: "<html>bad gateway</html>",
			want:   &pluspanel.Problem{Title: "Bad Gateway", Status: 502, RequestID: "hdr-id"},
			errMsg: "pluspanel: 502: Bad Gateway (request hdr-id)",
		},
		{
			name: "empty body", resp: &http.Response{StatusCode: 429, Header: hdr("")},
			want:   &pluspanel.Problem{Title: "Too Many Requests", Status: 429},
			errMsg: "pluspanel: 429: Too Many Requests",
		},
		{
			name: "unknown status keeps empty title", resp: &http.Response{StatusCode: 599, Header: hdr("")},
			want: &pluspanel.Problem{Status: 599}, errMsg: "pluspanel: 599",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := pluspanel.CheckResponse(tc.resp, []byte(tc.body))
			if tc.want == nil && tc.errMsg == "" {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if tc.errMsg != "" && (err == nil || err.Error() != tc.errMsg) {
				t.Errorf("Error() = %v, want %q", err, tc.errMsg)
			}
			if tc.want != nil {
				var p *pluspanel.Problem
				if !errors.As(err, &p) || *p != *tc.want {
					t.Errorf("problem = %+v, want %+v", err, *tc.want)
				}
			}
		})
	}
}

func TestNewWithAPIKeyValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, base, token string
		wantErr           error
		wantServer        string
		errContains       string
	}{
		{name: "origin gets the base path", base: "https://panel.example.com", token: token, wantServer: "https://panel.example.com/api/v1/"},
		{name: "root slash gets the base path", base: "https://panel.example.com/", token: token, wantServer: "https://panel.example.com/api/v1/"},
		{name: "explicit path kept", base: "https://example.com/pp/api/v1/", token: token, wantServer: "https://example.com/pp/api/v1/"},
		{name: "http on loopback v4", base: "http://127.0.0.1:8080", token: token, wantServer: "http://127.0.0.1:8080/api/v1/"},
		{name: "http on loopback v6", base: "http://[::1]:8080", token: token, wantServer: "http://[::1]:8080/api/v1/"},
		{name: "http on localhost", base: "http://LocalHost:8080", token: token, wantServer: "http://LocalHost:8080/api/v1/"},
		{name: "http elsewhere", base: "http://panel.example.com", token: token, wantErr: pluspanel.ErrInsecureBaseURL},
		{name: "other scheme", base: "ftp://panel.example.com", token: token, wantErr: pluspanel.ErrInsecureBaseURL},
		{name: "no host", base: "/api/v1", token: token, errContains: "must be scheme://host"},
		{name: "userinfo", base: "https://u:p@panel.example.com", token: token, errContains: "must be scheme://host"},
		{name: "query", base: "https://panel.example.com?x=1", token: token, errContains: "must be scheme://host"},
		{name: "fragment", base: "https://panel.example.com#x", token: token, errContains: "must be scheme://host"},
		{name: "unparsable", base: "https://panel.example.com/%zz", token: token, errContains: "parse base URL"},
		{name: "empty token", base: "https://panel.example.com", token: "", wantErr: pluspanel.ErrInvalidToken},
		{name: "token with space", base: "https://panel.example.com", token: "a b", wantErr: pluspanel.ErrInvalidToken},
		{name: "token with newline", base: "https://panel.example.com", token: "a\r\nX-Evil: 1", wantErr: pluspanel.ErrInvalidToken},
		{name: "token with non-ASCII", base: "https://panel.example.com", token: "tökén", wantErr: pluspanel.ErrInvalidToken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, err := pluspanel.NewWithAPIKey(tc.base, tc.token)
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("err = %v, want %v", err, tc.wantErr)
				}
			case tc.errContains != "":
				if err == nil || !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("err = %v, want %q", err, tc.errContains)
				}
			default:
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				inner, ok := c.ClientInterface.(*pluspanel.Client)
				if !ok || inner.Server != tc.wantServer {
					t.Errorf("server = %+v, want %s", c.ClientInterface, tc.wantServer)
				}
			}
		})
	}
}

func TestNewWithAPIKeyOptionError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	_, err := pluspanel.NewWithAPIKey("https://panel.example.com", token, func(*pluspanel.Client) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}
