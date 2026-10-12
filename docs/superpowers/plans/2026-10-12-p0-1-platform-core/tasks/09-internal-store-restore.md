> **P0-1 Platform Core Implementation Plan — Task 9.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 9: `internal/store` — Restore

**Files:**
- Create: `internal/store/restore.go`, `internal/store/restore_test.go`

**Interfaces:**
- Consumes: Task 8 `Manifest`, `ManifestFormat`, archive entry names, test helpers `seedDataDir`, `backupOpts`, `archiveEntries`; Task 7 `quickCheck`, `dsn`, `readAppVersion`, `SchemaTooNewError`, `SupportedSchema`; Task 6 `NewLayout`, `Layout.Ensure`.
- Produces (`package store`):
  - `const RestoreConfirmPhrase = "RESTORE"`; `var ErrBadArchive`, `var ErrNotConfirmed`, `var ErrDeploymentsActive`
  - `type RestoreRequest struct { Archive string; DataDir string; Confirmation string; DeploymentsFlat bool; Now func() time.Time }`
  - `type RestoreResult struct { Manifest Manifest; ReplacedDir string }`
  - `func InspectBackup(path string) (Manifest, error)` — the preview P0-4 shows (counts, schema, versions) before the typed confirmation
  - `func Restore(ctx context.Context, req RestoreRequest) (RestoreResult, error)` — caller (P0-3 service) closes `app.db` first, keeps the lock, relaunches afterwards; the previous directory becomes `<DataDir>.replaced-<UTC 20060102T150405Z>`.

- [ ] **Step 1: Write the failing tests**

`internal/store/restore_test.go`:

