package ta

import "math"

// SAREXT — Parabolic SAR - Extended (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/sarext and src/ta_func/ta_SAREXT.c.
//
// Unlike SAR, SAREXT returns negative values while the position is short, so a
// reversal is visible as a sign change.

// sarextValidate checks every optional input against its yaml range, in C
// prologue order.
func sarextValidate(optInStartValue, optInOffsetOnReverse,
	optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
	optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort float64) error {
	if err := checkReal("SAREXT", "optInStartValue", optInStartValue, taRealMin, taRealMax); err != nil {
		return err
	}
	if err := checkReal("SAREXT", "optInOffsetOnReverse", optInOffsetOnReverse, 0, taRealMax); err != nil {
		return err
	}
	if err := checkReal("SAREXT", "optInAccelerationInitLong", optInAccelerationInitLong, 0, taRealMax); err != nil {
		return err
	}
	if err := checkReal("SAREXT", "optInAccelerationLong", optInAccelerationLong, 0, taRealMax); err != nil {
		return err
	}
	if err := checkReal("SAREXT", "optInAccelerationMaxLong", optInAccelerationMaxLong, 0, taRealMax); err != nil {
		return err
	}
	if err := checkReal("SAREXT", "optInAccelerationInitShort", optInAccelerationInitShort, 0, taRealMax); err != nil {
		return err
	}
	if err := checkReal("SAREXT", "optInAccelerationShort", optInAccelerationShort, 0, taRealMax); err != nil {
		return err
	}
	return checkReal("SAREXT", "optInAccelerationMaxShort", optInAccelerationMaxShort, 0, taRealMax)
}

// SAREXTLookback returns the number of leading bars SAREXT leaves empty
// (always 1), or -1 if a parameter is invalid.
// optInStartValue: [-3e37, 3e37], default 0 (0 = auto, >0 long, <0 short).
// optInOffsetOnReverse and every acceleration factor: [0, 3e37]; defaults
// offset 0, init 0.02, step 0.02, max 0.2 for both directions.
func SAREXTLookback(optInStartValue, optInOffsetOnReverse,
	optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
	optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort float64) int {
	if sarextValidate(optInStartValue, optInOffsetOnReverse,
		optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
		optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort) != nil {
		return -1
	}
	return sarextLookback(optInStartValue, optInOffsetOnReverse,
		optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
		optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort)
}

// sarextLookback: SAR always sacrifices one price bar to establish the
// initial extreme price.
func sarextLookback(_, _, _, _, _, _, _, _ float64) int {
	return 1
}

