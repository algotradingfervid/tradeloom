package ta

import "math"

// BBANDS — Bollinger Bands (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/bbands and src/ta_func/ta_BBANDS.c.
//
// The C library is built with -ffp-contract=off. Go may fuse x*y+z into an
// FMA (also across statements), so every product that feeds an addition or
// subtraction is wrapped in an explicit float64() conversion, which the spec
// guarantees rounds and therefore blocks fusion. Where C calls fma()
// explicitly (the upper band with distinct multipliers), math.FMA is used.

// bbandsCheck validates the BBANDS optional inputs and applies the C
// prologue's MATypeDefault substitution (TA_MAType_DEFAULT -> SMA).
func bbandsCheck(optInTimePeriod int, optInNbDevUp, optInNbDevDn float64, optInMAType MAType) (MAType, error) {
	if err := checkInt("BBANDS", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return optInMAType, err
	}
	if err := checkReal("BBANDS", "optInNbDevUp", optInNbDevUp, taRealMin, taRealMax); err != nil {
		return optInMAType, err
	}
	if err := checkReal("BBANDS", "optInNbDevDn", optInNbDevDn, taRealMin, taRealMax); err != nil {
		return optInMAType, err
	}
	if err := checkMAType("BBANDS", "optInMAType", optInMAType); err != nil {
		return optInMAType, err
	}
	if optInMAType == MATypeDefault {
		optInMAType = MATypeSMA
	}
	return optInMAType, nil
}

// BBANDSLookback returns the number of leading bars BBANDS leaves empty: the
// larger of the moving-average and standard-deviation lookbacks. It returns
// -1 on an invalid parameter.
// optInTimePeriod: [2, 100000], default 20. optInNbDevUp, optInNbDevDn:
// [-3e37, 3e37], default 2. optInMAType: default SMA.
func BBANDSLookback(optInTimePeriod int, optInNbDevUp, optInNbDevDn float64, optInMAType MAType) int {
	t, err := bbandsCheck(optInTimePeriod, optInNbDevUp, optInNbDevDn, optInMAType)
	if err != nil {
		return -1
	}
	return bbandsLookback(optInTimePeriod, optInNbDevUp, optInNbDevDn, t)
}

// bbandsLookback is the unvalidated tail of TA_BBANDS_Lookback; optInMAType
// must already have MATypeDefault substituted.
func bbandsLookback(optInTimePeriod int, optInNbDevUp, optInNbDevDn float64, optInMAType MAType) int {
	maLb := maLookback(optInTimePeriod, optInMAType)
	stddevLb := stddevLookback(optInTimePeriod, 1.0)
	if maLb > stddevLb {
		return maLb
	}
	return stddevLb
}

// BBANDS returns the upper, middle and lower Bollinger Bands of inReal: the
// middle band is the optInMAType moving average over optInTimePeriod bars and
// the outer bands are offset by optInNbDevUp / optInNbDevDn population
// standard deviations. The outputs are index-aligned with inReal; the first
// BBANDSLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 20. optInNbDevUp, optInNbDevDn:
// [-3e37, 3e37], default 2. optInMAType: default SMA.
func BBANDS(inReal []float64, optInTimePeriod int, optInNbDevUp, optInNbDevDn float64, optInMAType MAType) (outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64, err error) {
	if _, err := bbandsCheck(optInTimePeriod, optInNbDevUp, optInNbDevDn, optInMAType); err != nil {
		return nil, nil, nil, err
	}
	n, err := checkInputs("BBANDS", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, nil, err
	}
	upper := make([]float64, n)
	middle := make([]float64, n)
	lower := make([]float64, n)
	if n == 0 {
		return upper, middle, lower, nil
	}
	beg, nb, err := bbandsRange(0, n-1, inReal, optInTimePeriod, optInNbDevUp, optInNbDevDn, optInMAType, upper, middle, lower)
	if err != nil {
		return nil, nil, nil, err
	}
	alignReal(upper, beg, nb)
	alignReal(middle, beg, nb)
	alignReal(lower, beg, nb)
	return upper, middle, lower, nil
}

// bbandsSame reports whether two (non-empty) buffers start at the same
// element: the Go equivalent of C's pointer comparison of two arrays.
func bbandsSame(a, b []float64) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}

