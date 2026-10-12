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

func TestCreateSupportBundle_BracketLogDirAndSymlinks(t *testing.T) {
	base := t.TempDir()
	logDir := filepath.Join(base, "lo[g]s")
	os.MkdirAll(logDir, 0o700)
	os.WriteFile(filepath.Join(logDir, "tradeloom.log"), []byte("x\n"), 0o600)
	os.Symlink(filepath.Join(logDir, "tradeloom.log"), filepath.Join(logDir, "tradeloom-link.log"))
	os.Mkdir(filepath.Join(logDir, "tradeloom-dir.log"), 0o700)
	p, err := CreateSupportBundle(BundleOptions{LogDir: logDir, ExportDir: filepath.Join(base, "e"), Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.OpenReader(p)
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "MANIFEST.txt,logs/tradeloom.log" {
		t.Fatalf("got %v", names)
	}
}

func TestCreateSupportBundle_NameValidation(t *testing.T) {
	for _, n := range []string{"", "/abs", `\x`, `a\b`, "c:x", "../x", "a/../b", "a//b", "./a", "a/", "MANIFEST.txt", "logs/x.log", "."} {
		dir := t.TempDir()
		if _, err := CreateSupportBundle(BundleOptions{LogDir: dir, ExportDir: dir, Now: time.Now, Files: []BundleFile{{Name: n}}}); err == nil {
			t.Errorf("name %q accepted", n)
		}
	}
	dir := t.TempDir()
	if _, err := CreateSupportBundle(BundleOptions{LogDir: dir, ExportDir: dir, Now: time.Now,
		Files: []BundleFile{{Name: "a.json"}, {Name: "a.json"}}}); err == nil {
		t.Error("duplicate accepted")
	}
	if _, err := CreateSupportBundle(BundleOptions{LogDir: dir, ExportDir: dir, Now: time.Now, Files: []BundleFile{{Name: "a..b"}, {Name: "sub/c.json"}}}); err != nil {
		t.Errorf("valid names rejected: %v", err)
	}
}

func TestCreateSupportBundle_NoOverwriteSameSecondAndNilNow(t *testing.T) {
	dir := t.TempDir()
	o := BundleOptions{LogDir: dir, ExportDir: dir, Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }}
	p1, err1 := CreateSupportBundle(o)
	p2, err2 := CreateSupportBundle(o)
	if err1 != nil || err2 != nil || p1 == p2 {
		t.Fatalf("%v %v %s %s", err1, err2, p1, p2)
	}
	if _, err := os.Stat(p1); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateSupportBundle(BundleOptions{LogDir: dir, ExportDir: dir}); err != nil {
		t.Fatalf("nil Now: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".support-*")); len(left) != 0 {
		t.Fatalf("temp left %v", left)
	}
}

func TestCreateSupportBundle_CopyFailureLeavesNothing(t *testing.T) {
	old := copyFile
	copyFile = func(io.Writer, io.Reader) (int64, error) { return 0, os.ErrInvalid }
	defer func() { copyFile = old }()
	dir := t.TempDir()
	if _, err := CreateSupportBundle(BundleOptions{LogDir: dir, ExportDir: dir, Now: time.Now}); err == nil {
		t.Fatal("want error")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*")); len(left) != 0 {
		t.Fatalf("files left: %v", left)
	}
}
