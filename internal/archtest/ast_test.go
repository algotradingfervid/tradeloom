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
	for _, v := range append(append(CheckOrderWrite("fixture", pkgs), CheckClock("fixture", pkgs)...), CheckKiteIndirect("fixture", pkgs)...) {
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
