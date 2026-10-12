// internal/settings/shortcuts_golden_test.go
package settings

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata golden files")

// shortcutsGolden is the registry as Settings.Shortcuts() serialises it. The frontend mock fixture
// (frontend/src/api/mock/fixtures.ts shortcutFixture) is compared against this file by
// frontend/src/api/mock/shortcuts.test.ts, so the two cannot drift (P0-4 Ruling 83-F I3).
var shortcutsGolden = filepath.Join("testdata", "shortcuts.golden.json")

func TestDefaultShortcuts_Golden(t *testing.T) {
	got, err := json.MarshalIndent(DefaultShortcuts(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(shortcutsGolden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(shortcutsGolden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(shortcutsGolden)
	if err != nil {
		t.Fatalf("read %s (run `go test ./internal/settings -run Golden -update`): %v", shortcutsGolden, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run `go test ./internal/settings -run Golden -update` and update the frontend shortcutFixture", shortcutsGolden)
	}
}
