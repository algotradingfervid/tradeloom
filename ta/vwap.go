package ta

import "math"

// VWAP — Volume Weighted Average Price (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/vwap and src/ta_func/ta_VWAP.c.

// VWAPLookback returns the number of leading bars VWAP leaves empty (always 0).
// VWAP has no optional inputs, so it never returns -1.
func VWAPLookback() int {
	return vwapLookback()
}

func vwapLookback() int {
	return 0
}

// VWAP returns the volume weighted average of the typical price
// (high+low+close)/3, cumulative from the first bar. Bars whose typical price
// or volume is not finite are skipped; until any volume has traded the output
// is 0. The output is index-aligned with the inputs; there is no lookback.
func VWAP(inHigh, inLow, inClose, inVolume []float64) ([]float64, error) {
	n, err := checkInputs("VWAP", []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := vwapRange(0, n-1, inHigh, inLow, inClose, inVolume, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// vwapRange mirrors TA_VWAP: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. The sums start at 0 at startIdx (as in C), so a sub-range is
// not a slice of the full-series result. outReal may alias any input.
func vwapRange(startIdx, endIdx int, inHigh, inLow, inClose, inVolume []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VWAP", endIdx, []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VWAP", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	sumPV := 0.0
	sumV := 0.0
	vwap := 0.0
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		typPrice := (inHigh[i] + inLow[i] + inClose[i]) / 3.0
		volume := inVolume[i]
		// The explicit conversion forces rounding of the product, so Go cannot
		// fuse it with the add below (C keeps it in its own statement for the
		// same reason).
		tempReal := float64(typPrice * volume)
		if vwapIsFinite(typPrice) && vwapIsFinite(volume) {
			sumPV += tempReal
			sumV += volume
		}
		if sumV > 0.0 {
			vwap = sumPV / sumV
		}
		outReal[outIdx] = vwap
		outIdx++
	}
	return startIdx, outIdx, nil
}

// vwapIsFinite is TA_IS_FINITE (C isfinite).
func vwapIsFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
