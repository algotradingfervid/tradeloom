package ta

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The C library is built with -ffp-contract=off, but Go on arm64 fuses a*b+c
// into one fused multiply-add (one rounding instead of two), even across
// statements, unless the product is wrapped in float64(...). Such a fusion
// changes results by an ulp, which feedback loops (MAMA, HT_*) amplify.
// This test disassembles the package and fails on any fused multiply-add
// whose source line does not call math.FMA explicitly (C's fma()).
func TestNoImplicitFMA(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("the audit reads arm64 instructions")
	}
	if testing.Short() {
		t.Skip("disassembles the test binary")
	}
	// go test strips the binary it runs, so build an unstripped pure-Go copy.
	bin := filepath.Join(t.TempDir(), "ta.test")
	build := exec.Command("go", "test", "-c", "-o", bin, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if b, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build the package for disassembly: %v\n%s", err, b)
	}
	out, err := exec.Command("go", "tool", "objdump", "-s", `^tradeloom/ta\.`, bin).Output()
	if err != nil {
		t.Skipf("go tool objdump unavailable: %v", err)
	}
	explicit := map[string]bool{}
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, "math.FMA(") {
				explicit[fmt.Sprintf("%s:%d", f, i+1)] = true
			}
		}
	}
	fused := regexp.MustCompile(`^\s*(\S+\.go:\d+)\s.*\b(FMADDD|FMSUBD|FNMADDD|FNMSUBD)\b`)
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	n := 0
	for sc.Scan() {
		m := fused.FindStringSubmatch(sc.Text())
		if m == nil {
			continue
		}
		n++
		site := filepath.Base(m[1])
		if strings.Contains(site, "_test.go") || explicit[site] || seen[site] {
			continue
		}
		seen[site] = true
		t.Errorf("%s: implicit fused multiply-add; wrap the product in float64(...)", site)
	}
	if n == 0 {
		t.Fatal("found no fused multiply-adds at all; the objdump parse is broken")
	}
}
