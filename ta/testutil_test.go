package ta

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

// Shared test fixtures and assertions. Indicator tests must use these so every
// function is exercised against the same boundary conditions.

// series is one OHLCV fixture. Price data always satisfies
// low <= min(open, close) <= max(open, close) <= high unless Name says
// "malformed".
type series struct {
	Name          string
	O, H, L, C, V []float64
}

// Real returns the close series (the usual single real input).
func (s series) Real() []float64 { return s.C }

// fixtures returns every fixture kind at length n. Generation is
// deterministic, so failures reproduce.
func fixtures(n int) []series {
	return []series{
		randomWalk("walk", 1, n, 1000, 1),
		randomWalk("walk2", 2, n, 250, 1),
		randomWalk("penny", 3, n, 0.5, 1e-3), // small prices
		randomWalk("tiny", 4, n, 1e-6, 1e-9), // scale far below TA_EPSILON-ish bands
		randomWalk("huge", 5, n, 1e9, 1e6),   // large magnitudes
		constant("constant", n, 100),         // flat: zero variance, zero range
		constant("zeros", n, 0),              // all zero, incl. volume
		trend("up", n, 100, 1),               // strictly rising
		trend("down", n, 1000, -1),           // strictly falling
		zigzag("zigzag", n),                  // alternating up/down
		dojis("doji", 6, n),                  // open == close on many bars
		gappy("gaps", 7, n),                  // frequent gaps between bars
		randomWalk("walk-zero-vol", 8, n, 500, 1).withVolume(0),
		negative("negative", 9, n), // negative values (math/transform funcs)
	}
}

// edgeLengths are the input lengths every indicator must be tested at,
// around its lookback lb.
func edgeLengths(lb int) []int {
	set := map[int]bool{}
	var out []int
	for _, n := range []int{0, 1, 2, 3, lb - 1, lb, lb + 1, lb + 2, lb + 10, 64, 257, 1000} {
		if n >= 0 && !set[n] {
			set[n] = true
			out = append(out, n)
		}
	}
	return out
}

func randomWalk(name string, seed uint64, n int, start, step float64) series {
	r := rand.New(rand.NewPCG(seed, 0x7a11))
	s := series{Name: name}
	price := start
	for range n {
		open := price
		close := open + (r.Float64()*2-1)*step*5
		if close <= 0 && start > 0 {
			close = math.Abs(close) + step
		}
		hi := math.Max(open, close) + r.Float64()*step*3
		lo := math.Min(open, close) - r.Float64()*step*3
		if lo <= 0 && start > 0 {
			lo = math.Min(open, close) * 0.99
		}
		s.O = append(s.O, open)
		s.H = append(s.H, hi)
		s.L = append(s.L, lo)
		s.C = append(s.C, close)
		s.V = append(s.V, math.Floor(r.Float64()*1e6)+1)
		price = close + (r.Float64()*2-1)*step // small gap to next open
	}
	return s
}

func constant(name string, n int, v float64) series {
	s := series{Name: name}
	for range n {
		s.O = append(s.O, v)
		s.H = append(s.H, v)
		s.L = append(s.L, v)
		s.C = append(s.C, v)
		s.V = append(s.V, v)
	}
	return s
}

func trend(name string, n int, start, step float64) series {
	s := series{Name: name}
	for i := range n {
		o := start + float64(i)*step
		c := o + step*0.8
		s.O = append(s.O, o)
		s.H = append(s.H, math.Max(o, c)+0.5)
		s.L = append(s.L, math.Min(o, c)-0.5)
		s.C = append(s.C, c)
		s.V = append(s.V, 1000+float64(i))
	}
	return s
}

func zigzag(name string, n int) series {
	s := series{Name: name}
	for i := range n {
		o, c := 100.0, 105.0
		if i%2 == 1 {
			o, c = 105, 100
		}
		s.O = append(s.O, o)
		s.H = append(s.H, 106)
		s.L = append(s.L, 99)
		s.C = append(s.C, c)
		s.V = append(s.V, float64(100*(i%3+1)))
	}
	return s
}

func dojis(name string, seed uint64, n int) series {
	s := randomWalk(name, seed, n, 300, 1)
	for i := range n {
		if i%3 != 2 {
			s.C[i] = s.O[i]
		}
	}
	return s
}

func gappy(name string, seed uint64, n int) series {
	r := rand.New(rand.NewPCG(seed, 0x9a95))
	s := series{Name: name}
	price := 500.0
	for range n {
		gap := (r.Float64()*2 - 1) * 20
		o := price + gap
		c := o + (r.Float64()*2-1)*8
		s.O = append(s.O, o)
		s.H = append(s.H, math.Max(o, c)+r.Float64()*2)
		s.L = append(s.L, math.Min(o, c)-r.Float64()*2)
		s.C = append(s.C, c)
		s.V = append(s.V, math.Floor(r.Float64()*5e5)+1)
		price = c
	}
	return s
}

func negative(name string, seed uint64, n int) series {
	s := randomWalk(name, seed, n, 0, 1)
	return s
}

func (s series) withVolume(v float64) series {
	s.V = make([]float64, len(s.C))
	for i := range s.V {
		s.V[i] = v
	}
	return s
}

// --- assertions ---

// relTol is the default comparison tolerance against the C oracle. Go (arm64)
// may fuse multiply-adds where C does not, so results can differ in the last
// few ulps; anything beyond this is a porting bug.
const relTol = 1e-9

func closeEnough(a, b, tol float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	if math.IsInf(a, 0) || math.IsInf(b, 0) {
		return a == b
	}
	diff := math.Abs(a - b)
	scale := math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	return diff <= tol*scale
}

// assertReal fails t if got and want differ in length or in any element
// beyond tol (NaN must match NaN). It reports the first few mismatches.
func assertReal(t *testing.T, label string, got, want []float64, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len(got)=%d len(want)=%d", label, len(got), len(want))
	}
	bad := 0
	for i := range got {
		if !closeEnough(got[i], want[i], tol) {
			if bad < 5 {
				t.Errorf("%s: [%d] got %.17g want %.17g", label, i, got[i], want[i])
			}
			bad++
		}
	}
	if bad > 5 {
		t.Errorf("%s: %d mismatches in total", label, bad)
	}
}

// assertInt is assertReal for integer outputs (exact match).
func assertInt(t *testing.T, label string, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: len(got)=%d len(want)=%d", label, len(got), len(want))
	}
	bad := 0
	for i := range got {
		if got[i] != want[i] {
			if bad < 5 {
				t.Errorf("%s: [%d] got %d want %d", label, i, got[i], want[i])
			}
			bad++
		}
	}
	if bad > 5 {
		t.Errorf("%s: %d mismatches in total", label, bad)
	}
}

// assertLeadingNaN checks out has exactly lb leading NaNs (clamped to len)
// and no NaN after them.
func assertLeadingNaN(t *testing.T, label string, out []float64, lb int) {
	t.Helper()
	for i, v := range out {
		if i < lb && !math.IsNaN(v) {
			t.Errorf("%s: [%d]=%g, want NaN inside lookback %d", label, i, v, lb)
			return
		}
		if i >= lb && math.IsNaN(v) {
			t.Errorf("%s: [%d] is NaN after lookback %d", label, i, lb)
			return
		}
	}
}

func lbl(parts ...any) string { return fmt.Sprint(parts...) }
