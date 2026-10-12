package ta

import "math"

// VHF — Vertical Horizontal Filter (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/vhf and src/ta_func/ta_VHF.c.

// VHFLookback returns the number of leading bars VHF leaves empty.
// optInTimePeriod: [2, 100000], default 28.
func VHFLookback(optInTimePeriod int) int {
	if checkInt("VHF", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return vhfLookback(optInTimePeriod)
}

// vhfLookback is unvalidated. The optInTimePeriod changes reach one bar
// further back than the extrema window, so the lookback is the period itself.
func vhfLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// VHF returns the Vertical Horizontal Filter of inReal: the range of the
// optInTimePeriod newest values divided by the sum of the optInTimePeriod
// absolute changes ending at the current bar (0 on an exactly flat window).
// The output is index-aligned with inReal; the first VHFLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 28.
func VHF(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("VHF", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("VHF", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := vhfRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// vhfRange mirrors TA_VHF: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx.
func vhfRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("VHF", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VHF", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VHF", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := vhfLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	outIdx := 0
	today := startIdx
	for today <= endIdx {
		highest := inReal[today]
		lowest := highest
		prev := highest
		sumChange := 0.0
		for i := optInTimePeriod; i >= 0; i-- {
			tempReal := inReal[today-i]
			if i < optInTimePeriod {
				sumChange += math.Abs(tempReal - prev)
				if tempReal > highest {
					highest = tempReal
				}
				if tempReal < lowest {
					lowest = tempReal
				}
			}
			prev = tempReal
		}
		// Exact-zero guard (not an epsilon band), as in C.
		if sumChange > 0.0 {
			outReal[outIdx] = (highest - lowest) / sumChange
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