```go
package store

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

// FR-DATA-005 AC1 (engine side): restore on "machine B" brings back every row, keeps no secret.
func TestRestore_RoundTripToFreshDataDir(t *testing.T) {
	la, dbA := seedDataDir(t)
	archive, err := dbA.Backup(context.Background(), backupOpts(la, t.TempDir(), false))
	if err != nil {
		t.Fatal(err)
	}
	dbA.Close()

	lb := NewLayout(filepath.Join(t.TempDir(), "tradeloom"))
	lb.Ensure()
	dbB, _ := Open(context.Background(), lb.AppDB, Options{AppVersion: "v0.1.0", BackupDir: lb.BackupsDir, Now: fixedNow})
	dbB.Close()

	later := func() time.Time { return fixedNow().Add(time.Hour) }
	res, err := Restore(context.Background(), RestoreRequest{Archive: archive, DataDir: lb.Root,
		Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true, Now: later})
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.Schema != 1 || res.Manifest.Counts["watchlists"] != 1 {
		t.Fatalf("manifest %+v", res.Manifest)
	}
	if m, err := InspectBackup(archive); err != nil || m.Counts["order_journal"] != 1 {
		t.Fatalf("InspectBackup: %+v %v", m, err)
	}
	if res.ReplacedDir != lb.Root+".replaced-20261012T110000Z" {
		t.Fatalf("replaced dir %s", res.ReplacedDir)
	}
	if _, err := os.Stat(filepath.Join(res.ReplacedDir, "app.db")); err != nil {
		t.Fatal("previous data dir not kept")
	}
	if _, err := os.Stat(lb.SecretsFile); !os.IsNotExist(err) {
		t.Fatal("restored dir must not contain secrets.enc")
	}
	db, err := Open(context.Background(), lb.AppDB, Options{AppVersion: "v0.1.0", BackupDir: lb.BackupsDir, Now: later})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var wl, items, journal int
	var theme string
	db.R.QueryRow("SELECT count(*) FROM watchlists").Scan(&wl)
	db.R.QueryRow("SELECT count(*) FROM watchlist_items").Scan(&items)
	db.R.QueryRow("SELECT count(*) FROM order_journal").Scan(&journal)
	db.R.QueryRow("SELECT value FROM settings WHERE key='ui.theme'").Scan(&theme)
	if wl != 1 || items != 1 || journal != 1 || theme != `"dark"` {
		t.Fatalf("restored rows: wl=%d items=%d journal=%d theme=%s", wl, items, journal, theme)
	}
}

func TestRestore_Refusals(t *testing.T) {
	l, db := seedDataDir(t)
	archive, _ := db.Backup(context.Background(), backupOpts(l, t.TempDir(), false))
	db.Close()
	base := RestoreRequest{Archive: archive, DataDir: l.Root, Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true, Now: fixedNow}
	r1 := base
	r1.Confirmation = "restore"
	if _, err := Restore(context.Background(), r1); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("want ErrNotConfirmed, got %v", err)
	}
	r2 := base
	r2.DeploymentsFlat = false
	if _, err := Restore(context.Background(), r2); !errors.Is(err, ErrDeploymentsActive) {
		t.Fatalf("want ErrDeploymentsActive, got %v", err)
	}
}

func writeTarZst(t *testing.T, path string, entries []tar.Header, bodies [][]byte) {
	t.Helper()
	fh, _ := os.Create(path)
	defer fh.Close()
	zw, _ := zstd.NewWriter(fh)
	tw := tar.NewWriter(zw)
	for i, h := range entries {
		h := h
		h.Size = int64(len(bodies[i]))
		if h.Typeflag == 0 {
			h.Typeflag = tar.TypeReg
		}
		h.Mode = 0o600
		tw.WriteHeader(&h)
		tw.Write(bodies[i])
	}
	tw.Close()
	zw.Close()
}

// Review Focus 4: newer schema (by manifest or by the app.db inside), path traversal and missing
// app.db are refused and the current data directory is untouched.
func TestRestore_RejectsHostileOrNewerArchives(t *testing.T) {
	l, db := seedDataDir(t)
	good, _ := db.Backup(context.Background(), backupOpts(l, t.TempDir(), false))
	db.Close()
	ents := archiveEntries(t, good)
	newerDB := filepath.Join(t.TempDir(), "newer.db")
	nd, _ := Open(context.Background(), newerDB, Options{AppVersion: "v9.0.0", BackupDir: t.TempDir(), Now: fixedNow,
		Migrations: withStep3("CREATE TABLE c (y INTEGER);")})
	nd.Close()
	newerBytes, _ := os.ReadFile(newerDB)
	manifestV3 := bytes.Replace(ents["manifest.json"], []byte(`"schema": 1`), []byte(`"schema": 3`), 1)

	cases := []struct {
		name    string
		entries []tar.Header
		bodies  [][]byte
		want    func(error) bool
	}{
		{"manifest schema newer", []tar.Header{{Name: "manifest.json"}, {Name: "app.db"}}, [][]byte{manifestV3, ents["app.db"]},
			func(err error) bool { var e *SchemaTooNewError; return errors.As(err, &e) }},
		{"app.db newer than manifest says", []tar.Header{{Name: "manifest.json"}, {Name: "app.db"}}, [][]byte{ents["manifest.json"], newerBytes},
			func(err error) bool { var e *SchemaTooNewError; return errors.As(err, &e) }},
		{"path traversal", []tar.Header{{Name: "manifest.json"}, {Name: "../../evil"}}, [][]byte{ents["manifest.json"], []byte("x")},
			func(err error) bool { return errors.Is(err, ErrBadArchive) }},
		{"symlink entry", []tar.Header{{Name: "manifest.json"}, {Name: "app.db", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}}, [][]byte{ents["manifest.json"], nil},
			func(err error) bool { return errors.Is(err, ErrBadArchive) }},
		{"missing app.db", []tar.Header{{Name: "manifest.json"}}, [][]byte{ents["manifest.json"]},
			func(err error) bool { return errors.Is(err, ErrBadArchive) }},
	}
	before, _ := os.ReadFile(l.AppDB)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "x.tar.zst")
			writeTarZst(t, p, tc.entries, tc.bodies)
			_, err := Restore(context.Background(), RestoreRequest{Archive: p, DataDir: l.Root,
				Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true, Now: fixedNow})
			if !tc.want(err) {
				t.Fatalf("unexpected error %v", err)
			}
			after, _ := os.ReadFile(l.AppDB)
			if !bytes.Equal(before, after) {
				t.Fatal("current app.db changed")
			}
			if left, _ := filepath.Glob(l.Root + ".restoring-*"); len(left) != 0 {
				t.Fatalf("staging left behind: %v", left)
			}
			if strings.Contains(tc.name, "traversal") {
				if _, err := os.Stat(filepath.Join(filepath.Dir(l.Root), "evil")); err == nil {
					t.Fatal("traversal entry was written")
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -tags noorders ./internal/store/`
Expected: FAIL — build errors `undefined: Restore`, `undefined: RestoreRequest`, `undefined: RestoreConfirmPhrase`, `undefined: InspectBackup`.

