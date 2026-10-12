package archtest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// loadFixtureCgo loads the fixture with an explicit CGO_ENABLED, so the untagged cgo file lands in
// CgoFiles (1) or IgnoredFiles (0).
func loadFixtureCgo(t *testing.T, cgo, tags string) []*packages.Package {
	t.Helper()
	dir, env := fixtureEnv(t)
	pkgs, err := Load(dir, append(env, "CGO_ENABLED="+cgo), tags, "./...")
	if err != nil {
		t.Fatal(err)
	}
	return pkgs
}

func ruleList(vs []Violation) []string {
	got := []string{}
	for _, v := range vs {
		got = append(got, v.Rule+" "+v.Pkg)
	}
	sort.Strings(got)
	return got
}

// Ruling 99 (1): import "C" is rejected in every .go file of a module package, whether this build compiles
// it (go list's GoFiles and CgoFiles, both in packages.Package.GoFiles) or not (IgnoredFiles: another tag,
// GOOS, or CGO_ENABLED=0). With CGO_ENABLED=1 the untagged cgo.go is a CgoFiles entry, with 0 an ignored one.
func TestCgo_Fixture(t *testing.T) {
	want := []string{"g0-cgo cgouser/cgo.go:5", "g0-cgo cgouser/hidden.go:7", "g0-cgo cgouser/leaky_c.go:6", "g0-cgo cgouser/tagged_c.go:6"}
	for _, cgo := range []string{"0", "1"} {
		for _, tags := range []string{"noorders", "orders", "orders,noorders"} {
			t.Run("CGO_ENABLED="+cgo+"/"+tags, func(t *testing.T) {
				assertList(t, ruleList(CheckCgo("fixture", loadFixtureCgo(t, cgo, tags))), want)
			})
		}
	}
}

// Each file list of packages.Package is scanned on its own (go/packages merges go list's CgoFiles into
// GoFiles and IgnoredGoFiles/IgnoredOtherFiles into IgnoredFiles).
func TestCgo_SourceFields(t *testing.T) {
	dir, _ := fixtureEnv(t)
	f := func(name string) []string { return []string{filepath.Join(dir, "internal", "cgouser", name)} }
	clean := f("clean.go")
	for _, tc := range []struct {
		name string
		pkg  *packages.Package
	}{
		{"GoFiles", &packages.Package{GoFiles: f("cgo.go")}},
		{"IgnoredFiles", &packages.Package{GoFiles: clean, IgnoredFiles: f("cgo.go")}},
	} {
		tc.pkg.PkgPath = "fixture/internal/cgouser"
		t.Run(tc.name, func(t *testing.T) {
			assertList(t, ruleList(CheckCgo("fixture", []*packages.Package{tc.pkg})), []string{"g0-cgo cgouser/cgo.go:5"})
		})
	}
	// Under testdata: exempt.
	p := &packages.Package{PkgPath: "fixture/internal/testdata/x", GoFiles: f("cgo.go")}
	assertList(t, ruleList(CheckCgo("fixture", []*packages.Package{p})), []string{})
}

// An allowance names one file and one tag; the file is allowed only if its //go:build line keeps it out of
// every build without that tag (leaky_c.go: "libmc || linux" is not).
func TestCgo_Allowance(t *testing.T) {
	allow := []cgoAllowance{
		{"internal/cgouser/tagged_c.go", "libmc", "test"},
		{"internal/cgouser/leaky_c.go", "libmc", "test"},
	}
	want := []string{"g0-cgo cgouser/cgo.go:5", "g0-cgo cgouser/hidden.go:7", "g0-cgo cgouser/leaky_c.go:6"}
	assertList(t, ruleList(checkCgo("fixture", loadFixtureCgo(t, "0", "noorders"), allow)), want)
}

// Ruling 99 (2): every non-test .go file of tradeloom/ta, under any build tags, imports only the standard
// library and tradeloom/ta/..., and none of net, os/exec, syscall, golang.org/x/sys, unsafe, plugin, C;
// C and unsafe only in the allow-listed test-oracle files that link solely under their tag.
func TestTASources_Fixture(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "tafixture"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"g0-ta tafixture/bad.go:7", "g0-ta tafixture/bad.go:8", "g0-ta tafixture/bad.go:9", "g0-ta tafixture/bad.go:10",
		"g0-ta tafixture/bad.go:11", "g0-ta tafixture/bad.go:13", "g0-ta tafixture/bad.go:14",
		"g0-ta tafixture/net.go:3", "g0-ta tafixture/hidden_c.go:6",
		"g0-ta oracle/oracle.go:7", "g0-ta oracle/oracle.go:8",
	}
	assertList(t, ruleList(CheckTASources(dir, "tradeloom/ta")), want)
}

