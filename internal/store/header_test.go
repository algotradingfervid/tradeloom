package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// CheckHeader and RestrictModes are shared with internal/candles/sqlite (candles.db).
func TestCheckHeaderAndRestrictModes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.db")
	if err := CheckHeader(path); err != nil {
		t.Fatalf("absent file: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckHeader(path); err != nil {
		t.Fatalf("empty file: %v", err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 4096), 0o644); err != nil {
		t.Fatal(err)
	}
	err := CheckHeader(path)
	var ce *CorruptError
	if !errors.Is(err, ErrCorrupt) || !errors.As(err, &ce) || ce.Path != path || ce.Detail == "" {
		t.Fatalf("garbage file: %v", err)
	}
	if err := os.WriteFile(path+"-wal", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RestrictModes(path); err != nil { // -shm absent: ignored
		t.Fatal(err)
	}
	for _, p := range []string{path, path + "-wal"} {
		if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode %o", p, fi.Mode().Perm())
		}
	}
}
