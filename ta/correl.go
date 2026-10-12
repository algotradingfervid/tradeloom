package ta

import "math"

// CORREL — Pearson's Correlation Coefficient (r) (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/correl and src/ta_func/ta_CORREL.c
// (the shifted-sums / reseed variant of the compiled C).
//
// The C library is built with -ffp-contract=off. Go may fuse x*y+z into an
// FMA (also across statements), so every product that feeds an addition or
// subtraction is wrapped in an explicit float64() conversion, which the spec
// guarantees rounds and therefore blocks fusion.

// CORRELLookback returns the number of leading bars CORREL leaves empty.
// optInTimePeriod: [1, 100000], default 30.
func CORRELLookback(optInTimePeriod int) int {
	if checkInt("CORREL", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return correlLookback(optInTimePeriod)
}

func correlLookback(optInTimePeriod int) int {
	return optInTimePeriod - 1
}

// CORREL returns the rolling Pearson correlation of inReal0 and inReal1 over
// optInTimePeriod bars. Windows where the correlation is undefined (a flat
// series) give 0. The output is index-aligned with the inputs; the first
// CORRELLookback bars are NaN.
// optInTimePeriod: [1, 100000], default 30.
func CORREL(inReal0, inReal1 []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("CORREL", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("CORREL", []string{"inReal0", "inReal1"}, inReal0, inReal1)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := correlRange(0, n-1, inReal0, inReal1, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// correlRange mirrors TA_CORREL: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias either input.
func correlRange(startIdx, endIdx int, inReal0, inReal1 []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("CORREL", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CORREL", endIdx, []string{"inReal0", "inReal1"}, inReal0, inReal1); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("CORREL", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return correlCore(startIdx, endIdx, inReal0, inReal1, optInTimePeriod, outReal)
}

// correlCore is the TA_CORREL body after validation.
func correlCore(startIdx, endIdx int, inReal0, inReal1 []float64, optInTimePeriod int, outReal []float64) (int, int, error) {
	var (
		sumXY, sumX, sumY, sumX2, sumY2 float64
		x, y                            float64
		trailingX, trailingY            float64
		shiftX, shiftY                  float64
		ssX, ssY, spXY                  float64
		leavingX, leavingY              float64
		tempReal                        float64
	)

	// One reciprocal instead of three divisions per bar, as TA_VAR does.
	invPeriod := 1.0 / float64(optInTimePeriod)
	lookbackTotal := optInTimePeriod - 1
	if startIdx < lookbackTotal {
		startIdx = lookbackTotal
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}
	outBegIdx := startIdx
	trailingIdx := startIdx - lookbackTotal

	// Measure both series against a shift near the window (anchored on the
	// first window value; later re-anchors use the window mean).
	shiftX = inReal0[trailingIdx]
	shiftY = inReal1[trailingIdx]
	// Initial sums (the window less its last bar).
	for j := trailingIdx; j < startIdx; j++ {
		x = inReal0[j] - shiftX
		sumX += x
		sumX2 += float64(x * x)
		y = inReal1[j] - shiftY
		sumXY += float64(x * y)
		sumY += y
		sumY2 += float64(y * y)
	}
	today := startIdx
	outIdx := 0
	barsSinceReseed := 32 * optInTimePeriod
	for {
		// Add the incoming value, measured against the shift.
		x = inReal0[today] - shiftX
		sumX += x
		sumX2 += float64(x * x)
		y = inReal1[today] - shiftY
		sumXY += float64(x * y)
		sumY += y
		sumY2 += float64(y * y)
		ssX = sumX2 - float64(sumX*sumX*invPeriod)
		ssY = sumY2 - float64(sumY*sumY*invPeriod)
		spXY = sumXY - float64(sumX*sumY*invPeriod)

		// Re-anchor and rebuild with a fresh two-pass when the shift has gone
		// stale (see ta_CORREL.c for the three triggers).
		barsSinceReseed--
		if ssX < 0.000001*sumX2 || ssY < 0.000001*sumY2 ||
			leavingX > 1000000.0*sumX2 || leavingY > 1000000.0*sumY2 || barsSinceReseed <= 0 {
			barsSinceReseed = 32 * optInTimePeriod
			windowStart := today - lookbackTotal
			tempReal = 0.0
			shiftY = 0.0
			for j := windowStart; j <= today; j++ {
				tempReal += inReal0[j]
				shiftY += inReal1[j]
			}
			// float64() keeps these products rounded: Go would otherwise fuse
			// them into the later `inReal - shift` subtractions.
			shiftX = float64(tempReal * invPeriod)
			shiftY = float64(shiftY * invPeriod)
			sumXY, sumX, sumY, sumX2, sumY2 = 0, 0, 0, 0, 0
			for j := windowStart; j <= today; j++ {
				x = inReal0[j] - shiftX
				sumX += x
				sumX2 += float64(x * x)
				y = inReal1[j] - shiftY
				sumXY += float64(x * y)
				sumY += y
				sumY2 += float64(y * y)
			}
			ssX = sumX2 - float64(sumX*sumX*invPeriod)
			ssY = sumY2 - float64(sumY*sumY*invPeriod)
			spXY = sumXY - float64(sumX*sumY*invPeriod)
			if ssX < 0.0 {
				ssX = 0.0
			}
			if ssY < 0.0 {
				ssY = 0.0
			}
		}

		// Save the trailing values before writing the output (buffers may alias).
		trailingX = inReal0[trailingIdx] - shiftX
		trailingY = inReal1[trailingIdx] - shiftY
		trailingIdx++

		// Each sum of squares is tested against its own scale (literal is
		// TA_EPSILON), and the product separately since it can underflow to 0.
		if ssX > 0.00000000000001*sumX2 && ssY > 0.00000000000001*sumY2 && ssX*ssY > 0.0 {
			tempReal = spXY / math.Sqrt(ssX*ssY)
			if tempReal > 1.0 {
				tempReal = 1.0
			} else if tempReal < 0-1.0 {
				tempReal = 0 - 1.0
			}
			outReal[outIdx] = tempReal
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++

		// Remove the trailing values (prepares the next window).
		leavingX = float64(trailingX * trailingX)
		leavingY = float64(trailingY * trailingY)
		sumX -= trailingX
		sumX2 -= leavingX
		sumXY -= float64(trailingX * trailingY)
		sumY -= trailingY
		sumY2 -= leavingY
		today++
		if today > endIdx {
			break
		}
	}
	return outBegIdx, outIdx, nil
}
