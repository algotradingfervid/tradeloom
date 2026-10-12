package ta

import "math"

// HMA — Hull Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/hma and src/ta_func/ta_HMA.c.
//
// HMA(n) = WMA( 2*WMA(price, Integer(n/2)) - WMA(price, n), Integer(SquareRoot(n)) )

// HMALookback returns the number of leading bars HMA leaves empty.
// optInTimePeriod: [1, 100000], default 20.
func HMALookback(optInTimePeriod int) int {
	if checkInt("HMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return hmaLookback(optInTimePeriod)
}

func hmaLookback(optInTimePeriod int) int {
	sqrtPeriod := int(math.Sqrt(float64(optInTimePeriod)))
	return wmaLookback(optInTimePeriod) + wmaLookback(sqrtPeriod)
}

// HMA returns the Hull moving average of inReal over optInTimePeriod bars.
// The output is index-aligned with inReal; the first HMALookback bars are NaN.
// optInTimePeriod: [1, 100000], default 20.
func HMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("HMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("HMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := hmaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// hmaRange mirrors TA_HMA: a single fused pass with three interleaved WMA
// rolling sums (full period, half period, and the outer sqrt-period WMA over
// the de-lagged series held in a small ring). It computes [startIdx, endIdx]
// and writes outNBElement values to outReal[0:], the first one belonging to
// bar outBegIdx. inReal and outReal may be the same slice.
func hmaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("HMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Period 1: the output is a copy of the input (no lookback clamp in C).
	if optInTimePeriod == 1 {
		outIdx := 0
		for today := startIdx; today <= endIdx; today++ {
			outReal[outIdx] = inReal[today]
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	halfPeriod := optInTimePeriod / 2
	sqrtPeriod := int(math.Sqrt(float64(optInTimePeriod)))
	lookbackSqrt := wmaLookback(sqrtPeriod)
	lookbackTotal := wmaLookback(optInTimePeriod) + lookbackSqrt
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	wmaStartIdx := startIdx - lookbackSqrt
	// Computed in double, as C does, to avoid int overflow.
	dividerFull := float64(optInTimePeriod) * float64(optInTimePeriod+1) / 2.0

	// Prime the full-period WMA (weights 1..period-1).
	lookbackFull := optInTimePeriod - 1
	periodSubFull := 0.0
	periodSumFull := 0.0
	trailingIdxFull := wmaStartIdx - lookbackFull
	i := trailingIdxFull
	w := 1
	for i < wmaStartIdx {
		tempReal := inReal[i]
		i++
		periodSubFull += tempReal
		periodSumFull += float64(tempReal * float64(w))
		w++
	}
	barsSinceReseedFull := 8 * optInTimePeriod
	trailingFull := 0.0
	outIdx := 0

	// stepFull advances the full-period WMA by bar today (value tempReal) and
	// returns its output, exactly as the repeated C block does.
	stepFull := func(today int, tempReal float64) float64 {
		periodSubFull += tempReal
		periodSubFull -= trailingFull
		periodSumFull += float64(tempReal * float64(optInTimePeriod))
		barsSinceReseedFull--
		if barsSinceReseedFull <= 0 {
			barsSinceReseedFull = 8 * optInTimePeriod
			periodSubFull = 0.0
			periodSumFull = 0.0
			rw := 1
			for jFull := today - lookbackFull; jFull <= today; jFull++ {
				tempReal2 := inReal[jFull]
				periodSubFull += tempReal2
				periodSumFull += float64(tempReal2 * float64(rw))
				rw++
			}
		}
		trailingFull = inReal[trailingIdxFull]
		trailingIdxFull++
		fullOut := periodSumFull / dividerFull
		periodSumFull -= periodSubFull
		return fullOut
	}

	if optInTimePeriod == 2 || optInTimePeriod == 3 {
		// Degenerate regime: halfPeriod and sqrtPeriod are both 1, so
		// HMA[t] = 2*price[t] - WMA(price, n)[t]. The trailing read precedes
		// the store, keeping in-place use safe.
		for today := startIdx; today <= endIdx; today++ {
			tempReal := inReal[today]
			fullOut := stepFull(today, tempReal)
			outReal[outIdx] = 2.0*tempReal - fullOut
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	// General regime: optInTimePeriod >= 4, halfPeriod >= 2, sqrtPeriod >= 2.
	dividerHalf := float64(halfPeriod) * float64(halfPeriod+1) / 2.0
	dividerSqrt := float64(sqrtPeriod) * float64(sqrtPeriod+1) / 2.0

	lookbackHalf := halfPeriod - 1
	periodSubHalf := 0.0
	periodSumHalf := 0.0
	trailingIdxHalf := wmaStartIdx - lookbackHalf
	i = trailingIdxHalf
	w = 1
	for i < wmaStartIdx {
		tempReal := inReal[i]
		i++
		periodSubHalf += tempReal
		periodSumHalf += float64(tempReal * float64(w))
		w++
	}
	barsSinceReseedHalf := 8 * halfPeriod
	trailingHalf := 0.0

	stepHalf := func(today int, tempReal float64) float64 {
		periodSubHalf += tempReal
		periodSubHalf -= trailingHalf
		periodSumHalf += float64(tempReal * float64(halfPeriod))
		barsSinceReseedHalf--
		if barsSinceReseedHalf <= 0 {
			barsSinceReseedHalf = 8 * halfPeriod
			periodSubHalf = 0.0
			periodSumHalf = 0.0
			rw := 1
			for jHalf := today - lookbackHalf; jHalf <= today; jHalf++ {
				tempReal2 := inReal[jHalf]
				periodSubHalf += tempReal2
				periodSumHalf += float64(tempReal2 * float64(rw))
				rw++
			}
		}
		trailingHalf = inReal[trailingIdxHalf]
		trailingIdxHalf++
		halfOut := periodSumHalf / dividerHalf
		periodSumHalf -= periodSubHalf
		return halfOut
	}

	// Ring of sqrtPeriod-1 de-lagged values (C uses a 50-slot stack buffer
	// or TA_Malloc; both behave the same).
	ringSize := sqrtPeriod - 1
	dRing := make([]float64, ringSize)
	maxIdxDRing := ringSize - 1
	dRingIdx := 0

	// Warm-up: the sqrtPeriod-1 de-lagged values before the first output
	// prime the outer WMA (weights 1..sqrtPeriod-1) and fill the ring.
	periodSubSqrt := 0.0
	periodSumSqrt := 0.0
	trailingSqrt := 0.0
	w = 1
	for today := wmaStartIdx; today < startIdx; today++ {
		tempReal := inReal[today]
		fullOut := stepFull(today, tempReal)
		halfOut := stepHalf(today, tempReal)
		diffReal := 2.0*halfOut - fullOut
		periodSubSqrt += diffReal
		periodSumSqrt += float64(diffReal * float64(w))
		w++
		dRing[dRingIdx] = diffReal
		dRingIdx++
		if dRingIdx > maxIdxDRing {
			dRingIdx = 0
		}
	}
	barsSinceReseedSqrt := 8 * sqrtPeriod

	// Steady state. Writes trail every read by at least sqrtPeriod-1 slots,
	// so outReal == inReal stays safe.
	for today := startIdx; today <= endIdx; today++ {
		tempReal := inReal[today]
		fullOut := stepFull(today, tempReal)
		halfOut := stepHalf(today, tempReal)
		diffReal := 2.0*halfOut - fullOut
		periodSubSqrt += diffReal
		periodSubSqrt -= trailingSqrt
		periodSumSqrt += float64(diffReal * float64(sqrtPeriod))

		// Re-anchor the outer WMA from the ring (oldest first, weight 1 up),
		// then add the newest value.
		barsSinceReseedSqrt--
		if barsSinceReseedSqrt <= 0 {
			barsSinceReseedSqrt = 8 * sqrtPeriod
			periodSubSqrt = 0.0
			periodSumSqrt = 0.0
			rw := 1
			ringWalk := dRingIdx
			for q := 0; q < ringSize; q++ {
				tempReal2 := dRing[ringWalk]
				periodSubSqrt += tempReal2
				periodSumSqrt += float64(tempReal2 * float64(rw))
				rw++
				ringWalk++
				if ringWalk >= ringSize {
					ringWalk = 0
				}
			}
			periodSubSqrt += diffReal
			periodSumSqrt += float64(diffReal * float64(sqrtPeriod))
		}
		trailingSqrt = dRing[dRingIdx]
		dRing[dRingIdx] = diffReal
		dRingIdx++
		if dRingIdx > maxIdxDRing {
			dRingIdx = 0
		}
		outReal[outIdx] = periodSumSqrt / dividerSqrt
		outIdx++
		periodSumSqrt -= periodSubSqrt
	}
	return startIdx, outIdx, nil
}
