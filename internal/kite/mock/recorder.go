// internal/kite/mock/recorder.go
package mock

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// Request is one call the app made to the fake Kite API. Header, RawQuery and Body keep what Form does
// not show (a DELETE body is never parsed into Form), so a test can scan everything that was sent.
// BodyErr is the error that cut reading the body short (Body then holds only what was read), so a
// test can fail closed instead of scanning a partial body.
type Request struct {
	Method   string
	Path     string
	Form     url.Values
	Header   http.Header
	RawQuery string
	Body     []byte
	BodyErr  error
}

// Recorder is a recording Kite HTTP fake: canned envelopes per route, 404 GeneralException otherwise.
// It records every request it receives, answered or not; Unmatched lists those no route answered, so a
// zero-writes test can fail closed on any path it did not expect (Task 24).
type Recorder struct {
	srv       *httptest.Server
	mu        sync.Mutex
	reqs      []Request
	unmatched []Request
	routes    map[string]route
}

type route struct {
	status int
	body   string
}

// NewRecorder starts the fake with default routes for session, profile, historical, orders, portfolio and LTP.
func NewRecorder(t testing.TB) *Recorder {
	r := &Recorder{routes: map[string]route{}}
	r.Route(http.MethodPost, "/session/token", 200, `{"status":"success","data":{"user_id":"AB1234","user_name":"Asha Bhat","api_key":"k","access_token":"acc-mock-0001"}}`)
	r.Route(http.MethodDelete, "/session/token", 200, `{"status":"success","data":true}`)
	r.Route(http.MethodGet, "/user/profile", 200, `{"status":"success","data":{"user_id":"AB1234","user_name":"Asha Bhat","email":"asha@example.com","broker":"ZERODHA","exchanges":["NSE","BSE"],"products":["CNC","MIS"]}}`)
	r.Route(http.MethodGet, "/instruments/historical/256265/day", 200, `{"status":"success","data":{"candles":[]}}`)
	r.Route(http.MethodGet, "/orders", 200, `{"status":"success","data":[]}`)
	r.Route(http.MethodGet, "/portfolio/positions", 200, `{"status":"success","data":{"net":[],"day":[]}}`)
	r.Route(http.MethodGet, "/portfolio/holdings", 200, `{"status":"success","data":[]}`)
	r.Route(http.MethodGet, "/quote/ltp", 200, `{"status":"success","data":{}}`)
	r.srv = httptest.NewServer(http.HandlerFunc(r.serve))
	t.Cleanup(r.srv.Close)
	return r
}

// Route sets (or replaces) the canned answer for method and path.
func (r *Recorder) Route(method, path string, status int, body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[method+" "+path] = route{status, body}
}

func (r *Recorder) URL() string          { return r.srv.URL }
func (r *Recorder) Client() *http.Client { return r.srv.Client() }

// Requests returns every request received so far, in arrival order.
func (r *Recorder) Requests() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Request(nil), r.reqs...)
}

// Unmatched returns the requests no route answered (they got the 404 GeneralException).
func (r *Recorder) Unmatched() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Request(nil), r.unmatched...)
}

func (r *Recorder) serve(w http.ResponseWriter, req *http.Request) {
	body, bodyErr := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(body))
	_ = req.ParseForm()
	rq := Request{Method: req.Method, Path: req.URL.Path, Form: req.Form, Header: req.Header.Clone(),
		RawQuery: req.URL.RawQuery, Body: body, BodyErr: bodyErr}
	r.mu.Lock()
	r.reqs = append(r.reqs, rq)
	rt, ok := r.routes[req.Method+" "+req.URL.Path]
	// A percent-encoded path (RawPath set) never matches a route: it would reach Kite as a different path.
	if req.URL.RawPath != "" {
		ok = false
	}
	if !ok {
		r.unmatched = append(r.unmatched, rq)
	}
	r.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"status":"error","error_type":"GeneralException","message":"route not mocked"}`)
		return
	}
	w.WriteHeader(rt.status)
	_, _ = io.WriteString(w, rt.body)
}

// IsWrite reports whether a request could change account state at Kite: every method except GET, on
// every path, except the login exchange (POST /session/token) and logout (DELETE /session/token).
// Methods and paths are compared exactly (no case folding, no trailing-slash or prefix match): fail closed.
func IsWrite(req Request) bool {
	if req.Method == http.MethodGet {
		return false
	}
	return !(req.Path == "/session/token" && (req.Method == http.MethodPost || req.Method == http.MethodDelete))
}
