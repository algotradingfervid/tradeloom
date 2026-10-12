package ta

import "math"

// AVGDEV — Average Deviation (Price Transform).
// Ported from TA-Lib v0.8.1 ta_codegen/input/avgdev and src/ta_func/ta_AVGDEV.c.

// AVGDEVLookback returns the number of leading bars AVGDEV leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func AVGDEVLookback(optInTimePeriod int) int {
	if checkInt("AVGDEV", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return avgdevLookback(optInTimePeriod)
}

func avgdevLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// AVGDEV returns the mean absolute deviation of inReal from its own simple
// moving average over optInTimePeriod bars. The output is index-aligned with
// inReal; the first AVGDEVLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func AVGDEV(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("AVGDEV", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("AVGDEV", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := avgdevRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// avgdevRange mirrors TA_AVGDEV: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func avgdevRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("AVGDEV", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AVGDEV", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AVGDEV", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookback := optInTimePeriod - 1
	if startIdx < lookback {
		startIdx = lookback
	}
	today := startIdx
	if today > endIdx {
		return 0, 0, nil
	}
	outIdx := 0
	for today <= endIdx {
		todaySum := 0.0
		for i := 0; i < optInTimePeriod; i++ {
			todaySum += inReal[today-i]
		}
		todayDev := 0.0
		for i := 0; i < optInTimePeriod; i++ {
			todayDev += math.Abs(inReal[today-i] - todaySum/float64(optInTimePeriod))
		}
		outReal[outIdx] = todayDev / float64(optInTimePeriod)
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
