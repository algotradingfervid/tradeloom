package ta

import "math"

// HT_TRENDMODE — Hilbert Transform - Trend vs Cycle Mode (Cycle Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ht_trendmode and
// src/ta_func/ta_HT_TRENDMODE.c.

// HTTRENDMODELookback returns the number of leading bars HTTRENDMODE leaves
// empty: 63 plus the UnstHTTrendMode unstable period.
func HTTRENDMODELookback() int {
	return httrendmodeLookback()
}

func httrendmodeLookback() int {
	return 63 + UnstablePeriod(UnstHTTrendMode)
}

// HTTRENDMODE labels each bar of inReal 1 (trend mode) or 0 (cycle mode).
// The output is index-aligned with inReal; the first HTTRENDMODELookback
// bars are 0.
func HTTRENDMODE(inReal []float64) ([]int, error) {
	n, err := checkInputs("HT_TRENDMODE", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]int, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := httrendmodeRange(0, n-1, inReal, out)
	if err != nil {
		return nil, err
	}
	alignInt(out, beg, nb)
	return out, nil
}

// httrendmodeRange mirrors TA_HT_TRENDMODE: it computes [startIdx, endIdx]
// and writes outNBElement values to outInteger[0:], the first one belonging
// to bar outBegIdx.
func httrendmodeRange(startIdx, endIdx int, inReal []float64, outInteger []int) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("HT_TRENDMODE", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkIntBuf("HT_TRENDMODE", endIdx-startIdx, "outInteger", outInteger); err != nil {
		return 0, 0, err
	}
	return httrendmodeCore(startIdx, endIdx, inReal, outInteger)
}

// httrendmodeCore is the TA_HT_TRENDMODE body after the validation prologue.
//
// The C library is built with -ffp-contract=off: only its explicit fma()
// calls are fused (ported as math.FMA). Every other product that feeds an
// addition or subtraction is wrapped in float64(...), which forbids Go from
// fusing it on arm64.
func httrendmodeCore(startIdx, endIdx int, inReal []float64, outInteger []int) (int, int, error) {
	const a = 0.0962
	const b = 0.5769
	const smoothPriceLen = 50
	// Runtime values, not untyped constants: C computes 0.67 * 360.0 in
	// double precision, while a Go constant expression would be exact.
	c067, c15, c360 := 0.67, 1.5, 360.0

	var smoothPrice [smoothPriceLen]float64
	maxIdxSmoothPrice := smoothPriceLen - 1
	smoothPriceIdx := 0

	iTrend1, iTrend2, iTrend3 := 0.0, 0.0, 0.0
	daysInTrend := 0
	dcPhase := 0.0
	prevDCPhase := 0.0
	sine, prevSine := 0.0, 0.0
	leadSine, prevLeadSine := 0.0, 0.0

	tempReal := libmAtan(1)
	rad2Deg := 45.0 / tempReal
	deg2Rad := 1.0 / rad2Deg
	constDeg2RadBy360 := tempReal * 8.0

	lookbackTotal := 63 + UnstablePeriod(UnstHTTrendMode)
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
	dcPhase = 0.0

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
		smoothPeriod = math.FMA(0.67, smoothPeriod, 0.33*period)

		// Dominant cycle phase.
		prevDCPhase = dcPhase
		dcPeriod := smoothPeriod + 0.5
		dcPeriodInt := int(dcPeriod)
		realPart := 0.0
		imagPart := 0.0

		idx := smoothPriceIdx
		for i := 0; i < dcPeriodInt; i++ {
			tempReal = float64(i) * constDeg2RadBy360 / float64(dcPeriodInt)
			tempReal2 = smoothPrice[idx]
			sinV, cosV := htSinCos(i, dcPeriodInt, tempReal)
			realPart += float64(sinV * tempReal2)
			imagPart += float64(cosV * tempReal2)
			if idx == 0 {
				idx = smoothPriceLen - 1
			} else {
				idx--
			}
		}

		tempReal = math.Abs(imagPart)
		if tempReal > 0.0 {
			dcPhase = libmAtan(realPart/imagPart) * rad2Deg
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

		prevSine = sine
		prevLeadSine = leadSine
		sine = libmSin(dcPhase * deg2Rad)
		leadSine = libmSin((dcPhase + 45) * deg2Rad)

		// Trendline: average of the raw price over the dominant cycle period.
		dcPeriod = smoothPeriod + 0.5
		dcPeriodInt = int(dcPeriod)
		tempReal = 0.0
		for j := 0; j < smoothPriceLen; j++ {
			if j < dcPeriodInt {
				tempReal += inReal[today-j]
			}
		}
		if dcPeriodInt > 0 {
			tempReal = tempReal / float64(dcPeriodInt)
		}
		trendline := (math.FMA(2.0, iTrend2, math.FMA(4.0, tempReal, 3.0*iTrend1)) + iTrend3) / 10.0
		iTrend3 = iTrend2
		iTrend2 = iTrend1
		iTrend1 = tempReal

		// Trend mode, trend by default.
		trend := 1

		// Days in trend since the last crossing of the SineWave lines.
		if (sine > leadSine && prevSine <= prevLeadSine) || (sine < leadSine && prevSine >= prevLeadSine) {
			daysInTrend = 0
			trend = 0
		}
		daysInTrend++
		if float64(daysInTrend) < float64(0.5*smoothPeriod) {
			trend = 0
		}

		tempReal = dcPhase - prevDCPhase
		if smoothPeriod != 0.0 && (tempReal > float64(c067*c360)/smoothPeriod && tempReal < float64(c15*c360)/smoothPeriod) {
			trend = 0
		}

		tempReal = smoothPrice[smoothPriceIdx]
		if trendline != 0.0 && math.Abs((tempReal-trendline)/trendline) >= 0.015 {
			trend = 1
		}

		if today >= startIdx {
			outInteger[outIdx] = trend
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
