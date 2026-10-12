package archtest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, tags string) []string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	pkgs, err := Load(dir, env, tags, "./...")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	vs := append(append(CheckOrderWrite("fixture", pkgs), CheckClock("fixture", pkgs)...), CheckKiteIndirect("fixture", pkgs)...)
	for _, v := range append(vs, CheckKiteRawWrite("fixture", pkgs)...) {
		got = append(got, v.Rule+" "+strings.SplitN(v.Pkg, ":", 2)[0])
	}
	sort.Strings(got)
	return got
}

func TestASTChecks_Fixture(t *testing.T) {
	// The reviewer's generic probe (Ruling 39) and its method-value / interface variants, in every configuration.
	indirect := []string{
		"g0-generic bad/generic.go", "g0-generic bad/genericvalue.go",
		"g0-iface bad/generic.go",
		"g0-kitecall bad/generic.go", "g0-kitecall bad/genericvalue.go", "g0-kitecall bad/genericvalue.go",
		"g0-typearg bad/generic.go", "g0-typearg bad/genericvalue.go", "g0-typearg bad/genericvalue.go",
		"g0-typearg bad/ifacecall.go", "g0-typearg bad/ifacecall.go",
	}
	// Alert-triggered and mutual-fund order writes (Ruling 47) and InitiateHoldingsAuth (Ruling 48): one g0-call per write selector.
	for i := 0; i < 9; i++ {
		indirect = append(indirect, "g0-call bad/alertsmf.go")
	}
	// Raw HTTP writes through gokiteconnect's HTTP client or write URI constants (Ruling 69): qualified,
	// aliased, method-value and dot-imported references, and an untagged orderwrite file. writer.go's
	// references (orders-tagged) are allowed; the test file kitehttp_test.go is not loaded.
	for i := 0; i < 8; i++ {
		indirect = append(indirect, "g0-kitehttp rawhttp/kitehttp.go")
	}
	indirect = append(indirect, "g0-kitehttp rawhttp/dotkite.go", "g0-kitehttp rawhttp/dotkite.go", "g0-kitehttp rawhttp/dotkite.go",
		"g0-kitehttp orderwrite/uri.go")
	with := func(vs ...string) []string {
		out := append(append([]string{}, vs...), indirect...)
		sort.Strings(out)
		return out
	}
	cases := []struct {
		tags string
		want []string
	}{
		{"noorders", with("g0-call bad/bad.go", "g0-call bad/embed.go", "g0-call bad/value.go", "g0-call orderwrite/untagged.go",
			"rule8 timeuser/dotuser.go", "rule8 timeuser/timeuser.go", "rule8 timeuser/timeuser.go")},
		// With the orders tag writer.go is loaded and allowed; orders-only files are flagged.
		{"orders", with("g0-call bad/bad.go", "g0-call bad/embed.go", "g0-call bad/value.go", "g0-call orderwrite/ordersonly.go", "g0-call orderwrite/untagged.go",
			"rule8 timeuser/dotuser.go", "rule8 timeuser/timeuser.go", "rule8 timeuser/timeuser.go")},
		// Both tags: writer.go is excluded; a file tagged only "orders" still links and is flagged.
		{"orders,noorders", with("g0-call bad/bad.go", "g0-call bad/embed.go", "g0-call bad/value.go", "g0-call orderwrite/ordersonly.go", "g0-call orderwrite/untagged.go",
			"rule8 timeuser/dotuser.go", "rule8 timeuser/timeuser.go", "rule8 timeuser/timeuser.go")},
	}
	for _, tc := range cases {
		t.Run(tc.tags, func(t *testing.T) {
			got := loadFixture(t, tc.tags)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("\n got %v\nwant %v", got, tc.want)
			}
		})
	}
}

func TestKitePkg_AnyMajorVersion(t *testing.T) {
	for path, want := range map[string]bool{
		"github.com/zerodha/gokiteconnect/v4":             true,
		"github.com/zerodha/gokiteconnect/v5":             true,
		"github.com/zerodha/gokiteconnect/v10":            true,
		"github.com/zerodha/gokiteconnect/v4/models":      false,
		"github.com/zerodha/gokiteconnect/v4/ticker":      false,
		"example.com/github.com/zerodha/gokiteconnect/v4": false,
	} {
		if got := kitePkg.MatchString(path); got != want {
			t.Errorf("kitePkg(%q) = %v, want %v", path, got, want)
		}
	}
}

// g0-http (Ruling 69) on the fixture module: rawhttp imports net/http and is flagged; internal/kite may
// import it; test files (rawhttp/kitehttp_test.go) are not loaded, so they are exempt.
func TestImportRules_FixtureNetHTTP(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	pkgs, err := Load(dir, env, "noorders", "./...")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, v := range CheckImports("fixture", Graph(pkgs)) {
		if v.Rule == "g0-http" {
			got = append(got, v.Pkg)
		}
	}
	if want := "fixture/internal/rawhttp"; len(got) != 1 || got[0] != want {
		t.Fatalf("g0-http violations %v, want [%s]", got, want)
	}
}

// Every Ruling 69 identifier must resolve in the real gokiteconnect; a module bump that renames one
// would otherwise disable that part of the rule silently.
func TestKiteRawWrite_NamesResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("loads and type-checks the whole module")
	}
	pkgs, err := Load(moduleRoot(t), nil, "webkit2_41,noorders", "./internal/kite")
	if err != nil {
		t.Fatal(err)
	}
	if missing := KiteRawWriteMissing(pkgs); len(missing) != 0 {
		t.Fatalf("Ruling 69 identifiers not found in gokiteconnect: %v", missing)
	}
}

// g0-path (Ruling 69 addendum) on the fixture module: literal, folded-concatenation, whitespace and
// host-prefixed write paths outside internal/kite/mock are flagged; read paths, the fake kite/mock and
// the orders-tagged writer.go are not; the rawhttp fixtures are flagged too, because their folded
// gokiteconnect URI constants are write paths.
func TestPathRule_Fixture(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "fixture"))
	if err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=off")
	rawhttp := []string{
		"g0-path orderwrite/uri.go",                                // kiteconnect.URICancelOrder, untagged file
		"g0-path rawhttp/dotkite.go", "g0-path rawhttp/dotkite.go", // URIAlerts+URIPlaceMFSIP, URIGetOrders
		"g0-path rawhttp/kitehttp.go", "g0-path rawhttp/kitehttp.go", "g0-path rawhttp/kitehttp.go", "g0-path rawhttp/kitehttp.go",
	}
	rawpath := []string{"g0-path rawpath/path.go", "g0-path rawpath/path.go", "g0-path rawpath/path.go", "g0-path rawpath/path.go", "g0-path rawpath/path.go"}
	for _, tags := range []string{"noorders", "orders", "orders,noorders"} {
		t.Run(tags, func(t *testing.T) {
			pkgs, err := Load(dir, env, tags, "./...")
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, v := range CheckKitePaths("fixture", pkgs) {
				got = append(got, v.Rule+" "+strings.SplitN(v.Pkg, ":", 2)[0])
			}
			sort.Strings(got)
			want := append(append([]string{}, rawhttp...), rawpath...)
			sort.Strings(want)
			if strings.Join(got, "|") != strings.Join(want, "|") {
				t.Fatalf("\n got %v\nwant %v", got, want)
			}
		})
	}
}
