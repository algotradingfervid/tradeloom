package archtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && strings.HasPrefix(string(b), "module tradeloom\n") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
}

// The real module obeys rules 1–8 and G-0 in both build configurations.
func TestArchitecture(t *testing.T) {
	if testing.Short() {
		t.Skip("loads and type-checks the whole module")
	}
	root := moduleRoot(t)
	for _, tags := range []string{"webkit2_41,noorders", "webkit2_41,orders", "webkit2_41,orders,noorders"} {
		t.Run(tags, func(t *testing.T) {
			pkgs, err := Load(root, nil, tags, "./...")
			if err != nil {
				t.Fatal(err)
			}
			vs := CheckImports("tradeloom", Graph(pkgs))
			vs = append(vs, CheckClock("tradeloom", pkgs)...)
			vs = append(vs, CheckOrderWrite("tradeloom", pkgs)...)
			vs = append(vs, CheckKiteIndirect("tradeloom", pkgs)...)
			vs = append(vs, CheckKiteRawWrite("tradeloom", pkgs)...)
			if missing := KiteRawWriteMissing(pkgs); len(missing) != 0 {
				t.Errorf("Ruling 69 identifiers not found in gokiteconnect: %v", missing)
			}
			for _, v := range vs {
				t.Error(v)
			}
		})
	}
}
