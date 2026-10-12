package ta

// CDLLONGLEGGEDDOJI — Long Legged Doji (Pattern Recognition).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cdllongleggeddoji and
// src/ta_func/ta_CDLLONGLEGGEDDOJI.c.

// CDLLONGLEGGEDDOJILookback returns the number of leading bars
// CDLLONGLEGGEDDOJI leaves empty. It depends on the current candle settings
// (BodyDoji, ShadowLong). It has no parameters, so it never returns -1.
func CDLLONGLEGGEDDOJILookback() int {
	return cdllongleggeddojiLookback()
}

func cdllongleggeddojiLookback() int {
	cs := candleSettingsSnapshot()
	return cdllongleggeddojiLookbackFrom(&cs)
}

func cdllongleggeddojiLookbackFrom(cs *[candleSettingCount]CandleSetting) int {
	return cMaxInt(cs[BodyDoji].AvgPeriod, cs[ShadowLong].AvgPeriod)
}

// CDLLONGLEGGEDDOJI recognises the Long Legged Doji: a doji body with one or
// two long shadows. The output is index-aligned with the inputs: 100 where
// the pattern is present (this signals indecision, not bullishness), 0
// otherwise and inside the lookback.
func CDLLONGLEGGEDDOJI(inOpen, inHigh, inLow, inClose []float64) ([]int, error) {
	n, err := checkInputs("CDLLONGLEGGEDDOJI", []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cdllongleggeddojiRange(0, n-1, inOpen, inHigh, inLow, inClose, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// cdllongleggeddojiRange mirrors TA_CDLLONGLEGGEDDOJI: it evaluates
// [startIdx, endIdx] and writes outNBElement values to outInteger[0:], the
// first belonging to bar outBegIdx.
func cdllongleggeddojiRange(startIdx, endIdx int, inOpen, inHigh, inLow, inClose []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CDLLONGLEGGEDDOJI", endIdx, []string{"inOpen", "inHigh", "inLow", "inClose"}, inOpen, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("CDLLONGLEGGEDDOJI", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}

	cs := candleSettingsSnapshot()
	bodyDoji := cs[BodyDoji]
	shadowLong := cs[ShadowLong]

	lookbackTotal := cdllongleggeddojiLookbackFrom(&cs)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	rng := func(s CandleSetting, i int) float64 {
		return candleRange(s.RangeType, inOpen[i], inHigh[i], inLow[i], inClose[i])
	}

	bodyDojiPeriodTotal := 0.0
	bodyDojiTrailingIdx := startIdx - bodyDoji.AvgPeriod
	shadowLongPeriodTotal := 0.0
	shadowLongTrailingIdx := startIdx - shadowLong.AvgPeriod

	for i := bodyDojiTrailingIdx; i < startIdx; i++ {
		bodyDojiPeriodTotal += rng(bodyDoji, i)
	}
	for i := shadowLongTrailingIdx; i < startIdx; i++ {
		shadowLongPeriodTotal += rng(shadowLong, i)
	}

	// Must have: a doji body; one or two long shadows.
	i := startIdx
	outIdx := 0
	for {
		o, h, l, c := inOpen[i], inHigh[i], inLow[i], inClose[i]
		if realBody(c, o) <= candleAverage(bodyDoji, bodyDojiPeriodTotal, o, h, l, c) &&
			(lowerShadow(l, c, o) > candleAverage(shadowLong, shadowLongPeriodTotal, o, h, l, c) ||
				upperShadow(h, c, o) > candleAverage(shadowLong, shadowLongPeriodTotal, o, h, l, c)) {
			outInteger[outIdx] = 100
		} else {
			outInteger[outIdx] = 0
		}
		outIdx++
		// Add the current range and subtract the first range, after the
		// recognition (avgPeriod != 0 compares with the previous candles).
		bodyDojiPeriodTotal += rng(bodyDoji, i) - rng(bodyDoji, bodyDojiTrailingIdx)
		shadowLongPeriodTotal += rng(shadowLong, i) - rng(shadowLong, shadowLongTrailingIdx)
		i++
		bodyDojiTrailingIdx++
		shadowLongTrailingIdx++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
