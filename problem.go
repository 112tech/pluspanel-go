// Copyright (c) 2026 2Lines Sistem SRL. Licensed under the MIT License (see LICENSE).

package pluspanel

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Problem is an API error: an RFC 9457 problem details object
// (application/problem+json) with the PlusPanel extensions code and request_id.
//
// Answers in the older envelope {"error": {"code", "message", "request_id"}} decode
// into the same fields, so callers handle one type.
type Problem struct { //nolint:errname // RFC 9457 name; the public API of this package.
	// Type is a URI identifying the problem, https://pluspanel.net/problems/<code>.
	Type string `json:"type,omitempty"`
	// Title is a short summary of the problem type.
	Title string `json:"title,omitempty"`
	// Status is the HTTP status code.
	Status int `json:"status,omitempty"`
	// Detail explains this occurrence of the problem.
	Detail string `json:"detail,omitempty"`
	// Instance is the request path.
	Instance string `json:"instance,omitempty"`
	// Code is the stable machine-readable error code, e.g. "not_found".
	Code string `json:"code,omitempty"`
	// RequestID identifies the request in the panel's logs (quote it to support).
	RequestID string `json:"request_id,omitempty"`
}

// Error implements error.
func (p *Problem) Error() string {
	var b strings.Builder
	b.WriteString("pluspanel: ")
	fmt.Fprintf(&b, "%d", p.Status)
	if p.Code != "" {
		b.WriteString(" " + p.Code)
	}
	switch {
	case p.Detail != "":
		b.WriteString(": " + p.Detail)
	case p.Title != "":
		b.WriteString(": " + p.Title)
	}
	if p.RequestID != "" {
		b.WriteString(" (request " + p.RequestID + ")")
	}
	return b.String()
}

// legacyEnvelope is the ADR-0002 error member, kept next to the RFC 9457 members.
type legacyEnvelope struct {
	Error *struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

// CheckResponse returns nil for a response below 400 and a *Problem otherwise, decoded
// from body when it is a problem (or legacy error) document. A generated response
// passes its fields: CheckResponse(res.HTTPResponse, res.Body).
func CheckResponse(resp *http.Response, body []byte) error {
	if resp == nil {
		return errors.New("pluspanel: no response")
	}
	if resp.StatusCode < http.StatusBadRequest {
		return nil
	}
	p := &Problem{}
	var legacy legacyEnvelope
	if json.Unmarshal(body, p) != nil {
		*p = Problem{} // not a JSON object: describe the response from its status
	} else if json.Unmarshal(body, &legacy) == nil && legacy.Error != nil {
		p.Code = firstNonEmpty(p.Code, legacy.Error.Code)
		p.Detail = firstNonEmpty(p.Detail, legacy.Error.Message)
		p.RequestID = firstNonEmpty(p.RequestID, legacy.Error.RequestID)
	}
	if p.Status == 0 {
		p.Status = resp.StatusCode
	}
	if p.Title == "" {
		p.Title = http.StatusText(resp.StatusCode)
	}
	p.RequestID = firstNonEmpty(p.RequestID, resp.Header.Get("X-Request-Id"))
	return p
}

// IsProblem reports whether err is (or wraps) a *Problem with the given code.
func IsProblem(err error, code string) bool {
	var p *Problem
	return errors.As(err, &p) && p.Code == code
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
