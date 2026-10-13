package logging

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"tradeloom/internal/clock"
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
	if o.Now == nil {
		o.Now = clock.SystemNow
	}
	seen := map[string]bool{"MANIFEST.txt": true}
	for _, f := range o.Files {
		if err := validBundleName(f.Name); err != nil {
			return "", err
		}
		if seen[f.Name] {
			return "", fmt.Errorf("logging: bundle file name %q is duplicate or reserved", f.Name)
		}
		seen[f.Name] = true
	}
	entries, err := logEntries(o.LogDir)
	if err != nil {
		return "", err
	}

	stamp := o.Now().Format("20060102-150405")
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
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: o.Now()})
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
	// Never overwrite a bundle made in the same second: O_EXCL on the final name, next suffix on
	// ErrExist. Copying (not link/rename) works on FAT/exFAT and SMB targets too.
	final, err := publishBundle(tmp.Name(), o.ExportDir, stamp)
	if err != nil {
		return "", err
	}
	if err := os.Remove(tmp.Name()); err != nil {
		slog.Warn("logging.bundle_tmp_remove_failed", "component", "logging", "path", tmp.Name(), "err", err.Error())
	}
	ok = true
	return final, nil
}

func suffix(i int) string {
	if i == 0 {
		return ""
	}
	return fmt.Sprintf("-%d", i)
}

// validBundleName accepts only clean, relative, forward-slash names outside the reserved logs/ prefix.
func validBundleName(n string) error {
	bad := n == "" || strings.HasPrefix(n, "/") || strings.HasPrefix(n, `\`) ||
		strings.ContainsAny(n, `\:`) || !fs.ValidPath(n) || path.Clean(n) != n || n == "." ||
		n == "MANIFEST.txt" || strings.HasPrefix(n, "logs/")
	if bad {
		return fmt.Errorf("logging: invalid bundle file name %q", n)
	}
	return nil
}

// logEntries lists regular tradeloom*.log and crash-*.json files in dir (no glob: dir may contain metacharacters).
func logEntries(dir string) ([]string, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, de := range des {
		n := de.Name()
		okLog, _ := filepath.Match("tradeloom*.log", n)
		okCrash, _ := filepath.Match("crash-*.json", n)
		if !okLog && !okCrash {
			continue
		}
		p := filepath.Join(dir, n)
		if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() {
			continue
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// copyFile is a seam so tests can force a copy failure.
var copyFile = func(dst io.Writer, src io.Reader) (int64, error) { return io.Copy(dst, src) }

func publishBundle(tmpPath, dir, stamp string) (string, error) {
	var out *os.File
	var final string
	for i := 0; i < 1000; i++ {
		final = filepath.Join(dir, "tradeloom-support-"+stamp+suffix(i)+".zip")
		f, err := os.OpenFile(final, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			out = f
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	if out == nil {
		return "", errors.New("logging: no free bundle name")
	}
	fail := func(err error) (string, error) {
		out.Close()
		os.Remove(final)
		return "", err
	}
	in, err := os.Open(tmpPath)
	if err != nil {
		return fail(err)
	}
	defer in.Close()
	if _, err := copyFile(out, in); err != nil {
		return fail(err)
	}
	if err := out.Sync(); err != nil {
		return fail(err)
	}
	if err := out.Close(); err != nil {
		os.Remove(final)
		return "", err
	}
	return final, nil
}
