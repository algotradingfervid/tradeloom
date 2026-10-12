package ta

import "math"

// ER — Kaufman Efficiency Ratio (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/er and src/ta_func/ta_ER.c.

// ERLookback returns the number of leading bars ER leaves empty.
// optInTimePeriod: [2, 100000], default 10.
func ERLookback(optInTimePeriod int) int {
	if checkInt("ER", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return erLookback(optInTimePeriod)
}

// erLookback: P one-bar changes need P+1 prices, so the first output is at
// index P.
func erLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// ER returns |inReal[t]-inReal[t-P]| / sum of |one-bar changes| over the
// same P bars, in [0, 1]. A flat window reports 1.0.
// The output is index-aligned with inReal; the first ERLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 10.
func ER(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ER", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ER", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := erRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// erRange mirrors TA_ER: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func erRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ER", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ER", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ER", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := erLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Prime the path sum over the optInTimePeriod one-bar changes ending at
	// the first output bar.
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

	// First output: today == startIdx.
	tempReal := inReal[today]
	tempReal2 := inReal[trailingIdx]
	trailingIdx++
	periodROC := tempReal - tempReal2
	trailingValue := tempReal2
	if sumROC1 <= 0.0 || sumROC1 <= periodROC {
		outReal[0] = 1.0
	} else {
		tempReal = math.Abs(periodROC / sumROC1)
		if tempReal > 1.0 {
			tempReal = 1.0
		}
		outReal[0] = tempReal
	}
	outIdx := 1
	today++
	for today <= endIdx {
		tempReal = inReal[today]
		tempReal2 = inReal[trailingIdx]
		trailingIdx++
		periodROC = tempReal - tempReal2
		// Subtract-then-add, TA_SUM's own order.
		sumROC1 -= math.Abs(trailingValue - tempReal2)
		sumROC1 += math.Abs(tempReal - inReal[today-1])
		// A whole window of exactly-zero changes: purge the rounding residue.
		if tempReal-inReal[today-1] == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
		if nullRun >= optInTimePeriod {
			nullRun = optInTimePeriod
			sumROC1 = 0.0
		}
		// Cached one iteration ahead so outReal may alias inReal.
		trailingValue = tempReal2
		if sumROC1 <= 0.0 || sumROC1 <= periodROC {
			outReal[outIdx] = 1.0
		} else {
			tempReal = math.Abs(periodROC / sumROC1)
			if tempReal > 1.0 {
				tempReal = 1.0
			}
			outReal[outIdx] = tempReal
		}
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
