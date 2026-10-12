package ta

// BOP — Balance Of Power (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/bop and src/ta_func/ta_BOP.c.

// BOPLookback returns the number of leading bars BOP leaves empty (always 0).
func BOPLookback() int {
	return bopLookback()
}

func bopLookback() int {
	return 0
}

// BOP returns the balance of power, (close - open) / (high - low), per bar.
// A bar whose high - low range is zero or negative yields 0.
// The output is index-aligned with the inputs.
func BOP(inOpen, inHigh, inLow, inClose []float64) ([]float64, error) {
	n, err := checkInputs("BOP", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := bopRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// bopRange mirrors TA_BOP: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias any input.
func bopRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("BOP", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("BOP", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// BOP = (Close - Open)/(High - Low). The divisor test is exact (<= 0),
	// not the TA_IS_ZERO_OR_NEG band (TA-Lib issue #253).
	outIdx := 0
	for i := startIdx; i <= endIdx; i++ {
		tempReal := inHigh[i] - inLow[i]
		if tempReal <= 0.0 {
			outReal[outIdx] = 0.0
		} else {
			outReal[outIdx] = (inClose[i] - inOpen[i]) / tempReal
		}
		outIdx++
	}
	return startIdx, outIdx, nil
}
