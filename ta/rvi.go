package ta

import "math"

// RVI — Relative Volatility Index (Volatility Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/rvi and src/ta_func/ta_RVI.c.
//
// The per-bar sigma is VAR's shifted-sums / reseed step, transcribed as the C
// does. The C library is built with -ffp-contract=off; Go may fuse x*y+z into
// an FMA, so every product that feeds an addition is wrapped in an explicit
// float64() conversion, which the spec guarantees rounds and therefore blocks
// fusion. The Wilder steps use C's explicit fma(), matched by math.FMA.

// RVILookback returns the number of leading bars RVI leaves empty:
// optInStdDevPeriod-1 + optInTimePeriod-1 plus the RVI unstable period.
// optInTimePeriod: [1, 100000], default 14. optInStdDevPeriod: [2, 100000], default 10.
func RVILookback(optInTimePeriod, optInStdDevPeriod int) int {
	if checkInt("RVI", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	if checkInt("RVI", "optInStdDevPeriod", optInStdDevPeriod, 2, 100000) != nil {
		return -1
	}
	return rviLookback(optInTimePeriod, optInStdDevPeriod)
}

func rviLookback(optInTimePeriod, optInStdDevPeriod int) int {
	return optInStdDevPeriod - 1 + (optInTimePeriod - 1) + UnstablePeriod(UnstRVI)
}

// RVI returns Dorsey's Relative Volatility Index of inReal: Wilder-smoothed
// (optInTimePeriod) population standard deviation over optInStdDevPeriod
// bars, split into up and down legs by the sign of the close-to-close change,
// as 100*up/(up+down) (50 when both legs are zero). The output is
// index-aligned with inReal; the first RVILookback bars are NaN.
// optInTimePeriod: [1, 100000], default 14. optInStdDevPeriod: [2, 100000], default 10.
func RVI(inReal []float64, optInTimePeriod, optInStdDevPeriod int) ([]float64, error) {
	if err := checkInt("RVI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	if err := checkInt("RVI", "optInStdDevPeriod", optInStdDevPeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("RVI", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := rviRange(0, n-1, inReal, optInTimePeriod, optInStdDevPeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// rviRange mirrors TA_RVI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal must not alias inReal (C reads inReal behind the write
// position).
func rviRange(startIdx, endIdx int, inReal []float64, optInTimePeriod, optInStdDevPeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("RVI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("RVI", "optInStdDevPeriod", optInStdDevPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("RVI", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("RVI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return rviCore(startIdx, endIdx, inReal, optInTimePeriod, optInStdDevPeriod, outReal)
}

func rviCore(startIdx, endIdx int, inReal []float64, optInTimePeriod, optInStdDevPeriod int, outReal []float64) (int, int, error) {
	// C reads the unstable period twice (lookback and skip loop); one read
	// here keeps both uses consistent under concurrent SetUnstablePeriod.
	unstable := UnstablePeriod(UnstRVI)
	lookbackTotal := optInStdDevPeriod - 1 + (optInTimePeriod - 1) + unstable
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// wAlpha is derived from wBeta so the pair sums to exactly 1.
	wBeta := float64(optInTimePeriod-1) / float64(optInTimePeriod)
	wAlpha := 1.0 - wBeta

	nbInitialElementNeeded := optInStdDevPeriod - 1
	invPeriod := 1.0 / float64(optInStdDevPeriod)
	anchorIdx := startIdx - lookbackTotal
	trailingIdx := anchorIdx
	shift := inReal[anchorIdx]
	periodTotal1 := 0.0
	periodTotal2 := 0.0
	today := anchorIdx + nbInitialElementNeeded
	var tempReal float64
	for j := anchorIdx; j < today; j++ {
		tempReal = inReal[j] - shift
		periodTotal1 += tempReal
		tempReal = float64(tempReal * tempReal)
		periodTotal2 += tempReal
	}
	barsSinceReseed := 32 * optInStdDevPeriod

	// sigmaStep is the per-bar body shared by the three C loops (seed, unstable
	// skip, output): add inReal[today], take the variance, remove the trailing
	// value, reseed when needed, and return sqrt(variance).
	sigmaStep := func() float64 {
		tempReal = inReal[today] - shift
		periodTotal1 += tempReal
		tempReal = float64(tempReal * tempReal)
		periodTotal2 += tempReal
		meanValue1 := float64(periodTotal1 * invPeriod)
		variance := float64(periodTotal2*invPeriod) - float64(meanValue1*meanValue1)
		tempReal = inReal[trailingIdx] - shift
		periodTotal1 -= tempReal
		tempReal = float64(tempReal * tempReal)
		periodTotal2 -= tempReal
		trailingIdx++
		barsSinceReseed--
		if variance < float64(0.000001*float64(periodTotal2*invPeriod)) ||
			tempReal > float64(1000000.0*periodTotal2) || barsSinceReseed <= 0 {
			barsSinceReseed = 32 * optInStdDevPeriod
			windowStart := today - nbInitialElementNeeded
			tempReal = 0.0
			for j := windowStart; j <= today; j++ {
				tempReal += inReal[j]
			}
			shift = float64(tempReal * invPeriod)
			periodTotal1 = 0.0
			periodTotal2 = 0.0
			for j := windowStart; j <= today; j++ {
				tempReal = inReal[j] - shift
				periodTotal1 += tempReal
				tempReal = float64(tempReal * tempReal)
				periodTotal2 += tempReal
			}
			meanValue1 = float64(periodTotal1 * invPeriod)
			variance = float64(periodTotal2*invPeriod) - float64(meanValue1*meanValue1)
			if variance < float64(0.000000000001*float64(periodTotal2*invPeriod)) {
				variance = 0.0
			}
			tempReal = inReal[windowStart] - shift
			periodTotal1 -= tempReal
			tempReal = float64(tempReal * tempReal)
			periodTotal2 -= tempReal
		}
		return math.Sqrt(variance)
	}

	// Seed both legs with the simple average of the first optInTimePeriod
	// volatilities. optInStdDevPeriod >= 2 keeps inReal[today-1] in bounds.
	upTotal := 0.0
	dnTotal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		sigma := sigmaStep()
		delta := inReal[today] - inReal[today-1]
		if delta > 0.0 {
			upTotal += sigma
		} else if delta < 0.0 {
			dnTotal += sigma
		}
		today++
	}
	prevUp := upTotal / float64(optInTimePeriod)
	prevDn := dnTotal / float64(optInTimePeriod)

	// wilder is one smoothing step of both legs (C: explicit fma()).
	wilder := func() {
		sigma := sigmaStep()
		delta := inReal[today] - inReal[today-1]
		upValue := 0.0
		dnValue := 0.0
		if delta > 0.0 {
			upValue = sigma
		} else if delta < 0.0 {
			dnValue = sigma
		}
		prevUp = math.FMA(wBeta, prevUp, float64(wAlpha*upValue))
		prevDn = math.FMA(wBeta, prevDn, float64(wAlpha*dnValue))
		today++
	}

	// Skip the unstable period: smoothed but not stored.
	for i := unstable; i != 0; i-- {
		wilder()
	}

	total := prevUp + prevDn
	if total == 0.0 {
		outReal[0] = 50.0
	} else {
		outReal[0] = 100.0 * (prevUp / total)
	}
	outIdx := 1
	for today <= endIdx {
		wilder()
		total = prevUp + prevDn
		if total == 0.0 {
			outReal[outIdx] = 50.0
		} else {
			outReal[outIdx] = 100.0 * (prevUp / total)
		}
		outIdx++
	}
	return startIdx, outIdx, nil
}
