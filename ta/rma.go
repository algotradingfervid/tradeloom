package ta

import "math"

// RMA — Wilder's Smoothed Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/rma and src/ta_func/ta_RMA.c.

// RMALookback returns the number of leading bars RMA leaves empty:
// optInTimePeriod-1 plus the RMA unstable period (UnstRMA).
// optInTimePeriod: [1, 100000], default 30.
func RMALookback(optInTimePeriod int) int {
	if checkInt("RMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return rmaLookback(optInTimePeriod)
}

func rmaLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1 + UnstablePeriod(UnstRMA)
}

// RMA returns Wilder's smoothed moving average of inReal: smoothing factor
// 1/optInTimePeriod, seeded with the SMA of the first optInTimePeriod bars.
// The output is index-aligned with inReal; the first RMALookback bars are
// NaN. Period 1 returns a copy of the input.
// optInTimePeriod: [1, 100000], default 30.
func RMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("RMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("RMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := rmaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// rmaRange mirrors TA_RMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func rmaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("RMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("RMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("RMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return rmaCore(startIdx, endIdx, inReal, optInTimePeriod, outReal)
}

// rmaCore is the TA_RMA body after the validation prologue; other indicators
// (e.g. MA with MATypeRMA) may call it when they have already validated.
func rmaCore(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (int, int, error) {
	// C reads the unstable period twice (lookback and skip loop); one read
	// here keeps both uses consistent under concurrent SetUnstablePeriod.
	unstable := UnstablePeriod(UnstRMA)
	lookbackTotal := optInTimePeriod - 1 + unstable
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// wAlpha is derived from wBeta (not 1/period) so wAlpha+wBeta == 1
	// exactly; at period 1 the pair is exactly (1, 0).
	wBeta := float64(optInTimePeriod-1) / float64(optInTimePeriod)
	wAlpha := 1.0 - wBeta

	today := startIdx - lookbackTotal

	// Seed with a simple average of the first optInTimePeriod values.
	periodTotal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		periodTotal += inReal[today]
		today++
	}
	prevRMA := periodTotal / float64(optInTimePeriod)

	// Skip the unstable period. C uses an explicit fma(); math.FMA matches
	// it bit for bit.
	for i := unstable; i != 0; i-- {
		prevRMA = math.FMA(wBeta, prevRMA, wAlpha*inReal[today])
		today++
	}

	outIdx := 1
	outReal[0] = prevRMA
	for nbRMA := endIdx - startIdx + 1; nbRMA > 1; nbRMA-- {
		prevRMA = math.FMA(wBeta, prevRMA, wAlpha*inReal[today])
		outReal[outIdx] = prevRMA
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
