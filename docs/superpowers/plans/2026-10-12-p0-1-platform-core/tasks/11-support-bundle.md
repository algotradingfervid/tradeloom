> **P0-1 Platform Core Implementation Plan — Task 11.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 11: Support bundle (`logging.CreateSupportBundle` + `store.SupportFiles`)

**Files:**
- Create: `internal/logging/bundle.go`, `internal/logging/bundle_test.go`, `internal/store/support.go`, `internal/store/support_test.go`

**Interfaces:**
- Consumes: Task 4 `Redactor`, `ScrubBytes`; Task 5 crash file naming (`crash-*.json`); Task 7 `DB`, `readAppVersion`; Task 8 test helper `mustExec`; Task 10 `DB.SettingsRepo()`.
- Produces:
  - `package logging`: `type BundleFile struct { Name string; Data []byte }`; `type BundleOptions struct { LogDir string; ExportDir string; Files []BundleFile; Now func() time.Time; Redactor *Redactor }`; `func CreateSupportBundle(o BundleOptions) (string, error)` → `<ExportDir>/tradeloom-support-YYYYMMDD-HHMMSS.zip` containing `logs/tradeloom*.log`, `logs/crash-*.json`, each `Files` entry and `MANIFEST.txt`; every entry is scrubbed with the redactor; temp file + rename.
  - `package store`: `const JournalTailRows = 200`; `func (db *DB) SupportFiles(ctx context.Context) ([]logging.BundleFile, error)` → `settings.json`, `versions.json` (`schema`, `app_version_last`), `order_journal.json` (newest 200 rows), `deployment_logs.json` (`[]` until P4 adds `deployment_events`).
  - P0-3's service composes `CreateSupportBundle(BundleOptions{LogDir: Layout.LogsDir, ExportDir: Layout.ExportsDir, Files: SupportFiles(...) + calendar/library version files, Now: clock})`; P0-4 adds the button and "Open logs folder".

- [ ] **Step 1: Write the failing tests**

`internal/logging/bundle_test.go`:

```go
package logging

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// FR-SHELL-018 AC4: the bundle opens with the listed contents and contains no keyring value,
// even when a producer accidentally passes one in.
func TestCreateSupportBundle_ContentsAndNoSecrets(t *testing.T) {
	data := t.TempDir()
	logDir, exp := filepath.Join(data, "logs"), filepath.Join(data, "exports")
	r := NewRedactor()
	r.Register(tAccess)
	r.Register(tSecret)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(logDir, "tradeloom.log"), []byte(`{"event":"x","leak":"`+tSecret+`"}`+"\n"), 0o600)
	os.WriteFile(filepath.Join(logDir, "crash-20261012T043000.000Z.json"), []byte(`{"panic":"p"}`), 0o600)
	os.WriteFile(filepath.Join(data, "secrets.enc"), []byte("TLSEC1..."), 0o600)

	path, err := CreateSupportBundle(BundleOptions{
		LogDir: logDir, ExportDir: exp, Redactor: r,
		Now: func() time.Time { return time.Date(2026, 10, 12, 16, 5, 9, 0, time.UTC) },
		Files: []BundleFile{
			{Name: "settings.json", Data: []byte(`{"kite.api_key":"k","oops":"` + tAccess + `"}`)},
			{Name: "versions.json", Data: []byte(`{"schema":1,"app":"v0.1.0","calendar":"2026.1","library":"0"}`)},
			{Name: "order_journal.json", Data: []byte(`[]`)},
			{Name: "deployment_logs.json", Data: []byte(`[]`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "tradeloom-support-20261012-160509.zip" {
		t.Fatalf("name %s", path)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		for _, s := range []string{tAccess, tSecret} {
			if strings.Contains(string(b), s) {
				t.Fatalf("%s contains secret %q", f.Name, s)
			}
		}
	}
	sort.Strings(names)
	want := []string{"MANIFEST.txt", "deployment_logs.json", "logs/crash-20261012T043000.000Z.json", "logs/tradeloom.log", "order_journal.json", "settings.json", "versions.json"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("contents\n got %v\nwant %v", names, want)
	}
	if left, _ := filepath.Glob(filepath.Join(exp, ".support-*")); len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}

func TestCreateSupportBundle_RejectsEscapingNames(t *testing.T) {
	dir := t.TempDir()
	_, err := CreateSupportBundle(BundleOptions{LogDir: dir, ExportDir: dir, Now: time.Now,
		Files: []BundleFile{{Name: "../evil", Data: nil}}})
	if err == nil {
		t.Fatal("want error for ../ name")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.zip")); len(left) != 0 {
		t.Fatalf("partial bundle left: %v", left)
	}
}
```

