package ta

// PVT — Price Volume Trend (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/pvt and src/ta_func/ta_PVT.c.

// PVTLookback returns the number of leading bars PVT leaves empty (always 0).
// PVT has no optional inputs, so it never returns -1.
func PVTLookback() int {
	return pvtLookback()
}

func pvtLookback() int {
	return 0
}

// PVT returns the Price Volume Trend of inClose and inVolume: a running total,
// seeded at zero on the first bar, of each bar's volume weighted by the
// fractional change of the close from the previous bar. A bar whose previous
// close is exactly zero contributes nothing. The output is index-aligned with
// the inputs; there is no lookback.
func PVT(inClose, inVolume []float64) ([]float64, error) {
	n, err := checkInputs("PVT", []string{"inClose", "inVolume"}, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := pvtRange(0, n-1, inClose, inVolume, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// pvtRange mirrors TA_PVT: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. The running total starts at zero on bar startIdx (as in C), so a
// sub-range is the full-series result shifted by a constant. outReal may
// alias either input.
func pvtRange(startIdx, endIdx int, inClose, inVolume []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PVT", endIdx, []string{"inClose", "inVolume"}, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PVT", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	prevPVT := 0.0
	prevClose := inClose[startIdx]
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		tempClose := inClose[i]
		// Exact test, never an epsilon band (as in C): a zero previous close
		// contributes nothing rather than Inf/NaN.
		if prevClose != 0.0 {
			prevPVT += float64((tempClose - prevClose) / prevClose * inVolume[i])
		}
		outReal[outIdx] = prevPVT
		outIdx++
		prevClose = tempClose
	}
	return startIdx, outIdx, nil
}
