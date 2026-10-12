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
	"sort"
	"time"

	"github.com/klauspost/compress/zstd"

	"tradeloom/internal/clock"
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

// Decompression-bomb guards: per entry and for the whole archive (variables so tests can lower them).
var (
	maxEntryBytes int64 = 64 << 30
	maxTotalBytes int64 = 128 << 30
)

// RestoreRequest describes one restore. The caller has closed app.db (and the candle store);
// it keeps holding the data-dir lock. After a successful swap that held lock (an open fd) belongs to
// the file now inside ReplacedDir, so the caller must only MarkClean/Release and relaunch immediately.
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
	zr, err := zstd.NewReader(fh, zstd.WithDecoderMaxWindow(64<<20), zstd.WithDecoderConcurrency(1))
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
	if req.Now == nil {
		req.Now = clock.SystemNow
	}
	ts := req.Now().UTC().Format("20060102T150405Z")
	root := filepath.Clean(req.DataDir)
	staging := root + ".restoring-" + ts
	if _, err := os.Lstat(root + ".replaced-" + ts); err == nil {
		return RestoreResult{}, fmt.Errorf("store: %s already exists", root+".replaced-"+ts)
	}
	// Mkdir (not MkdirAll): never adopt or later delete a directory this call did not create.
	if err := os.Mkdir(staging, 0o700); err != nil {
		return RestoreResult{}, err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(staging)
		}
	}()
	if err := extract(ctx, req.Archive, staging); err != nil {
		return RestoreResult{}, err
	}
	nl := NewLayout(staging)
	if err := verifyRestored(ctx, nl.AppDB, man.Schema); err != nil {
		return RestoreResult{}, err
	}
	if _, err := os.Stat(nl.CandlesDB); err == nil {
		if detail := rawHeaderCheck(nl.CandlesDB); detail != "" {
			return RestoreResult{}, fmt.Errorf("%w: candles.db: %s", ErrBadArchive, detail)
		}
	}
	if err := nl.Ensure(); err != nil {
		return RestoreResult{}, err
	}
	// Make the staged tree durable before it can become the data directory.
	for _, d := range []string{nl.Root, nl.CandlesDir, nl.LogsDir, nl.ExportsDir, nl.BackupsDir, nl.LibraryDir, nl.InstrumentsDir} {
		if err := syncDir(d); err != nil {
			return RestoreResult{}, err
		}
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
	// The swap itself must survive power loss: sync the parent holding both renames.
	if err := syncDir(filepath.Dir(root)); err != nil {
		return RestoreResult{Manifest: man, ReplacedDir: replaced}, fmt.Errorf("store: restored but could not sync parent directory: %w", err)
	}
	return RestoreResult{Manifest: man, ReplacedDir: replaced}, nil
}

// extract writes only the allow-listed entries; anything else (paths with "..", absolute paths,
// links, devices, unknown names) makes the archive invalid.
func extract(ctx context.Context, archive, dst string) error {
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
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrBadArchive, err)
		}
		target, ok := allowed[h.Name]
		if !ok || h.Typeflag != tar.TypeReg {
			return fmt.Errorf("%w: unexpected entry %q (type %q)", ErrBadArchive, h.Name, h.Typeflag)
		}
		if h.Size < 0 || h.Size > maxEntryBytes || h.Size > maxTotalBytes-total {
			return fmt.Errorf("%w: entry %q too large", ErrBadArchive, h.Name)
		}
		delete(allowed, h.Name) // duplicates are rejected on the second occurrence
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, io.LimitReader(tr, h.Size+1))
		if err == nil {
			err = out.Sync()
		}
		cerr := out.Close()
		if err != nil {
			return err
		}
		if cerr != nil {
			return cerr
		}
		total += n
		if n > h.Size || n > maxEntryBytes {
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
	// Same raw check as store.Open (Review Focus 1): never hand a garbage file to SQLite.
	if detail := rawHeaderCheck(path); detail != "" {
		return fmt.Errorf("%w: app.db: %s", ErrBadArchive, detail)
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() == 0 {
		return fmt.Errorf("%w: app.db is empty", ErrBadArchive)
	}
	c, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		return err
	}
	defer func() { // opening in WAL mode leaves sidecars in staging; they must not reach the data dir
		c.Close()
		os.Remove(path + "-wal")
		os.Remove(path + "-shm")
	}()
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

func syncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	serr := d.Sync()
	cerr := d.Close()
	if serr != nil {
		return fmt.Errorf("store: sync %s: %w", path, serr)
	}
	return cerr
}

// RecoverInterruptedRestore repairs a restore that crashed between its two renames. If root does not
// exist and <root>.replaced-* directories do, the newest (by UTC timestamp in the name) is renamed
// back to root and its path returned. Stale <root>.restoring-* staging directories are always removed.
// When root exists nothing but the staging sweep happens. The app's start-up (app.Start) must call this
// before Layout.Ensure, which would otherwise create an empty root and hide the interrupted restore.
func RecoverInterruptedRestore(root string) (recovered string, err error) {
	root = filepath.Clean(root)
	stale, _ := filepath.Glob(root + ".restoring-*")
	for _, p := range stale {
		if err := os.RemoveAll(p); err != nil {
			return "", err
		}
	}
	if _, err := os.Lstat(root); err == nil {
		return "", nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	olds, _ := filepath.Glob(root + ".replaced-*")
	if len(olds) == 0 {
		return "", nil
	}
	sort.Strings(olds) // fixed-width UTC timestamp: lexical order is chronological
	newest := olds[len(olds)-1]
	if err := os.Rename(newest, root); err != nil {
		return "", fmt.Errorf("store: recover interrupted restore: %w", err)
	}
	if err := syncDir(filepath.Dir(root)); err != nil {
		return newest, err
	}
	return newest, nil
}
