package ta

import "math"

// SMI — Stochastic Momentum Index (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/smi and src/ta_func/ta_SMI.c.

// SMILookback returns the number of leading bars SMI leaves empty:
// optInTimePeriod-1 plus the EMA lookbacks of the slow, fast and signal
// periods (so it includes the EMA unstable period three times).
// All periods: [2, 100000]; defaults 13, 2, 25, 9.
func SMILookback(optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) int {
	if smiCheckParams(optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod) != nil {
		return -1
	}
	return smiLookback(optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod)
}

func smiLookback(optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) int {
	return optInTimePeriod - 1 + emaLookback(optInSlowPeriod) + emaLookback(optInFastPeriod) + emaLookback(optInSignalPeriod)
}

// smiCheckParams validates the optional inputs in C prologue order.
func smiCheckParams(optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) error {
	if err := checkInt("SMI", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return err
	}
	if err := checkInt("SMI", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return err
	}
	if err := checkInt("SMI", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return err
	}
	return checkInt("SMI", "optInSignalPeriod", optInSignalPeriod, 2, 100000)
}

// SMI returns Blau's Stochastic Momentum Index and its signal line:
// the close relative to the midpoint of the optInTimePeriod high/low range,
// numerator and denominator each smoothed by EMA(slow) then EMA(fast),
// scaled to [-100, 100]; the signal is EMA(SMI, signal). A window with a
// zero smoothed range yields 0. Outputs are index-aligned with the inputs;
// the first SMILookback bars are NaN.
// All periods: [2, 100000]; defaults 13, 2, 25, 9.
func SMI(inHigh, inLow, inClose []float64, optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) (outSMI, outSMISignal []float64, err error) {
	if err := smiCheckParams(optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod); err != nil {
		return nil, nil, err
	}
	n, err := checkInputs("SMI", []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose)
	if err != nil {
		return nil, nil, err
	}
	outSMI = make([]float64, n)
	outSMISignal = make([]float64, n)
	if n == 0 {
		return outSMI, outSMISignal, nil
	}
	beg, nb, err := smiRange(0, n-1, inHigh, inLow, inClose, optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod, outSMI, outSMISignal)
	if err != nil {
		return nil, nil, err
	}
	alignReal(outSMI, beg, nb)
	alignReal(outSMISignal, beg, nb)
	return outSMI, outSMISignal, nil
}

