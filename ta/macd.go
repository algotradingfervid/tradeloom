package ta

import "math"

// MACD — Moving Average Convergence/Divergence (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/macd and src/ta_func/ta_MACD.c.
//
// The v0.8.1 C does not call TA_EMA: it advances the fast EMA, slow EMA and
// signal EMA in one lockstep pass (bit-exact with the older three-pass
// algorithm). This port follows that single pass statement by statement and
// uses emaLookback for the lookback arithmetic, as C uses TA_EMA_Lookback.

// MACDLookback returns the number of leading bars MACD leaves empty:
// EMALookback(slow) + EMALookback(signal), slow being the larger period.
// optInFastPeriod: [2, 100000], default 12.
// optInSlowPeriod: [2, 100000], default 26.
// optInSignalPeriod: [1, 100000], default 9.
func MACDLookback(optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) int {
	if macdCheckParams(optInFastPeriod, optInSlowPeriod, optInSignalPeriod) != nil {
		return -1
	}
	return macdLookback(optInFastPeriod, optInSlowPeriod, optInSignalPeriod)
}

func macdLookback(optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) int {
	if optInSlowPeriod < optInFastPeriod {
		optInSlowPeriod, optInFastPeriod = optInFastPeriod, optInSlowPeriod
	}
	return emaLookback(optInSlowPeriod) + emaLookback(optInSignalPeriod)
}

func macdCheckParams(optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) error {
	if err := checkInt("MACD", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return err
	}
	if err := checkInt("MACD", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return err
	}
	return checkInt("MACD", "optInSignalPeriod", optInSignalPeriod, 1, 100000)
}

// MACD returns the MACD line (fast EMA minus slow EMA), its signal line (an
// EMA of the MACD line) and the histogram (MACD minus signal). If the slow
// period is smaller than the fast one the two are swapped. The outputs are
// index-aligned with inReal; the first MACDLookback bars are NaN.
func MACD(inReal []float64, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) (outMACD, outMACDSignal, outMACDHist []float64, err error) {
	if err := macdCheckParams(optInFastPeriod, optInSlowPeriod, optInSignalPeriod); err != nil {
		return nil, nil, nil, err
	}
	n, err := checkInputs("MACD", []string{"inReal"}, inReal)
	if err != nil {
		return nil, nil, nil, err
	}
	outMACD = make([]float64, n)
	outMACDSignal = make([]float64, n)
	outMACDHist = make([]float64, n)
	if n == 0 {
		return outMACD, outMACDSignal, outMACDHist, nil
	}
	beg, nb, err := macdRange(0, n-1, inReal, optInFastPeriod, optInSlowPeriod, optInSignalPeriod, outMACD, outMACDSignal, outMACDHist)
	if err != nil {
		return nil, nil, nil, err
	}
	alignReal(outMACD, beg, nb)
	alignReal(outMACDSignal, beg, nb)
	alignReal(outMACDHist, beg, nb)
	return outMACD, outMACDSignal, outMACDHist, nil
}

// macdSameBuf reports whether two non-empty slices start at the same element
// (C's outMACD == outMACDSignal pointer comparison).
func macdSameBuf(a, b []float64) bool {
	return len(a) > 0 && len(b) > 0 && &a[0] == &b[0]
}

// macdRange mirrors TA_MACD: it computes [startIdx, endIdx] and writes
// outNBElement values to each output from index 0, the first one belonging
// to bar outBegIdx. An output may be inReal, but the three outputs must be
// distinct buffers (C returns TA_BAD_PARAM otherwise).
func macdRange(startIdx, endIdx int, inReal []float64, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int, outMACD, outMACDSignal, outMACDHist []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := macdCheckParams(optInFastPeriod, optInSlowPeriod, optInSignalPeriod); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MACD", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("MACD", endIdx-startIdx, []string{"outMACD", "outMACDSignal", "outMACDHist"}, outMACD, outMACDSignal, outMACDHist); err != nil {
		return 0, 0, err
	}
	if macdSameBuf(outMACD, outMACDSignal) || macdSameBuf(outMACD, outMACDHist) || macdSameBuf(outMACDSignal, outMACDHist) {
		return 0, 0, badParam("MACD", "outMACD", "output buffers must be distinct")
	}

	if optInSlowPeriod < optInFastPeriod {
		optInSlowPeriod, optInFastPeriod = optInFastPeriod, optInSlowPeriod
	}
	// C's "fix 26/12" arms (period 0) are unreachable after validation.
	slowK := 2.0 / float64(optInSlowPeriod+1)
	fastK := 2.0 / float64(optInFastPeriod+1)
	signalK := 2.0 / float64(optInSignalPeriod+1)
	lookbackSignal := emaLookback(optInSignalPeriod)

	lookbackTotal := lookbackSignal
	lookbackTotal += emaLookback(optInSlowPeriod)
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
