package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

const (
	tAccess  = "acc3ssT0kenValue9876"
	tSecret  = "ap1S3cretValue12345"
	tRequest = "reqT0kenValue55555"
	tBot     = "123456789:AAH4bc_def-ghijklmnopqrstuvwxyz01234"
)

func newTestLogger(r *Redactor) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(NewRedactHandler(h, r)), &buf
}

type tokenHolder struct{ Token string }

func TestRedactHandler_RemovesSecrets(t *testing.T) {
	r := NewRedactor()
	r.Register(tAccess)
	r.Register(tSecret)
	cases := []struct {
		name string
		log  func(l *slog.Logger)
	}{
		{"sensitive key", func(l *slog.Logger) { l.Info("x", "access_token", "anything-at-all") }},
		{"sensitive key any case", func(l *slog.Logger) { l.Info("x", "Authorization", "token key:"+tAccess) }},
		{"registered value in plain attr", func(l *slog.Logger) { l.Info("x", "note", "value is "+tSecret) }},
		{"registered value in message", func(l *slog.Logger) { l.Info("login ok " + tAccess) }},
		{"query string request_token", func(l *slog.Logger) { l.Debug("http", "path", "/session/token?request_token="+tRequest+"&x=1") }},
		{"header dump", func(l *slog.Logger) { l.Debug("http", "raw", "Authorization: token abc:"+tRequest) }},
		{"telegram bot url", func(l *slog.Logger) { l.Info("tg", "url", "https://api.telegram.org/bot"+tBot+"/getMe") }},
		{"nested group", func(l *slog.Logger) { l.Info("x", slog.Group("req", slog.String("api_secret", tSecret))) }},
		{"error value", func(l *slog.Logger) { l.Error("x", "err", errors.New("kite: bad token "+tAccess)) }},
		{"struct value", func(l *slog.Logger) { l.Info("x", "holder", tokenHolder{Token: tAccess}) }},
		{"WithAttrs", func(l *slog.Logger) { l.With("checksum", "abcdef123456").Info("x") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l, buf := newTestLogger(r)
			tc.log(l)
			out := buf.String()
			for _, s := range []string{tAccess, tSecret, tRequest, tBot, "anything-at-all", "abcdef123456"} {
				if strings.Contains(out, s) {
					t.Fatalf("secret %q leaked: %s", s, out)
				}
			}
			if !strings.Contains(out, Redacted) {
				t.Fatalf("expected %q marker in %s", Redacted, out)
			}
		})
	}
}

func TestRedactHandler_KeepsHarmlessValues(t *testing.T) {
	l, buf := newTestLogger(NewRedactor())
	l.Info("store.migrated", "component", "store", "from", 0, "to", 1, "path", "/x/app.db")
	if strings.Contains(buf.String(), Redacted) {
		t.Fatalf("unexpected redaction: %s", buf.String())
	}
}

func TestRegister_IgnoresShortValues(t *testing.T) {
	r := NewRedactor()
	r.Register("true")
	if got := r.RedactString("true story"); got != "true story" {
		t.Fatalf("short value was registered: %q", got)
	}
}

func TestRedactHandler_Enabled(t *testing.T) {
	var lv slog.LevelVar
	h := NewRedactHandler(slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: &lv}), NewRedactor())
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug enabled at INFO")
	}
	lv.Set(slog.LevelDebug)
	if !h.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("debug not enabled after switching level at runtime")
	}
}
