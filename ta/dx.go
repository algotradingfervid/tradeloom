package ta

import "math"

// DX — Directional Movement Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/dx and src/ta_func/ta_DX.c.

// DXLookback returns the number of leading bars DX leaves empty: the period
// plus the DX unstable period.
// optInTimePeriod: [2, 100000], default 14.
func DXLookback(optInTimePeriod int) int {
	if checkInt("DX", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return dxLookback(optInTimePeriod)
}

func dxLookback(optInTimePeriod int) int {
	if optInTimePeriod > 1 {
		return optInTimePeriod + UnstablePeriod(UnstDX)
	}
	return 2
}

// DX returns Wilder's directional movement index of (inHigh, inLow, inClose)
// over optInTimePeriod bars. The output is index-aligned with the inputs; the
// first DXLookback bars are NaN. optInTimePeriod: [2, 100000], default 14.
func DX(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("DX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("DX", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := dxRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// dxRange mirrors TA_DX: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func dxRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("DX", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DX", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DX", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	unstable := UnstablePeriod(UnstDX)
	var lookbackTotal int
	if optInTimePeriod > 1 {
		lookbackTotal = optInTimePeriod + unstable
	} else {
		lookbackTotal = 2
	}
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	period := float64(optInTimePeriod)
	var prevHigh, prevLow, prevClose, tempReal, diffP, diffM, minusDI, plusDI float64

	// Process the initial DM and TR.
	outBegIdx = startIdx
	prevMinusDM := 0.0
	prevPlusDM := 0.0
	prevTR := 0.0
	today := startIdx - lookbackTotal
	prevHigh = inHigh[today]
	prevLow = inLow[today]
	prevClose = inClose[today]
	for i := optInTimePeriod - 1; i > 0; i-- {
		today++
		tempReal = inHigh[today]
		diffP = tempReal - prevHigh // Plus Delta
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM = prevLow - tempReal // Minus Delta
		prevLow = tempReal
		if diffM > 0 && diffP < diffM {
			prevMinusDM += diffM // Case 2 and 4: +DM=0, -DM=diffM
		} else if diffP > 0 && diffP > diffM {
			prevPlusDM += diffP // Case 1 and 3: +DM=diffP, -DM=0
		}
		tempReal = trueRange(prevHigh, prevLow, prevClose)
		prevTR += tempReal
		prevClose = inClose[today]
	}

	// Skip the unstable period. This loop runs at least once to compute the
	// first DI.
	for i := unstable + 1; i != 0; i-- {
		today++
		tempReal = inHigh[today]
		diffP = tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM = prevLow - tempReal
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
	}

	// First DX output. prevTR is tested exactly (see the C comment on issue
	// #253); the DI sum keeps the fixed TA_IS_ZERO band.
	if prevTR > 0.0 {
		minusDI = float64(100.0 * (prevMinusDM / prevTR))
		plusDI = float64(100.0 * (prevPlusDM / prevTR))
		tempReal = minusDI + plusDI
		if !isZero(tempReal) {
			outReal[0] = float64(100.0 * (math.Abs(minusDI-plusDI) / tempReal))
		} else {
			outReal[0] = 0.0
		}
	} else {
		outReal[0] = 0.0
	}
	outIdx := 1

	for today < endIdx {
		today++
		tempReal = inHigh[today]
		diffP = tempReal - prevHigh
		prevHigh = tempReal
		tempReal = inLow[today]
		diffM = prevLow - tempReal
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
			minusDI = float64(100.0 * (prevMinusDM / prevTR))
			plusDI = float64(100.0 * (prevPlusDM / prevTR))
			tempReal = minusDI + plusDI
			if !isZero(tempReal) {
				outReal[outIdx] = float64(100.0 * (math.Abs(minusDI-plusDI) / tempReal))
			} else {
				outReal[outIdx] = outReal[outIdx-1]
			}
		} else {
			outReal[outIdx] = outReal[outIdx-1]
		}
		outIdx++
	}
	return outBegIdx, outIdx, nil
}