// The real ta: clean, and the two allow-listed cgo files exist, import C and link only under their tag.
func TestTASources_RealModule(t *testing.T) {
	ta := filepath.Join(moduleRoot(t), "ta")
	if vs := CheckTASources(ta, "tradeloom/ta"); len(vs) != 0 {
		t.Fatalf("ta violations: %v", vs)
	}
	for _, a := range cgoAllowed {
		path := filepath.Join(moduleRoot(t), filepath.FromSlash(a.file))
		imps, expr, err := parseImports(path)
		if err != nil {
			t.Fatal(err)
		}
		if !containsStr(importPaths(imps), "C") || expr == nil || !onlyWithTag(expr, a.tag) {
			t.Errorf("%s: want a cgo file that links only with the %s tag", a.file, a.tag)
		}
	}
}

// The default build of ta (no libmc / oracle tag, as any release would link it) compiles no cgo file, even
// with CGO_ENABLED=1, and does not contain the oracle package; libm_c.go is excluded by its tag.
func TestTA_DefaultBuildNoCgo(t *testing.T) {
	root := moduleRoot(t)
	for _, tags := range []string{"webkit2_41,noorders", "noorders", "webkit2_41,orders"} {
		pkgs, err := Load(filepath.Join(root, "ta"), append(os.Environ(), "CGO_ENABLED=1"), tags, "./...")
		if err != nil {
			t.Fatal(err)
		}
		if len(pkgs) == 0 {
			t.Fatalf("%s: no ta packages loaded", tags)
		}
		for _, p := range pkgs {
			if strings.Contains(p.PkgPath, "oracle") {
				t.Errorf("%s: %s in the default build", tags, p.PkgPath)
			}
			for _, f := range p.GoFiles {
				imps, _, err := parseImports(f)
				if err != nil || containsStr(importPaths(imps), "C") {
					t.Errorf("%s: %s is compiled into the default build and uses cgo (err %v)", tags, f, err)
				}
			}
			if p.PkgPath == "tradeloom/ta" && !containsBase(p.IgnoredFiles, "libm_c.go") {
				t.Errorf("%s: libm_c.go not among the ignored files of tradeloom/ta", tags)
			}
		}
	}
}

func containsBase(files []string, base string) bool {
	for _, f := range files {
		if filepath.Base(f) == base {
			return true
		}
	}
	return false
}

// Ruling 99 (3): g0-sys allows the listed package-level objects by identity. unixnames.go uses the field
// Attr.Exec and the method Proc.Exec, which share the "golang.org/x/sys/unix.Exec" name with the allowed
// function: both are flagged, the function is not.
func TestSysObjects_Identity(t *testing.T) {
	allow := map[string]sysAllowance{"internal/app": {objects: []string{
		"golang.org/x/sys/unix.Exec", "golang.org/x/sys/unix.Attr", "golang.org/x/sys/unix.Proc", "syscall.Exec", "syscall.Getpid"}}}
	var got []string
	for _, s := range ruleList(checkSysObjects("fixture", loadFixtureCgo(t, "0", "noorders"), allow)) {
		if strings.Contains(s, "app/") {
			got = append(got, s)
		}
	}
	assertList(t, got, []string{"g0-sys app/unixnames.go:8", "g0-sys app/unixnames.go:9"})
}

// An allow-list entry that does not resolve to a package-level object is reported, so a typo or a
// toolchain change cannot leave an allowance that silently matches nothing (or a field of that name).
func TestSysAllowMissing(t *testing.T) {
	pkgs := loadFixtureCgo(t, "0", "noorders")
	if m := SysAllowMissing(pkgs, sysImporters); len(m) != 0 {
		t.Fatalf("real allow-list on the fixture: missing %v", m)
	}
	allow := map[string]sysAllowance{"internal/app": {objects: []string{"syscall.NoSuchThing", "golang.org/x/sys/unix.Exec", "nodot"}}}
	assertList(t, SysAllowMissing(pkgs, allow), []string{"internal/app: nodot", "internal/app: syscall.NoSuchThing"})
}