// SAREXT returns the extended Parabolic SAR of (inHigh, inLow). Values are
// positive while long and negative while short. The output is index-aligned
// with the inputs; the first bar is NaN. See SAREXTLookback for the
// parameter ranges.
func SAREXT(inHigh, inLow []float64, optInStartValue, optInOffsetOnReverse,
	optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
	optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort float64) ([]float64, error) {
	if err := sarextValidate(optInStartValue, optInOffsetOnReverse,
		optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
		optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort); err != nil {
		return nil, err
	}
	n, err := checkInputs("SAREXT", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := sarextRange(0, n-1, inHigh, inLow, optInStartValue, optInOffsetOnReverse,
		optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
		optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// sarextRange mirrors TA_SAREXT: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx.
func sarextRange(startIdx, endIdx int, inHigh, inLow []float64, optInStartValue, optInOffsetOnReverse,
	optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
	optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort float64,
	outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := sarextValidate(optInStartValue, optInOffsetOnReverse,
		optInAccelerationInitLong, optInAccelerationLong, optInAccelerationMaxLong,
		optInAccelerationInitShort, optInAccelerationShort, optInAccelerationMaxShort); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SAREXT", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SAREXT", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	if startIdx < 1 {
		startIdx = 1
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Make the acceleration factors coherent with their maximum.
	afLong := optInAccelerationInitLong
	afShort := optInAccelerationInitShort
	if afLong > optInAccelerationMaxLong {
		optInAccelerationInitLong = optInAccelerationMaxLong
		afLong = optInAccelerationInitLong
	}
	if optInAccelerationLong > optInAccelerationMaxLong {
		optInAccelerationLong = optInAccelerationMaxLong
	}
	if afShort > optInAccelerationMaxShort {
		optInAccelerationInitShort = optInAccelerationMaxShort
		afShort = optInAccelerationInitShort
	}
	if optInAccelerationShort > optInAccelerationMaxShort {
		optInAccelerationShort = optInAccelerationMaxShort
	}

	// Initial direction.
	var isLong bool
	if optInStartValue == 0 {
		// Default: -DM between the first two bars decides (tie = long).
		var epTemp [1]float64
		if _, _, err := minusdmRange(startIdx, startIdx, inHigh, inLow, 1, epTemp[:]); err != nil {
			return 0, 0, err
		}
		isLong = !(epTemp[0] > 0)
	} else if optInStartValue > 0 {
		isLong = true
	} else {
		isLong = false
	}

	outBegIdx = startIdx
	outIdx := 0

	// Write the first SAR.
	todayIdx := startIdx
	newHigh := inHigh[todayIdx-1]
	newLow := inLow[todayIdx-1]
	var ep, sar float64
	if optInStartValue == 0 {
		if isLong {
			ep = inHigh[todayIdx]
			sar = newLow
		} else {
			ep = inLow[todayIdx]
			sar = newHigh
		}
	} else if optInStartValue > 0 {
		ep = inHigh[todayIdx]
		sar = optInStartValue
	} else {
		ep = inLow[todayIdx]
		sar = math.Abs(optInStartValue)
	}

	// Cheat on newLow and newHigh for the first iteration.
	newLow = inLow[todayIdx]
	newHigh = inHigh[todayIdx]

	var prevHigh, prevLow float64
	for todayIdx <= endIdx {
		prevLow = newLow
		prevHigh = newHigh
		newLow = inLow[todayIdx]
		newHigh = inHigh[todayIdx]
		todayIdx++

		if isLong {
			if newLow <= sar {
				// Switch to short; override the SAR with the ep.
				isLong = false
				sar = ep
				if sar < prevHigh {
					sar = prevHigh
				}
				if sar < newHigh {
					sar = newHigh
				}
				if optInOffsetOnReverse != 0.0 {
					sar += float64(sar * optInOffsetOnReverse)
				}
				outReal[outIdx] = 0 - sar
				outIdx++
				afShort = optInAccelerationInitShort
				ep = newLow
				// C uses fma() explicitly.
				sar = math.FMA(afShort, ep-sar, sar)
				if sar < prevHigh {
					sar = prevHigh
				}
				if sar < newHigh {
					sar = newHigh
				}
			} else {
				outReal[outIdx] = sar
				outIdx++
				if newHigh > ep {
					ep = newHigh
					afLong += optInAccelerationLong
					if afLong > optInAccelerationMaxLong {
						afLong = optInAccelerationMaxLong
					}
				}
				sar = math.FMA(afLong, ep-sar, sar)
				if sar > prevLow {
					sar = prevLow
				}
				if sar > newLow {
					sar = newLow
				}
			}
		} else if newHigh >= sar {
			// Switch to long; override the SAR with the ep.
			isLong = true
			sar = ep
			if sar > prevLow {
				sar = prevLow
			}
			if sar > newLow {
				sar = newLow
			}
			if optInOffsetOnReverse != 0.0 {
				sar -= float64(sar * optInOffsetOnReverse)
			}
			outReal[outIdx] = sar
			outIdx++
			afLong = optInAccelerationInitLong
			ep = newHigh
			sar = math.FMA(afLong, ep-sar, sar)
			if sar > prevLow {
				sar = prevLow
			}
			if sar > newLow {
				sar = newLow
			}
		} else {
			outReal[outIdx] = 0 - sar
			outIdx++
			if newLow < ep {
				ep = newLow
				afShort += optInAccelerationShort
				if afShort > optInAccelerationMaxShort {
					afShort = optInAccelerationMaxShort
				}
			}
			sar = math.FMA(afShort, ep-sar, sar)
			if sar < prevHigh {
				sar = prevHigh
			}
			if sar < newHigh {
				sar = newHigh
			}
		}
	}
	return outBegIdx, outIdx, nil
}
