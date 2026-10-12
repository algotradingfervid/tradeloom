package ta

import "math"

// EFI — Elder's Force Index (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/efi and src/ta_func/ta_EFI.c.
//
//	force[t] = (close[t] - close[t-1]) * volume[t]
//	EFI      = EMA(force, optInTimePeriod)

// EFILookback returns the number of leading bars EFI leaves empty:
// optInTimePeriod plus the EMA unstable period.
// optInTimePeriod: [1, 100000], default 13.
func EFILookback(optInTimePeriod int) int {
	if checkInt("EFI", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return efiLookback(optInTimePeriod)
}

func efiLookback(optInTimePeriod int) int {
	return optInTimePeriod + UnstablePeriod(UnstEMA)
}

// EFI returns Elder's Force Index of inClose/inVolume smoothed by an EMA of
// optInTimePeriod bars. The output is index-aligned with the inputs; the first
// EFILookback bars are NaN.
// optInTimePeriod: [1, 100000], default 13.
func EFI(inClose, inVolume []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("EFI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("EFI", []string{"inClose", "inVolume"}, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := efiRange(0, n-1, inClose, inVolume, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// efiRange mirrors TA_EFI: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias an input (prevClose is carried in a scalar).
func efiRange(startIdx, endIdx int, inClose, inVolume []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("EFI", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("EFI", endIdx, []string{"inClose", "inVolume"}, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("EFI", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	optInK1 := 2.0 / float64(optInTimePeriod+1)
	lookbackTotal := efiLookback(optInTimePeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Period 1: the raw one-bar Force Index, no smoothing.
	if optInTimePeriod == 1 {
		outIdx := 0
		today := startIdx
		prevClose := inClose[today-1]
		for today <= endIdx {
			force := (inClose[today] - prevClose) * inVolume[today]
			prevClose = inClose[today]
			outReal[outIdx] = force
			outIdx++
			today++
		}
		return startIdx, outIdx, nil
	}

	// Seed: simple average of the first optInTimePeriod force values.
	today := startIdx - lookbackTotal + 1
	prevClose := inClose[today-1]
	tempReal := 0.0
	for i := optInTimePeriod; i > 0; i-- {
		// float64() rounds the product so Go cannot fuse it into the sum
		// (C keeps them as separate statements, unfused).
		force := float64((inClose[today] - prevClose) * inVolume[today])
		prevClose = inClose[today]
		tempReal += force
		today++
	}
	prevMA := tempReal / float64(optInTimePeriod)
	// C uses fma() explicitly, so math.FMA reproduces it bit for bit.
	for today <= startIdx {
		force := float64((inClose[today] - prevClose) * inVolume[today])
		prevClose = inClose[today]
		prevMA = math.FMA(force-prevMA, optInK1, prevMA)
		today++
	}
	outReal[0] = prevMA
	outIdx := 1
	for today <= endIdx {
		force := float64((inClose[today] - prevClose) * inVolume[today])
		prevClose = inClose[today]
		prevMA = math.FMA(force-prevMA, optInK1, prevMA)
		outReal[outIdx] = prevMA
		outIdx++
		today++
	}
	return startIdx, outIdx, nil
}
