package ta

import "math"

// NVI — Negative Volume Index (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/nvi and src/ta_func/ta_NVI.c.

// NVILookback returns the number of leading bars NVI leaves empty (always 0).
func NVILookback() int {
	return nviLookback()
}

func nviLookback() int {
	return 0
}

// NVI returns the Negative Volume Index of inClose/inVolume: a running index
// seeded at 1000 at the first bar that compounds the percentage close change
// only on bars whose volume fell versus the prior bar. The output is
// index-aligned with the inputs and has no lookback.
func NVI(inClose, inVolume []float64) ([]float64, error) {
	n, err := checkInputs("NVI", []string{"inClose", "inVolume"}, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := nviRange(0, n-1, inClose, inVolume, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// nviRange mirrors TA_NVI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx (always startIdx). The index is re-seeded at 1000 at startIdx.
func nviRange(startIdx, endIdx int, inClose, inVolume []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("NVI", endIdx, []string{"inClose", "inVolume"}, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("NVI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	prevNVI := 1000.0
	prevClose := inClose[startIdx]
	prevVolume := inVolume[startIdx]
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		tempClose := inClose[i]
		tempVolume := inVolume[i]
		// prevClose != 0 guards the division; a zero previous close carries
		// the index forward unchanged.
		if tempVolume < prevVolume && prevClose != 0.0 {
			tempNVI := prevNVI
			// The explicit float64 conversion forbids Go from fusing the
			// multiply into the add (C keeps them separately rounded).
			tempNVI += float64((tempClose - prevClose) / prevClose * tempNVI)
			// TA_IS_FINITE: keep the last representable value on overflow.
			if !math.IsInf(tempNVI, 0) && !math.IsNaN(tempNVI) {
				prevNVI = tempNVI
			}
		}
		outReal[outIdx] = prevNVI
		outIdx++
		prevClose = tempClose
		prevVolume = tempVolume
	}
	return startIdx, outIdx, nil
}
