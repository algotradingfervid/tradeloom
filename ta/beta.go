package ta

// BETA — Beta (Statistic Functions).
// Ported from TA-Lib v0.8.1 ta_codegen/input/beta and src/ta_func/ta_BETA.c
// (the shifted-returns / reseed variant of the compiled C).
//
// The C library is built with -ffp-contract=off. Go may fuse x*y+z into an
// FMA (also across statements), so every product that feeds an addition or
// subtraction is wrapped in an explicit float64() conversion, which the spec
// guarantees rounds and therefore blocks fusion.

// BETALookback returns the number of leading bars BETA leaves empty.
// optInTimePeriod: [1, 100000], default 5.
func BETALookback(optInTimePeriod int) int {
	if checkInt("BETA", "optInTimePeriod", optInTimePeriod, 1, 100000) != nil {
		return -1
	}
	return betaLookback(optInTimePeriod)
}

func betaLookback(optInTimePeriod int) int {
	return optInTimePeriod
}

// BETA returns the rolling beta of inReal1's returns regressed on inReal0's
// returns over optInTimePeriod returns. The output is index-aligned with the
// inputs; the first BETALookback bars are NaN.
// optInTimePeriod: [1, 100000], default 5.
func BETA(inReal0, inReal1 []float64, optInTimePeriod int) ([]float64, error) {
	if err := checkInt("BETA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return nil, err
	}
	n, err := checkInputs("BETA", []string{"inReal0", "inReal1"}, inReal0, inReal1)
	if err != nil {
		return nil, err
	}
	out := make([]float64, n)
	if n == 0 {
		return out, nil
	}
	beg, nb, err := betaRange(0, n-1, inReal0, inReal1, optInTimePeriod, out)
	if err != nil {
		return nil, err
	}
	alignReal(out, beg, nb)
	return out, nil
}

// betaRange mirrors TA_BETA: it computes [startIdx, endIdx] and writes
// outNBElement values to outReal[0:], the first one belonging to bar
// outBegIdx. outReal may alias either input.
func betaRange(startIdx, endIdx int, inReal0, inReal1 []float64, optInTimePeriod int, outReal []float64) (outBegIdx, outNBElement int, err error) {
	if err := checkRange(startIdx, endIdx); err != nil {
		return 0, 0, err
	}
	if err := checkInt("BETA", "optInTimePeriod", optInTimePeriod, 1, 100000); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("BETA", endIdx, []string{"inReal0", "inReal1"}, inReal0, inReal1); err != nil {
		return 0, 0, err
	}
	if err := checkBuf("BETA", endIdx-startIdx, []string{"outReal"}, outReal); err != nil {
		return 0, 0, err
	}
	return betaCore(startIdx, endIdx, inReal0, inReal1, optInTimePeriod, outReal)
}

// betaReturn is the C return expression
// `(last != 0.0) ? (price - last) / last - shift : 0 - shift`.
func betaReturn(price, last, shift float64) float64 {
	if last != 0.0 {
		return (price-last)/last - shift
	}
	return 0 - shift
}

