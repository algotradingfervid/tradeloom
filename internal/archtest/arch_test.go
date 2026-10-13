package archtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
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

// The real module obeys rules 1–8 and G-0 in both build configurations. The nested module tradeloom/ta
// (replaced into the main module; it will be linked through internal/candles/builder but is not yet
// imported, and ./... does not match it) is loaded too, from its own directory so neither go.work nor a
// require line is needed, and every type-based and import G-0 rule covers it (Ruling 99). The source walks
// (CheckTASources, CheckCgoSources) do not depend on tags and run once, outside the tag loop.
func TestArchitecture(t *testing.T) {
	if testing.Short() {
		t.Skip("loads and type-checks the whole module")
	}
	root := moduleRoot(t)
	for _, v := range CheckTASources(filepath.Join(root, "ta"), "tradeloom/ta") {
		t.Error(v)
	}
	for _, v := range CheckCgoSources(root, "tradeloom", []string{"ta"}) {
		t.Error(v)
	}
	for _, tags := range []string{"webkit2_41,noorders", "webkit2_41,orders", "webkit2_41,orders,noorders"} {
		t.Run(tags, func(t *testing.T) {
			pkgs, err := Load(root, nil, tags, "./...")
			if err != nil {
				t.Fatal(err)
			}
			taPkgs, err := Load(filepath.Join(root, "ta"), nil, tags, "./...")
			if err != nil {
				t.Fatal(err)
			}
			if !loaded(taPkgs, "tradeloom/ta") {
				t.Fatal("tradeloom/ta not loaded")
			}
			// Each load is its own type universe (objects are matched by identity), so the checks run per load.
			var vs []Violation
			for _, set := range [][]*packages.Package{pkgs, taPkgs} {
				vs = append(vs, CheckImports("tradeloom", Graph(set))...)
				vs = append(vs, CheckClock("tradeloom", set)...)
				vs = append(vs, CheckOrderWrite("tradeloom", set)...)
				vs = append(vs, CheckKiteIndirect("tradeloom", set)...)
				vs = append(vs, CheckKiteRawWrite("tradeloom", set)...)
				vs = append(vs, CheckKitePaths("tradeloom", set)...)
				vs = append(vs, CheckExec("tradeloom", set)...)
				vs = append(vs, CheckSysObjects("tradeloom", set)...)
				vs = append(vs, CheckKiteConstruct("tradeloom", set)...)
				vs = append(vs, CheckHTTPUse("tradeloom", set)...)
				vs = append(vs, CheckOtherFiles("tradeloom", set)...)
				vs = append(vs, CheckCgo("tradeloom", set)...)
			}
			if missing := SysAllowMissing(pkgs, sysImporters); len(missing) != 0 {
				t.Errorf("Ruling 99 g0-sys allow-list entries not found: %v", missing)
			}
			if missing := HTTPUseMissing(pkgs, httpUseAllowed); len(missing) != 0 {
				t.Errorf("I3 g0-httpuse names or allow-list entries not found: %v", missing)
			}
			if missing := KiteConstructMissing(pkgs); len(missing) != 0 {
				t.Errorf("Ruling 91 g0-kitenew names not found in gokiteconnect: %v", missing)
			}
			if missing := ExecNamesMissing(pkgs); len(missing) != 0 {
				t.Errorf("Ruling 83 g0-exec names not found: %v", missing)
			}
			if missing := KiteRawWriteMissing(pkgs); len(missing) != 0 {
				t.Errorf("Ruling 69 identifiers not found in gokiteconnect: %v", missing)
			}
			for _, v := range vs {
				t.Error(v)
			}
		})
	}
}

func loaded(pkgs []*packages.Package, path string) bool {
	for _, p := range pkgs {
		if p.PkgPath == path {
			return true
		}
	}
	return false
}