// bbandsRange mirrors TA_BBANDS: it computes [startIdx, endIdx] and writes
// outNBElement values to each output's [0:], the first one belonging to bar
// outBegIdx. inReal may be one of the outputs; two outputs may not be the
// same buffer (ErrBadParam, as in C).
func bbandsRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, optInNbDevUp, optInNbDevDn float64, optInMAType MAType, outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	optInMAType, err = bbandsCheck(optInTimePeriod, optInNbDevUp, optInNbDevDn, optInMAType)
	if err != nil {
		return 0, 0, err
	}
	if err := checkBuf("BBANDS", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("BBANDS", endIdx-startIdx, []string{"outRealUpperBand", "outRealMiddleBand", "outRealLowerBand"},
		outRealUpperBand, outRealMiddleBand, outRealLowerBand); err != nil {
		return 0, 0, err
	}
	if bbandsSame(outRealUpperBand, outRealMiddleBand) || bbandsSame(outRealUpperBand, outRealLowerBand) ||
		bbandsSame(outRealMiddleBand, outRealLowerBand) {
		return 0, 0, badParam("BBANDS", "outReal*Band", "two outputs share one buffer")
	}

	if optInMAType == MATypeSMA {
		return bbandsSMA(startIdx, endIdx, inReal, optInTimePeriod, optInNbDevUp, optInNbDevDn,
			outRealUpperBand, outRealMiddleBand, outRealLowerBand)
	}

	// General path (every MA type other than SMA): the middle band is the
	// moving average and the deviation is the standard deviation of the input,
	// combined at the same bar.

	// Nothing to produce: the range is shorter than the lookback.
	if bbandsLookback(optInTimePeriod, optInNbDevUp, optInNbDevDn, optInMAType) > endIdx {
		return 0, 0, nil
	}
	tempBuffer1 := make([]float64, endIdx-startIdx+1)
	tempBuffer2 := make([]float64, endIdx-startIdx+1)

	// Calculate the middle band moving average.
	outBegIdx, outNBElement, err = maRange(startIdx, endIdx, inReal, optInTimePeriod, optInMAType, tempBuffer1)
	if err != nil || outNBElement == 0 {
		return outBegIdx, 0, err
	}
	// Remember where the moving average begins, to realign it below.
	maBegIdx := outBegIdx
	// Calculate the standard deviation into tempBuffer2.
	outBegIdx, outNBElement, err = stddevRange(outBegIdx, endIdx, inReal, optInTimePeriod, 1.0, tempBuffer2)
	if err != nil {
		return outBegIdx, 0, err
	}
	// Realign the MA to the (possibly later) begIdx of the deviation.
	shiftIdx := 0
	if outBegIdx > maBegIdx {
		shiftIdx = outBegIdx - maBegIdx
	}
	copy(outRealMiddleBand[:outNBElement], tempBuffer1[shiftIdx:shiftIdx+outNBElement])

	// Now do a tight loop to calculate the upper/lower band at the same time.
	if optInNbDevUp == optInNbDevDn {
		for i := 0; i < outNBElement; i++ {
			tempReal := float64(tempBuffer2[i] * optInNbDevUp)
			tempReal2 := outRealMiddleBand[i]
			outRealUpperBand[i] = tempReal2 + tempReal
			outRealLowerBand[i] = tempReal2 - tempReal
		}
	} else {
		for i := 0; i < outNBElement; i++ {
			tempReal2 := outRealMiddleBand[i]
			// C calls fma() explicitly; math.FMA matches it bit for bit.
			outRealUpperBand[i] = math.FMA(tempBuffer2[i], optInNbDevUp, tempReal2)
			outRealLowerBand[i] = tempReal2 - float64(tempBuffer2[i]*optInNbDevDn)
		}
	}
	return outBegIdx, outNBElement, nil
}

