package ta

import "math"

// ADX — Average Directional Movement Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/adx and src/ta_func/ta_ADX.c.

// ADXLookback returns the number of leading bars ADX leaves empty:
// 2*optInTimePeriod - 1 plus the ADX unstable period.
// optInTimePeriod: [2, 100000], default 14.
func ADXLookback(optInTimePeriod int) int {
	if checkInt("ADX", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return adxLookback(optInTimePeriod)
}

func adxLookback(optInTimePeriod int) int {
	return 2*optInTimePeriod + UnstablePeriod(UnstADX) - 1
}

// ADX returns Wilder's Average Directional Movement Index of
// (inHigh, inLow, inClose) over optInTimePeriod bars. The output is
// index-aligned with the inputs; the first ADXLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func ADX(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ADX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ADX", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := adxRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// adxRange mirrors TA_ADX: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func adxRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ADX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADX", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADX", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	unstable := UnstablePeriod(UnstADX)
	lookbackTotal := 2*optInTimePeriod + unstable - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	period := float64(optInTimePeriod)
	periodM1 := float64(optInTimePeriod - 1)

	outBegIdx = startIdx
	prevMinusDM := 0.0
	prevPlusDM := 0.0
	prevTR := 0.0
	today := startIdx - lookbackTotal
	prevHigh := inHigh[today]
	prevLow := inLow[today]
	prevClose := inClose[today]

	// Initial DM and TR sums over optInTimePeriod-1 one-bar values.
	for i := optInTimePeriod - 1; i > 0; i-- {
		today++
		tempReal := inHigh[today]
		diffP := tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM := prevLow - tempReal
		prevLow = tempReal
		if diffM > 0 && diffP < diffM {
			prevMinusDM += diffM
		} else if diffP > 0 && diffP > diffM {
			prevPlusDM += diffP
		}
		prevTR += trueRange(prevHigh, prevLow, prevClose)
		prevClose = inClose[today]
	}

	// step advances one bar with Wilder smoothing and returns the DX, or
	// ok=false when C skips the DX (zero TR or zero DI sum).
	step := func() (dx float64, ok bool) {
		today++
		tempReal := inHigh[today]
		diffP := tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM := prevLow - tempReal
		prevLow = tempReal
		prevMinusDM -= prevMinusDM / period
		prevPlusDM -= prevPlusDM / period
		if diffM > 0 && diffP < diffM {
			prevMinusDM += diffM
		} else if diffP > 0 && diffP > diffM {
			prevPlusDM += diffP
		}
		tempReal = trueRange(prevHigh, prevLow, prevClose)
		prevTR = prevTR - prevTR/period + tempReal
		prevClose = inClose[today]
		if prevTR > 0.0 {
			minusDI := float64(100.0 * (prevMinusDM / prevTR))
			plusDI := float64(100.0 * (prevPlusDM / prevTR))
			tempReal = minusDI + plusDI
			if !isZero(tempReal) {
				return float64(100.0 * (math.Abs(minusDI-plusDI) / tempReal)), true
			}
		}
		return 0, false
	}

	// Add up the initial DX.
	sumDX := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		if dx, ok := step(); ok {
			sumDX += dx
		}
	}
	prevADX := sumDX / period

	// Skip the unstable period.
	for i := unstable; i > 0; i-- {
		if dx, ok := step(); ok {
			prevADX = (float64(prevADX*periodM1) + dx) / period
		}
	}

	outReal[0] = prevADX
	outIdx := 1
	for today < endIdx {
		if dx, ok := step(); ok {
			prevADX = (float64(prevADX*periodM1) + dx) / period
		}
		outReal[outIdx] = prevADX
		outIdx++
	}
	return outBegIdx, outIdx, nil
}
