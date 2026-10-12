package ta

// AC — Accelerator/Decelerator Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ac and src/ta_func/ta_AC.c.
//
// TA_AC does not call TA_AO or TA_SMA: it inlines the two median-price SMA
// running sums of TA_AO plus a third running sum over a ring of the last
// optInSignalPeriod oscillator values, all in TA_SMA's add / snapshot /
// subtract / divide order. The port follows that compiled code; only the
// lookback delegates to aoLookback and smaLookback, as C does.

// ACLookback returns the number of leading bars AC leaves empty.
// optInFastPeriod: [2, 100000], default 5. optInSlowPeriod: [2, 100000],
// default 34. optInSignalPeriod: [2, 100000], default 5.
func ACLookback(optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) int {
	if acCheckParams(optInFastPeriod, optInSlowPeriod, optInSignalPeriod) != nil {
		return -1
	}
	return acLookback(optInFastPeriod, optInSlowPeriod, optInSignalPeriod)
}

func acLookback(optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) int {
	return aoLookback(optInFastPeriod, optInSlowPeriod) + smaLookback(optInSignalPeriod)
}

func acCheckParams(optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) error {
	if err := checkInt("AC", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return err
	}
	if err := checkInt("AC", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return err
	}
	return checkInt("AC", "optInSignalPeriod", optInSignalPeriod, 2, 100000)
}

// AC returns AO - SMA(AO, signal), with AO = SMA(median, fast) -
// SMA(median, slow) and median = (high+low)/2. The output is index-aligned
// with the inputs; the first ACLookback bars are NaN. A fast period longer
// than the slow one is allowed (not swapped).
func AC(inHigh, inLow []float64, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int) ([]float64, error) {
	if err := acCheckParams(optInFastPeriod, optInSlowPeriod, optInSignalPeriod); err != nil {
		return nil, err
	}
	n, err := checkInputs("AC", []string{"inHigh", "inLow"}, inHigh, inLow)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := acRange(0, n-1, inHigh, inLow, optInFastPeriod, optInSlowPeriod, optInSignalPeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// acRange mirrors TA_AC. outReal may alias inHigh or inLow.
func acRange(startIdx, endIdx int, inHigh, inLow []float64, optInFastPeriod, optInSlowPeriod, optInSignalPeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := acCheckParams(optInFastPeriod, optInSlowPeriod, optInSignalPeriod); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AC", endIdx, []string{"inHigh", "inLow"}, inHigh, inLow); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("AC", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := acLookback(optInFastPeriod, optInSlowPeriod, optInSignalPeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Circular buffer of optInSignalPeriod oscillator values.
	oscBuffer := make([]float64, optInSignalPeriod)
	maxIdxOscBuffer := optInSignalPeriod - 1
	oscBufferIdx := 0

	oscStartIdx := startIdx - (optInSignalPeriod - 1)
	sumFast := 0.0
	sumSlow := 0.0
	sumSignal := 0.0
	trailingFastIdx := oscStartIdx - (optInFastPeriod - 1)
	trailingSlowIdx := oscStartIdx - (optInSlowPeriod - 1)

	// Add up both initial periods, except for the last value.
	i := startIdx - lookbackTotal
	for i < oscStartIdx {
		medianPrice := float64((inHigh[i] + inLow[i]) / 2.0)
		if i >= trailingFastIdx {
			sumFast += medianPrice
		}
		if i >= trailingSlowIdx {
			sumSlow += medianPrice
		}
		i++
	}

	// Fill the signal ring with the oscillator values preceding the first output.
	for i < startIdx {
		medianPrice := float64((inHigh[i] + inLow[i]) / 2.0)
		sumFast += medianPrice
		sumSlow += medianPrice
		osc := sumFast/float64(optInFastPeriod) - sumSlow/float64(optInSlowPeriod)
		sumFast -= float64((inHigh[trailingFastIdx] + inLow[trailingFastIdx]) / 2.0)
		sumSlow -= float64((inHigh[trailingSlowIdx] + inLow[trailingSlowIdx]) / 2.0)
		trailingFastIdx++
		trailingSlowIdx++
		i++
		oscBuffer[oscBufferIdx] = osc
		sumSignal += osc
		oscBufferIdx++
		if oscBufferIdx > maxIdxOscBuffer {
			oscBufferIdx = 0
		}
	}

	outIdx := 0
	for i <= endIdx {
		medianPrice := float64((inHigh[i] + inLow[i]) / 2.0)
		sumFast += medianPrice
		sumSlow += medianPrice
		osc := sumFast/float64(optInFastPeriod) - sumSlow/float64(optInSlowPeriod)
		sumFast -= float64((inHigh[trailingFastIdx] + inLow[trailingFastIdx]) / 2.0)
		sumSlow -= float64((inHigh[trailingSlowIdx] + inLow[trailingSlowIdx]) / 2.0)
		trailingFastIdx++
		trailingSlowIdx++
		i++
		oscBuffer[oscBufferIdx] = osc
		sumSignal += osc
		tempReal := osc - sumSignal/float64(optInSignalPeriod)
		oscBufferIdx++
		if oscBufferIdx > maxIdxOscBuffer {
			oscBufferIdx = 0
		}
		sumSignal -= oscBuffer[oscBufferIdx]
		// All inputs for this bar are read above; the store is safe when
		// outReal aliases an input.
		outReal[outIdx] = tempReal
		outIdx++
	}
	return startIdx, outIdx, nil
}