// bbandsSMA is the TA_BBANDS SMA fast path: the SMA running sum and the
// shifted-data variance (bit-identical to TA_VAR) share one pass.
func bbandsSMA(startIdx, endIdx int, inReal []float64, optInTimePeriod int, optInNbDevUp, optInNbDevDn float64, outRealUpperBand, outRealMiddleBand, outRealLowerBand []float64) (int, int, error) {
	// Pick two temporary buffers among the outputs so no allocation is
	// needed; whenever possible tempBuffer1 is the middle band itself.
	var tempBuffer1, tempBuffer2 []float64
	switch {
	case bbandsSame(inReal, outRealUpperBand):
		tempBuffer1 = outRealMiddleBand
		tempBuffer2 = outRealLowerBand
	case bbandsSame(inReal, outRealLowerBand):
		tempBuffer1 = outRealMiddleBand
		tempBuffer2 = outRealUpperBand
	case bbandsSame(inReal, outRealMiddleBand):
		tempBuffer1 = outRealLowerBand
		tempBuffer2 = outRealUpperBand
	default:
		tempBuffer1 = outRealMiddleBand
		tempBuffer2 = outRealUpperBand
	}
	if bbandsSame(tempBuffer1, inReal) || bbandsSame(tempBuffer2, inReal) {
		return 0, 0, badParam("BBANDS", "inReal", "input shares a buffer with two outputs")
	}

	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	invPeriod := 1.0 / float64(optInTimePeriod)
	trailingIdx := startIdx - lookbackTotal
	shift := inReal[trailingIdx]
	maTotal := 0.0
	varTotal1 := 0.0
	varTotal2 := 0.0
	var tempReal float64
	for j := trailingIdx; j < startIdx; j++ {
		maTotal += inReal[j]
		tempReal = inReal[j] - shift
		varTotal1 += tempReal
		tempReal = float64(tempReal * tempReal)
		varTotal2 += tempReal
	}
	i := startIdx
	outIdx := 0
	barsSinceReseed := 32 * optInTimePeriod
	for {
		maTotal += inReal[i]
		tempReal = inReal[i] - shift
		varTotal1 += tempReal
		tempReal = float64(tempReal * tempReal)
		varTotal2 += tempReal
		meanValue1 := float64(varTotal1 * invPeriod)
		variance := float64(varTotal2*invPeriod) - float64(meanValue1*meanValue1)
		tempBuffer1[outIdx] = maTotal / float64(optInTimePeriod)
		maTotal -= inReal[trailingIdx]
		tempReal = inReal[trailingIdx] - shift
		varTotal1 -= tempReal
		tempReal = float64(tempReal * tempReal)
		varTotal2 -= tempReal
		trailingIdx++
		barsSinceReseed--
		if variance < float64(0.000001*float64(varTotal2*invPeriod)) ||
			tempReal > float64(1000000.0*varTotal2) || barsSinceReseed <= 0 {
			barsSinceReseed = 32 * optInTimePeriod
			windowStart := i - lookbackTotal
			tempReal = 0.0
			for j := windowStart; j <= i; j++ {
				tempReal += inReal[j]
			}
			shift = float64(tempReal * invPeriod)
			varTotal1 = 0.0
			varTotal2 = 0.0
			for j := windowStart; j <= i; j++ {
				tempReal = inReal[j] - shift
				varTotal1 += tempReal
				tempReal = float64(tempReal * tempReal)
				varTotal2 += tempReal
			}
			meanValue1 = float64(varTotal1 * invPeriod)
			variance = float64(varTotal2*invPeriod) - float64(meanValue1*meanValue1)
			// The floor from var.c, verbatim.
			if variance < float64(0.000000000001*float64(varTotal2*invPeriod)) {
				variance = 0.0
			}
			tempReal = inReal[windowStart] - shift
			varTotal1 -= tempReal
			tempReal = float64(tempReal * tempReal)
			varTotal2 -= tempReal
		}
		// `!= 0.0`, not `> 0.0`: NaN must reach sqrt and stay NaN, as in C.
		if variance != 0.0 {
			tempBuffer2[outIdx] = math.Sqrt(variance)
		} else {
			tempBuffer2[outIdx] = 0.0
		}
		outIdx++
		i++
		if i > endIdx {
			break
		}
	}
	outNBElement := outIdx
	outBegIdx := startIdx
	// Copy the MA into the middle band output, unless it is already there.
	if !bbandsSame(tempBuffer1, outRealMiddleBand) {
		copy(outRealMiddleBand[:outNBElement], tempBuffer1[:outNBElement])
	}
	// Now do a tight loop to calculate the upper/lower band at the same time.
	if optInNbDevUp == optInNbDevDn {
		for k := 0; k < outNBElement; k++ {
			tr := float64(tempBuffer2[k] * optInNbDevUp)
			tr2 := outRealMiddleBand[k]
			outRealUpperBand[k] = tr2 + tr
			outRealLowerBand[k] = tr2 - tr
		}
	} else {
		for k := 0; k < outNBElement; k++ {
			tr := tempBuffer2[k]
			tr2 := outRealMiddleBand[k]
			// C calls fma() explicitly; math.FMA matches it bit for bit.
			outRealUpperBand[k] = math.FMA(tr, optInNbDevUp, tr2)
			outRealLowerBand[k] = tr2 - float64(tr*optInNbDevDn)
		}
	}
	return outBegIdx, outNBElement, nil
}
