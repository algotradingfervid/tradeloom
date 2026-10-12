// internal/kite/netguard_test.go
package kite

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
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
