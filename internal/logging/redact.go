package logging

import (
	"encoding/json"
	"fmt"
	"reflect"
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
	{regexp.MustCompile(`(?i)\b(authorization["']?\s*[:=]\s*[\["']*)[^"'\r\n\]]+`), "${1}" + Redacted},
	{regexp.MustCompile(`(?i)("?\b(?:request_token|access_token|api_secret|checksum|bot_token|passphrase)"?\s*[:=]\s*"?)[^"&\s,}\]]+`), "${1}" + Redacted},
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
		// json.Marshal escapes e.g. < > & and quotes; redact that form too.
		if b, err := json.Marshal(s); err == nil {
			if esc := string(b[1 : len(b)-1]); esc != s {
				pairs = append(pairs, esc, Redacted)
			}
		}
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
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Ptr && rv.IsNil() {
		return v, false // nil pointer: calling Error/String could panic
	}
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

var secretLikeKey = regexp.MustCompile(`(?i)(secret|token|password|passwd|passphrase|api_?secret|totp|authorization|checksum)`)

// IsSecretLikeKey reports whether a key (setting name, JSON field) looks like it holds a secret:
// the union of the redactor's sensitive keys and common secret words. A bare "api_key" is not
// matched (the Kite API key is not secret).
func IsSecretLikeKey(k string) bool {
	return sensitiveKey.MatchString(k) || secretLikeKey.MatchString(k)
}
