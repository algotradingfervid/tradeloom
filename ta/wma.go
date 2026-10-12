package ta

// WMA — Weighted Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/wma and src/ta_func/ta_WMA.c.

// WMALookback returns the number of leading bars WMA leaves empty.
// optInTimePeriod: [1, 100000], default 30.
func WMALookback(optInTimePeriod int) int {
	if checkInt("WMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return wmaLookback(optInTimePeriod)
}

func wmaLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// WMA returns the linearly weighted moving average of inReal over
// optInTimePeriod bars (oldest weight 1, newest weight optInTimePeriod).
// The output is index-aligned with inReal; the first WMALookback bars are NaN.
// optInTimePeriod: [1, 100000], default 30.
func WMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("WMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("WMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := wmaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// wmaRange mirrors TA_WMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func wmaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("WMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("WMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("WMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Period 1: outputs equal inputs (forward element copy keeps in-place safe).
	if optInTimePeriod == 1 {
		nb := endIdx - startIdx + 1
		inIdx := startIdx
		for i := 0; i < nb; i++ {
			outReal[i] = inReal[inIdx]
			inIdx++
		}
		return startIdx, nb, nil
	}

	// Computed in double, as C does, to avoid int overflow.
	divider := float64(optInTimePeriod) * float64(optInTimePeriod+1) / 2.0

	outIdx := 0
	trailingIdx := startIdx - lookbackTotal
	lookbackWin := optInTimePeriod - 1
	periodSub := 0.0
	periodSum := periodSub
	inIdx := trailingIdx
	i := 1
	for inIdx < startIdx {
		tempReal := inReal[inIdx]
		inIdx++
		periodSub += tempReal
		periodSum += float64(tempReal * float64(i))
		i++
	}
	barsSinceReseed := 8 * optInTimePeriod
	trailingValue := 0.0
	for inIdx <= endIdx {
		tempReal := inReal[inIdx]
		periodSub += tempReal
		periodSub -= trailingValue
		periodSum += float64(tempReal * float64(optInTimePeriod))

		// Periodic re-anchor (every 8*period bars): rebuild both totals from
		// the window, oldest first with weight counting up from 1.
		barsSinceReseed--
		if barsSinceReseed <= 0 {
			barsSinceReseed = 8 * optInTimePeriod
			periodSub = 0.0
			periodSum = 0.0
			rw := 1
			for j := inIdx - lookbackWin; j <= inIdx; j++ {
				tempReal = inReal[j]
				periodSub += tempReal
				periodSum += float64(tempReal * float64(rw))
				rw++
			}
		}

		// Saved before writing the output in case outReal aliases inReal.
		trailingValue = inReal[trailingIdx]
		trailingIdx++
		outReal[outIdx] = periodSum / divider
		outIdx++
		periodSum -= periodSub
		inIdx++
	}
	return startIdx, outIdx, nil
}
