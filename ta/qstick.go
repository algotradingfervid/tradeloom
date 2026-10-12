package ta

// QSTICK — Qstick (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/qstick and src/ta_func/ta_QSTICK.c.

// QSTICKLookback returns the number of leading bars QSTICK leaves empty.
// optInTimePeriod: [1, 100000], default 10.
func QSTICKLookback(optInTimePeriod int) int {
	if checkInt("QSTICK", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return qstickLookback(optInTimePeriod)
}

func qstickLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// QSTICK returns the simple moving average of the candle body
// (inClose - inOpen) over optInTimePeriod bars. The output is index-aligned
// with the inputs; the first QSTICKLookback bars are NaN.
// optInTimePeriod: [1, 100000], default 10.
func QSTICK(inOpen, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("QSTICK", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("QSTICK", []string{"inOpen", "inClose"}, inOpen, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := qstickRange(0, n-1, inOpen, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// qstickRange mirrors TA_QSTICK: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be the same slice as either input.
func qstickRange(startIdx, endIdx int, inOpen, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("QSTICK", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("QSTICK", endIdx, []string{"inOpen", "inClose"}, inOpen, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("QSTICK", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
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
			periodTotal += inClose[i] - inOpen[i]
			i++
		}
	}

	outIdx := 0
	for i <= endIdx {
		periodTotal += inClose[i] - inOpen[i]
		i++
		tempReal := periodTotal
		periodTotal -= inClose[trailingIdx] - inOpen[trailingIdx]
		trailingIdx++
		outReal[outIdx] = tempReal / float64(optInTimePeriod)
		outIdx++
	}
	return startIdx, outIdx, nil
}
