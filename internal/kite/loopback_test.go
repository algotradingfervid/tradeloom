package kite

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
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
