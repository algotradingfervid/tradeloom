package ta

import "math"

// KAMA — Kaufman Adaptive Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/kama and src/ta_func/ta_KAMA.c.

// KAMALookback returns the number of leading bars KAMA leaves empty,
// including the KAMA unstable period. Returns -1 on an invalid parameter.
// optInTimePeriod: [1, 100000], default 30.
func KAMALookback(optInTimePeriod int) int {
	if checkInt("KAMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return kamaLookback(optInTimePeriod)
}

func kamaLookback(optInTimePeriod int) int {
	if optInTimePeriod == 1 {
		return UnstablePeriod(UnstKAMA)
	}
	return optInTimePeriod + UnstablePeriod(UnstKAMA)
}

// KAMA returns the Kaufman Adaptive Moving Average of inReal.
// The output is index-aligned with inReal; the first KAMALookback bars are NaN.
// Period 1 is the identity (after the unstable period).
// optInTimePeriod: [1, 100000], default 30.
func KAMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("KAMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("KAMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := kamaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// kamaRange mirrors TA_KAMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func kamaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("KAMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("KAMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("KAMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Computed at run time (not as Go constants) so constDiff gets C's two
	// roundings: round(2/3) - round(2/31).
	thirty, two := 30.0, 2.0
	constMax := 2.0 / (thirty + 1.0)
	constDiff := 2.0/(two+1.0) - constMax

	unstable := UnstablePeriod(UnstKAMA)

	// No smoothing at period 1: the output is a copy of the input, delayed
	// by the unstable period.
	if optInTimePeriod == 1 {
		lookbackTotal := unstable
		if startIdx < lookbackTotal {
			startIdx = lookbackTotal
		}
		if startIdx > endIdx {
			return 0, 0, nil
		}
		outIdx := 0
		for today := startIdx; today <= endIdx; today++ {
			outReal[outIdx] = inReal[today]
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	lookbackTotal := optInTimePeriod + unstable
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	sumROC1 := 0.0
	nullRun := 0
	today := startIdx - lookbackTotal
	trailingIdx := today
	for i := optInTimePeriod; i > 0; i-- {
		tempReal := inReal[today]
		today++
		tempReal -= inReal[today]
		sumROC1 += math.Abs(tempReal)
		if tempReal == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
	}

	// The yesterday price is used as the previous KAMA.
	prevKAMA := inReal[today-1]
	tempReal := inReal[today]
	tempReal2 := inReal[trailingIdx]
	trailingIdx++
	periodROC := tempReal - tempReal2
	trailingValue := tempReal2

	tempReal = kamaEfficiency(sumROC1, periodROC)
	tempReal = math.FMA(tempReal, constDiff, constMax)
	tempReal *= tempReal
	prevKAMA = math.FMA(inReal[today]-prevKAMA, tempReal, prevKAMA)
	today++

	// step advances one bar (shared by the unstable-period skip and the
	// output loop, which are identical in C apart from the write).
	step := func() {
		tempReal := inReal[today]
		tempReal2 := inReal[trailingIdx]
		trailingIdx++
		periodROC := tempReal - tempReal2
		sumROC1 -= math.Abs(trailingValue - tempReal2)
		sumROC1 += math.Abs(tempReal - inReal[today-1])
		if tempReal-inReal[today-1] == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
		if nullRun >= optInTimePeriod {
			nullRun = optInTimePeriod
			sumROC1 = 0.0
		}
		trailingValue = tempReal2
		tempReal = kamaEfficiency(sumROC1, periodROC)
		tempReal = math.FMA(tempReal, constDiff, constMax)
		tempReal *= tempReal
		prevKAMA = math.FMA(inReal[today]-prevKAMA, tempReal, prevKAMA)
		today++
	}

	for today <= startIdx {
		step()
	}

	outReal[0] = prevKAMA
	outIdx := 1
	outBegIdx = today - 1

	for today <= endIdx {
		step()
		outReal[outIdx] = prevKAMA
		outIdx++
	}
	return outBegIdx, outIdx, nil
}

// kamaEfficiency is the clamped efficiency ratio |periodROC / sumROC1|.
func kamaEfficiency(sumROC1, periodROC float64) float64 {
	if sumROC1 <= 0.0 || sumROC1 <= periodROC {
		return 1.0
	}
	r := math.Abs(periodROC / sumROC1)
	if r > 1.0 {
		r = 1.0
	}
	return r
}
