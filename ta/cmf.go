package ta

// CMF — Chaikin Money Flow (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cmf and src/ta_func/ta_CMF.c.

// CMFLookback returns the number of leading bars CMF leaves empty, or -1 when
// optInTimePeriod is out of range.
// optInTimePeriod: [2, 100000], default 20.
func CMFLookback(optInTimePeriod int) int {
	if checkInt("CMF", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return cmfLookback(optInTimePeriod)
}

func cmfLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// CMF returns the Chaikin Money Flow: over a trailing window of
// optInTimePeriod bars, the sum of each bar's money flow volume
// ((close-low)-(high-close))/(high-low)*volume (0 when high-low <= 0) divided
// by the sum of volume; 0 when the window's volume sum is not positive.
// The output is index-aligned with the inputs; the first CMFLookback bars are
// NaN. optInTimePeriod: [2, 100000], default 20.
func CMF(inHigh, inLow, inClose, inVolume []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("CMF", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("CMF", []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cmfRange(0, n-1, inHigh, inLow, inClose, inVolume, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// cmfRange mirrors TA_CMF: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias any of the inputs: each bar's flow and volume
// are kept in circular buffers, so a consumed input bar is never re-read.
func cmfRange(startIdx, endIdx int, inHigh, inLow, inClose, inVolume []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("CMF", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CMF", endIdx, []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CMF", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// CIRCBUF_PROLOG/INIT for the (flow, volume) pairs.
	mfvFlow := make([]float64, optInTimePeriod)
	mfvVolume := make([]float64, optInTimePeriod)
	maxIdxMfv := optInTimePeriod - 1
	mfvIdx := 0
	outIdx := 0

	today := startIdx - lookbackTotal
	sumMFV := 0.0
	sumVol := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		high := inHigh[today]
		low := inLow[today]
		close := inClose[today]
		tmp := high - low
		var mfv float64
		if tmp > 0.0 {
			mfv = (close - low - (high - close)) / tmp * inVolume[today]
		} else {
			mfv = 0.0
		}
		mfvFlow[mfvIdx] = mfv
		mfvVolume[mfvIdx] = inVolume[today]
		sumMFV += mfv
		sumVol += inVolume[today]
		today++
		mfvIdx++
		if mfvIdx > maxIdxMfv {
			mfvIdx = 0
		}
	}

	if sumVol > 0.0 {
		outReal[outIdx] = sumMFV / sumVol
	} else {
		outReal[outIdx] = 0.0
	}
	outIdx++

	for today <= endIdx {
		sumMFV -= mfvFlow[mfvIdx]
		sumVol -= mfvVolume[mfvIdx]
		high := inHigh[today]
		low := inLow[today]
		close := inClose[today]
		tmp := high - low
		var mfv float64
		if tmp > 0.0 {
			mfv = (close - low - (high - close)) / tmp * inVolume[today]
		} else {
			mfv = 0.0
		}
		mfvFlow[mfvIdx] = mfv
		mfvVolume[mfvIdx] = inVolume[today]
		sumMFV += mfv
		sumVol += inVolume[today]
		today++
		if sumVol > 0.0 {
			outReal[outIdx] = sumMFV / sumVol
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
		mfvIdx++
		if mfvIdx > maxIdxMfv {
			mfvIdx = 0
		}
	}
	return startIdx, outIdx, nil
}
