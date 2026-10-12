package ta

// CDLENGULFING — Engulfing Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlengulfing and src/ta_func/ta_CDLENGULFING.c.

// CDLENGULFINGLookback returns the number of leading bars CDLENGULFING leaves
// empty. It is always 2 (as in C, although the pattern only reads the prior
// bar; it uses no candle settings) and never returns -1.
func CDLENGULFINGLookback() int {
	return cdlengulfingLookback()
}

func cdlengulfingLookback() int {
	return 2
}

// CDLENGULFING recognises the Engulfing pattern: a candle whose real body
// engulfs the prior candle's opposite-coloured real body. The output is
// index-aligned with the inputs: 100 / -100 (bullish / bearish, strictly
// engulfing at both ends), 80 / -80 (the bodies share one exact endpoint),
// 0 otherwise and inside the lookback. The prior trend is not checked.
func CDLENGULFING(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLENGULFING", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlengulfingRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlengulfingRange mirrors TA_CDLENGULFING: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx.
func cdlengulfingRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLENGULFING", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLENGULFING", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	lookbackTotal := cdlengulfingLookback()
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	i := startIdx
	outIdx := 0
	for {
		if (candleColor(inClose[i], inOpen[i]) == 1 &&
			candleColor(inClose[i-1], inOpen[i-1]) == -1 && // white engulfs black
			((inClose[i] >= inOpen[i-1] && inOpen[i] < inClose[i-1]) ||
				(inClose[i] > inOpen[i-1] && inOpen[i] <= inClose[i-1]))) ||
			(candleColor(inClose[i], inOpen[i]) == -1 &&
				candleColor(inClose[i-1], inOpen[i-1]) == 1 && // black engulfs white
				((inOpen[i] >= inClose[i-1] && inClose[i] < inOpen[i-1]) ||
					(inOpen[i] > inClose[i-1] && inClose[i] <= inOpen[i-1]))) {
			if inOpen[i] != inClose[i-1] && inClose[i] != inOpen[i-1] {
				outInteger[outIdx] = candleColor(inClose[i], inOpen[i]) * 100
			} else {
				outInteger[outIdx] = candleColor(inClose[i], inOpen[i]) * 80
			}
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
