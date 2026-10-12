package ta

import "math"

// PVI — Positive Volume Index (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/pvi and src/ta_func/ta_PVI.c.

// PVILookback returns the number of leading bars PVI leaves empty (always 0).
// PVI has no optional inputs, so it never returns -1.
func PVILookback() int {
	return pviLookback()
}

func pviLookback() int {
	return 0
}

// PVI returns the Positive Volume Index of inClose and inVolume: a running
// index seeded at 1000 that compounds the bar's percentage close change on
// bars whose volume rose versus the prior bar, and carries forward otherwise
// (also when the prior close is zero or the update would not be finite).
// The output is index-aligned with the inputs; there is no lookback.
func PVI(inClose, inVolume []float64) ([]float64, error) {
	n, err := checkInputs("PVI", []string{"inClose", "inVolume"}, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := pviRange(0, n-1, inClose, inVolume, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// pviRange mirrors TA_PVI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. The index is seeded at 1000 on bar startIdx (as in C), so a
// sub-range is not a slice of the full-series result. outReal may alias
// either input.
func pviRange(startIdx, endIdx int, inClose, inVolume []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PVI", endIdx, []string{"inClose", "inVolume"}, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PVI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	prevPVI := 1000.0
	prevClose := inClose[startIdx]
	prevVolume := inVolume[startIdx]
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		tempClose := inClose[i]
		tempVolume := inVolume[i]
		if tempVolume > prevVolume && prevClose != 0.0 {
			// The C library is built with -ffp-contract=off; Go may fuse
			// a*b+c into an FMA, which re-rounds and drifts badly when the
			// update nearly cancels the index. The explicit float64()
			// conversion rounds the product and so blocks fusion.
			tempPVI := prevPVI
			tempPVI += float64((tempClose - prevClose) / prevClose * tempPVI)
			if !math.IsInf(tempPVI, 0) && !math.IsNaN(tempPVI) {
				prevPVI = tempPVI
			}
		}
		outReal[outIdx] = prevPVI
		outIdx++
		prevClose = tempClose
		prevVolume = tempVolume
	}
	return startIdx, outIdx, nil
}