`internal/store/support_test.go`:

```go
package store

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSupportFiles(t *testing.T) {
	db := mustOpen(t, t.TempDir(), nil, "v0.1.0")
	defer db.Close()
	for i := 0; i < 250; i++ {
		mustExec(t, db, `INSERT INTO order_journal(at,tag,kind,source) VALUES(?,?,?,?)`, i, "t", "intent", "manual")
	}
	files, err := db.SupportFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string][]byte{}
	for _, f := range files {
		byName[f.Name] = f.Data
	}
	var journal []journalRow
	if err := json.Unmarshal(byName["order_journal.json"], &journal); err != nil {
		t.Fatal(err)
	}
	if len(journal) != JournalTailRows || journal[0].At != 249 {
		t.Fatalf("journal tail: %d rows, first at=%d", len(journal), journal[0].At)
	}
	var v map[string]any
	json.Unmarshal(byName["versions.json"], &v)
	if v["schema"].(float64) != 1 || v["app_version_last"] != "v0.1.0" {
		t.Fatalf("versions %v", v)
	}
	if string(byName["deployment_logs.json"]) != "[]" || byName["settings.json"] == nil {
		t.Fatalf("files %v", byName)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -tags noorders ./internal/logging/ ./internal/store/`
Expected: FAIL — build errors `undefined: CreateSupportBundle` (logging) and `undefined: journalRow` / `db.SupportFiles undefined` (store).

- [ ] **Step 3: Write the implementation**

`internal/logging/bundle.go`:

```go
package logging

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BundleFile is one generated document added to a support bundle
// (settings.json, versions.json, order_journal.json, deployment_logs.json …).
type BundleFile struct {
	Name string // path inside the archive, e.g. "settings.json"
	Data []byte
}

// BundleOptions configures CreateSupportBundle.
type BundleOptions struct {
	LogDir    string           // <datadir>/logs: tradeloom*.log and crash-*.json are included
	ExportDir string           // <datadir>/exports: the archive is written here
	Files     []BundleFile     // generated documents (secrets must already be excluded by the producer)
	Now       func() time.Time // clock.SystemNow in production
	Redactor  *Redactor        // nil means Default; every entry is scrubbed again (defence in depth)
}

// CreateSupportBundle writes exports/tradeloom-support-YYYYMMDD-HHMMSS.zip and returns its path.
// The archive is written to a temp file and renamed, so a failed run leaves no partial bundle.
func CreateSupportBundle(o BundleOptions) (string, error) {
	if o.Redactor == nil {
		o.Redactor = Default
	}
	if err := os.MkdirAll(o.ExportDir, 0o700); err != nil {
		return "", err
	}
	logs, err := filepath.Glob(filepath.Join(o.LogDir, "tradeloom*.log"))
	if err != nil {
		return "", err
	}
	crashes, err := filepath.Glob(filepath.Join(o.LogDir, "crash-*.json"))
	if err != nil {
		return "", err
	}
	entries := append(logs, crashes...)
	sort.Strings(entries)

	final := filepath.Join(o.ExportDir, "tradeloom-support-"+o.Now().Format("20060102-150405")+".zip")
	tmp, err := os.CreateTemp(o.ExportDir, ".support-*.zip.tmp")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	zw := zip.NewWriter(tmp)
	var manifest []string
	add := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(o.Redactor.ScrubBytes(data))
		manifest = append(manifest, name)
		return err
	}
	for _, p := range entries {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("logging: bundle read %s: %w", p, err)
		}
		if err := add("logs/"+filepath.Base(p), b); err != nil {
			return "", err
		}
	}
	for _, f := range o.Files {
		if f.Name == "" || strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
			return "", errors.New("logging: bundle file name must be relative without ..")
		}
		if err := add(f.Name, f.Data); err != nil {
			return "", err
		}
	}
	if err := add("MANIFEST.txt", []byte(strings.Join(manifest, "\n")+"\n")); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		return "", err
	}
	ok = true
	return final, nil
}
```

