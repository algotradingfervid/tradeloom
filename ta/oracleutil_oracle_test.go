//go:build oracle

package ta

import (
	"testing"

	"tradeloom/ta/internal/oracle"
)

// Oracle helpers shared by every *_oracle_test.go file.

// ohlcv returns the oracle price inputs for a fixture.
func (s series) ohlcv() oracle.Inputs {
	return oracle.Inputs{Open: s.O, High: s.H, Low: s.L, Close: s.C, Volume: s.V}
}

// realIn returns oracle inputs with the given real series.
func realIn(reals ...[]float64) oracle.Inputs {
	return oracle.Inputs{Real: reals}
}

// mustCall runs the C function over the whole input and fails t on a harness
// error (not on a non-zero TA_RetCode, which is returned for the test to check).
func mustCall(t *testing.T, name string, in oracle.Inputs, opt ...float64) oracle.Result {
	t.Helper()
	r, err := oracle.Call(name, in, opt...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// mustCallRange is mustCall for an explicit [startIdx, endIdx].
func mustCallRange(t *testing.T, name string, in oracle.Inputs, startIdx, endIdx int, opt ...float64) oracle.Result {
	t.Helper()
	r, err := oracle.CallRange(name, in, startIdx, endIdx, opt...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// mustLookback returns the C lookback, failing t on error.
func mustLookback(t *testing.T, name string, opt ...float64) int {
	t.Helper()
	lb, err := oracle.Lookback(name, opt...)
	if err != nil {
		t.Fatal(err)
	}
	return lb
}
