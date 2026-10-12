package ta

// ADR — Average Day Range (Volatility Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/adr and src/ta_func/ta_ADR.c.
//
// TA_ADR is the TA_SMA loop with inReal[x] replaced by (inHigh[x] - inLow[x]):
// the same running-sum order, the same read-before-write of the trailing term
// and the same final divide by the period. The port keeps all three.

// ADRLookback returns the number of leading bars ADR leaves empty.
// optInTimePeriod: [1, 100000], default 14.
func ADRLookback(optInTimePeriod int) int {
	if checkInt("ADR", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return adrLookback(optInTimePeriod)
}

func adrLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// ADR returns the simple moving average of (high - low) over optInTimePeriod
// bars. The output is index-aligned with the inputs; the first ADRLookback
// bars are NaN.
// optInTimePeriod: [1, 100000], default 14.
func ADR(inHigh, inLow []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ADR", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ADR", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := adrRange(0, n-1, inHigh, inLow, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// adrRange mirrors TA_ADR: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be the same slice as inHigh or inLow.
func adrRange(startIdx, endIdx int, inHigh, inLow []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ADR", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADR", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADR", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	periodTotal := 0.0
	trailingIdx := startIdx - lookbackTotal
	i := trailingIdx
	if optInTimePeriod > 1 {
		for i < startIdx {
			periodTotal += inHigh[i] - inLow[i]
			i++
		}
	}

	outIdx := 0
	for i <= endIdx {
		periodTotal += inHigh[i] - inLow[i]
		i++
		tempReal := periodTotal
		periodTotal -= inHigh[trailingIdx] - inLow[trailingIdx]
		trailingIdx++
		outReal[outIdx] = tempReal / float64(optInTimePeriod)
		outIdx++
	}
	return startIdx, outIdx, nil
}
