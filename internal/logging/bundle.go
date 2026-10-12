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
