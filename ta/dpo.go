package ta

// DPO — Detrended Price Oscillator (Momentum Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/dpo and src/ta_func/ta_DPO.c.

// DPOLookback returns the number of leading bars DPO leaves empty.
// optInTimePeriod: [2, 100000], default 20.
func DPOLookback(optInTimePeriod int) int {
	if checkInt("DPO", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	return dpoLookback(optInTimePeriod)
}

// dpoLookback is max(period-1, period/2+1): the displaced read reaches back
// period/2+1 bars, which exceeds the moving average's period-1 at period 2.
func dpoLookback(optInTimePeriod int) int {
	return cMaxInt(optInTimePeriod-1, optInTimePeriod/2+1)
}

// DPO returns inReal[i-t] - SMA(inReal, optInTimePeriod)[i] with
// t = optInTimePeriod/2 + 1 (integer division). The output is index-aligned
// with inReal; the first DPOLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 20.
func DPO(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("DPO", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("DPO", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := dpoRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// dpoRange mirrors TA_DPO: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func dpoRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("DPO", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DPO", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("DPO", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// The running sum is accumulated, snapshot and divided in the same order
	// as SMA, so the moving-average term is bit-identical to smaCore.
	lookbackTotal := dpoLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	periodTotal := 0.0
	trailingIdx := startIdx - (optInTimePeriod - 1)
	dispIdx := startIdx - (optInTimePeriod/2 + 1)
	i := trailingIdx
	for i < startIdx {
		periodTotal += inReal[i]
		i++
	}
	outIdx := 0
	for i <= endIdx {
		periodTotal += inReal[i]
		i++
		tempReal := periodTotal
		periodTotal -= inReal[trailingIdx]
		trailingIdx++
		// Both reads precede the store, so aliasing outReal over inReal is safe.
		dispVal := inReal[dispIdx]
		dispIdx++
		outReal[outIdx] = dispVal - tempReal/float64(optInTimePeriod)
		outIdx++
	}
	return startIdx, outIdx, nil
}
