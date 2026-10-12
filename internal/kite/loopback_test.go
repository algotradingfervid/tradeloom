package kite

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return p
}

func TestLoopback_ServesCallbackOnLoopbackOnly(t *testing.T) {
	port := freePort(t)
	var gotQuery string
	lb, err := startLoopback(port, func(r *http.Request) (int, string) {
		gotQuery = r.URL.RawQuery
		return http.StatusOK, resultPage(pageTitleOK, pageMsgOK)
	}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer lb.Close()
	if host := lb.ln.Addr().(*net.TCPAddr).IP.String(); host != "127.0.0.1" {
		t.Fatalf("listening on %s, want 127.0.0.1", host)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d%s?request_token=AbC123xyz789", port, CallbackPath))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "you can close this browser tab") {
		t.Fatalf("status %d body %s", resp.StatusCode, body)
	}
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers %v", resp.Header)
	}
	if gotQuery != "request_token=AbC123xyz789" {
		t.Fatalf("handler saw %q", gotQuery)
	}

	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/other", port))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("/other status %d", resp.StatusCode)
	}

	resp, err = http.Post(fmt.Sprintf("http://127.0.0.1:%d%s", port, CallbackPath), "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST status %d", resp.StatusCode)
	}
}

func TestLoopback_PortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	_, err = startLoopback(port, func(*http.Request) (int, string) { return 200, "" }, slog.Default())
	if err == nil {
		t.Fatal("expected bind error")
	}
	hint := listenerHint(port, err)
	if !strings.Contains(hint, fmt.Sprint(port)) || !strings.Contains(hint, "paste") {
		t.Fatalf("hint %q", hint)
	}
}

func TestResultPage_EscapesHTML(t *testing.T) {
	p := resultPage("<b>", "<script>alert(1)</script>")
	if strings.Contains(p, "<script>") || !strings.Contains(p, "&lt;script&gt;") {
		t.Fatalf("not escaped: %s", p)
	}
}

// Ruling 57: the listener answers only requests addressed to 127.0.0.1:<port> (DNS-rebinding guard),
// and every page forbids framing.
func TestLoopback_RejectsForeignHostAndForbidsFraming(t *testing.T) {
	port := freePort(t)
	var calls atomic.Int32
	lb, err := startLoopback(port, func(r *http.Request) (int, string) {
		calls.Add(1)
		return http.StatusOK, resultPage(pageTitleOK, pageMsgOK)
	}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	defer lb.Close()
	u := fmt.Sprintf("http://127.0.0.1:%d%s?request_token=AbC123xyz789", port, CallbackPath)
	for _, host := range []string{"localhost:" + fmt.Sprint(port), "evil.example", fmt.Sprintf("evil.example:%d", port), "127.0.0.1", "127.0.0.1:1", "[::1]:" + fmt.Sprint(port)} {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("host %q: status %d", host, resp.StatusCode)
		}
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("handler ran %d times for foreign hosts", n)
	}
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("own host: status %d calls %d", resp.StatusCode, calls.Load())
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("CSP %q", csp)
	}
}
