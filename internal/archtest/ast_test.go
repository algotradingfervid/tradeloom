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
	for _, v := range append(CheckOrderWrite("fixture", pkgs), CheckClock("fixture", pkgs)...) {
		got = append(got, v.Rule+" "+strings.SplitN(v.Pkg, ":", 2)[0])
	}
	sort.Strings(got)
	return got
}

func TestASTChecks_Fixture(t *testing.T) {
	cases := []struct {
		tags string
		want []string
	}{
		{"noorders", []string{"g0-call bad/bad.go", "g0-call bad/embed.go", "g0-call bad/value.go", "g0-call orderwrite/untagged.go",
			"rule8 timeuser/dotuser.go", "rule8 timeuser/timeuser.go", "rule8 timeuser/timeuser.go"}},
		// With the orders tag writer.go is loaded and allowed; orders-only files are flagged.
		{"orders", []string{"g0-call bad/bad.go", "g0-call bad/embed.go", "g0-call bad/value.go", "g0-call orderwrite/ordersonly.go", "g0-call orderwrite/untagged.go",
			"rule8 timeuser/dotuser.go", "rule8 timeuser/timeuser.go", "rule8 timeuser/timeuser.go"}},
		// Both tags: writer.go is excluded; a file tagged only "orders" still links and is flagged.
		{"orders,noorders", []string{"g0-call bad/bad.go", "g0-call bad/embed.go", "g0-call bad/value.go", "g0-call orderwrite/ordersonly.go", "g0-call orderwrite/untagged.go",
			"rule8 timeuser/dotuser.go", "rule8 timeuser/timeuser.go", "rule8 timeuser/timeuser.go"}},
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
