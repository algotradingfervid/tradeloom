// internal/app/stdstreams_test.go
package app

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
)

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuf) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func TestCaptureStdStreams_ScrubsQueryStrings(t *testing.T) {
	buf := &lockedBuf{}
	restore := CaptureStdStreams(slog.New(slog.NewJSONHandler(buf, nil)))
	fmt.Fprintln(os.Stdout, `base.HTTP: Request failed: Delete "https://api.kite.trade/session/token?api_key=k&access_token=SECRETTOKEN123": dial tcp: lookup api.kite.trade: no such host`)
	restore()
	out := buf.String()
	if strings.Contains(out, "SECRETTOKEN123") || !strings.Contains(out, "session/token?[redacted]") {
		t.Fatalf("log: %s", out)
	}
}
