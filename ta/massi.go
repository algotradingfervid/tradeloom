package ta

import "math"

// MASSI — Mass Index (Volatility Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/massi and src/ta_func/ta_MASSI.c.

// MASSILookback returns the number of leading bars MASSI leaves empty:
// twice the EMA lookback of optInFastPeriod (so the EMA unstable period counts
// twice) plus optInSlowPeriod-1.
// optInFastPeriod: [2, 100000], default 9. optInSlowPeriod: [2, 100000], default 25.
func MASSILookback(optInFastPeriod, optInSlowPeriod int) int {
	if checkInt("MASSI", "optInFastPeriod", optInFastPeriod, 2, 100000) != nil {
		return -1
	}
	if checkInt("MASSI", "optInSlowPeriod", optInSlowPeriod, 2, 100000) != nil {
		return -1
	}
	return massiLookback(optInFastPeriod, optInSlowPeriod)
}

func massiLookback(optInFastPeriod, optInSlowPeriod int) int {
	return emaLookback(optInFastPeriod)*2 + (optInSlowPeriod - 1)
}

// MASSI returns Dorsey's Mass Index: the sum over optInSlowPeriod bars of
// EMA(high-low)/EMA(EMA(high-low)), both EMAs of length optInFastPeriod. A
// zero double EMA (flat market) contributes a ratio of 1. The output is
// index-aligned with the inputs; the first MASSILookback bars are NaN.
// optInFastPeriod: [2, 100000], default 9. optInSlowPeriod: [2, 100000], default 25.
func MASSI(inHigh, inLow []float64, optInFastPeriod, optInSlowPeriod int) ([]float64, error) {
	if err := checkInt("MASSI", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return nil, err
	}
	if err := checkInt("MASSI", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MASSI", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := massiRange(0, n-1, inHigh, inLow, optInFastPeriod, optInSlowPeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// massiRange mirrors TA_MASSI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias an input slice.
func massiRange(startIdx, endIdx int, inHigh, inLow []float64, optInFastPeriod, optInSlowPeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MASSI", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MASSI", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MASSI", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MASSI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackEma := emaLookback(optInFastPeriod)
	lookbackEma2 := lookbackEma * 2
	lookbackTotal := lookbackEma2 + (optInSlowPeriod - 1)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	ratioRing := make([]float64, optInSlowPeriod)
	maxIdxRatioRing := optInSlowPeriod - 1
	ratioRingIdx := 0
	outBegIdx = startIdx

	// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
	optInK1 := 2.0 / float64(optInFastPeriod+1)
	ema1 := 0.0
	ema2 := 0.0
	sum1 := 0.0
	sum2 := 0.0
	total := 0.0
	tempReal := 0.0
	var ratio float64
	today := startIdx - lookbackTotal
	nBar := 0
	for today <= startIdx {
		hl := inHigh[today] - inLow[today]
		if nBar < optInFastPeriod {
			sum1 = sum1 + hl
			if nBar == optInFastPeriod-1 {
				ema1 = sum1 / float64(optInFastPeriod)
			}
		} else {
			ema1 = math.FMA(hl-ema1, optInK1, ema1)
		}
		if nBar >= lookbackEma {
			n2 := nBar - lookbackEma
			if n2 < optInFastPeriod {
				sum2 = sum2 + ema1
				if n2 == optInFastPeriod-1 {
					ema2 = sum2 / float64(optInFastPeriod)
				}
			} else {
				ema2 = math.FMA(ema1-ema2, optInK1, ema2)
			}
		}
		if nBar >= lookbackEma2 {
			// Exact zero test, as in C (no epsilon band).
			if ema2 == 0.0 {
				ratio = 1.0
			} else {
				ratio = ema1 / ema2
			}
			ratioRing[ratioRingIdx] = ratio
			total = total + ratio
			ratioRingIdx++
			if ratioRingIdx > maxIdxRatioRing {
				ratioRingIdx = 0
			}
			if nBar == lookbackTotal {
				tempReal = total
				total = total - ratioRing[ratioRingIdx]
			}
		}
		nBar++
		today++
	}
	outReal[0] = tempReal
	outIdx := 1
	for today <= endIdx {
		hl := inHigh[today] - inLow[today]
		ema1 = math.FMA(hl-ema1, optInK1, ema1)
		ema2 = math.FMA(ema1-ema2, optInK1, ema2)
		if ema2 == 0.0 {
			ratio = 1.0
		} else {
			ratio = ema1 / ema2
		}
		ratioRing[ratioRingIdx] = ratio
		total = total + ratio
		ratioRingIdx++
		if ratioRingIdx > maxIdxRatioRing {
			ratioRingIdx = 0
		}
		tempReal = total
		total = total - ratioRing[ratioRingIdx]
		outReal[outIdx] = tempReal
		outIdx++
		today++
	}
	return outBegIdx, outIdx, nil
}
