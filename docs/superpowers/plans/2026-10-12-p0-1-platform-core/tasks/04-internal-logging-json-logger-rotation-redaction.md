> **P0-1 Platform Core Implementation Plan — Task 4.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 4: `internal/logging` — JSON logger, rotation, redaction, runtime level

**Files:**
- Create: `internal/logging/redact.go`, `internal/logging/handler.go`, `internal/logging/logging.go`, `internal/logging/redact_test.go`, `internal/logging/logging_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces (`package logging`):
  - `const Redacted = "[redacted]"`; `const LogFileName = "tradeloom.log"`; `const DefaultMaxSizeMB = 20`; `const DefaultMaxBackups = 9`
  - `type Redactor struct{…}`; `func NewRedactor() *Redactor`; `var Default = NewRedactor()`; `func RegisterSecret(v string)`; `func (r *Redactor) Register(v string)`; `func (r *Redactor) IsSensitiveKey(key string) bool`; `func (r *Redactor) RedactString(s string) string`; `func (r *Redactor) ScrubBytes(b []byte) []byte`
  - `type RedactHandler struct{…}` (implements `slog.Handler`); `func NewRedactHandler(next slog.Handler, r *Redactor) *RedactHandler`
  - `type Options struct { Dir string; Level *slog.LevelVar; MaxSizeMB int; MaxBackups int; Redactor *Redactor; Stderr io.Writer }`
  - `type Logger struct { *slog.Logger; Level *slog.LevelVar; … }`; `func New(opts Options) (*Logger, error)`; `func (l *Logger) Close() error`
  - `func ParseLevel(s string) (slog.Level, error)` — `"debug"|"info"`
  - Convention for every later package: `log.With("component", "<pkg>")`, message = dotted event name (`"store.migrated"`); the JSON key for the message is `event`. Sensitive keys (substring, case-insensitive): `authorization`, `request_token`, `access_token`, `api_secret`, `checksum`, `bot_token`, `passphrase`. Kite HTTP debug logging (P0-2) must log through this handler and add its own call-site test (FR-SHELL-018 "unit-tested against every call site that logs HTTP traffic").

- [ ] **Step 1: Write the failing tests**

`internal/logging/redact_test.go`:

```go
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
```

`internal/logging/logging_test.go`:

```go
package logging

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNew_WritesJSONWithEventAndComponent(t *testing.T) {
	dir := t.TempDir()
	lg, err := New(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	lg.With("component", "store").Info("store.opened", "path", "app.db")
	lg.Debug("hidden.at.info")
	if err := lg.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, LogFileName))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line at INFO, got %d: %s", len(lines), b)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"time", "level", "event", "component", "path"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q in %v", k, m)
		}
	}
	if m["event"] != "store.opened" || m["component"] != "store" {
		t.Fatalf("bad event/component: %v", m)
	}
}

func TestDefaults_MatchSpec(t *testing.T) {
	if DefaultMaxSizeMB != 20 || DefaultMaxBackups+1 != 10 {
		t.Fatalf("rotation defaults drifted from FR-SHELL-018: %d MB, %d backups", DefaultMaxSizeMB, DefaultMaxBackups)
	}
}

// FR-SHELL-018 AC3, scaled 1:20 (1 MB files, 25 MB written) so it runs in CI time;
// Review Focus: concurrent writers must not interleave or corrupt lines across rotations.
func TestRotation_BoundsFilesUnderConcurrentWriters(t *testing.T) {
	dir := t.TempDir()
	lg, err := New(Options{Dir: dir, MaxSizeMB: 1})
	if err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("x", 900)
	const writers, perWriter = 16, 1700 // ≈ 16 × 1700 × 1 KB ≈ 26 MB
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				lg.Info("load", "writer", w, "seq", i, "pad", payload)
			}
		}(w)
	}
	wg.Wait()
	if err := lg.Close(); err != nil {
		t.Fatal(err)
	}
	var files []string
	deadline := time.Now().Add(5 * time.Second) // lumberjack deletes old backups asynchronously
	for {
		files, _ = filepath.Glob(filepath.Join(dir, "tradeloom*.log"))
		if len(files) <= 10 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(files) > 10 {
		t.Fatalf("want ≤ 10 log files, got %d", len(files))
	}
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		if st.Size() > 1<<20 {
			t.Errorf("%s is %d bytes, over the 1 MB cap", f, st.Size())
		}
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
		n := 0
		for sc.Scan() {
			n++
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Fatalf("%s line %d is not JSON (interleaved write?): %v", f, n, err)
			}
		}
		fh.Close()
	}
}

func TestParseLevel(t *testing.T) {
	for _, tc := range []struct {
		in      string
		wantErr bool
	}{{"debug", false}, {"INFO", false}, {"trace", true}} {
		_, err := ParseLevel(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseLevel(%q) err=%v", tc.in, err)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -tags noorders ./internal/logging/`
Expected: FAIL — build errors such as `undefined: Redactor` and `undefined: NewRedactHandler`.

- [ ] **Step 3: Write the implementation**

```bash
go get gopkg.in/natefinch/lumberjack.v2@v2.2.1
```

`internal/logging/redact.go`:

```go
package logging

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Redacted is the replacement text for every removed secret.
const Redacted = "[redacted]"

// minSecretLen keeps short, common strings (e.g. "true") from being registered by mistake.
const minSecretLen = 6

var sensitiveKey = regexp.MustCompile(`(?i)(authorization|request_token|access_token|api_secret|checksum|bot_token|passphrase)`)

// valuePatterns catch secrets embedded in free text before they are registered
// (query strings, header dumps, Telegram bot URLs).
var valuePatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)\b(request_token|access_token|api_secret|checksum|bot_token)=[^&\s"']+`), "${1}=" + Redacted},
	{regexp.MustCompile(`(?i)\b(authorization["']?\s*[:=]\s*["']?)[^"'\r\n]+`), "${1}" + Redacted},
	{regexp.MustCompile(`\bbot\d{6,}:[A-Za-z0-9_-]{30,}`), "bot" + Redacted},
}

