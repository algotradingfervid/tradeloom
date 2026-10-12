package ta

import "math"

// ZLEMA — Zero-Lag Exponential Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/zlema and src/ta_func/ta_ZLEMA.c.

// ZLEMALookback returns the number of leading bars ZLEMA leaves empty:
// (optInTimePeriod-1)/2 plus EMALookback(optInTimePeriod). ZLEMA has no
// unstable period of its own; it borrows EMA's (UnstEMA).
// optInTimePeriod: [1, 100000], default 30.
func ZLEMALookback(optInTimePeriod int) int {
	if checkInt("ZLEMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return zlemaLookback(optInTimePeriod)
}

func zlemaLookback(optInTimePeriod int) int {
	return (optInTimePeriod-1)/2 + emaLookback(optInTimePeriod)
}

// ZLEMA returns the zero-lag EMA of inReal: an EMA with k = 2/(period+1)
// over the de-lagged series 2*x[i] - x[i-lag], lag = (period-1)/2, seeded
// with the plain average of the first period de-lagged values. The output is
// index-aligned with inReal; the first ZLEMALookback bars are NaN. Period 1
// returns a copy of the input.
// optInTimePeriod: [1, 100000], default 30.
func ZLEMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("ZLEMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ZLEMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := zlemaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// zlemaRange mirrors TA_ZLEMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func zlemaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ZLEMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ZLEMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ZLEMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	optInK1 := 2.0 / float64(optInTimePeriod+1)
	lag := (optInTimePeriod - 1) / 2
	lookbackTotal := zlemaLookback(optInTimePeriod)

	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// No smoothing at period 1: the output is a copy of the input. The
	// unstable period still delays the first output.
	if optInTimePeriod == 1 {
		outIdx := 0
		for today := startIdx; today <= endIdx; today++ {
			outReal[outIdx] = inReal[today]
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	// In-place safe: both reads of a bar precede its write, and the trailing
	// read never reaches a slot already written.
	trailingIdx := startIdx - lookbackTotal
	today := trailingIdx + lag
	tempReal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		// 2.0*x is exact, so a fused multiply-subtract cannot change the result.
		tempReal += 2.0*inReal[today] - inReal[trailingIdx]
		today++
		trailingIdx++
	}
	prevMA := tempReal / float64(optInTimePeriod)
	// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
	for today <= startIdx {
		prevMA = math.FMA(2.0*inReal[today]-inReal[trailingIdx]-prevMA, optInK1, prevMA)
		today++
		trailingIdx++
	}
	outReal[0] = prevMA
	outIdx := 1
	for today <= endIdx {
		prevMA = math.FMA(2.0*inReal[today]-inReal[trailingIdx]-prevMA, optInK1, prevMA)
		today++
		trailingIdx++
		outReal[outIdx] = prevMA
		outIdx++
	}
	return startIdx, outIdx, nil
}
