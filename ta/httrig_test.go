package ta

import (
	"math"
	"testing"
)

// TestHTSinCosTable checks that every tabulated libm value belongs to the
// angle the DFT loop computes for its (i, n): it may differ from Go's
// math.Sin/math.Cos only in the last ulp. Bit-exactness against C is proven
// by the HT_DCPHASE, HT_SINE and HT_TRENDMODE oracle tests on flat and
// periodic fixtures, where the phase depends on that last ulp.
func TestHTSinCosTable(t *testing.T) {
	c := math.Atan(1) * 8.0
	k := 0
	for n := 1; n <= htTrigMaxN; n++ {
		for i := 0; i < n; i++ {
			if n*(n-1)/2+i != k {
				t.Fatalf("index of n=%d i=%d is %d, want %d", n, i, n*(n-1)/2+i, k)
			}
			k++
			x := float64(float64(i)*c) / float64(n)
			s, co := htSinCos(i, n, x)
			if ws := math.Sin(x); math.Abs(s-ws) > 2.3e-16*math.Max(1, math.Abs(ws)) {
				t.Errorf("sin n=%d i=%d: table %.17g math %.17g", n, i, s, ws)
			}
			if wc := math.Cos(x); math.Abs(co-wc) > 2.3e-16*math.Max(1, math.Abs(wc)) {
				t.Errorf("cos n=%d i=%d: table %.17g math %.17g", n, i, co, wc)
			}
		}
	}
	if k != htTrigLen {
		t.Fatalf("visited %d entries, table has %d", k, htTrigLen)
	}
	// Anything outside the table falls back to math.Sin/math.Cos.
	for _, a := range [][2]int{{0, 0}, {0, 51}, {-1, 5}, {5, 5}} {
		if s, co := htSinCos(a[0], a[1], 0.5); s != libmSin(0.5) || co != libmCos(0.5) {
			t.Errorf("fallback i=%d n=%d: %g %g", a[0], a[1], s, co)
		}
	}
}