// betaCore is the TA_BETA body after validation.
func betaCore(startIdx, endIdx int, inReal0, inReal1 []float64, optInTimePeriod int, outReal []float64) (int, int, error) {
	var (
		sXX, sXY, sX, sY, sYY        float64
		lastPriceX, lastPriceY       float64
		trailingLastX, trailingLastY float64
		tmpReal                      float64
		shiftX, shiftY               float64
		denom, denomScale            float64
		prevX, prevY                 float64
		leavingXX, leavingYY         float64
		x, y                         float64
	)

	nbInitialElementNeeded := optInTimePeriod
	if startIdx < nbInitialElementNeeded {
		startIdx = nbInitialElementNeeded
	}
	if startIdx > endIdx {
		return 0, 0, nil
	}

	// Consume first input.
	trailingIdx := startIdx - nbInitialElementNeeded
	trailingLastX = inReal0[trailingIdx]
	lastPriceX = trailingLastX
	trailingLastY = inReal1[trailingIdx]
	lastPriceY = trailingLastY

	// Anchor the shift on the window's first return.
	trailingIdx++
	i := trailingIdx
	if lastPriceX != 0.0 {
		shiftX = (inReal0[i] - lastPriceX) / lastPriceX
	}
	if lastPriceY != 0.0 {
		shiftY = (inReal1[i] - lastPriceY) / lastPriceY
	}
	for i < startIdx {
		tmpReal = inReal0[i]
		x = betaReturn(tmpReal, lastPriceX, shiftX)
		lastPriceX = tmpReal
		tmpReal = inReal1[i]
		i++
		y = betaReturn(tmpReal, lastPriceY, shiftY)
		lastPriceY = tmpReal
		sXX += float64(x * x)
		sYY += float64(y * y)
		sXY += float64(x * y)
		sX += x
		sY += y
	}

	outIdx := 0
	n := float64(optInTimePeriod)
	barsSinceReseed := 32 * optInTimePeriod
	for {
		tmpReal = inReal0[i]
		x = betaReturn(tmpReal, lastPriceX, shiftX)
		lastPriceX = tmpReal
		tmpReal = inReal1[i]
		i++
		y = betaReturn(tmpReal, lastPriceY, shiftY)
		lastPriceY = tmpReal
		sXX += float64(x * x)
		sYY += float64(y * y)
		sXY += float64(x * y)
		sX += x
		sY += y
		denomScale = float64(n * sXX)
		denom = denomScale - float64(sX*sX)

		// Re-anchor and rebuild when the shift has gone stale (see ta_BETA.c).
		barsSinceReseed--
		if denom < float64(0.000001*denomScale) || leavingXX > float64(1000.0*sXX) ||
			leavingYY > float64(1000.0*sYY) || barsSinceReseed <= 0 {
			barsSinceReseed = 32 * optInTimePeriod
			windowStart := trailingIdx
			prevX = trailingLastX
			prevY = trailingLastY
			tmpReal = 0.0
			shiftY = 0.0
			for j := windowStart; j < i; j++ {
				if prevX != 0.0 {
					tmpReal += (inReal0[j] - prevX) / prevX
				}
				prevX = inReal0[j]
				if prevY != 0.0 {
					shiftY += (inReal1[j] - prevY) / prevY
				}
				prevY = inReal1[j]
			}
			shiftX = tmpReal / n
			shiftY = shiftY / n
			prevX = trailingLastX
			prevY = trailingLastY
			sXX = 0.0
			sYY = 0.0
			sXY = 0.0
			sX = 0.0
			sY = 0.0
			for j := windowStart; j < i; j++ {
				x = betaReturn(inReal0[j], prevX, shiftX)
				prevX = inReal0[j]
				y = betaReturn(inReal1[j], prevY, shiftY)
				prevY = inReal1[j]
				sXX += float64(x * x)
				sYY += float64(y * y)
				sXY += float64(x * y)
				sX += x
				sY += y
			}
			denomScale = float64(n * sXX)
			denom = denomScale - float64(sX*sX)
			if denom < 0.0 {
				denom = 0.0
			}
		}

		// Read the trailing before writing the output (buffers may alias).
		tmpReal = inReal0[trailingIdx]
		x = betaReturn(tmpReal, trailingLastX, shiftX)
		trailingLastX = tmpReal
		tmpReal = inReal1[trailingIdx]
		trailingIdx++
		y = betaReturn(tmpReal, trailingLastY, shiftY)
		trailingLastY = tmpReal

		if denom > float64(0.00000000000001*denomScale) {
			outReal[outIdx] = (float64(n*sXY) - float64(sX*sY)) / denom
		} else {
			outReal[outIdx] = 0.0
		}
		outIdx++

		// Remove the calculation starting with the trailingIdx.
		leavingXX = float64(x * x)
		leavingYY = float64(y * y)
		sXX -= float64(x * x)
		sYY -= float64(y * y)
		sXY -= float64(x * y)
		sX -= x
		sY -= y

		if i > endIdx {
			break
		}
	}
	return startIdx, outIdx, nil
}
