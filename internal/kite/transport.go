package kite

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	kiteconnect "github.com/zerodha/gokiteconnect/v4"

	"tradeloom/internal/clock"
)

// recorder is the RoundTripper under every gokiteconnect client. It (a) hands every response's
// Date header to the app clock (FR-DATA-004) and (b) remembers the error envelope of a failed
// instrument dump, because gokiteconnect's GetInstruments does not check the HTTP status
// (market.go parseInstruments) and would otherwise report a CSV parse error.
type recorder struct {
	next http.RoundTripper
	clk  clock.Clock
	sink func(date, sent, received time.Time)

	mu      sync.Mutex
	dumpErr *kiteconnect.Error
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	sent := r.clk.System()
	resp, err := r.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	received := r.clk.System()
	if r.sink != nil {
		if d, perr := http.ParseTime(resp.Header.Get("Date")); perr == nil {
			r.sink(d, sent, received)
		}
	}
	if isDump(req.URL.Path) && resp.StatusCode >= 400 {
		// An error envelope is small; cap the read so a hostile or broken server cannot make us
		// buffer an unbounded body (the rest is discarded with the connection).
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxDumpErrBody))
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		var env struct {
			ErrorType string `json:"error_type"`
			Message   string `json:"message"`
		}
		_ = json.Unmarshal(body, &env)
		if env.ErrorType == "" {
			env.ErrorType = kiteconnect.GetErrorName(resp.StatusCode)
		}
		if env.Message == "" {
			env.Message = http.StatusText(resp.StatusCode)
		}
		r.mu.Lock()
		r.dumpErr = &kiteconnect.Error{Code: resp.StatusCode, ErrorType: env.ErrorType, Message: env.Message}
		r.mu.Unlock()
	}
	return resp, nil
}

const maxDumpErrBody = 64 << 10

// ctxTransport binds one attempt's requests to the call's ctx: gokiteconnect builds requests
// with context.Background, so without it a cancelled call would wait out the http.Client
// timeout. The request keeps its own context too (http.Client's deadline lives there); the
// merged context is released when the response body is closed (gokiteconnect always closes it).
type ctxTransport struct {
	ctx  context.Context
	next http.RoundTripper
}

func (t ctxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(t.ctx, cancel)
	release := func() { stop(); cancel() }
	resp, err := t.next.RoundTrip(req.WithContext(ctx))
	if err != nil {
		release()
		return nil, err
	}
	resp.Body = &releaseBody{ReadCloser: resp.Body, release: release}
	return resp, nil
}

type releaseBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *releaseBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}

func isDump(path string) bool {
	return path == "/instruments" || (strings.HasPrefix(path, "/instruments/") && !strings.HasPrefix(path, "/instruments/historical/"))
}

// takeDumpErr returns and clears the last recorded dump failure.
func (r *recorder) takeDumpErr() *kiteconnect.Error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.dumpErr
	r.dumpErr = nil
	return e
}
