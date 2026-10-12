package ta

import "math"

// HT_DCPHASE — Hilbert Transform - Dominant Cycle Phase (Cycle Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ht_dcphase and
// src/ta_func/ta_HT_DCPHASE.c.

// HTDCPHASELookback returns the number of leading bars HT_DCPHASE leaves
// empty: a fixed 63 plus the HT_DCPHASE unstable period (UnstHTDCPhase).
func HTDCPHASELookback() int {
	return htdcphaseLookback()
}

func htdcphaseLookback() int {
	return 63 + UnstablePeriod(UnstHTDCPhase)
}

// HTDCPHASE returns the Hilbert Transform dominant cycle phase (in degrees)
// of inReal. The output is index-aligned with inReal; the first
// HTDCPHASELookback bars are NaN.
func HTDCPHASE(inReal []float64) ([]float64, error) {
	n, err := checkInputs("HT_DCPHASE", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := htdcphaseRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// htdcphaseRange mirrors TA_HT_DCPHASE: it computes [startIdx, endIdx] and
// writes outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice (every input read
// precedes the output write at a lower or equal index).
func htdcphaseRange(startIdx, endIdx int, inReal []float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HT_DCPHASE", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HT_DCPHASE", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return htdcphaseCore(startIdx, endIdx, inReal, outReal)
}

// htdcphaseCore is the TA_HT_DCPHASE body after the validation prologue.
//
// The C library is built with -ffp-contract=off: only its explicit fma()
// calls are fused (ported as math.FMA). Every product whose result is later
// added (in the same statement or a following one: the Go spec allows fusion
// across statements) is wrapped in float64(...), which forbids Go from fusing
// it on arm64. Without that, Q2 already differs from C in the last ulp after
// a few bars, and on a linear trend (where Im is pure rounding noise) the
// period estimate then diverges by whole units.
//
// sin and cos come from htSinCos (httrig.go), not math.Sin/math.Cos.
func htdcphaseCore(startIdx, endIdx int, inReal []float64, outReal []float64) (int, int, error) {
	const a = 0.0962
	const b = 0.5769

	// In C smoothPrice is a 50-slot circular buffer.
	var smoothPrice [50]float64
	maxIdxSmoothPrice := len(smoothPrice) - 1
	smoothPriceIdx := 0

	tempReal := libmAtan(1)
	rad2Deg := 45.0 / tempReal
	constDeg2RadBy360 := tempReal * 8.0

	lookbackTotal := 63 + UnstablePeriod(UnstHTDCPhase)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	outBegIdx := startIdx

	// Price smoother: a 4-bar WMA, initialised with the loop unrolled as in C.
	trailingWMAIdx := startIdx - lookbackTotal
	today := trailingWMAIdx

	tempReal = inReal[today]
	today++
	periodWMASub := tempReal
	periodWMASum := tempReal
	tempReal = inReal[today]
	today++
	periodWMASub += tempReal
	periodWMASum += float64(tempReal * 2.0)
	tempReal = inReal[today]
	today++
	periodWMASub += tempReal
	periodWMASum += float64(tempReal * 3.0)
	trailingWMAValue := 0.0

	for i := 34; i != 0; i-- {
		tempReal = inReal[today]
		today++
		periodWMASub += tempReal
		periodWMASub -= trailingWMAValue
		periodWMASum += float64(tempReal * 4.0)
		trailingWMAValue = inReal[trailingWMAIdx]
		trailingWMAIdx++
		// smoothedValue = periodWMASum * 0.1 is computed but unused here in C.
		periodWMASum -= periodWMASub
	}

	// Circular buffers for the Hilbert transform: one set for odd bars, one
	// for even bars.
	hilbertIdx := 0
	var detrenderOdd, detrenderEven [3]float64
	var prevDetrenderOdd, prevDetrenderEven float64
	var prevDetrenderInputOdd, prevDetrenderInputEven float64
	var q1Odd, q1Even [3]float64
	var prevQ1Odd, prevQ1Even float64
	var prevQ1InputOdd, prevQ1InputEven float64
	var jIOdd, jIEven [3]float64
	var prevJIOdd, prevJIEven float64
	var prevJIInputOdd, prevJIInputEven float64
	var jQOdd, jQEven [3]float64
	var prevJQOdd, prevJQEven float64
	var prevJQInputOdd, prevJQInputEven float64

	var detrender, q1, jI, jQ, q2, i2, tempReal2 float64
	period := 0.0
	outIdx := 0
	prevQ2, prevI2 := 0.0, 0.0
	re, im := 0.0, 0.0
	i1ForOddPrev3, i1ForEvenPrev3 := 0.0, 0.0
	i1ForOddPrev2, i1ForEvenPrev2 := 0.0, 0.0
	smoothPeriod := 0.0
	dcPhase := 0.0

	for today <= endIdx {
		adjustedPrevPeriod := math.FMA(0.075, period, 0.54)
		todayValue := inReal[today]
		periodWMASub += todayValue
		periodWMASub -= trailingWMAValue
		periodWMASum += float64(todayValue * 4.0)
		trailingWMAValue = inReal[trailingWMAIdx]
		trailingWMAIdx++
		smoothedValue := float64(periodWMASum * 0.1)
		periodWMASum -= periodWMASub

		smoothPrice[smoothPriceIdx] = smoothedValue

		if today%2 == 0 {
			// Hilbert transforms for an even price bar.
			hilbertTempReal := float64(a * smoothedValue)
			detrender = 0 - detrenderEven[hilbertIdx]
			detrenderEven[hilbertIdx] = hilbertTempReal
			detrender += hilbertTempReal
			detrender -= prevDetrenderEven
			prevDetrenderEven = float64(b * prevDetrenderInputEven)
			detrender += prevDetrenderEven
			prevDetrenderInputEven = smoothedValue
			detrender = float64(detrender * adjustedPrevPeriod)

			hilbertTempReal = float64(a * detrender)
			q1 = 0 - q1Even[hilbertIdx]
			q1Even[hilbertIdx] = hilbertTempReal
			q1 += hilbertTempReal
			q1 -= prevQ1Even
			prevQ1Even = float64(b * prevQ1InputEven)
			q1 += prevQ1Even
			prevQ1InputEven = detrender
			q1 = float64(q1 * adjustedPrevPeriod)

			hilbertTempReal = float64(a * i1ForEvenPrev3)
			jI = 0 - jIEven[hilbertIdx]
			jIEven[hilbertIdx] = hilbertTempReal
			jI += hilbertTempReal
			jI -= prevJIEven
			prevJIEven = float64(b * prevJIInputEven)
			jI += prevJIEven
			prevJIInputEven = i1ForEvenPrev3
			jI = float64(jI * adjustedPrevPeriod)

			hilbertTempReal = float64(a * q1)
			jQ = 0 - jQEven[hilbertIdx]
			jQEven[hilbertIdx] = hilbertTempReal
			jQ += hilbertTempReal
			jQ -= prevJQEven
			prevJQEven = float64(b * prevJQInputEven)
			jQ += prevJQEven
			prevJQInputEven = q1
			jQ = float64(jQ * adjustedPrevPeriod)

			hilbertIdx++
			if hilbertIdx == 3 {
				hilbertIdx = 0
			}

			q2 = math.FMA(0.2, q1+jI, 0.8*prevQ2)
			i2 = math.FMA(0.2, i1ForEvenPrev3-jQ, 0.8*prevI2)

			// I1 is the detrender delayed by 3 bars; save it for the odd logic.
			i1ForOddPrev3 = i1ForOddPrev2
			i1ForOddPrev2 = detrender
		} else {
			// Hilbert transforms for an odd price bar.
			hilbertTempReal := float64(a * smoothedValue)
			detrender = 0 - detrenderOdd[hilbertIdx]
			detrenderOdd[hilbertIdx] = hilbertTempReal
			detrender += hilbertTempReal
			detrender -= prevDetrenderOdd
			prevDetrenderOdd = float64(b * prevDetrenderInputOdd)
			detrender += prevDetrenderOdd
			prevDetrenderInputOdd = smoothedValue
			detrender = float64(detrender * adjustedPrevPeriod)

			hilbertTempReal = float64(a * detrender)
			q1 = 0 - q1Odd[hilbertIdx]
			q1Odd[hilbertIdx] = hilbertTempReal
			q1 += hilbertTempReal
			q1 -= prevQ1Odd
			prevQ1Odd = float64(b * prevQ1InputOdd)
			q1 += prevQ1Odd
			prevQ1InputOdd = detrender
			q1 = float64(q1 * adjustedPrevPeriod)

			hilbertTempReal = float64(a * i1ForOddPrev3)
			jI = 0 - jIOdd[hilbertIdx]
			jIOdd[hilbertIdx] = hilbertTempReal
			jI += hilbertTempReal
			jI -= prevJIOdd
			prevJIOdd = float64(b * prevJIInputOdd)
			jI += prevJIOdd
			prevJIInputOdd = i1ForOddPrev3
			jI = float64(jI * adjustedPrevPeriod)

			hilbertTempReal = float64(a * q1)
			jQ = 0 - jQOdd[hilbertIdx]
			jQOdd[hilbertIdx] = hilbertTempReal
			jQ += hilbertTempReal
			jQ -= prevJQOdd
			prevJQOdd = float64(b * prevJQInputOdd)
			jQ += prevJQOdd
			prevJQInputOdd = q1
			jQ = float64(jQ * adjustedPrevPeriod)

			q2 = math.FMA(0.2, q1+jI, 0.8*prevQ2)
			i2 = math.FMA(0.2, i1ForOddPrev3-jQ, 0.8*prevI2)

			// I1 is the detrender delayed by 3 bars; save it for the even logic.
			i1ForEvenPrev3 = i1ForEvenPrev2
			i1ForEvenPrev2 = detrender
		}

		// Adjust the period for the next price bar.
		re = math.FMA(0.8, re, 0.2*math.FMA(i2, prevI2, q2*prevQ2))
		im = math.FMA(0.8, im, 0.2*(float64(i2*prevQ2)-float64(q2*prevI2)))
		prevQ2 = q2
		prevI2 = i2
		tempReal = period
		if im != 0.0 && re != 0.0 {
			period = 360.0 / (libmAtan(im/re) * rad2Deg)
		}
		tempReal2 = 1.5 * tempReal
		if period > tempReal2 {
			period = tempReal2
		}
		tempReal2 = 0.67 * tempReal
		if period < tempReal2 {
			period = tempReal2
		}
		if period < 6 {
			period = 6
		} else if period > 50 {
			period = 50
		}
		period = math.FMA(0.2, period, 0.8*tempReal)
		smoothPeriod = math.FMA(0.67, smoothPeriod, 0.33*period)

		// Dominant cycle phase.
		dcPeriod := smoothPeriod + 0.5
		dcPeriodInt := int(dcPeriod)
		realPart := 0.0
		imagPart := 0.0

		// idx walks back over up to 50 of the latest smoothPrice values.
		idx := smoothPriceIdx
		for i := 0; i < dcPeriodInt; i++ {
			tempReal = float64(float64(i)*constDeg2RadBy360) / float64(dcPeriodInt)
			tempReal2 = smoothPrice[idx]
			sinV, cosV := htSinCos(i, dcPeriodInt, tempReal)
			realPart += float64(sinV * tempReal2)
			imagPart += float64(cosV * tempReal2)
			if idx == 0 {
				idx = 50 - 1
			} else {
				idx--
			}
		}

		tempReal = math.Abs(imagPart)
		if tempReal > 0.0 {
			dcPhase = float64(libmAtan(realPart/imagPart) * rad2Deg)
		} else if tempReal <= 0.01 {
			if realPart < 0.0 {
				dcPhase -= 90.0
			} else if realPart > 0.0 {
				dcPhase += 90.0
			}
		}
		dcPhase += 90.0

		// Compensate for the one-bar lag of the weighted moving average.
		dcPhase += 360.0 / smoothPeriod
		if imagPart < 0.0 {
			dcPhase += 180.0
		}
		if dcPhase > 315.0 {
			dcPhase -= 360.0
		}

		if today >= startIdx {
			outReal[outIdx] = dcPhase
			outIdx++
		}

		smoothPriceIdx++
		if smoothPriceIdx > maxIdxSmoothPrice {
			smoothPriceIdx = 0
		}
		today++
	}
	return outBegIdx, outIdx, nil
}
