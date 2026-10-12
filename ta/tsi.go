package ta

import "math"

// TSI — True Strength Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/tsi and src/ta_func/ta_TSI.c.

// TSILookback returns the number of leading bars TSI leaves empty:
// 1 + EMALookback(optInFirstPeriod) + EMALookback(optInSecondPeriod), so it
// includes the EMA unstable period (UnstEMA) twice.
// optInFirstPeriod: [2, 100000], default 25.
// optInSecondPeriod: [2, 100000], default 13.
func TSILookback(optInFirstPeriod, optInSecondPeriod int) int {
	if checkInt("TSI", "optInFirstPeriod", optInFirstPeriod, 2, 100000) != nil {
		return -1
	}
	if checkInt("TSI", "optInSecondPeriod", optInSecondPeriod, 2, 100000) != nil {
		return -1
	}
	return tsiLookback(optInFirstPeriod, optInSecondPeriod)
}

func tsiLookback(optInFirstPeriod, optInSecondPeriod int) int {
	return 1 + emaLookback(optInFirstPeriod) + emaLookback(optInSecondPeriod)
}

// TSI returns Blau's True Strength Index of inReal:
// 100 * EMA(EMA(m, first), second) / EMA(EMA(|m|, first), second), with
// m the one-bar change; 0 where the denominator is exactly zero. The output
// is index-aligned with inReal; the first TSILookback bars are NaN.
// optInFirstPeriod: [2, 100000], default 25.
// optInSecondPeriod: [2, 100000], default 13.
func TSI(inReal []float64, optInFirstPeriod, optInSecondPeriod int) ([]float64, error) {
	if err := checkInt("TSI", "optInFirstPeriod", optInFirstPeriod, 2, 100000); err != nil {
		return nil, err
	}
	if err := checkInt("TSI", "optInSecondPeriod", optInSecondPeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("TSI", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := tsiRange(0, n-1, inReal, optInFirstPeriod, optInSecondPeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// tsiRange mirrors TA_TSI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
//
// TA_TSI does not call TA_EMA; it inlines both EMA stages (seeded with a
// simple average, recurrence fma(x-prev, k, prev)) and only uses
// TA_EMA_Lookback, so this port uses emaLookback rather than emaRange.
func tsiRange(startIdx, endIdx int, inReal []float64, optInFirstPeriod, optInSecondPeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("TSI", "optInFirstPeriod", optInFirstPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("TSI", "optInSecondPeriod", optInSecondPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TSI", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("TSI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := tsiLookback(optInFirstPeriod, optInSecondPeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	outBegIdx = startIdx

	kFirst := 2.0 / float64(optInFirstPeriod+1)
	kSecond := 2.0 / float64(optInSecondPeriod+1)
	lookbackFirst := emaLookback(optInFirstPeriod)
	emaFirstNum := 0.0
	emaFirstDen := 0.0
	emaSecondNum := 0.0
	emaSecondDen := 0.0
	sumFirstNum := 0.0
	sumFirstDen := 0.0
	sumSecondNum := 0.0
	sumSecondDen := 0.0

	// The first bar carrying a close-to-close change.
	today := startIdx - lookbackTotal + 1
	prevClose := inReal[today-1]
	nBar := 0

	// Warm-up, through startIdx inclusive.
	for today <= startIdx {
		mom := inReal[today] - prevClose
		prevClose = inReal[today]
		absMom := math.Abs(mom)
		if nBar < optInFirstPeriod {
			sumFirstNum = sumFirstNum + mom
			sumFirstDen = sumFirstDen + absMom
			if nBar == optInFirstPeriod-1 {
				emaFirstNum = sumFirstNum / float64(optInFirstPeriod)
				emaFirstDen = sumFirstDen / float64(optInFirstPeriod)
			}
		} else {
			emaFirstNum = math.FMA(mom-emaFirstNum, kFirst, emaFirstNum)
			emaFirstDen = math.FMA(absMom-emaFirstDen, kFirst, emaFirstDen)
		}
		if nBar >= lookbackFirst {
			nSecond := nBar - lookbackFirst
			if nSecond < optInSecondPeriod {
				sumSecondNum = sumSecondNum + emaFirstNum
				sumSecondDen = sumSecondDen + emaFirstDen
				if nSecond == optInSecondPeriod-1 {
					emaSecondNum = sumSecondNum / float64(optInSecondPeriod)
					emaSecondDen = sumSecondDen / float64(optInSecondPeriod)
				}
			} else {
				emaSecondNum = math.FMA(emaFirstNum-emaSecondNum, kSecond, emaSecondNum)
				emaSecondDen = math.FMA(emaFirstDen-emaSecondDen, kSecond, emaSecondDen)
			}
		}
		nBar = nBar + 1
		today = today + 1
	}

	outReal[0] = tsiValue(emaSecondNum, emaSecondDen)
	outIdx := 1

	// Stable zone: both stages are a pure recursion.
	for today <= endIdx {
		mom := inReal[today] - prevClose
		prevClose = inReal[today]
		absMom := math.Abs(mom)
		emaFirstNum = math.FMA(mom-emaFirstNum, kFirst, emaFirstNum)
		emaFirstDen = math.FMA(absMom-emaFirstDen, kFirst, emaFirstDen)
		emaSecondNum = math.FMA(emaFirstNum-emaSecondNum, kSecond, emaSecondNum)
		emaSecondDen = math.FMA(emaFirstDen-emaSecondDen, kSecond, emaSecondDen)
		outReal[outIdx] = tsiValue(emaSecondNum, emaSecondDen)
		outIdx = outIdx + 1
		today = today + 1
	}
	return outBegIdx, outIdx, nil
}

// tsiValue is the guarded ratio: 0/0 (every change exactly zero) reports 0.
func tsiValue(num, den float64) float64 {
	if den > 0.0 {
		return 100.0 * num / den
	}
	return 0.0
}