`internal/store/support.go`:

```go
package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"tradeloom/internal/logging"
)

// JournalTailRows is how many order-journal rows a support bundle carries (FR-SHELL-018).
const JournalTailRows = 200

type journalRow struct {
	ID          int64   `json:"id"`
	At          int64   `json:"at"`
	Tag         string  `json:"tag"`
	Kind        string  `json:"kind"`
	Source      string  `json:"source"`
	Params      *string `json:"params,omitempty"`
	KiteOrderID *string `json:"kite_order_id,omitempty"`
	Status      *string `json:"status,omitempty"`
	Message     *string `json:"message,omitempty"`
	Stage       *string `json:"stage,omitempty"`
}

// SupportFiles returns the app.db part of a support bundle: settings.json (the settings table never
// holds secrets), versions.json, order_journal.json (newest 200 rows) and deployment_logs.json
// (empty until the P4 deployment_events table exists).
func (db *DB) SupportFiles(ctx context.Context) ([]logging.BundleFile, error) {
	set, err := db.SettingsRepo().Load(ctx)
	if err != nil {
		return nil, err
	}
	setJSON := map[string]json.RawMessage{}
	for k, v := range set {
		if json.Valid([]byte(v)) {
			setJSON[k] = json.RawMessage(v)
		}
	}
	rows, err := db.R.QueryContext(ctx,
		`SELECT id, at, tag, kind, source, params, kite_order_id, status, message, stage
		 FROM order_journal ORDER BY at DESC, id DESC LIMIT ?`, JournalTailRows)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	journal := []journalRow{}
	for rows.Next() {
		var r journalRow
		var p, k, s, m, st sql.NullString
		if err := rows.Scan(&r.ID, &r.At, &r.Tag, &r.Kind, &r.Source, &p, &k, &s, &m, &st); err != nil {
			return nil, err
		}
		r.Params, r.KiteOrderID, r.Status, r.Message, r.Stage = ns(p), ns(k), ns(s), ns(m), ns(st)
		journal = append(journal, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	versions := map[string]any{"schema": db.schema, "app_version_last": readAppVersion(ctx, db.R)}
	files := []logging.BundleFile{}
	for _, f := range []struct {
		name string
		v    any
	}{{"settings.json", setJSON}, {"versions.json", versions}, {"order_journal.json", journal}, {"deployment_logs.json", []any{}}} {
		b, err := json.MarshalIndent(f.v, "", "  ")
		if err != nil {
			return nil, err
		}
		files = append(files, logging.BundleFile{Name: f.name, Data: b})
	}
	return files, nil
}

func ns(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race -count=1 -tags noorders ./internal/logging/ ./internal/store/ ./internal/archtest/`
Expected: `ok` for all three.

- [ ] **Step 5: Commit**

```bash
git add internal/logging/bundle.go internal/logging/bundle_test.go internal/store/support.go internal/store/support_test.go
git commit -m "feat(logging,store): support bundle with logs, crash reports, settings, versions and journal tail" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
