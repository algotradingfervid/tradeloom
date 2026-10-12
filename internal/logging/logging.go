// Package logging owns Tradeloom's structured logs (FR-SHELL-018): slog JSON
// lines, size-based rotation, redaction of secrets, crash reports and the
// support bundle. It is a leaf package: it imports nothing from internal/.
package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// LogFileName is the active log file inside the logs directory.
	LogFileName = "tradeloom.log"
	// DefaultMaxSizeMB is the rotation size (FR-SHELL-018: 20 MB).
	DefaultMaxSizeMB = 20
	// DefaultMaxBackups keeps 9 rotated files + the active file = 10 files (FR-SHELL-018 AC3).
	DefaultMaxBackups = 9
)

// Options configures New.
type Options struct {
	Dir        string         // <datadir>/logs; created with mode 0700 if missing
	Level      *slog.LevelVar // runtime-switchable level; nil means a new LevelVar at INFO
	MaxSizeMB  int            // 0 means DefaultMaxSizeMB
	MaxBackups int            // 0 means DefaultMaxBackups
	Redactor   *Redactor      // nil means Default
	Stderr     io.Writer      // optional mirror (development); nil disables
}

// Logger is the configured root logger plus the handles needed at runtime.
type Logger struct {
	*slog.Logger
	Level *slog.LevelVar
	out   *lumberjack.Logger
}

// New creates the JSON logger writing to <Dir>/tradeloom.log with rotation and redaction.
// The message key is renamed to "event"; callers add "component" via Logger.With("component", name).
func New(opts Options) (*Logger, error) {
	if opts.Dir == "" {
		return nil, errors.New("logging: Options.Dir is required")
	}
	if err := os.MkdirAll(opts.Dir, 0o700); err != nil {
		return nil, fmt.Errorf("logging: create %s: %w", opts.Dir, err)
	}
	if opts.Level == nil {
		opts.Level = new(slog.LevelVar)
	}
	if opts.MaxSizeMB == 0 {
		opts.MaxSizeMB = DefaultMaxSizeMB
	}
	if opts.MaxBackups == 0 {
		opts.MaxBackups = DefaultMaxBackups
	}
	out := &lumberjack.Logger{
		Filename:   filepath.Join(opts.Dir, LogFileName),
		MaxSize:    opts.MaxSizeMB,
		MaxBackups: opts.MaxBackups,
	}
	var w io.Writer = out
	if opts.Stderr != nil {
		w = io.MultiWriter(out, opts.Stderr)
	}
	jh := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: opts.Level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.MessageKey {
				a.Key = "event"
			}
			return a
		},
	})
	return &Logger{Logger: slog.New(NewRedactHandler(jh, opts.Redactor)), Level: opts.Level, out: out}, nil
}

// Close flushes and closes the active log file.
func (l *Logger) Close() error { return l.out.Close() }

// ParseLevel maps the settings value of "log.level" ("debug"|"info") to a slog.Level.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	}
	return slog.LevelInfo, fmt.Errorf("logging: unknown level %q (want debug|info)", s)
}
