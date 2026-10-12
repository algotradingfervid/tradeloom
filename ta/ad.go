package ta

// AD — Chaikin A/D Line (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ad and src/ta_func/ta_AD.c.

// ADLookback returns the number of leading bars AD leaves empty (always 0).
// AD has no optional inputs, so it never returns -1.
func ADLookback() int {
	return adLookback()
}

func adLookback() int {
	return 0
}

// AD returns the Chaikin Accumulation/Distribution line: the running sum,
// seeded at 0, of ((close-low)-(high-close))/(high-low) * volume. Bars with
// high-low <= 0 add nothing. The output is index-aligned with the inputs;
// there is no lookback.
func AD(inHigh, inLow, inClose, inVolume []float64) ([]float64, error) {
	n, err := checkInputs("AD", []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := adRange(0, n-1, inHigh, inLow, inClose, inVolume, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// adRange mirrors TA_AD: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. The running sum starts at 0 at startIdx (as in C), so a
// sub-range is not a slice of the full-series result. outReal may alias any
// of the inputs.
func adRange(startIdx, endIdx int, inHigh, inLow, inClose, inVolume []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AD", endIdx, []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AD", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	nbBar := endIdx - startIdx + 1
	outNBElement = nbBar
	outBegIdx = startIdx
	currentBar := startIdx
	outIdx := 0
	ad := 0.0

	for nbBar != 0 {
		high := inHigh[currentBar]
		low := inLow[currentBar]
		tmp := high - low
		close := inClose[currentBar]
		if tmp > 0.0 {
			ad += float64((close - low - (high - close)) / tmp * inVolume[currentBar])
		}
		outReal[outIdx] = ad
		outIdx++
		currentBar++
		nbBar--
	}
	return outBegIdx, outNBElement, nil
}
