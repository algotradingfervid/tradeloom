package ta

// CDL3OUTSIDE — Three Outside Up/Down (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdl3outside and src/ta_func/ta_CDL3OUTSIDE.c.

// CDL3OUTSIDELookback returns the number of leading bars CDL3OUTSIDE leaves
// empty. It is always 3 (it uses no candle settings) and never returns -1.
func CDL3OUTSIDELookback() int {
	return cdl3outsideLookback()
}

func cdl3outsideLookback() int {
	return 3
}

// CDL3OUTSIDE recognises Three Outside Up/Down: an engulfing pair (candle 2's
// real body engulfs candle 1's, with opposite colours) followed by a third
// candle closing beyond candle 2's close in the engulfing direction.
// The output is index-aligned with the inputs: 100 (up), -100 (down), 0
// otherwise and inside the lookback.
func CDL3OUTSIDE(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDL3OUTSIDE", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdl3outsideRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdl3outsideRange mirrors TA_CDL3OUTSIDE: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx.
func cdl3outsideRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDL3OUTSIDE", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDL3OUTSIDE", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	lookbackTotal := cdl3outsideLookback()
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	i := startIdx
	outIdx := 0
	for {
		if (candleColor(inClose[i-1], inOpen[i-1]) == 1 &&
			candleColor(inClose[i-2], inOpen[i-2]) == -1 && // white engulfs black
			inClose[i-1] > inOpen[i-2] &&
			inOpen[i-1] < inClose[i-2] &&
			inClose[i] > inClose[i-1]) || // third candle higher
			(candleColor(inClose[i-1], inOpen[i-1]) == -1 &&
				candleColor(inClose[i-2], inOpen[i-2]) == 1 && // black engulfs white
				inOpen[i-1] > inClose[i-2] &&
				inClose[i-1] < inOpen[i-2] &&
				inClose[i] < inClose[i-1]) { // third candle lower
			outInteger[outIdx] = candleColor(inClose[i-1], inOpen[i-1]) * 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		i++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
