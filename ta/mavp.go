package ta

// MAVP — Moving average with variable period (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/mavp and src/ta_func/ta_MAVP.c.

// mavpCheck validates the MAVP optional inputs in the order of the C
// prologue, applies the MATypeDefault substitution (-> SMA), and rejects an
// inverted period window (optInMinPeriod > optInMaxPeriod) as C does.
func mavpCheck(optInMinPeriod, optInMaxPeriod int, optInMAType MAType) (MAType, error) {
	if err := checkInt("MAVP", "optInMinPeriod", optInMinPeriod, 1, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkInt("MAVP", "optInMaxPeriod", optInMaxPeriod, 1, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkMAType("MAVP", "optInMAType", optInMAType); err != nil {
		return optInMAType, err
	}
	if optInMAType == MATypeDefault {
		optInMAType = MATypeSMA
	}
	if optInMinPeriod > optInMaxPeriod {
		return optInMAType, badParam("MAVP", "optInMinPeriod", "%d greater than optInMaxPeriod %d", optInMinPeriod, optInMaxPeriod)
	}
	return optInMAType, nil
}

// MAVPLookback returns the number of leading bars MAVP leaves empty: the MA
// lookback at optInMaxPeriod. It returns -1 on an invalid parameter,
// including optInMinPeriod > optInMaxPeriod.
// optInMinPeriod: [1, 100000], default 2. optInMaxPeriod: [1, 100000],
// default 30. optInMAType: default SMA.
func MAVPLookback(optInMinPeriod, optInMaxPeriod int, optInMAType MAType) int {
	t, err := mavpCheck(optInMinPeriod, optInMaxPeriod, optInMAType)
	if err != nil {
		return -1
	}
	return mavpLookback(optInMinPeriod, optInMaxPeriod, t)
}

// mavpLookback is the unvalidated tail of TA_MAVP_Lookback; optInMAType must
// already have MATypeDefault substituted.
func mavpLookback(optInMinPeriod, optInMaxPeriod int, optInMAType MAType) int {
	_ = optInMinPeriod
	return maLookback(optInMaxPeriod, optInMAType)
}

// MAVP returns, for each bar, the moving average of inReal of type
// optInMAType over the period inPeriods[i], truncated to an integer and
// clamped to [optInMinPeriod, optInMaxPeriod] (NaN periods use the minimum).
// The output is index-aligned with inReal; the first MAVPLookback bars are NaN.
// optInMinPeriod: [1, 100000], default 2. optInMaxPeriod: [1, 100000],
// default 30. optInMAType: default SMA.
func MAVP(inReal, inPeriods []float64, optInMinPeriod, optInMaxPeriod int, optInMAType MAType) ([]float64, error) {
	if _, err := mavpCheck(optInMinPeriod, optInMaxPeriod, optInMAType); err != nil {
		return nil, err
	}
	n, err := checkInputs("MAVP", []string{"inReal", "inPeriods"}, inReal, inPeriods)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := mavpRange(0, n-1, inReal, inPeriods, optInMinPeriod, optInMaxPeriod, optInMAType, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// mavpRange mirrors TA_MAVP: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func mavpRange(startIdx, endIdx int, inReal, inPeriods []float64, optInMinPeriod, optInMaxPeriod int, optInMAType MAType, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MAVP", "optInMinPeriod", optInMinPeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MAVP", "optInMaxPeriod", optInMaxPeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkMAType("MAVP", "optInMAType", optInMAType); err != nil {
		return 0, 0, err
	}
	if optInMAType == MATypeDefault {
		optInMAType = MATypeSMA
	}
	if err := checkBuf("MAVP", endIdx, []string{"inReal", "inPeriods"}, inReal, inPeriods); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MAVP", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// An inverted period window is an invalid parameter combination.
	if optInMinPeriod > optInMaxPeriod {
		return 0, 0, badParam("MAVP", "optInMinPeriod", "%d greater than optInMaxPeriod %d", optInMinPeriod, optInMaxPeriod)
	}

	lookbackTotal := maLookback(optInMaxPeriod, optInMAType)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	var firstOut int
	if lookbackTotal > startIdx {
		firstOut = lookbackTotal
	} else {
		firstOut = startIdx
	}
	if firstOut > endIdx {
		return 0, 0, nil
	}
	outputSize := endIdx - firstOut + 1

	localOutputArray := make([]float64, outputSize)
	localPeriodArray := make([]int, outputSize)
	sortedIdx := make([]int, outputSize)

	// In-place defence: each MA pass re-reads inReal over the full range, so
	// with outReal aliasing inReal the results are staged in a scratch buffer.
	var localFinalArray []float64
	finalIsAllocated := len(outReal) > 0 && len(inReal) > 0 && &outReal[0] == &inReal[0]
	if finalIsAllocated {
		localFinalArray = make([]float64, outputSize)
	} else {
		localFinalArray = outReal
	}

	// Read the period array, clamp to [min, max] in the real domain, then
	// truncate; track the range of periods actually used.
	minUsed := optInMaxPeriod
	if minUsed < 1 {
		minUsed = 1
	}
	maxUsed := 1
	minPeriodReal := float64(optInMinPeriod)
	maxPeriodReal := float64(optInMaxPeriod)
	var tempInt int
	for i := 0; i < outputSize; i++ {
		tempPeriod := inPeriods[startIdx+i]
		if !(tempPeriod >= minPeriodReal) { // also catches NaN
			tempInt = optInMinPeriod
		} else if tempPeriod > maxPeriodReal {
			tempInt = optInMaxPeriod
		} else {
			tempInt = int(tempPeriod)
		}
		if tempInt < 1 {
			tempInt = 1
		}
		localPeriodArray[i] = tempInt
		if tempInt < minUsed {
			minUsed = tempInt
		}
		if tempInt > maxUsed {
			maxUsed = tempInt
		}
	}

	// Unreachable through the validated API; kept to mirror C.
	if maxUsed < minUsed || maxUsed-minUsed > 100000 {
		return 0, 0, badParam("MAVP", "inPeriods", "period spread %d..%d too wide", minUsed, maxUsed)
	}

	bucketOfs := make([]int, maxUsed-minUsed+2)

	if minUsed == maxUsed {
		// Single distinct period: one MA pass straight into the destination.
		if _, _, err := maRange(startIdx, endIdx, inReal, minUsed, optInMAType, localFinalArray); err != nil {
			return 0, 0, err
		}
	} else {
		// Counting sort of output indices by period.
		for curPeriod := minUsed; curPeriod <= maxUsed+1; curPeriod++ {
			bucketOfs[curPeriod-minUsed] = 0
		}
		for i := 0; i < outputSize; i++ {
			tempInt = localPeriodArray[i]
			bucketOfs[tempInt+1-minUsed] = bucketOfs[tempInt+1-minUsed] + 1
		}
		for curPeriod := minUsed; curPeriod <= maxUsed; curPeriod++ {
			bucketOfs[curPeriod+1-minUsed] = bucketOfs[curPeriod+1-minUsed] + bucketOfs[curPeriod-minUsed]
		}
		for i := 0; i < outputSize; i++ {
			tempInt = localPeriodArray[i]
			sortedIdx[bucketOfs[tempInt-minUsed]] = i
			bucketOfs[tempInt-minUsed] = bucketOfs[tempInt-minUsed] + 1
		}

		// One MA pass per period used, from startIdx to its last use.
		bucketStart := 0
		for curPeriod := minUsed; curPeriod <= maxUsed; curPeriod++ {
			bucketEnd := bucketOfs[curPeriod-minUsed]
			if bucketEnd > bucketStart {
				firstOccurrence := sortedIdx[bucketStart]
				lastOccurrence := sortedIdx[bucketEnd-1]
				if _, _, err := maRange(startIdx, startIdx+lastOccurrence, inReal, curPeriod, optInMAType, localOutputArray); err != nil {
					return 0, 0, err
				}
				if lastOccurrence-firstOccurrence == bucketEnd-1-bucketStart {
					copy(localFinalArray[firstOccurrence:firstOccurrence+(bucketEnd-bucketStart)],
						localOutputArray[firstOccurrence:firstOccurrence+(bucketEnd-bucketStart)])
				} else {
					for i := bucketStart; i < bucketEnd; i++ {
						tempInt = sortedIdx[i]
						localFinalArray[tempInt] = localOutputArray[tempInt]
					}
				}
			}
			bucketStart = bucketEnd
		}
	}

	if finalIsAllocated {
		copy(outReal[:outputSize], localFinalArray)
	}
	return startIdx, outputSize, nil
}
