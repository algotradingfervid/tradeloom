package ta

import "math"

// MACDFIX — Moving Average Convergence/Divergence Fix 12/26 (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/macdfix and src/ta_func/ta_MACDFIX.c.
//
// Like TA_MACD in v0.8.1, the C does not call TA_EMA: it advances the fixed
// 12 and 26 EMAs (k = 0.15 and 0.075, not 2/(p+1)) and the signal EMA in one
// lockstep pass. This port follows that pass statement by statement and uses
// emaLookback where C uses TA_EMA_Lookback.

// MACDFIXLookback returns the number of leading bars MACDFIX leaves empty:
// EMALookback(26) + EMALookback(signal).
// optInSignalPeriod: [1, 100000], default 9.
func MACDFIXLookback(optInSignalPeriod int) int {
	if checkInt("MACDFIX", "optInSignalPeriod", optInSignalPeriod, 1, 100000) != nil {
		return -1
	}
	return macdfixLookback(optInSignalPeriod)
}

func macdfixLookback(optInSignalPeriod int) int {
	return emaLookback(26) + emaLookback(optInSignalPeriod)
}

// MACDFIX returns the MACD line (fixed EMA12 minus EMA26), its signal line
// (an EMA of the MACD line over optInSignalPeriod) and the histogram (MACD
// minus signal). The outputs are index-aligned with inReal; the first
// MACDFIXLookback bars are NaN.
// optInSignalPeriod: [1, 100000], default 9.
func MACDFIX(inReal []float64, optInSignalPeriod int) (outMACD, outMACDSignal, outMACDHist []float64, err error) {
	if err := checkInt("MACDFIX", "optInSignalPeriod", optInSignalPeriod, 1, 100000); err != nil {
		return nil, nil, nil, err
	}
	n, err := checkInputs("MACDFIX", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, nil, err
	}
	outMACD = make([]float64, n)
	outMACDSignal = make([]float64, n)
	outMACDHist = make([]float64, n)
	if n == 0 {
		return outMACD, outMACDSignal, outMACDHist, nil
	}
	beg, nb, err := macdfixRange(0, n-1, inReal, optInSignalPeriod, outMACD, outMACDSignal, outMACDHist)
	if err != nil {
		return nil, nil, nil, err
	}
	alignReal(outMACD, beg, nb)
	alignReal(outMACDSignal, beg, nb)
	alignReal(outMACDHist, beg, nb)
	return outMACD, outMACDSignal, outMACDHist, nil
}

// macdfixSameBuf reports whether two non-empty slices start at the same
// element (C's outMACD == outMACDSignal pointer comparison).
func macdfixSameBuf(a, b []float64) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}

// macdfixRange mirrors TA_MACDFIX: it computes [startIdx, endIdx] and writes
// outNBElement values to each output from index 0, the first one belonging
// to bar outBegIdx. An output may be inReal, but the three outputs must be
// distinct buffers (C returns TA_BAD_PARAM otherwise).
func macdfixRange(startIdx, endIdx int, inReal []float64, optInSignalPeriod int, outMACD, outMACDSignal, outMACDHist []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("MACDFIX", "optInSignalPeriod", optInSignalPeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MACDFIX", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MACDFIX", endIdx-startIdx, []string{"outMACD", "outMACDSignal", "outMACDHist"}, outMACD, outMACDSignal, outMACDHist); err != nil {
		return 0, 0, err
	}
	if macdfixSameBuf(outMACD, outMACDSignal) || macdfixSameBuf(outMACD, outMACDHist) || macdfixSameBuf(outMACDSignal, outMACDHist) {
		return 0, 0, badParam("MACDFIX", "outMACD", "output buffers must be distinct")
	}

	optInFastPeriod := 12
	optInSlowPeriod := 26
	fastK := 0.15
	slowK := 0.075
	signalK := 2.0 / float64(optInSignalPeriod+1)
	lookbackSignal := emaLookback(optInSignalPeriod)

	lookbackTotal := lookbackSignal
	lookbackTotal += emaLookback(26)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Seed both price EMAs; the fast window is the tail of the slow window.
	today := startIdx - lookbackTotal
	tempReal := 0.0
	for i := optInSlowPeriod - optInFastPeriod; i > 0; i-- {
		tempReal += inReal[today]
		today++
	}
	prevFast := 0.0
	for i := optInFastPeriod; i > 0; i-- {
		prevFast += inReal[today]
		tempReal += inReal[today]
		today++
	}
	prevSlow := tempReal / float64(optInSlowPeriod)
	prevFast = prevFast / float64(optInFastPeriod)

	// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
	for today <= startIdx-lookbackSignal {
		tempReal = inReal[today]
		today++
		prevFast = math.FMA(tempReal-prevFast, fastK, prevFast)
		prevSlow = math.FMA(tempReal-prevSlow, slowK, prevSlow)
	}
	macdValue := prevFast - prevSlow

	// Seed the signal EMA with the average of the first signal-period MACD values.
	prevSignal := 0.0
	prevSignal += macdValue
	for i := optInSignalPeriod - 1; i > 0; i-- {
		tempReal = inReal[today]
		today++
		prevFast = math.FMA(tempReal-prevFast, fastK, prevFast)
		prevSlow = math.FMA(tempReal-prevSlow, slowK, prevSlow)
		macdValue = prevFast - prevSlow
		prevSignal += macdValue
	}
	prevSignal = prevSignal / float64(optInSignalPeriod)

	// Unstable period of the signal EMA, up to the first output bar.
	for today <= startIdx {
		tempReal = inReal[today]
		today++
		prevFast = math.FMA(tempReal-prevFast, fastK, prevFast)
		prevSlow = math.FMA(tempReal-prevSlow, slowK, prevSlow)
		macdValue = prevFast - prevSlow
		if optInSignalPeriod == 1 {
			prevSignal = macdValue
		} else {
			prevSignal = math.FMA(macdValue-prevSignal, signalK, prevSignal)
		}
	}

	outMACD[0] = macdValue
	outMACDSignal[0] = prevSignal
	outMACDHist[0] = macdValue - prevSignal
	outIdx := 1
	for today <= endIdx {
		tempReal = inReal[today]
		today++
		prevFast = math.FMA(tempReal-prevFast, fastK, prevFast)
		prevSlow = math.FMA(tempReal-prevSlow, slowK, prevSlow)
		macdValue = prevFast - prevSlow
		if optInSignalPeriod == 1 {
			prevSignal = macdValue
		} else {
			prevSignal = math.FMA(macdValue-prevSignal, signalK, prevSignal)
		}
		outMACD[outIdx] = macdValue
		outMACDSignal[outIdx] = prevSignal
		outMACDHist[outIdx] = macdValue - prevSignal
		outIdx++
	}
	return startIdx, outIdx, nil
}
