package ta

import "math"

// ADOSC — Chaikin A/D Oscillator (Volume Indicators).
// Ported from TA-Lib v0.8.1 ta_codegen/input/adosc and src/ta_func/ta_ADOSC.c.

// ADOSCLookback returns the number of leading bars ADOSC leaves empty: the EMA
// lookback of the slower of the two periods (including the EMA unstable period).
// optInFastPeriod: [2, 100000], default 3. optInSlowPeriod: [2, 100000], default 10.
func ADOSCLookback(optInFastPeriod, optInSlowPeriod int) int {
	if checkInt("ADOSC", "optInFastPeriod", optInFastPeriod, 2, 100000) != nil {
		return -1
	}
	if checkInt("ADOSC", "optInSlowPeriod", optInSlowPeriod, 2, 100000) != nil {
		return -1
	}
	return adoscLookback(optInFastPeriod, optInSlowPeriod)
}

func adoscLookback(optInFastPeriod, optInSlowPeriod int) int {
	slowestPeriod := optInFastPeriod
	if optInFastPeriod < optInSlowPeriod {
		slowestPeriod = optInSlowPeriod
	}
	return emaLookback(slowestPeriod)
}

// ADOSC returns EMA(fast, AD) - EMA(slow, AD), where AD is the Chaikin
// Accumulation/Distribution line. Both EMAs are seeded with the first A/D
// value. "Fast" is always the first period, even when it is the longer one.
// The output is index-aligned with the inputs; the first ADOSCLookback bars
// are NaN.
// optInFastPeriod: [2, 100000], default 3. optInSlowPeriod: [2, 100000], default 10.
func ADOSC(inHigh, inLow, inClose, inVolume []float64, optInFastPeriod, optInSlowPeriod int) ([]float64, error) {
	if err := checkInt("ADOSC", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return nil, err
	}
	if err := checkInt("ADOSC", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("ADOSC", []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := adoscRange(0, n-1, inHigh, inLow, inClose, inVolume, optInFastPeriod, optInSlowPeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// adoscRange mirrors TA_ADOSC: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx.
func adoscRange(startIdx, endIdx int, inHigh, inLow, inClose, inVolume []float64, optInFastPeriod, optInSlowPeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ADOSC", "optInFastPeriod", optInFastPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkInt("ADOSC", "optInSlowPeriod", optInSlowPeriod, 2, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADOSC", endIdx, []string{"inHigh", "inLow", "inClose", "inVolume"}, inHigh, inLow, inClose, inVolume); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("ADOSC", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}

	// The fast EMA is not necessarily the faster one: ADOSC is always
	// EMA(optInFastPeriod) - EMA(optInSlowPeriod). The slowest period only
	// sets the lookback.
	lookbackTotal := adoscLookback(optInFastPeriod, optInSlowPeriod)
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	outBegIdx = startIdx
	today := startIdx - lookbackTotal

	ad := 0.0
	fastk := 2.0 / (float64(optInFastPeriod) + 1.0)
	oneMinusFastk := 1.0 - fastk
	slowk := 2.0 / (float64(optInSlowPeriod) + 1.0)
	oneMinusSlowk := 1.0 - slowk

	// Both EMAs are seeded with the first A/D value.
	high := inHigh[today]
	low := inLow[today]
	tmp := high - low
	close := inClose[today]
	if tmp > 0.0 {
		ad += float64((close - low - (high - close)) / tmp * inVolume[today])
	}
	today++
	fastEMA := ad
	slowEMA := ad

	// Skip the unstable period. C uses fma() explicitly, so math.FMA
	// reproduces it bit for bit.
	for today < startIdx {
		high = inHigh[today]
		low = inLow[today]
		tmp = high - low
		close = inClose[today]
		if tmp > 0.0 {
			ad += float64((close - low - (high - close)) / tmp * inVolume[today])
		}
		today++
		fastEMA = math.FMA(oneMinusFastk, fastEMA, fastk*ad)
		slowEMA = math.FMA(oneMinusSlowk, slowEMA, slowk*ad)
	}

	outIdx := 0
	for today <= endIdx {
		high = inHigh[today]
		low = inLow[today]
		tmp = high - low
		close = inClose[today]
		if tmp > 0.0 {
			ad += float64((close - low - (high - close)) / tmp * inVolume[today])
		}
		today++
		fastEMA = math.FMA(oneMinusFastk, fastEMA, fastk*ad)
		slowEMA = math.FMA(oneMinusSlowk, slowEMA, slowk*ad)
		outReal[outIdx] = fastEMA - slowEMA
		outIdx++
	}
	return outBegIdx, outIdx, nil
}
