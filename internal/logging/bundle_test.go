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
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("bundle mode %v err %v", fi, err)
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
	if left, _ := filepath.Glob(filepath.Join(dir, ".support-*")); len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}
