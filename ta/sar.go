package ta

import "math"

// SAR — Parabolic SAR (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/sar and src/ta_func/ta_SAR.c.

// SARLookback returns the number of leading bars SAR leaves empty (always 1:
// the first bar only establishes the initial extreme point), or -1 on an
// invalid parameter.
// optInAcceleration: [0, TA_REAL_MAX], default 0.02.
// optInMaximum: [0, TA_REAL_MAX], default 0.2.
func SARLookback(optInAcceleration, optInMaximum float64) int {
	if checkReal("SAR", "optInAcceleration", optInAcceleration, 0, taRealMax) != nil {
		return -1
	}
	if checkReal("SAR", "optInMaximum", optInMaximum, 0, taRealMax) != nil {
		return -1
	}
	return sarLookback(optInAcceleration, optInMaximum)
}

func sarLookback(optInAcceleration, optInMaximum float64) int {
	return 1
}

// SAR returns Wilder's Parabolic Stop And Reverse of (inHigh, inLow). The
// initial direction is taken from the +DM/-DM of the first two bars (long on
// a tie). The output is index-aligned with the inputs; the first
// SARLookback bars are NaN. An acceleration above the maximum is clamped to
// the maximum.
// optInAcceleration: [0, TA_REAL_MAX], default 0.02.
// optInMaximum: [0, TA_REAL_MAX], default 0.2.
func SAR(inHigh, inLow []float64, optInAcceleration, optInMaximum float64) ([]float64, error) {
	if err := checkReal("SAR", "optInAcceleration", optInAcceleration, 0, taRealMax); err != nil {
		return nil, err
	}
	if err := checkReal("SAR", "optInMaximum", optInMaximum, 0, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("SAR", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := sarRange(0, n-1, inHigh, inLow, optInAcceleration, optInMaximum, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// sarRange mirrors TA_SAR: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may be one of the input slices.
func sarRange(startIdx, endIdx int, inHigh, inLow []float64, optInAcceleration, optInMaximum float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkReal("SAR", "optInAcceleration", optInAcceleration, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkReal("SAR", "optInMaximum", optInMaximum, 0, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SAR", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("SAR", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	if startIdx < 1 {
		startIdx = 1
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Make the acceleration and maximum coherent.
	af := optInAcceleration
	if af > optInMaximum {
		optInAcceleration = optInMaximum
		af = optInAcceleration
	}

	// Initial direction from the one-bar -DM at startIdx (C: TA_MINUS_DM
	// with period 1 into a one-element temp buffer).
	var epTemp [1]float64
	if _, _, err := minusdmRange(startIdx, startIdx, inHigh, inLow, 1, epTemp[:]); err != nil {
		return 0, 0, err
	}
	isLong := epTemp[0] <= 0

	outBegIdx = startIdx
	outIdx := 0

	// First SAR.
	todayIdx := startIdx
	newHigh := inHigh[todayIdx-1]
	newLow := inLow[todayIdx-1]
	var ep, sar float64
	if isLong {
		ep = inHigh[todayIdx]
		sar = newLow
	} else {
		ep = inLow[todayIdx]
		sar = newHigh
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
				outReal[outIdx] = sar
				outIdx++
				af = optInAcceleration
				ep = newLow
				sar = math.FMA(af, ep-sar, sar) // C uses fma() explicitly
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
					af += optInAcceleration
					if af > optInMaximum {
						af = optInMaximum
					}
				}
				sar = math.FMA(af, ep-sar, sar)
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
			outReal[outIdx] = sar
			outIdx++
			af = optInAcceleration
			ep = newHigh
			sar = math.FMA(af, ep-sar, sar)
			if sar > prevLow {
				sar = prevLow
			}
			if sar > newLow {
				sar = newLow
			}
		} else {
			outReal[outIdx] = sar
			outIdx++
			if newLow < ep {
				ep = newLow
				af += optInAcceleration
				if af > optInMaximum {
					af = optInMaximum
				}
			}
			sar = math.FMA(af, ep-sar, sar)
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
