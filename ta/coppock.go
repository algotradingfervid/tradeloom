package ta

// COPPOCK — Coppock Curve (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/coppock and src/ta_func/ta_COPPOCK.c.
//
// COPPOCK = WMA(ROC(optInROC1Period) + ROC(optInROC2Period), optInWMAPeriod),
// with ROC's zero guard (a zero base price yields 0 for that term).

// COPPOCKLookback returns the number of leading bars COPPOCK leaves empty:
// max(optInROC1Period, optInROC2Period) + optInWMAPeriod - 1, or -1 on an
// invalid parameter.
// optInWMAPeriod: [1, 100000], default 10.
// optInROC1Period: [1, 100000], default 11.
// optInROC2Period: [1, 100000], default 14.
func COPPOCKLookback(optInWMAPeriod, optInROC1Period, optInROC2Period int) int {
	if coppockCheckParams(optInWMAPeriod, optInROC1Period, optInROC2Period) != nil {
		return -1
	}
	return coppockLookback(optInWMAPeriod, optInROC1Period, optInROC2Period)
}

func coppockLookback(optInWMAPeriod, optInROC1Period, optInROC2Period int) int {
	if optInROC1Period > optInROC2Period {
		return optInROC1Period + optInWMAPeriod - 1
	}
	return optInROC2Period + optInWMAPeriod - 1
}

func coppockCheckParams(optInWMAPeriod, optInROC1Period, optInROC2Period int) error {
	if err := checkInt("COPPOCK", "optInWMAPeriod", optInWMAPeriod, 1, 100000); err != nil {
		return err
	}
	if err := checkInt("COPPOCK", "optInROC1Period", optInROC1Period, 1, 100000); err != nil {
		return err
	}
	return checkInt("COPPOCK", "optInROC2Period", optInROC2Period, 1, 100000)
}

// COPPOCK returns the Coppock Curve of inReal. The output is index-aligned
// with inReal; the first COPPOCKLookback bars are NaN.
func COPPOCK(inReal []float64, optInWMAPeriod, optInROC1Period, optInROC2Period int) ([]float64, error) {
	if err := coppockCheckParams(optInWMAPeriod, optInROC1Period, optInROC2Period); err != nil {
		return nil, err
	}
	n, err := checkInputs("COPPOCK", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := coppockRange(0, n-1, inReal, optInWMAPeriod, optInROC1Period, optInROC2Period, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// coppockRange mirrors TA_COPPOCK: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice (every input read for
// output k happens at an index >= k + lookback before outReal[k] is written).
func coppockRange(startIdx, endIdx int, inReal []float64, optInWMAPeriod, optInROC1Period, optInROC2Period int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := coppockCheckParams(optInWMAPeriod, optInROC1Period, optInROC2Period); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("COPPOCK", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("COPPOCK", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := coppockLookback(optInWMAPeriod, optInROC1Period, optInROC2Period)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Triangle divider in double (TA-Lib #142).
	divider := float64(optInWMAPeriod) * float64(optInWMAPeriod+1) / 2.0
	outIdx := 0

	// Ring of the w-1 previous S values (at least one slot).
	ringSize := optInWMAPeriod - 1
	if ringSize < 1 {
		ringSize = 1
	}
	sRing := make([]float64, ringSize)
	maxIdxSRing := ringSize - 1
	sRingIdx := 0
	sRing[0] = 0.0

	inIdx := startIdx - (optInWMAPeriod - 1)
	roc1Idx := inIdx - optInROC1Period
	roc2Idx := inIdx - optInROC2Period

	// Priming: the w-1 S values before the first output, weights 1..w-1.
	periodSub := 0.0
	periodSum := periodSub
	i := 1
	for inIdx < startIdx {
		base1 := inReal[roc1Idx]
		roc1Idx++
		base2 := inReal[roc2Idx]
		roc2Idx++
		roc1 := 0.0
		if base1 != 0.0 {
			roc1 = (inReal[inIdx]/base1 - 1.0) * 100.0
		}
		roc2 := 0.0
		if base2 != 0.0 {
			roc2 = (inReal[inIdx]/base2 - 1.0) * 100.0
		}
		tempReal := roc1 + roc2
		periodSub += tempReal
		periodSum += float64(tempReal * float64(i))
		i++
		sRing[sRingIdx] = tempReal
		sRingIdx++
		if sRingIdx > maxIdxSRing {
			sRingIdx = 0
		}
		inIdx++
	}

	barsSinceReseed := 8 * optInWMAPeriod
	trailingValue := 0.0
	for inIdx <= endIdx {
		base1 := inReal[roc1Idx]
		roc1Idx++
		base2 := inReal[roc2Idx]
		roc2Idx++
		roc1 := 0.0
		if base1 != 0.0 {
			roc1 = (inReal[inIdx]/base1 - 1.0) * 100.0
		}
		roc2 := 0.0
		if base2 != 0.0 {
			roc2 = (inReal[inIdx]/base2 - 1.0) * 100.0
		}
		tempReal := roc1 + roc2
		periodSub += tempReal
		periodSub -= trailingValue
		periodSum += float64(tempReal * float64(optInWMAPeriod))

		// Re-anchor every 8*w bars (TA_WMA's #254 fix): rebuild both totals
		// from the ring (oldest first, weights from 1) plus the current bar.
		barsSinceReseed--
		if barsSinceReseed <= 0 {
			barsSinceReseed = 8 * optInWMAPeriod
			periodSub = 0.0
			periodSum = 0.0
			rw := 1
			ringWalk := sRingIdx
			for q := 0; q < ringSize; q++ {
				tempReal2 := sRing[ringWalk]
				periodSub += tempReal2
				periodSum += float64(tempReal2 * float64(rw))
				rw++
				ringWalk++
				if ringWalk >= ringSize {
					ringWalk = 0
				}
			}
			periodSub += tempReal
			periodSum += float64(tempReal * float64(optInWMAPeriod))
		}

		// Read the expiring S value before overwriting its slot.
		trailingValue = sRing[sRingIdx]
		sRing[sRingIdx] = tempReal
		sRingIdx++
		if sRingIdx > maxIdxSRing {
			sRingIdx = 0
		}

		// WMA(1) is the identity; the recurrence is wrong at w == 1 because
		// the ring clamps to one slot, so C emits tempReal directly.
		if optInWMAPeriod == 1 {
			outReal[outIdx] = tempReal
		} else {
			outReal[outIdx] = periodSum / divider
		}
		outIdx++
		periodSum -= periodSub
		inIdx++
	}
	return startIdx, outIdx, nil
}
