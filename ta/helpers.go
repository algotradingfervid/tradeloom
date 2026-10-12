package ta

import "math"

// Shared numeric helpers, ported from TA-Lib's ta_utility.h and
// ta_codegen/input/helpers/*.c. Indicator files must use these rather than
// defining their own copies.

const (
	taEpsilon = 0.00000000000001 // TA_EPSILON
	taRealMin = -3e+37           // TA_REAL_MIN
	taRealMax = 3e+37            // TA_REAL_MAX
	taMaxIdx  = 100000000        // TA_MAX_INDEX
)

// isZero is TA_IS_ZERO.
func isZero(v float64) bool { return -taEpsilon < v && v < taEpsilon }

// isZeroOrNeg is TA_IS_ZERO_OR_NEG.
func isZeroOrNeg(v float64) bool { return v < taEpsilon }

// isZeroScaled is TA_IS_ZERO_SCALED.
func isZeroScaled(v, scale float64) bool { return math.Abs(v) <= taEpsilon*scale }

// realEq is TA_REAL_EQ.
func realEq(x, v, ep float64) bool { return v-ep <= x && x <= v+ep }

// cMax and cMin are TA-Lib's max/min macros: (a > b) ? a : b. Use them, not
// Go's builtin max/min or math.Max/Min, which treat NaN and -0 differently.
func cMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func cMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func cMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func cMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// trueRange is ta_true_range: max(high-low, |high-prevClose|, |low-prevClose|).
func trueRange(high, low, prevClose float64) float64 {
	r := high - low
	if t := math.Abs(high - prevClose); t > r {
		r = t
	}
	if t := math.Abs(low - prevClose); t > r {
		r = t
	}
	return r
}

// --- candlestick helpers (helpers/candlestick.c) ---

func realBody(close, open float64) float64 { return math.Abs(close - open) }

// candleColor is 1 for a white (close >= open) candle, -1 for black.
func candleColor(close, open float64) int {
	if close >= open {
		return 1
	}
	return -1
}

func upperShadow(high, close, open float64) float64 {
	if close >= open {
		return high - close
	}
	return high - open
}

func lowerShadow(low, close, open float64) float64 {
	if close >= open {
		return open - low
	}
	return close - low
}

func highLowRange(high, low float64) float64 { return high - low }

// realBodyGapUp is TA_REALBODYGAPUP(IDX2,IDX1): candle 2's body is entirely
// above candle 1's. Arguments are (open2, close2, open1, close1).
func realBodyGapUp(open2, close2, open1, close1 float64) bool {
	return cMin(open2, close2) > cMax(open1, close1)
}

// realBodyGapDown is TA_REALBODYGAPDOWN(IDX2,IDX1): candle 2's body is
// entirely below candle 1's. Arguments are (open2, close2, open1, close1).
func realBodyGapDown(open2, close2, open1, close1 float64) bool {
	return cMax(open2, close2) < cMin(open1, close1)
}

// candleGapUp is TA_CANDLEGAPUP(IDX2,IDX1): low2 > high1.
func candleGapUp(low2, high1 float64) bool { return low2 > high1 }

// candleGapDown is TA_CANDLEGAPDOWN(IDX2,IDX1): high2 < low1.
func candleGapDown(high2, low1 float64) bool { return high2 < low1 }

// candleRange is TA_CANDLERANGE. Shadows is upper+lower (not high-low-body):
// the two differ by floating-point reassociation (TA-Lib #217).
func candleRange(rt RangeType, open, high, low, close float64) float64 {
	switch rt {
	case RangeRealBody:
		return math.Abs(close - open)
	case RangeHighLow:
		return high - low
	case RangeShadows:
		return upperShadow(high, close, open) + lowerShadow(low, close, open)
	}
	return 0
}

// candleAverage is TA_CANDLEAVERAGE: factor * (sum/avgPeriod, or the current
// candle's range when avgPeriod is 0), halved for RangeShadows.
func candleAverage(s CandleSetting, sum, open, high, low, close float64) float64 {
	var avg float64
	if s.AvgPeriod != 0 {
		avg = sum / float64(s.AvgPeriod)
	} else {
		avg = candleRange(s.RangeType, open, high, low, close)
	}
	div := 1.0
	if s.RangeType == RangeShadows {
		div = 2.0
	}
	return s.Factor * avg / div
}

// candleSettingsSnapshot returns all candle settings under one lock, so a
// pattern sees a consistent set for the whole call.
func candleSettingsSnapshot() [candleSettingCount]CandleSetting {
	settings.RLock()
	defer settings.RUnlock()
	return settings.candle
}

// --- output shaping for the public API ---

// nanSlice returns a slice of n NaNs.
func nanSlice(n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = math.NaN()
	}
	return s
}

// alignReal shifts the nb values a range call wrote at out[0:nb] to
// out[beg:beg+nb] and fills the rest with NaN, so out is index-aligned with
// the input. out must have the full input length.
func alignReal(out []float64, beg, nb int) {
	copy(out[beg:beg+nb], out[:nb])
	nan := math.NaN()
	for i := range beg {
		out[i] = nan
	}
	for i := beg + nb; i < len(out); i++ {
		out[i] = nan
	}
}

// alignInt is alignReal for integer outputs; the padding is 0.
func alignInt(out []int, beg, nb int) {
	copy(out[beg:beg+nb], out[:nb])
	for i := range beg {
		out[i] = 0
	}
	for i := beg + nb; i < len(out); i++ {
		out[i] = 0
	}
}
