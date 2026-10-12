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
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8 << 10,
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
