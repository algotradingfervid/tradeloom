package ta

import "math"

// MAMA — MESA Adaptive Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/mama and src/ta_func/ta_MAMA.c.

// MAMALookback returns the number of leading bars MAMA leaves empty: a fixed
// 32 plus the MAMA unstable period (UnstMAMA). The limits do not change the
// lookback but are validated like C.
// optInFastLimit: [0.01, 0.99], default 0.5.
// optInSlowLimit: [0.01, 0.99], default 0.05.
func MAMALookback(optInFastLimit, optInSlowLimit float64) int {
	if mamaCheckParams(optInFastLimit, optInSlowLimit) != nil {
		return -1
	}
	return mamaLookback(optInFastLimit, optInSlowLimit)
}

func mamaLookback(optInFastLimit, optInSlowLimit float64) int {
	return 32 + UnstablePeriod(UnstMAMA)
}

func mamaCheckParams(optInFastLimit, optInSlowLimit float64) error {
	if err := checkReal("MAMA", "optInFastLimit", optInFastLimit, 0.01, 0.99); err != nil {
		return err
	}
	return checkReal("MAMA", "optInSlowLimit", optInSlowLimit, 0.01, 0.99)
}

// MAMA returns the MESA adaptive moving average of inReal and its follower
// FAMA. Both outputs are index-aligned with inReal; the first MAMALookback
// bars are NaN.
// optInFastLimit: [0.01, 0.99], default 0.5.
// optInSlowLimit: [0.01, 0.99], default 0.05.
func MAMA(inReal []float64, optInFastLimit, optInSlowLimit float64) (outMAMA, outFAMA []float64, err error) {
	if err := mamaCheckParams(optInFastLimit, optInSlowLimit); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("MAMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, err
	}
	outMAMA = make([]float64, n)
	outFAMA = make([]float64, n)
	if n == 0 {
		return outMAMA, outFAMA, nil
	}
	beg, nb, err := mamaRange(0, n-1, inReal, optInFastLimit, optInSlowLimit, outMAMA, outFAMA)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outMAMA, beg, nb)
	alignReal(outFAMA, beg, nb)
	return outMAMA, outFAMA, nil
}

// mamaRange mirrors TA_MAMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outMAMA[0:] and outFAMA[0:], the first one belonging
// to bar outBegIdx. outFAMA may be nil (it is nullable in C); outMAMA and
// outFAMA must not be the same buffer.
func mamaRange(startIdx, endIdx int, inReal []float64, optInFastLimit, optInSlowLimit float64, outMAMA, outFAMA []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := mamaCheckParams(optInFastLimit, optInSlowLimit); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MAMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MAMA", endIdx-startIdx, []string{"outMAMA"}, outMAMA); err != nil {
		return 0, 0, err
	}
	if outFAMA != nil {
		if err := checkBuf("MAMA", endIdx-startIdx, []string{"outFAMA"}, outFAMA); err != nil {
			return 0, 0, err
		}
		// C: outFAMA != NULL && outMAMA == outFAMA -> TA_BAD_PARAM.
		if &outFAMA[0] == &outMAMA[0] {
			return 0, 0, badParam("MAMA", "outFAMA", "must not be the same buffer as outMAMA")
		}
	}
	return mamaCore(startIdx, endIdx, inReal, optInFastLimit, optInSlowLimit, outMAMA, outFAMA)
}

// mamaCore is the TA_MAMA body after the validation prologue.
//
// The C library is built with -ffp-contract=off: only its explicit fma()
// calls are fused (ported as math.FMA). Every other product that feeds an
// addition is wrapped in float64(...), which forbids Go from fusing it on
// arm64. This matters: on a flat series I2*prevQ2 - Q2*prevI2 is exactly 0
// in C, and a fused version leaves a residue that changes the period.
func mamaCore(startIdx, endIdx int, inReal []float64, optInFastLimit, optInSlowLimit float64, outMAMA, outFAMA []float64) (int, int, error) {
	const a = 0.0962
	const b = 0.5769
	rad2Deg := 180.0 / (4.0 * libmAtan(1))

	lookbackTotal := 32 + UnstablePeriod(UnstMAMA)
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

	tempReal := inReal[today]
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

	for i := 9; i != 0; i-- {
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
	mama, fama := 0.0, 0.0
	i1ForOddPrev3, i1ForEvenPrev3 := 0.0, 0.0
	i1ForOddPrev2, i1ForEvenPrev2 := 0.0, 0.0
	prevPhase := 0.0

	for today <= endIdx {
		adjustedPrevPeriod := math.FMA(0.075, period, 0.54)
		todayValue := inReal[today]
		periodWMASub += todayValue
		periodWMASub -= trailingWMAValue
		periodWMASum += float64(todayValue * 4.0)
		trailingWMAValue = inReal[trailingWMAIdx]
		trailingWMAIdx++
		smoothedValue := periodWMASum * 0.1
		periodWMASum -= periodWMASub

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

			if i1ForEvenPrev3 != 0.0 {
				tempReal2 = float64(libmAtan(q1/i1ForEvenPrev3) * rad2Deg)
			} else {
				tempReal2 = 0.0
			}
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

			i1ForEvenPrev3 = i1ForEvenPrev2
			i1ForEvenPrev2 = detrender

			if i1ForOddPrev3 != 0.0 {
				tempReal2 = float64(libmAtan(q1/i1ForOddPrev3) * rad2Deg)
			} else {
				tempReal2 = 0.0
			}
		}

		// Delta phase.
		tempReal = prevPhase - tempReal2
		prevPhase = tempReal2
		if tempReal < 1.0 {
			tempReal = 1.0
		}

		// Alpha.
		if tempReal > 1.0 {
			tempReal = optInFastLimit / tempReal
			if tempReal < optInSlowLimit {
				tempReal = optInSlowLimit
			}
		} else {
			tempReal = optInFastLimit
		}

		mama = math.FMA(1-tempReal, mama, tempReal*todayValue)
		tempReal = float64(tempReal * 0.5)
		fama = math.FMA(1-tempReal, fama, tempReal*mama)
		if today >= startIdx {
			if outFAMA != nil {
				outFAMA[outIdx] = fama
			}
			outMAMA[outIdx] = mama
			outIdx++
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
		tempReal2 = float64(1.5 * tempReal)
		if period > tempReal2 {
			period = tempReal2
		}
		tempReal2 = float64(0.67 * tempReal)
		if period < tempReal2 {
			period = tempReal2
		}
		if period < 6 {
			period = 6
		} else if period > 50 {
			period = 50
		}
		period = math.FMA(0.2, period, 0.8*tempReal)

		today++
	}
	return outBegIdx, outIdx, nil
}
