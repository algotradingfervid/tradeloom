package ta

import "math"

// NATR — Normalized Average True Range (Volatility Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/natr and src/ta_func/ta_NATR.c.

// NATRLookback returns the number of leading bars NATR leaves empty: the
// period plus the NATR unstable period. optInTimePeriod: [1, 100000], default 14.
func NATRLookback(optInTimePeriod int) int {
	if checkInt("NATR", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return natrLookback(optInTimePeriod)
}

func natrLookback(optInTimePeriod int) int {
	return optInTimePeriod + UnstablePeriod(UnstNATR)
}

// NATR returns the Average True Range of (inHigh, inLow, inClose) as a
// percentage of each bar's close: ATR / close * 100. A close of exactly zero
// yields 0. With optInTimePeriod 1 the output is the raw true range, not
// normalized (as in TA-Lib). The output is index-aligned with the inputs; the
// first NATRLookback bars are NaN. optInTimePeriod: [1, 100000], default 14.
func NATR(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("NATR", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("NATR", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := natrRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// natrRange mirrors TA_NATR: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func natrRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("NATR", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("NATR", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("NATR", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := natrLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// wAlpha is derived from wBeta (not 1/period) so wAlpha+wBeta == 1 exactly.
	wBeta := float64(optInTimePeriod-1) / float64(optInTimePeriod)
	wAlpha := 1.0 - wBeta

	today := startIdx - lookbackTotal + 1

	// Seed: simple average of the first optInTimePeriod true ranges.
	periodTotal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		periodTotal += trueRange(inHigh[today], inLow[today], inClose[today-1])
		today++
	}
	prevATR := periodTotal / float64(optInTimePeriod)

	// Skip the unstable period. C uses an explicit fma(); math.FMA matches it bit for bit.
	for i := UnstablePeriod(UnstNATR); i != 0; i-- {
		greatest := trueRange(inHigh[today], inLow[today], inClose[today-1])
		prevATR = math.FMA(wBeta, prevATR, wAlpha*greatest)
		today++
	}

	outIdx := 1
	if optInTimePeriod <= 1 {
		// Period 1 is the raw true range and is deliberately not normalized.
		outReal[0] = prevATR
	} else {
		// Exact zero test on the close (not the TA_IS_ZERO band).
		if tempValue := inClose[startIdx]; tempValue != 0.0 {
			outReal[0] = prevATR / tempValue * 100.0
		} else {
			outReal[0] = 0.0
		}
	}

	for nbATR := endIdx - startIdx + 1; nbATR > 1; nbATR-- {
		greatest := trueRange(inHigh[today], inLow[today], inClose[today-1])
		prevATR = math.FMA(wBeta, prevATR, wAlpha*greatest)
		if optInTimePeriod <= 1 {
			outReal[outIdx] = prevATR
		} else {
			if tempValue := inClose[today]; tempValue != 0.0 {
				outReal[outIdx] = prevATR / tempValue * 100.0
			} else {
				outReal[outIdx] = 0.0
			}
		}
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
