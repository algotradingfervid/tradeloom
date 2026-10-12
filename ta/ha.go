package ta

// HA — Heikin-Ashi Candles (Price Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ha and src/ta_func/ta_HA.c.

// HALookback returns the number of leading bars HA leaves empty, which is the
// unstable period set for UnstHA (0 by default). HA has no optional inputs,
// so it never returns -1.
func HALookback() int {
	return haLookback()
}

func haLookback() int {
	return UnstablePeriod(UnstHA)
}

// HA returns the Heikin-Ashi open, high, low and close of each bar. The
// outputs are index-aligned with the inputs; the first HALookback bars are
// NaN. The first emitted candle is seeded from bar outBegIdx-lookback, whose
// open is (open+close)/2.
func HA(inOpen, inHigh, inLow, inClose []float64) (outHAOpen, outHAHigh, outHALow, outHAClose []float64, err error) {
	n, err := checkInputs("HA", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	outHAOpen = make([]float64, n)
	outHAHigh = make([]float64, n)
	outHALow = make([]float64, n)
	outHAClose = make([]float64, n)
	if n == 0 {
		return outHAOpen, outHAHigh, outHALow, outHAClose, nil
	}
	beg, nb, err := haRange(0, n-1, inOpen, inHigh, inLow, inClose, outHAOpen, outHAHigh, outHALow, outHAClose)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	alignReal(outHAOpen, beg, nb)
	alignReal(outHAHigh, beg, nb)
	alignReal(outHALow, beg, nb)
	alignReal(outHAClose, beg, nb)
	return outHAOpen, outHAHigh, outHALow, outHAClose, nil
}

// haRange mirrors TA_HA: it computes [startIdx, endIdx] and writes
// outNBElement values to each output starting at index 0, the first one
// belonging to bar outBegIdx. Any output may alias any input, but the four
// outputs must be distinct buffers (C rejects two equal output pointers).
func haRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outHAOpen, outHAHigh, outHALow, outHAClose []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HA", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HA", endIdx-startIdx, []string{"outHAOpen", "outHAHigh", "outHALow", "outHAClose"}, outHAOpen, outHAHigh, outHALow, outHAClose); err != nil {
		return 0, 0, err
	}
	o, h, l, c := &outHAOpen[0], &outHAHigh[0], &outHALow[0], &outHAClose[0]
	if o == h || o == l || o == c || h == l || h == c || l == c {
		return 0, 0, badParam("HA", "outHAOpen/outHAHigh/outHALow/outHAClose", "output buffers must be distinct")
	}

	lookbackTotal := haLookback()
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	// Make sure there is still something to evaluate.
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// The summation order ((O+H)+L)+C is part of the C contract; AVGPRICE
	// sums ((H+L)+C)+O, which can differ in the last bit.
	today := startIdx - lookbackTotal
	haOpen := (inOpen[today] + inClose[today]) / 2.0
	haClose := (inOpen[today] + inHigh[today] + inLow[today] + inClose[today]) / 4.0
	today++

	// Skip the unstable period. haOpen consumes the previous candle, so it
	// advances before haClose is overwritten.
	for i := lookbackTotal; i != 0; i-- {
		haOpen = (haOpen + haClose) / 2.0
		haClose = (inOpen[today] + inHigh[today] + inLow[today] + inClose[today]) / 4.0
		today++
	}

	// The extremum uses plain comparisons exactly as C does (the first
	// operand wins ties), which fixes the sign of a zero result.
	tempHigh := inHigh[startIdx]
	tempLow := inLow[startIdx]
	haHigh := tempHigh
	if haOpen > haHigh {
		haHigh = haOpen
	}
	if haClose > haHigh {
		haHigh = haClose
	}
	haLow := tempLow
	if haOpen < haLow {
		haLow = haOpen
	}
	if haClose < haLow {
		haLow = haClose
	}
	outHAOpen[0] = haOpen
	outHAHigh[0] = haHigh
	outHALow[0] = haLow
	outHAClose[0] = haClose

	outIdx := 1
	for today <= endIdx {
		// Read every price of the bar before writing any output, so outputs
		// may alias inputs.
		tempOpen := inOpen[today]
		tempHigh = inHigh[today]
		tempLow = inLow[today]
		tempClose := inClose[today]
		haOpen = (haOpen + haClose) / 2.0
		haClose = (tempOpen + tempHigh + tempLow + tempClose) / 4.0
		haHigh = tempHigh
		if haOpen > haHigh {
			haHigh = haOpen
		}
		if haClose > haHigh {
			haHigh = haClose
		}
		haLow = tempLow
		if haOpen < haLow {
			haLow = haOpen
		}
		if haClose < haLow {
			haLow = haClose
		}
		outHAOpen[outIdx] = haOpen
		outHAHigh[outIdx] = haHigh
		outHALow[outIdx] = haLow
		outHAClose[outIdx] = haClose
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