// Redactor removes secrets from log attributes, messages and arbitrary bytes.
// It is safe for concurrent use.
type Redactor struct {
	mu       sync.RWMutex
	secrets  map[string]struct{}
	replacer *strings.Replacer
}

// NewRedactor returns an empty Redactor.
func NewRedactor() *Redactor {
	return &Redactor{secrets: map[string]struct{}{}, replacer: strings.NewReplacer()}
}

// Default is the process-wide Redactor used by New and RegisterSecret.
var Default = NewRedactor()

// RegisterSecret registers v with the Default redactor (the secrets store calls it for every value it loads or stores).
func RegisterSecret(v string) { Default.Register(v) }

// Register adds v to the set of literal values that are always replaced. Values shorter than 6 bytes are ignored.
func (r *Redactor) Register(v string) {
	if len(v) < minSecretLen {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.secrets[v]; ok {
		return
	}
	r.secrets[v] = struct{}{}
	vals := make([]string, 0, len(r.secrets))
	for s := range r.secrets {
		vals = append(vals, s)
	}
	// Longest first so a secret containing another secret is replaced whole.
	sort.Slice(vals, func(i, j int) bool { return len(vals[i]) > len(vals[j]) })
	pairs := make([]string, 0, 2*len(vals))
	for _, s := range vals {
		pairs = append(pairs, s, Redacted)
	}
	r.replacer = strings.NewReplacer(pairs...)
}

// IsSensitiveKey reports whether an attribute key names a secret.
func (r *Redactor) IsSensitiveKey(key string) bool { return sensitiveKey.MatchString(key) }

// RedactString removes registered secrets and secret-looking patterns from s.
func (r *Redactor) RedactString(s string) string {
	r.mu.RLock()
	rep := r.replacer
	r.mu.RUnlock()
	s = rep.Replace(s)
	for _, p := range valuePatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}

// ScrubBytes is RedactString for file contents (support bundle, backups).
func (r *Redactor) ScrubBytes(b []byte) []byte { return []byte(r.RedactString(string(b))) }

// redactAny turns an arbitrary attribute value into a redacted string when it
// could carry a secret; values whose JSON form is unchanged by redaction are kept as is.
func (r *Redactor) redactAny(v any) (any, bool) {
	switch x := v.(type) {
	case error:
		return r.RedactString(x.Error()), true
	case fmt.Stringer:
		return r.RedactString(x.String()), true
	case []byte:
		return r.RedactString(string(x)), true
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return r.RedactString(fmt.Sprintf("%+v", v)), true
	}
	red := r.RedactString(string(raw))
	if red == string(raw) {
		return v, false
	}
	return red, true
}
```

`internal/logging/handler.go`:

```go
package logging

import (
	"context"
	"log/slog"
)

// RedactHandler wraps another slog.Handler and removes secrets from the
// message and from every attribute (including nested groups) before the record
// reaches the wrapped handler.
type RedactHandler struct {
	next slog.Handler
	r    *Redactor
}

// NewRedactHandler wraps next. If r is nil, Default is used.
func NewRedactHandler(next slog.Handler, r *Redactor) *RedactHandler {
	if r == nil {
		r = Default
	}
	return &RedactHandler{next: next, r: r}
}

func (h *RedactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *RedactHandler) Handle(ctx context.Context, rec slog.Record) error {
	out := slog.NewRecord(rec.Time, rec.Level, h.r.RedactString(rec.Message), rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		out.AddAttrs(h.redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, out)
}

func (h *RedactHandler) WithAttrs(as []slog.Attr) slog.Handler {
	red := make([]slog.Attr, len(as))
	for i, a := range as {
		red[i] = h.redactAttr(a)
	}
	return &RedactHandler{next: h.next.WithAttrs(red), r: h.r}
}

func (h *RedactHandler) WithGroup(name string) slog.Handler {
	return &RedactHandler{next: h.next.WithGroup(name), r: h.r}
}

func (h *RedactHandler) redactAttr(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		red := make([]slog.Attr, len(attrs))
		for i, g := range attrs {
			red[i] = h.redactAttr(g)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(red...)}
	}
	if h.r.IsSensitiveKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, h.r.RedactString(a.Value.String()))
	case slog.KindAny:
		if v, changed := h.r.redactAny(a.Value.Any()); changed {
			return slog.Any(a.Key, v)
		}
	}
	return a
}
```

`internal/logging/logging.go`:

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go mod tidy && go test -race -count=1 -tags noorders ./internal/logging/ ./internal/archtest/`
Expected: `ok  tradeloom/internal/logging` and `ok  tradeloom/internal/archtest`.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/logging
git commit -m "feat(logging): slog JSON logs with rotation, redaction handler and runtime level" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
