// internal/kite/netguard_test.go
package kite

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestAllowListTransport_RefusesOtherHosts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	hc := &http.Client{Transport: NewAllowListTransport(http.DefaultTransport, []string{u.Hostname()})}
	resp, err := hc.Get(srv.URL + "/user/profile")
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("allowed host: %v %v", resp, err)
	}
	resp.Body.Close()
	_, err = hc.Get("http://example.invalid/orders")
	if !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("other host: %v", err)
	}
	if len(AllowedHosts) != 3 || AllowedHosts[0] != "api.kite.trade" || AllowedHosts[1] != "kite.zerodha.com" || AllowedHosts[2] != "ws.kite.trade" {
		t.Fatalf("AllowedHosts = %v", AllowedHosts)
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Ruling 52 (5): plain http is refused for every host but loopback test hosts.
func TestAllowListTransport_RequiresHTTPSExceptLoopback(t *testing.T) {
	reached := 0
	next := rtFunc(func(*http.Request) (*http.Response, error) {
		reached++
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
	})
	tr := NewAllowListTransport(next, []string{"api.kite.trade", "127.0.0.1", "localhost", "::1"})
	for _, u := range []string{"https://api.kite.trade/user/profile", "http://127.0.0.1:7420/x", "http://localhost:1/x", "http://[::1]:1/x"} {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		if _, err := tr.RoundTrip(req); err != nil {
			t.Fatalf("%s: %v", u, err)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, "http://api.kite.trade/user/profile", nil)
	if _, err := tr.RoundTrip(req); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("plain http to Kite: %v", err)
	}
	if reached != 4 {
		t.Fatalf("reached next %d times", reached)
	}
}

// Ruling 52 (5): NewAuth(nil, …) never falls back to the library's unguarded default client.
func TestNewAuth_NilClientUsesGuardedTransport(t *testing.T) {
	hc := authHTTPClient(nil)
	tr, ok := hc.Transport.(*allowListTransport)
	if !ok || hc.Timeout != 10*time.Second {
		t.Fatalf("client %+v", hc)
	}
	for _, h := range AllowedHosts {
		if !tr.allowed[h] {
			t.Fatalf("%s missing from %v", h, tr.allowed)
		}
	}
	if len(tr.allowed) != len(AllowedHosts) {
		t.Fatalf("allowed %v", tr.allowed)
	}
	// gokiteconnect reports every transport failure as "Request failed.", so the refusal is checked on the client.
	req, _ := http.NewRequest(http.MethodPost, "https://example.invalid/session/token", nil)
	if _, err := hc.Transport.RoundTrip(req); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("err %v", err)
	}
}
