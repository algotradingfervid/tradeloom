package ta

import "math"

// VORTEX — Vortex Indicator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/vortex and src/ta_func/ta_VORTEX.c.

// VORTEXLookback returns the number of leading bars VORTEX leaves empty.
// optInTimePeriod: [1, 100000], default 14.
func VORTEXLookback(optInTimePeriod int) int {
	if checkInt("VORTEX", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return vortexLookback(optInTimePeriod)
}

// vortexLookback: every per-bar term needs the prior bar, so the first
// output sits at index optInTimePeriod.
func vortexLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// VORTEX returns the positive (+VI) and negative (-VI) vortex lines:
// SUM(|H-prevL|, n)/SUM(TR, n) and SUM(|L-prevH|, n)/SUM(TR, n). A window
// whose summed true range is not positive yields 0 on both lines.
// Outputs are index-aligned with the inputs; the first VORTEXLookback bars
// are NaN. optInTimePeriod: [1, 100000], default 14.
func VORTEX(inHigh, inLow, inClose []float64, optInTimePeriod int) (outPlusVI, outMinusVI []float64, err error) {
	if err := checkInt("VORTEX", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("VORTEX", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, err
	}
	outPlusVI = make([]float64, n)
	outMinusVI = make([]float64, n)
	if n == 0 {
		return outPlusVI, outMinusVI, nil
	}
	beg, nb, err := vortexRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, outPlusVI, outMinusVI)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outPlusVI, beg, nb)
	alignReal(outMinusVI, beg, nb)
	return outPlusVI, outMinusVI, nil
}

// vortexRange mirrors TA_VORTEX: it computes [startIdx, endIdx] and writes
// outNBElement values to outPlusVI[0:] and outMinusVI[0:], the first one
// belonging to bar outBegIdx. The outputs may alias any input (but not each
// other, which C rejects).
func vortexRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outPlusVI, outMinusVI []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("VORTEX", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VORTEX", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VORTEX", endIdx-startIdx, []string{"outPlusVI", "outMinusVI"}, outPlusVI, outMinusVI); err != nil {
		return 0, 0, err
	}
	// C: if( outPlusVI == outMinusVI ) return TA_BAD_PARAM;
	if &outPlusVI[0] == &outMinusVI[0] {
		return 0, 0, badParam("VORTEX", "outMinusVI", "must not alias outPlusVI")
	}

	lookbackTotal := vortexLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Prime the window sums over the optInTimePeriod-1 terms before the
	// first output bar. trueRange is TA_TRANGE's operation order, which is
	// the inline expansion in ta_VORTEX.c.
	sTR := 0.0
	sVMP := 0.0
	sVMM := 0.0
	nullRun := 0
	for i := startIdx - optInTimePeriod + 1; i < startIdx; i++ {
		tr := trueRange(inHigh[i], inLow[i], inClose[i-1])
		sTR += tr
		sVMP += math.Abs(inHigh[i] - inLow[i-1])
		sVMM += math.Abs(inLow[i] - inHigh[i-1])
		if tr == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
	}

	outIdx := 0
	today := startIdx
	trailingIdx := startIdx - optInTimePeriod + 1
	for today <= endIdx {
		// Add on today's terms.
		tr := trueRange(inHigh[today], inLow[today], inClose[today-1])
		sTR += tr
		sVMP += math.Abs(inHigh[today] - inLow[today-1])
		sVMM += math.Abs(inLow[today] - inHigh[today-1])
		if tr == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
		// A fully flat window: purge sTR's rounding residue (only sTR).
		if nullRun >= optInTimePeriod {
			nullRun = optInTimePeriod
			sTR = 0.0
		}
		curTR := sTR
		curVMP := sVMP
		curVMM := sVMM
		// Retire the trailing bar's terms.
		tr = trueRange(inHigh[trailingIdx], inLow[trailingIdx], inClose[trailingIdx-1])
		sTR -= tr
		sVMP -= math.Abs(inHigh[trailingIdx] - inLow[trailingIdx-1])
		sVMM -= math.Abs(inLow[trailingIdx] - inHigh[trailingIdx-1])
		trailingIdx++
		// Outputs are written last so they may alias the inputs.
		if curTR > 0.0 {
			outPlusVI[outIdx] = curVMP / curTR
			outMinusVI[outIdx] = curVMM / curTR
		} else {
			outPlusVI[outIdx] = 0.0
			outMinusVI[outIdx] = 0.0
		}
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
