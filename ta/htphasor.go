package ta

import "math"

// HT_PHASOR — Hilbert Transform - Phasor Components (Cycle Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ht_phasor and src/ta_func/ta_HT_PHASOR.c.

// HTPHASORLookback returns the number of leading bars HTPHASOR leaves empty:
// a fixed 32 plus the HT_PHASOR unstable period (UnstHTPhasor). HT_PHASOR has
// no optional inputs, so the lookback is never -1.
func HTPHASORLookback() int {
	return htphasorLookback()
}

func htphasorLookback() int {
	return 32 + UnstablePeriod(UnstHTPhasor)
}

// HTPHASOR returns the in-phase and quadrature components of the Hilbert
// transform of inReal, in TA-Lib output order (outInPhase, outQuadrature).
// Both outputs are index-aligned with inReal; the first HTPHASORLookback bars
// are NaN.
func HTPHASOR(inReal []float64) (outInPhase, outQuadrature []float64, err error) {
	n, err := checkInputs("HT_PHASOR", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, err
	}
	outInPhase = make([]float64, n)
	outQuadrature = make([]float64, n)
	if n == 0 {
		return outInPhase, outQuadrature, nil
	}
	beg, nb, err := htphasorRange(0, n-1, inReal, outInPhase, outQuadrature)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outInPhase, beg, nb)
	alignReal(outQuadrature, beg, nb)
	return outInPhase, outQuadrature, nil
}

// htphasorRange mirrors TA_HT_PHASOR: it computes [startIdx, endIdx] and
// writes outNBElement values to outInPhase[0:] and outQuadrature[0:], the
// first one belonging to bar outBegIdx. outInPhase and outQuadrature must not
// be the same buffer (TA_BAD_PARAM in C).
func htphasorRange(startIdx, endIdx int, inReal []float64, outInPhase, outQuadrature []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HT_PHASOR", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HT_PHASOR", endIdx-startIdx, []string{"outInPhase", "outQuadrature"}, outInPhase, outQuadrature); err != nil {
		return 0, 0, err
	}
	// C: outInPhase == outQuadrature -> TA_BAD_PARAM.
	if &outInPhase[0] == &outQuadrature[0] {
		return 0, 0, badParam("HT_PHASOR", "outQuadrature", "must not be the same buffer as outInPhase")
	}
	return htphasorCore(startIdx, endIdx, inReal, outInPhase, outQuadrature)
}

// htphasorCore is the TA_HT_PHASOR body after the validation prologue.
//
// The C library is built with -ffp-contract=off: only its explicit fma()
// calls are fused (ported as math.FMA). Every other product that feeds an
// addition is wrapped in float64(...), which forbids Go from fusing it on
// arm64.
func htphasorCore(startIdx, endIdx int, inReal []float64, outInPhase, outQuadrature []float64) (int, int, error) {
	const a = 0.0962
	const b = 0.5769
	rad2Deg := 180.0 / (4.0 * libmAtan(1))

	lookbackTotal := 32 + UnstablePeriod(UnstHTPhasor)
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
	i1ForOddPrev3, i1ForEvenPrev3 := 0.0, 0.0
	i1ForOddPrev2, i1ForEvenPrev2 := 0.0, 0.0

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

			if today >= startIdx {
				outQuadrature[outIdx] = q1
				outInPhase[outIdx] = i1ForEvenPrev3
				outIdx++
			}

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

			if today >= startIdx {
				outQuadrature[outIdx] = q1
				outInPhase[outIdx] = i1ForOddPrev3
				outIdx++
			}

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

			// Save the current detrender for the even logic.
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
