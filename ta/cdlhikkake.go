package ta

// CDLHIKKAKE — Hikkake Pattern (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdlhikkake and src/ta_func/ta_CDLHIKKAKE.c.

// CDLHIKKAKELookback returns the number of leading bars CDLHIKKAKE leaves
// empty. It is always 5 (as in C; it uses no candle settings) and never
// returns -1.
func CDLHIKKAKELookback() int {
	return cdlhikkakeLookback()
}

func cdlhikkakeLookback() int {
	return 5
}

// CDLHIKKAKE recognises the Hikkake pattern: an inside bar (lower high and
// higher low than the bar before it) followed by a bar that breaks out of it
// on both ends in one direction. The output is index-aligned with the inputs:
// 100 / -100 at the hikkake bar (bullish: lower high and lower low; bearish:
// higher high and higher low), 200 / -200 at a confirmation bar within the
// next 3 bars (close above the inside bar's high / below its low), 0
// otherwise and inside the lookback. A new hikkake on a confirmation bar is
// reported instead of the confirmation.
func CDLHIKKAKE(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLHIKKAKE", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdlhikkakeRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdlhikkakeIsPattern is the C pattern test at bar i: bars i-2/i-1 form an
// inside bar and bar i breaks out of bar i-1 (both lower or both higher).
func cdlhikkakeIsPattern(inHigh, inLow []float64, i int) bool {
	return inHigh[i-1] < inHigh[i-2] &&
		inLow[i-1] > inLow[i-2] && // 1st + 2nd: lower high and higher low
		((inHigh[i] < inHigh[i-1] &&
			inLow[i] < inLow[i-1]) || // (bull) 3rd: lower high and lower low
			(inHigh[i] > inHigh[i-1] &&
				inLow[i] > inLow[i-1])) // (bear) 3rd: higher high and higher low
}

// cdlhikkakeSign is C's `(cond) ? 1 : 0 - 1`.
func cdlhikkakeSign(cond bool) int {
	if cond {
		return 1
	}
	return -1
}

// cdlhikkakeRange mirrors TA_CDLHIKKAKE: it evaluates [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first belonging to
// bar outBegIdx. The pattern state is warmed up over the 3 bars before
// startIdx, exactly as C does.
func cdlhikkakeRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLHIKKAKE", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLHIKKAKE", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	savedHigh := 0.0
	savedLow := 0.0

	lookbackTotal := cdlhikkakeLookback()
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	cd := 0
	patternResult := 0
	i := startIdx - 3
	for i < startIdx {
		if cdlhikkakeIsPattern(inHigh, inLow, i) {
			patternResult = 100 * cdlhikkakeSign(inHigh[i] < inHigh[i-1])
			savedHigh = inHigh[i-1]
			savedLow = inLow[i-1]
			cd = 4
		} else if cd > 0 &&
			((patternResult > 0 && inClose[i] > savedHigh) ||
				(patternResult < 0 && inClose[i] < savedLow)) {
			cd = 0
		}
		if cd > 0 {
			cd--
		}
		i++
	}

	i = startIdx
	outIdx := 0
	for {
		if cdlhikkakeIsPattern(inHigh, inLow, i) {
			patternResult = 100 * cdlhikkakeSign(inHigh[i] < inHigh[i-1])
			savedHigh = inHigh[i-1]
			savedLow = inLow[i-1]
			cd = 4
			outInteger[outIdx] = patternResult
			outIdx++
		} else if cd > 0 &&
			((patternResult > 0 && inClose[i] > savedHigh) || // close higher than the high of 2nd
				(patternResult < 0 && inClose[i] < savedLow)) { // close lower than the low of 2nd
			outInteger[outIdx] = patternResult + 100*cdlhikkakeSign(patternResult > 0)
			outIdx++
			cd = 0
		} else {
			outInteger[outIdx] = 0
			outIdx++
		}
		if cd > 0 {
			cd--
		}
		i++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
