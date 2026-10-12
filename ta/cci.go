package ta

import "math"

// CCI — Commodity Channel Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/cci and src/ta_func/ta_CCI.c.

// CCILookback returns the number of leading bars CCI leaves empty.
// optInTimePeriod: [2, 100000], default 14.
func CCILookback(optInTimePeriod int) int {
	if checkInt("CCI", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return cciLookback(optInTimePeriod)
}

func cciLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// CCI returns the Commodity Channel Index of (inHigh, inLow, inClose) over
// optInTimePeriod bars: (TP - SMA(TP)) / (0.015 * meanDev(TP)), where TP is
// the typical price. A flat window yields 0. The output is index-aligned with
// the inputs; the first CCILookback bars are NaN.
// optInTimePeriod: [2, 100000], default 14.
func CCI(inHigh, inLow, inClose []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("CCI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("CCI", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := cciRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// cciRange mirrors TA_CCI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func cciRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("CCI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CCI", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CCI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	circBuffer := make([]float64, optInTimePeriod)
	maxIdxCircBuffer := optInTimePeriod - 1
	circBufferIdx := 0

	i := startIdx - lookbackTotal
	if optInTimePeriod > 1 {
		for i < startIdx {
			circBuffer[circBufferIdx] = (inHigh[i] + inLow[i] + inClose[i]) / 3
			i++
			circBufferIdx++
			if circBufferIdx > maxIdxCircBuffer {
				circBufferIdx = 0
			}
		}
	}

	outIdx := 0
	for {
		lastValue := (inHigh[i] + inLow[i] + inClose[i]) / 3
		circBuffer[circBufferIdx] = lastValue

		theAverage := 0.0
		for j := 0; j < optInTimePeriod; j++ {
			theAverage += circBuffer[j]
		}
		theAverage /= float64(optInTimePeriod)

		tempReal2 := 0.0
		for j := 0; j < optInTimePeriod; j++ {
			tempReal2 += math.Abs(circBuffer[j] - theAverage)
		}
		tempReal2 /= float64(optInTimePeriod)

		tempReal := lastValue - theAverage
		tempReal3 := math.Abs(theAverage)
		// Explicit float64 conversion keeps the product rounded on its own
		// (no fusing), exactly as C evaluates it.
		divisor := float64(0.015 * tempReal2)
		if !isZeroScaled(tempReal, tempReal3) && !isZeroScaled(tempReal2, tempReal3) && divisor != 0.0 {
			outReal[outIdx] = tempReal / divisor
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++

		circBufferIdx++
		if circBufferIdx > maxIdxCircBuffer {
			circBufferIdx = 0
		}
		i++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
