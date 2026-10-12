package ta

import "math"

// MFI — Money Flow Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/mfi and src/ta_func/ta_MFI.c.

// MFILookback returns the number of leading bars MFI leaves empty, or -1 when
// optInTimePeriod is out of range.
// optInTimePeriod: [2, 100000], default 14.
func MFILookback(optInTimePeriod int) int {
	if checkInt("MFI", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return mfiLookback(optInTimePeriod)
}

func mfiLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// MFI returns the Money Flow Index: 100 * positive money flow / total money
// flow over a trailing window of optInTimePeriod bars, where each bar's money
// flow is its typical price (high+low+close)/3 times its volume, classed by
// the direction of the typical price versus the previous bar (unchanged
// within a scaled epsilon: neither). A window with no money flow yields 0.
// The output is index-aligned with the inputs; the first MFILookback bars are
// NaN. optInTimePeriod: [2, 100000], default 14.
func MFI(inHigh, inLow, inClose, inVolume []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("MFI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("MFI", []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := mfiRange(0, n-1, inHigh, inLow, inClose, inVolume, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// mfiRange mirrors TA_MFI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias any of the inputs: output k is written only
// after input bar startIdx+k has been consumed, and no input bar is re-read.
func mfiRange(startIdx, endIdx int, inHigh, inLow, inClose, inVolume []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MFI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MFI", endIdx, []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MFI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	mflowPositive := make([]float64, optInTimePeriod)
	mflowNegative := make([]float64, optInTimePeriod)
	maxIdxMflow := optInTimePeriod - 1
	mflowIdx := 0

	outIdx := 0
	today := startIdx - lookbackTotal
	prevValue := (inHigh[today] + inLow[today] + inClose[today]) / 3.0
	posSumMF := 0.0
	negSumMF := 0.0
	nullRun := 0
	today++

	// step is one bar of the C loop bodies: compute the bar's flow, store it
	// in the ring and add it to the sums, with the empty-window reset.
	step := func() {
		tempValue1 := (inHigh[today] + inLow[today] + inClose[today]) / 3.0
		tempValue2 := tempValue1 - prevValue
		tempValue3 := math.Abs(tempValue1) + math.Abs(prevValue)
		prevValue = tempValue1
		tempValue1 *= inVolume[today]
		today++
		moneyFlow := tempValue1
		if isZeroScaled(tempValue2, tempValue3) {
			moneyFlow = 0.0
		}
		posFlow, negFlow := moneyFlow, 0.0
		if tempValue2 < 0.0 {
			posFlow, negFlow = 0.0, moneyFlow
		}
		mflowPositive[mflowIdx] = posFlow
		mflowNegative[mflowIdx] = negFlow
		posSumMF += posFlow
		negSumMF += negFlow
		if moneyFlow == 0.0 {
			nullRun++
		} else {
			nullRun = 0
		}
		if nullRun >= optInTimePeriod {
			nullRun = optInTimePeriod
			posSumMF = 0.0
			negSumMF = 0.0
		}
	}
	emit := func() {
		tempValue1 := posSumMF + negSumMF
		posClamped := posSumMF
		if posSumMF < 0.0 {
			posClamped = 0.0
		} else if posSumMF > tempValue1 {
			posClamped = tempValue1
		}
		if tempValue1 <= 0.0 {
			outReal[outIdx] = 0.0
		} else {
			outReal[outIdx] = 100.0 * (posClamped / tempValue1)
		}
		outIdx++
	}

	for i := optInTimePeriod; i > 0; i-- {
		step()
		mflowIdx++
		if mflowIdx > maxIdxMflow {
			mflowIdx = 0
		}
	}
	emit()

	for today <= endIdx {
		posSumMF -= mflowPositive[mflowIdx]
		negSumMF -= mflowNegative[mflowIdx]
		step()
		emit()
		mflowIdx++
		if mflowIdx > maxIdxMflow {
			mflowIdx = 0
		}
	}
	return startIdx, outIdx, nil
}
