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
