package ta

import "math"

// STDDEV — Standard Deviation (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/stddev and src/ta_func/ta_STDDEV.c.

// STDDEVLookback returns the number of leading bars STDDEV leaves empty.
// optInTimePeriod: [2, 100000], default 5. optInNbDev: [-3e37, 3e37], default 1.
func STDDEVLookback(optInTimePeriod int, optInNbDev float64) int {
	if checkInt("STDDEV", "optInTimePeriod", optInTimePeriod, 2, 100000) != nil {
		return -1
	}
	if checkReal("STDDEV", "optInNbDev", optInNbDev, taRealMin, taRealMax) != nil {
		return -1
	}
	return stddevLookback(optInTimePeriod, optInNbDev)
}

func stddevLookback(optInTimePeriod int, optInNbDev float64) int {
	// Lookback is driven by the variance.
	return varLookback(optInTimePeriod, optInNbDev)
}

// STDDEV returns the rolling population standard deviation of inReal over
// optInTimePeriod bars, multiplied by optInNbDev. The output is index-aligned
// with inReal; the first STDDEVLookback bars are NaN.
// optInTimePeriod: [2, 100000], default 5. optInNbDev: [-3e37, 3e37], default 1.
func STDDEV(inReal []float64, optInTimePeriod int, optInNbDev float64) ([]float64, error) {
	if err := checkInt("STDDEV", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return nil, err
	}
	if err := checkReal("STDDEV", "optInNbDev", optInNbDev, taRealMin, taRealMax); err != nil {
		return nil, err
	}
	n, err := checkInputs("STDDEV", []string{"inReal"}, inReal)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := stddevRange(0, n-1, inReal, optInTimePeriod, optInNbDev, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// stddevRange mirrors TA_STDDEV: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. inReal and outReal may be the same slice.
func stddevRange(startIdx, endIdx int, inReal []float64, optInTimePeriod int, optInNbDev float64, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("STDDEV", "optInTimePeriod", optInTimePeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkReal("STDDEV", "optInNbDev", optInNbDev, taRealMin, taRealMax); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("STDDEV", endIdx, []string{"inReal"}, inReal); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("STDDEV", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// Nothing to produce: the range is shorter than the lookback.
	if stddevLookback(optInTimePeriod, optInNbDev) > endIdx {
		return 0, 0, nil
	}
	// Calculate the variance.
	outBegIdx, outNBElement, err = varRange(startIdx, endIdx, inReal, optInTimePeriod, 1.0, outReal)
	if err != nil {
		return 0, 0, err
	}
	// Square root of each variance, scaled by the requested ratio
	// (unconditional: VAR already returns a non-negative, floored variance).
	if optInNbDev != 1.0 {
		for i := 0; i < outNBElement; i++ {
			outReal[i] = math.Sqrt(outReal[i]) * optInNbDev
		}
	} else {
		for i := 0; i < outNBElement; i++ {
			outReal[i] = math.Sqrt(outReal[i])
		}
	}
	return outBegIdx, outNBElement, nil
}
