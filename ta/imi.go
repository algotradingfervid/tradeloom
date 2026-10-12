package ta

// IMI — Intraday Momentum Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/imi and src/ta_func/ta_IMI.c.

// IMILookback returns the number of leading bars IMI leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func IMILookback(optInTimePeriod int) int {
	if checkInt("IMI", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return imiLookback(optInTimePeriod)
}

func imiLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// IMI returns the Intraday Momentum Index: over each window of
// optInTimePeriod bars, 100 * upsum / (upsum + downsum), where upsum sums
// close-open on bars with close > open and downsum sums open-close on the
// others. A window with no body movement at all yields 50.
// The output is index-aligned with the inputs; the first IMILookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func IMI(inOpen, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("IMI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("IMI", []string{"inOpen", "inClose"}, inOpen, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := imiRange(0, n-1, inOpen, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// imiRange mirrors TA_IMI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar outBegIdx.
func imiRange(startIdx, endIdx int, inOpen, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("IMI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("IMI", endIdx, []string{"inOpen", "inClose"}, inOpen, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("IMI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	outIdx := 0
	lookback := imiLookback(optInTimePeriod)
	if startIdx < lookback {
		startIdx = lookback
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	outBegIdx = startIdx
	for startIdx <= endIdx {
		upsum := 0.0
		downsum := 0.0
		for i := startIdx - (optInTimePeriod - 1); i <= startIdx; i++ {
			close := inClose[i]
			open := inOpen[i]
			if close > open {
				upsum += close - open
			} else {
				downsum += open - close
			}
			// C writes the output inside the window loop (TA-Lib #112 guard
			// included); the last iteration's value is the one that remains.
			if upsum+downsum == 0.0 {
				outReal[outIdx] = 50.0
			} else {
				outReal[outIdx] = 100.0 * (upsum / (upsum + downsum))
			}
		}
		startIdx++
		outIdx++
	}
	return outBegIdx, outIdx, nil
}