// smiRange mirrors TA_SMI: it computes [startIdx, endIdx] and writes
// outNBElement values to outSMI[0:] and outSMISignal[0:], the first pair
// belonging to bar outBegIdx. The two outputs must not be the same buffer.
func smiRange(startIdx, endIdx int, inHigh, inLow, inClose []float64, optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int, outSMI, outSMISignal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := smiCheckParams(optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SMI", endIdx, []string{"inHigh", "inLow", "inClose"}, inHigh, inLow, inClose); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SMI", endIdx-startIdx, []string{"outSMI", "outSMISignal"}, outSMI, outSMISignal); err != nil {
		return 0, 0, err
	}
	// C: if( outSMI == outSMISignal ) return TA_BAD_PARAM;
	if &outSMI[0] == &outSMISignal[0] {
		return 0, 0, badParam("SMI", "outSMISignal", "must not be the same buffer as outSMI")
	}

	lookbackTotal := smiLookback(optInTimePeriod, optInFastPeriod, optInSlowPeriod, optInSignalPeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	outBegIdx = startIdx

	kSlow := 2.0 / float64(optInSlowPeriod+1)
	kFast := 2.0 / float64(optInFastPeriod+1)
	kSignal := 2.0 / float64(optInSignalPeriod+1)
	lookbackSlow := emaLookback(optInSlowPeriod)
	lookbackFast := emaLookback(optInFastPeriod)
	var (
		emaSlowNum, emaSlowDen, emaFastNum, emaFastDen float64
		prevSignal, smiValue                           float64
		sumSlowNum, sumSlowDen, sumFastNum, sumFastDen float64
		sumSignal, highest, lowest, tmp                float64
		num, den, halfDen                              float64
	)
	highestIdx := -1
	lowestIdx := -1

	trailingIdx := startIdx - lookbackTotal
	today := trailingIdx + (optInTimePeriod - 1)
	nBar := 0

	// window updates the running lowest low / highest high exactly as the
	// two inlined blocks in C do.
	window := func() {
		tmp = inLow[today]
		if lowestIdx < trailingIdx {
			lowestIdx = trailingIdx
			lowest = inLow[lowestIdx]
			for i := lowestIdx + 1; i <= today; i++ {
				tmp = inLow[i]
				if tmp < lowest {
					lowestIdx = i
					lowest = tmp
				}
			}
		} else if tmp <= lowest {
			lowestIdx = today
			lowest = tmp
		}
		tmp = inHigh[today]
		if highestIdx < trailingIdx {
			highestIdx = trailingIdx
			highest = inHigh[highestIdx]
			for i := highestIdx + 1; i <= today; i++ {
				tmp = inHigh[i]
				if tmp > highest {
					highestIdx = i
					highest = tmp
				}
			}
		} else if tmp >= highest {
			highestIdx = today
			highest = tmp
		}
		den = highest - lowest
		num = inClose[today] - float64((highest+lowest)*0.5)
	}

	// Warm-up, through startIdx inclusive.
	for today <= startIdx {
		window()

		// Stage 1: slow EMA over the raw momentum.
		if nBar < optInSlowPeriod {
			sumSlowNum = sumSlowNum + num
			sumSlowDen = sumSlowDen + den
			if nBar == optInSlowPeriod-1 {
				emaSlowNum = sumSlowNum / float64(optInSlowPeriod)
				emaSlowDen = sumSlowDen / float64(optInSlowPeriod)
			}
		} else {
			emaSlowNum = math.FMA(num-emaSlowNum, kSlow, emaSlowNum)
			emaSlowDen = math.FMA(den-emaSlowDen, kSlow, emaSlowDen)
		}

		// Stage 2: fast EMA over what stage 1 publishes.
		if nBar >= lookbackSlow {
			nFast := nBar - lookbackSlow
			if nFast < optInFastPeriod {
				sumFastNum = sumFastNum + emaSlowNum
				sumFastDen = sumFastDen + emaSlowDen
				if nFast == optInFastPeriod-1 {
					emaFastNum = sumFastNum / float64(optInFastPeriod)
					emaFastDen = sumFastDen / float64(optInFastPeriod)
				}
			} else {
				emaFastNum = math.FMA(emaSlowNum-emaFastNum, kFast, emaFastNum)
				emaFastDen = math.FMA(emaSlowDen-emaFastDen, kFast, emaFastDen)
			}
		}

		// Stage 3: the SMI line, then the signal EMA over it.
		if nBar >= lookbackSlow+lookbackFast {
			nSignal := nBar - lookbackSlow - lookbackFast
			halfDen = 0.5 * emaFastDen
			if halfDen > 0.0 {
				smiValue = 100.0 * emaFastNum / halfDen
			} else {
				smiValue = 0.0
			}
			if nSignal < optInSignalPeriod {
				sumSignal = sumSignal + smiValue
				if nSignal == optInSignalPeriod-1 {
					prevSignal = sumSignal / float64(optInSignalPeriod)
				}
			} else {
				prevSignal = math.FMA(smiValue-prevSignal, kSignal, prevSignal)
			}
		}

		nBar++
		trailingIdx++
		today++
	}
	outSMI[0] = smiValue
	outSMISignal[0] = prevSignal
	outIdx := 1

	// Stable zone: every stage is a pure recursion.
	for today <= endIdx {
		window()
		emaSlowNum = math.FMA(num-emaSlowNum, kSlow, emaSlowNum)
		emaSlowDen = math.FMA(den-emaSlowDen, kSlow, emaSlowDen)
		emaFastNum = math.FMA(emaSlowNum-emaFastNum, kFast, emaFastNum)
		emaFastDen = math.FMA(emaSlowDen-emaFastDen, kFast, emaFastDen)
		halfDen = 0.5 * emaFastDen
		if halfDen > 0.0 {
			smiValue = 100.0 * emaFastNum / halfDen
		} else {
			smiValue = 0.0
		}
		prevSignal = math.FMA(smiValue-prevSignal, kSignal, prevSignal)
		outSMI[outIdx] = smiValue
		outSMISignal[outIdx] = prevSignal
		outIdx++
		trailingIdx++
		today++
	}
	return outBegIdx, outIdx, nil
}
