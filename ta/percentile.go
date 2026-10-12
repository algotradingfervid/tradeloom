package ta

import "math"

// PERCENTILE — Percentile, nearest rank (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/percentile and src/ta_func/ta_PERCENTILE.c.

// PERCENTILELookback returns the number of leading bars PERCENTILE leaves empty.
// optInTimePeriod: [2, 100000], default 30. optInPercentile: [0, 100], default 50.
func PERCENTILELookback(optInTimePeriod int, optInPercentile float64) int {
	if checkInt("PERCENTILE", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	if checkReal("PERCENTILE", "optInPercentile", optInPercentile, 0, 100) != nil {
		return -1
	}
	return percentileLookback(optInTimePeriod, optInPercentile)
}

func percentileLookback(optInTimePeriod int, optInPercentile float64) int {
	_ = optInPercentile
	return optInTimePeriod - 1
}

// PERCENTILE returns the rolling nearest-rank optInPercentile-th percentile
// of inReal over a trailing window of optInTimePeriod bars: the window is
// sorted ascending and the value at 1-based rank ceil(P*N/100), clamped to
// [1, N], is reported. The output is index-aligned with inReal; the first
// PERCENTILELookback bars are NaN.
// optInTimePeriod: [2, 100000], default 30. optInPercentile: [0, 100], default 50.
func PERCENTILE(inReal []float64, optInTimePeriod int, optInPercentile float64) ([]float64, error) {
	if err := checkInt("PERCENTILE", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	if err := checkReal("PERCENTILE", "optInPercentile", optInPercentile, 0, 100); err != nil {
		return nil, err
	}
	n, err := checkInputs("PERCENTILE", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := percentileRange(0, n-1, inReal, optInTimePeriod, optInPercentile, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// percentileRange mirrors TA_PERCENTILE: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func percentileRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, optInPercentile float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("PERCENTILE", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkReal("PERCENTILE", "optInPercentile", optInPercentile, 0, 100); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PERCENTILE", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("PERCENTILE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// The window is carried twice: "ring" by age, "sorted" by value.
	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	ring := make([]float64, optInTimePeriod)
	maxIdxRing := optInTimePeriod - 1
	ringIdx := 0
	sorted := make([]float64, optInTimePeriod)

	// Multiply before dividing, as C does: (P*n)/100.
	rank := int(math.Ceil(optInPercentile * float64(optInTimePeriod) / 100.0))
	if rank < 1 {
		rank = 1
	}
	if rank > optInTimePeriod {
		rank = optInTimePeriod
	}

	nbSorted := 0
	i := startIdx - lookbackTotal
	for i < startIdx {
		newValue := inReal[i]
		j := nbSorted
		for j > 0 && sorted[j-1] > newValue {
			sorted[j] = sorted[j-1]
			j--
		}
		sorted[j] = newValue
		nbSorted++
		ring[ringIdx] = newValue
		i++
		ringIdx++
		if ringIdx > maxIdxRing {
			ringIdx = 0
		}
	}

	outIdx := 0
	for {
		newValue := inReal[i]
		pos := 0
		for pos < lookbackTotal && sorted[pos] <= newValue {
			pos++
		}
		var result float64
		if rank-1 < pos {
			result = sorted[rank-1]
		} else if rank-1 == pos {
			result = newValue
		} else {
			result = sorted[rank-2]
		}
		outReal[outIdx] = result
		outIdx++

		j := lookbackTotal
		for j > pos {
			sorted[j] = sorted[j-1]
			j--
		}
		sorted[pos] = newValue
		ring[ringIdx] = newValue
		ringIdx++
		if ringIdx > maxIdxRing {
			ringIdx = 0
		}
		oldValue := ring[ringIdx]
		j = 0
		for j < lookbackTotal && sorted[j] < oldValue {
			j++
		}
		for j < lookbackTotal {
			sorted[j] = sorted[j+1]
			j++
		}
		i++
		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
