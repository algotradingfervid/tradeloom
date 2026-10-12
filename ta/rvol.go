package ta

// RVOL — Relative Volume (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/rvol and src/ta_func/ta_RVOL.c.

// RVOLLookback returns the number of leading bars RVOL leaves empty: one more
// than a moving average of the same period, since the current bar is excluded
// from its own baseline.
// optInTimePeriod: [1, 100000], default 20.
func RVOLLookback(optInTimePeriod int) int {
	if checkInt("RVOL", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return rvolLookback(optInTimePeriod)
}

func rvolLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// RVOL returns the relative volume of inVolume: each bar's volume divided by
// the mean volume of the optInTimePeriod bars before it. The output is
// index-aligned with inVolume; the first RVOLLookback bars are NaN. A window
// whose baseline is zero yields ±Inf (or NaN when the bar's volume is also
// zero), as in C.
// optInTimePeriod: [1, 100000], default 20.
func RVOL(inVolume []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("RVOL", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("RVOL", []string{"inVolume"}, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := rvolRange(0, n-1, inVolume, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// rvolRange mirrors TA_RVOL: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inVolume and outReal may be the same slice (each input bar is
// read before the output slot that could alias it is written).
func rvolRange(startIdx, endIdx int, inVolume []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("RVOL", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("RVOL", endIdx, []string{"inVolume"}, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("RVOL", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	periodTotal := 0.0
	trailingIdx := startIdx - lookbackTotal
	i := trailingIdx
	for i < startIdx {
		periodTotal += inVolume[i]
		i++
	}

	outIdx := 0
	for i <= endIdx {
		// Drop the trailing bar before adding today's (C order).
		baseline := periodTotal / float64(optInTimePeriod)
		periodTotal -= inVolume[trailingIdx]
		trailingIdx++
		todayVolume := inVolume[i]
		i++
		periodTotal += todayVolume
		outReal[outIdx] = todayVolume / baseline
		outIdx++
	}
	return startIdx, outIdx, nil
}
