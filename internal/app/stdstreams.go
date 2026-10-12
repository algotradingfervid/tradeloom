// internal/app/stdstreams.go
package app

import (
	"bufio"
	"io"
	"log/slog"
	"os"
	"regexp"

	"tradeloom/internal/logging"
)

var urlQuery = regexp.MustCompile(`\?[^\s"']*`)

// CaptureStdStreams sends library output on stdout/stderr through the (redacting) logger, with URL query
// strings removed. Call it before the first kiteconnect client is created.
//
// Limits (Ruling 89 M3): it swaps the os.Stdout/os.Stderr variables only. Writers that hold the original
// file descriptors bypass it: Go runtime panics and fatal errors (the runtime writes to fd 2 directly; the
// crash reporter in internal/logging covers panics in logging.Go goroutines), and the standard library's
// default "log" logger, whose output was bound to the original os.Stderr at init (unless log.SetOutput is
// called later). A library that captured os.Stdout before this call also bypasses it.
func CaptureStdStreams(log *slog.Logger) (restore func()) {
	r, w, err := os.Pipe()
	if err != nil {
		log.Warn("capturing stdout/stderr failed", "err", err)
		return func() {}
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	done := make(chan struct{})
	logging.Go("app.stdstreams", func() {
		defer close(done)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			log.Warn("library output", "line", urlQuery.ReplaceAllString(sc.Text(), "?[redacted]"))
		}
		if err := sc.Err(); err != nil {
			// A line over 1 MiB stops the scanner; keep draining so writers never block on a full pipe.
			log.Warn("library output dropped", "err", err)
			_, _ = io.Copy(io.Discard, r)
		}
	})
	return func() {
		os.Stdout, os.Stderr = origOut, origErr
		_ = w.Close()
		<-done
		_ = r.Close()
	}
}
