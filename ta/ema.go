package ta

import "math"

// EMA — Exponential Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/ema and src/ta_func/ta_EMA.c.

// EMALookback returns the number of leading bars EMA leaves empty:
// optInTimePeriod-1 plus the EMA unstable period (UnstEMA).
// optInTimePeriod: [1, 100000], default 30.
func EMALookback(optInTimePeriod int) int {
	if checkInt("EMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return emaLookback(optInTimePeriod)
}

func emaLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1 + UnstablePeriod(UnstEMA)
}

// EMA returns the exponential moving average of inReal with smoothing factor
// k = 2/(optInTimePeriod+1), seeded with the SMA of the first optInTimePeriod
// bars. The output is index-aligned with inReal; the first EMALookback bars
// are NaN. Period 1 returns a copy of the input.
// optInTimePeriod: [1, 100000], default 30.
func EMA(inReal []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("EMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("EMA", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := emaRange(0, n-1, inReal, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// emaRange mirrors TA_EMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func emaRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("EMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("EMA", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("EMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return emaCore(startIdx, endIdx, inReal, optInTimePeriod, outReal)
}

// emaCore is the TA_EMA body after the validation prologue; other indicators
// may call it when they have already validated their arguments.
func emaCore(startIdx, endIdx int, inReal []float64, optInTimePeriod int, outReal []float64) (int, int, error) {
	optInK1 := 2.0 / float64(optInTimePeriod+1)

	lookbackTotal := emaLookback(optInTimePeriod)
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

	today := startIdx - lookbackTotal
	tempReal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		tempReal += inReal[today]
		today++
	}
	prevMA := tempReal / float64(optInTimePeriod)
	// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
	for today <= startIdx {
		prevMA = math.FMA(inReal[today]-prevMA, optInK1, prevMA)
		today++
	}
	outReal[0] = prevMA
	outIdx := 1
	for today <= endIdx {
		prevMA = math.FMA(inReal[today]-prevMA, optInK1, prevMA)
		today++
		outReal[outIdx] = prevMA
		outIdx++
	}
	return startIdx, outIdx, nil
}
