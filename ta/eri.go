package ta

import "math"

// ERI — Elder Ray Index, Bull Power / Bear Power (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/eri and src/ta_func/ta_ERI.c.
//
// TA_ERI does not call TA_EMA: it fuses an op-for-op copy of the EMA
// (seed sum, one divide, fma recursion) with the per-bar differences, so the
// port fuses it the same way. Its lookback is TA_EMA_Lookback.

// ERILookback returns the number of leading bars ERI leaves empty:
// optInTimePeriod-1 plus the EMA unstable period (UnstEMA).
// optInTimePeriod: [1, 100000], default 13.
func ERILookback(optInTimePeriod int) int {
	if checkInt("ERI", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return eriLookback(optInTimePeriod)
}

func eriLookback(optInTimePeriod int) int {
	return emaLookback(optInTimePeriod)
}

// ERI returns Bull Power (inHigh - EMA(inClose)) and Bear Power
// (inLow - EMA(inClose)), both against the same EMA of optInTimePeriod bars.
// The outputs are index-aligned with the inputs; the first ERILookback bars
// are NaN. optInTimePeriod: [1, 100000], default 13.
func ERI(inHigh, inLow, inClose []float64, optInTimePeriod int) (outBullPower, outBearPower []float64, err error) {
	if err := checkInt("ERI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("ERI", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, err
	}
	outBullPower = make([]float64, n)
	outBearPower = make([]float64, n)
	if n == 0 {
		return outBullPower, outBearPower, nil
	}
	beg, nb, err := eriRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, outBullPower, outBearPower)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outBullPower, beg, nb)
	alignReal(outBearPower, beg, nb)
	return outBullPower, outBearPower, nil
}

// eriRange mirrors TA_ERI: it computes [startIdx, endIdx] and writes
// outNBElement values to outBullPower[0:] and outBearPower[0:], the first
// belonging to bar outBegIdx. An output may alias any input, but the two
// outputs must be distinct (as in C).
func eriRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outBullPower, outBearPower []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ERI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ERI", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ERI", endIdx-startIdx, []string{"outBullPower", "outBearPower"}, outBullPower, outBearPower); err != nil {
		return 0, 0, err
	}
	// C: if( outBullPower == outBearPower ) return TA_BAD_PARAM;
	if &outBullPower[0] == &outBearPower[0] {
		return 0, 0, badParam("ERI", "outBearPower", "must not be the same buffer as outBullPower")
	}

	lookbackTotal := eriLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Period 1: EMA's copy arm; the unstable period still delays the start.
	if optInTimePeriod == 1 {
		outIdx := 0
		for today := startIdx; today <= endIdx; today++ {
			tempHT := inHigh[today]
			tempLT := inLow[today]
			tempReal := inClose[today]
			outBullPower[outIdx] = tempHT - tempReal
			outBearPower[outIdx] = tempLT - tempReal
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	k := 2.0 / (float64(optInTimePeriod) + 1.0)
	today := startIdx - lookbackTotal
	tempReal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		tempReal += inClose[today]
		today++
	}
	prevMA := tempReal / float64(optInTimePeriod)
	// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
	for today <= startIdx {
		prevMA = math.FMA(inClose[today]-prevMA, k, prevMA)
		today++
	}
	// Load both extremes before writing either output (outputs may alias inputs).
	tempHT := inHigh[startIdx]
	tempLT := inLow[startIdx]
	outBullPower[0] = tempHT - prevMA
	outBearPower[0] = tempLT - prevMA
	outIdx := 1
	for today <= endIdx {
		prevMA = math.FMA(inClose[today]-prevMA, k, prevMA)
		tempHT = inHigh[today]
		tempLT = inLow[today]
		outBullPower[outIdx] = tempHT - prevMA
		outBearPower[outIdx] = tempLT - prevMA
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
