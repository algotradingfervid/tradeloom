package ta

// SMA — Simple Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/sma and src/ta_func/ta_SMA.c.

// SMALookback returns the number of leading bars SMA leaves empty.
// optInTimePeriod: [1, 100000], default 30.
func SMALookback(optInTimePeriod int) int {
	if checkInt("SMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return smaLookback(optInTimePeriod)
}

func smaLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// SMA returns the simple moving average of inReal over optInTimePeriod bars.
// The output is index-aligned with inReal; the first SMALookback bars are NaN.
// optInTimePeriod: [1, 100000], default 30.
func SMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("SMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("SMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := smaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// smaRange mirrors TA_SMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func smaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("SMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return smaCore(startIdx, endIdx, inReal, optInTimePeriod, outReal)
}

// smaCore is the TA_INT_SMA body without validation; other indicators call it
// (via smaRange or directly when they have already validated).
func smaCore(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (int, int, error) {
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
			periodTotal += inReal[i]
			i++
		}
	}

	outIdx := 0
	for i <= endIdx {
		periodTotal += inReal[i]
		i++
		tempReal := periodTotal
		periodTotal -= inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = tempReal / float64(optInTimePeriod)
		outIdx++
	}
	return startIdx, outIdx, nil
}
