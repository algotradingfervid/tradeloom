package ta

import "math"

// CVI — Chaikin's Volatility (Volatility Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cvi and src/ta_func/ta_CVI.c.
//
// TA_CVI is a fused pass of TA_SUB(high, low), TA_EMA and TA_ROCP*100: the
// spread is never materialised and a ring of the last optInROCPeriod EMA
// values supplies the lagged term. The port keeps the C's seed-sum order, its
// explicit fma() for the EMA step (math.FMA here), and the exact `!= 0.0`
// zero guard, so it is bit-identical to composing emaRange and rocpRange.

// CVILookback returns the number of leading bars CVI leaves empty:
// the EMA lookback (optInTimePeriod-1 plus UnstablePeriod(UnstEMA)) plus
// optInROCPeriod.
// optInTimePeriod: [2, 100000], default 10. optInROCPeriod: [1, 100000], default 10.
func CVILookback(optInTimePeriod, optInROCPeriod int) int {
	if checkInt("CVI", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	if checkInt("CVI", "optInROCPeriod", optInROCPeriod, 1, 100000) != nil {
		return -1
	}
	return cviLookback(optInTimePeriod, optInROCPeriod)
}

func cviLookback(optInTimePeriod, optInROCPeriod int) int {
	return emaLookback(optInTimePeriod) + rocpLookback(optInROCPeriod)
}

// CVI returns 100*(E[t]-E[t-optInROCPeriod])/E[t-optInROCPeriod], where E is
// the optInTimePeriod EMA of (high - low); 0 where the lagged EMA is exactly 0.
// The output is index-aligned with the inputs; the first CVILookback bars are
// NaN.
// optInTimePeriod: [2, 100000], default 10. optInROCPeriod: [1, 100000], default 10.
func CVI(inHigh, inLow []float64, optInTimePeriod, optInROCPeriod int) ([]float64, error) {
	if err := checkInt("CVI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	if err := checkInt("CVI", "optInROCPeriod", optInROCPeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("CVI", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cviRange(0, n-1, inHigh, inLow, optInTimePeriod, optInROCPeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// cviRange mirrors TA_CVI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be the same slice as inHigh or inLow (each input bar
// is read before the output slot at or below it is written).
func cviRange(startIdx, endIdx int, inHigh, inLow []float64, optInTimePeriod, optInROCPeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("CVI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("CVI", "optInROCPeriod", optInROCPeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CVI", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CVI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := cviLookback(optInTimePeriod, optInROCPeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// CIRCBUF of optInROCPeriod EMA values.
	emaRing := make([]float64, optInROCPeriod)
	maxIdxEmaRing := optInROCPeriod - 1
	emaRingIdx := 0

	optInK1 := 2.0 / float64(optInTimePeriod+1)
	today := startIdx - lookbackTotal
	tempReal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		tempReal += inHigh[today] - inLow[today]
		today++
	}
	prevEMA := tempReal / float64(optInTimePeriod)

	emaRing[emaRingIdx] = prevEMA
	emaRingIdx++
	if emaRingIdx > maxIdxEmaRing {
		emaRingIdx = 0
	}
	for today < startIdx {
		tempReal = inHigh[today] - inLow[today]
		// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
		prevEMA = math.FMA(tempReal-prevEMA, optInK1, prevEMA)
		today++
		emaRing[emaRingIdx] = prevEMA
		emaRingIdx++
		if emaRingIdx > maxIdxEmaRing {
			emaRingIdx = 0
		}
	}

	// Read the expiring slot before overwriting it: the lag is exactly
	// optInROCPeriod.
	outIdx := 0
	for today <= endIdx {
		tempReal = inHigh[today] - inLow[today]
		prevEMA = math.FMA(tempReal-prevEMA, optInK1, prevEMA)
		today++
		laggedEMA := emaRing[emaRingIdx]
		emaRing[emaRingIdx] = prevEMA
		emaRingIdx++
		if emaRingIdx > maxIdxEmaRing {
			emaRingIdx = 0
		}
		if laggedEMA != 0.0 {
			outReal[outIdx] = 100.0 * ((prevEMA - laggedEMA) / laggedEMA)
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
	}
	return startIdx, outIdx, nil
}
