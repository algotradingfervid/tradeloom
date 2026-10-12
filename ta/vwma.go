package ta

// VWMA — Volume Weighted Moving Average (Overlap Studies).
// Ported from TA-Lib v0.8.1 ta_codegen/input/vwma and src/ta_func/ta_VWMA.c.

// VWMALookback returns the number of leading bars VWMA leaves empty.
// optInTimePeriod: [1, 100000], default 30.
func VWMALookback(optInTimePeriod int) int {
	if checkInt("VWMA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return vwmaLookback(optInTimePeriod)
}

func vwmaLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// VWMA returns the volume weighted moving average of inReal over
// optInTimePeriod bars, each bar weighted by inVolume. The output is
// index-aligned with inReal; the first VWMALookback bars are NaN. A window
// whose volumes are all zero yields NaN (0/0), as in TA-Lib. Period 1 is the
// identity (the volume is ignored).
// optInTimePeriod: [1, 100000], default 30.
func VWMA(inReal, inVolume []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("VWMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("VWMA", []string{"inReal", "inVolume"}, inReal, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := vwmaRange(0, n-1, inReal, inVolume, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// vwmaRange mirrors TA_VWMA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. An input and outReal may be the same slice.
func vwmaRange(startIdx, endIdx int, inReal, inVolume []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("VWMA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VWMA", endIdx, []string{"inReal", "inVolume"}, inReal, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("VWMA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// No smoothing at period 1: the output is a copy of the input.
	if optInTimePeriod == 1 {
		outIdx := 0
		for i := startIdx; i <= endIdx; i++ {
			outReal[outIdx] = inReal[i]
			outIdx++
		}
		return startIdx, outIdx, nil
	}

	// The explicit float64 conversions round each price*volume product, so
	// the Go compiler may not fuse it into an FMA with the following add or
	// subtract (the C keeps the product in its own statement for the same
	// reason).
	sumPV := 0.0
	sumV := 0.0
	trailingIdx := startIdx - lookbackTotal
	i := trailingIdx
	if optInTimePeriod > 1 {
		for i < startIdx {
			tempReal := float64(inReal[i] * inVolume[i])
			sumPV += tempReal
			sumV += inVolume[i]
			i++
		}
	}

	outIdx := 0
	for i <= endIdx {
		tempReal := float64(inReal[i] * inVolume[i])
		sumPV += tempReal
		sumV += inVolume[i]
		i++
		tempPV := sumPV
		tempV := sumV
		// Read the trailing values before writing the output (in-place use).
		tempReal = float64(inReal[trailingIdx] * inVolume[trailingIdx])
		sumPV -= tempReal
		sumV -= inVolume[trailingIdx]
		outReal[outIdx] = tempPV / float64(optInTimePeriod) / (tempV / float64(optInTimePeriod))
		trailingIdx++
		outIdx++
	}
	return startIdx, outIdx, nil
}
