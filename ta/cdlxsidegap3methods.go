package ta

// CDLXSIDEGAP3METHODS — Upside/Downside Gap Three Methods (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlxsidegap3methods and
// src/ta_func/ta_CDLXSIDEGAP3METHODS.c.

// CDLXSIDEGAP3METHODSLookback returns the number of leading bars
// CDLXSIDEGAP3METHODS leaves empty. It is always 2 (it uses no candle
// settings) and never returns -1.
func CDLXSIDEGAP3METHODSLookback() int {
	return cdlxsidegap3methodsLookback()
}

func cdlxsidegap3methodsLookback() int {
	return 2
}

// CDLXSIDEGAP3METHODS recognises Upside/Downside Gap Three Methods: two
// same-colour candles with a real-body gap between them, then an
// opposite-colour candle that opens within the second real body and closes
// within the first. The output is index-aligned with the inputs: 100 (first
// two white, upside gap), -100 (first two black, downside gap), 0 otherwise
// and inside the lookback.
func CDLXSIDEGAP3METHODS(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLXSIDEGAP3METHODS", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlxsidegap3methodsRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlxsidegap3methodsRange mirrors TA_CDLXSIDEGAP3METHODS: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdlxsidegap3methodsRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLXSIDEGAP3METHODS", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLXSIDEGAP3METHODS", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	lookbackTotal := cdlxsidegap3methodsLookback()
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	i := startIdx
	outIdx := 0
	for {
		if candleColor(inClose[i-2], inOpen[i-2]) == candleColor(inClose[i-1], inOpen[i-1]) && // 1st and 2nd of same color
			candleColor(inClose[i-1], inOpen[i-1]) == -candleColor(inClose[i], inOpen[i]) && // 3rd opposite color
			inOpen[i] < cMax(inClose[i-1], inOpen[i-1]) && // 3rd opens within 2nd rb
			inOpen[i] > cMin(inClose[i-1], inOpen[i-1]) &&
			inClose[i] < cMax(inClose[i-2], inOpen[i-2]) && // 3rd closes within 1st rb
			inClose[i] > cMin(inClose[i-2], inOpen[i-2]) &&
			((candleColor(inClose[i-2], inOpen[i-2]) == 1 && // when 1st is white
				realBodyGapUp(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2])) || // upside gap
				(candleColor(inClose[i-2], inOpen[i-2]) == -1 && // when 1st is black
					realBodyGapDown(inOpen[i-1], inClose[i-1], inOpen[i-2], inClose[i-2]))) { // downside gap
			outInteger[outIdx] = candleColor(inClose[i-2], inOpen[i-2]) * 100
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