- [ ] **Step 3: Write the implementation**

`internal/store/restore.go`:

```go
package store

import (
	"archive/tar"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/klauspost/compress/zstd"
)

// RestoreConfirmPhrase must be typed by the user to confirm a restore (FR-DATA-005).
const RestoreConfirmPhrase = "RESTORE"

var (
	// ErrBadArchive: not a Tradeloom backup, unexpected or unsafe entries, or missing app.db.
	ErrBadArchive = errors.New("store: not a valid Tradeloom backup archive")
	// ErrNotConfirmed: Confirmation differs from RestoreConfirmPhrase.
	ErrNotConfirmed = errors.New("store: restore not confirmed")
	// ErrDeploymentsActive: running deployments must be flat before a restore.
	ErrDeploymentsActive = errors.New("store: restore refused: running deployments must be flat")
)

const maxEntryBytes = 64 << 30 // 64 GiB guard against a decompression bomb in a hostile archive

// RestoreRequest describes one restore. The caller has closed app.db (and the candle store);
// it keeps holding the data-dir lock and relaunches the app after Restore returns.
type RestoreRequest struct {
	Archive         string
	DataDir         string // Layout.Root of the running app
	Confirmation    string // must equal RestoreConfirmPhrase
	DeploymentsFlat bool   // P0: always true (no deployments exist before P4)
	Now             func() time.Time
}

// RestoreResult reports what was restored and where the previous data directory went.
type RestoreResult struct {
	Manifest    Manifest
	ReplacedDir string // <DataDir>.replaced-<ts>
}

func openArchive(path string) (*tar.Reader, func(), error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	zr, err := zstd.NewReader(fh)
	if err != nil {
		fh.Close()
		return nil, nil, fmt.Errorf("%w: %v", ErrBadArchive, err)
	}
	return tar.NewReader(zr), func() { zr.Close(); fh.Close() }, nil
}

// InspectBackup reads and validates manifest.json: format known and schema ≤ SupportedSchema().
func InspectBackup(path string) (Manifest, error) {
	tr, closeFn, err := openArchive(path)
	if err != nil {
		return Manifest{}, err
	}
	defer closeFn()
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return Manifest{}, fmt.Errorf("%w: manifest.json missing", ErrBadArchive)
		}
		if err != nil {
			return Manifest{}, fmt.Errorf("%w: %v", ErrBadArchive, err)
		}
		if h.Name != manifestName {
			continue
		}
		var m Manifest
		if err := json.NewDecoder(io.LimitReader(tr, 1<<20)).Decode(&m); err != nil {
			return Manifest{}, fmt.Errorf("%w: manifest: %v", ErrBadArchive, err)
		}
		if m.Format != ManifestFormat {
			return Manifest{}, fmt.Errorf("%w: unknown format %d", ErrBadArchive, m.Format)
		}
		if m.Schema > SupportedSchema() {
			return Manifest{}, &SchemaTooNewError{Found: m.Schema, Supported: SupportedSchema(), WrittenBy: m.AppVersion}
		}
		return m, nil
	}
}

// Restore validates the archive, extracts it into <DataDir>.restoring-<ts>, verifies the extracted
// app.db (integrity + schema), then renames DataDir to <DataDir>.replaced-<ts> and the new directory
// into place. Secrets are not part of an archive, so the keyring and secrets.enc of the new
// directory are empty and Kite credentials are requested again (FR-DATA-005 AC1).
// On any error before the final rename, DataDir is untouched and the staging directory is removed.
func Restore(ctx context.Context, req RestoreRequest) (RestoreResult, error) {
	if req.Confirmation != RestoreConfirmPhrase {
		return RestoreResult{}, ErrNotConfirmed
	}
	if !req.DeploymentsFlat {
		return RestoreResult{}, ErrDeploymentsActive
	}
	man, err := InspectBackup(req.Archive)
	if err != nil {
		return RestoreResult{}, err
	}
	ts := req.Now().UTC().Format("20060102T150405Z")
	root := filepath.Clean(req.DataDir)
	staging := root + ".restoring-" + ts
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return RestoreResult{}, err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(staging)
		}
	}()
	if err := extract(req.Archive, staging); err != nil {
		return RestoreResult{}, err
	}
	nl := NewLayout(staging)
	if err := verifyRestored(ctx, nl.AppDB, man.Schema); err != nil {
		return RestoreResult{}, err
	}
	if err := nl.Ensure(); err != nil {
		return RestoreResult{}, err
	}
	replaced := root + ".replaced-" + ts
	if err := os.Rename(root, replaced); err != nil {
		return RestoreResult{}, fmt.Errorf("store: move current data directory aside: %w", err)
	}
	if err := os.Rename(staging, root); err != nil {
		// Put the original back so the app still starts.
		if rerr := os.Rename(replaced, root); rerr != nil {
			return RestoreResult{}, errors.Join(err, rerr)
		}
		return RestoreResult{}, err
	}
	ok = true
	return RestoreResult{Manifest: man, ReplacedDir: replaced}, nil
}

// extract writes only the allow-listed entries; anything else (paths with "..", absolute paths,
// links, devices, unknown names) makes the archive invalid.
func extract(archive, dst string) error {
	tr, closeFn, err := openArchive(archive)
	if err != nil {
		return err
	}
	defer closeFn()
	allowed := map[string]string{
		manifestName:   filepath.Join(dst, manifestName),
		archiveAppDB:   filepath.Join(dst, "app.db"),
		archiveCandles: filepath.Join(dst, "candles", "candles.db"),
	}
	seenApp := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrBadArchive, err)
		}
		target, ok := allowed[h.Name]
		if !ok || h.Typeflag != tar.TypeReg {
			return fmt.Errorf("%w: unexpected entry %q", ErrBadArchive, h.Name)
		}
		delete(allowed, h.Name) // duplicates are rejected on the second occurrence
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, io.LimitReader(tr, maxEntryBytes+1))
		cerr := out.Close()
		if err != nil {
			return err
		}
		if cerr != nil {
			return cerr
		}
		if n > maxEntryBytes {
			return fmt.Errorf("%w: entry %q too large", ErrBadArchive, h.Name)
		}
		if h.Name == archiveAppDB {
			seenApp = true
		}
	}
	if !seenApp {
		return fmt.Errorf("%w: app.db missing", ErrBadArchive)
	}
	return nil
}

func verifyRestored(ctx context.Context, path string, manifestSchema int) error {
	c, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		return err
	}
	defer c.Close()
	if err := quickCheck(ctx, c); err != nil {
		return fmt.Errorf("%w: app.db: %v", ErrBadArchive, err)
	}
	var v int
	if err := c.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("%w: app.db: %v", ErrBadArchive, err)
	}
	if v > SupportedSchema() {
		return &SchemaTooNewError{Found: v, Supported: SupportedSchema(), WrittenBy: readAppVersion(ctx, c)}
	}
	if v != manifestSchema {
		return fmt.Errorf("%w: manifest says schema %d, app.db is %d", ErrBadArchive, manifestSchema, v)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race -count=1 -tags noorders ./internal/store/`
Expected: `ok  tradeloom/internal/store`.

- [ ] **Step 5: Commit**

```bash
git add internal/store/restore.go internal/store/restore_test.go
git commit -m "feat(store): validated restore into a fresh data dir keeping the previous one as .replaced" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
