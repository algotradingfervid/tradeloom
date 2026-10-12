> **P0-3 Session & Service Layer Implementation Plan — Task 4.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 4: Loopback callback listener

**Files:**
- Create: `internal/kite/loopback.go`
- Test: `internal/kite/loopback_test.go`

**Interfaces:**
- Consumes: `logging.Go`, `CallbackPath`.
- Produces (package-internal, used by Tasks 7 and 25):
  ```go
  type callbackHandler func(r *http.Request) (status int, html string)
  type loopback struct{ port int /* … */ }
  func startLoopback(port int, h callbackHandler, log *slog.Logger) (*loopback, error) // binds 127.0.0.1:<port>
  func (l *loopback) Close()
  func listenerHint(port int, err error) string
  func resultPage(title, msg string) string // HTML-escaped
  const pageTitleOK = "Logged in"; const pageMsgOK = "Logged in — you can close this browser tab."
  ```

The server never logs request lines. A request URL holds the `request_token`, and `net/http` does not log requests by default. `Cache-Control: no-store` and `Referrer-Policy: no-referrer` keep the token out of the browser cache and referrers.

- [ ] **Step 1: Write the failing test**

```go
// internal/kite/loopback_test.go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestLoopback|TestResultPage' -v`
Expected: FAIL to compile with `undefined: startLoopback`.

- [ ] **Step 3: Write the implementation**

```go
// internal/kite/loopback.go
package kite

import (
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"tradeloom/internal/logging"
)

const (
	pageTitleOK = "Logged in"
	pageMsgOK   = "Logged in — you can close this browser tab."
)

type callbackHandler func(r *http.Request) (status int, html string)

type loopback struct {
	port int
	ln   net.Listener
	srv  *http.Server
}

func startLoopback(port int, h callbackHandler, log *slog.Logger) (*loopback, error) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		status, body := h(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	l := &loopback{port: ln.Addr().(*net.TCPAddr).Port, ln: ln, srv: srv}
	logging.Go("kite.loopback", func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Warn("loopback listener stopped", "err", err)
		}
	})
	return l, nil
}

func (l *loopback) Close() { _ = l.srv.Close() }

func listenerHint(port int, err error) string {
	return fmt.Sprintf("Tradeloom could not listen on 127.0.0.1:%d (%v). Finish the login in the browser, then paste the address from the browser's address bar below, or choose another port in Settings → Kite connection.", port, err)
}

func resultPage(title, msg string) string {
	return "<!doctype html><html><head><meta charset=\"utf-8\"><title>Tradeloom</title></head>" +
		"<body style=\"font-family:sans-serif;max-width:40em;margin:4em auto\"><h1>" + html.EscapeString(title) +
		"</h1><p>" + html.EscapeString(msg) + "</p></body></html>"
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestLoopback|TestResultPage' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/kite/loopback.go internal/kite/loopback_test.go
git commit -m "feat(kite): 127.0.0.1 loopback callback listener

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
