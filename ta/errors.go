package ta

import (
	"errors"
	"fmt"
)

// ErrBadParam is wrapped by every parameter-validation error, so callers can
// test with errors.Is(err, ta.ErrBadParam). It corresponds to TA_BAD_PARAM.
var ErrBadParam = errors.New("ta: bad parameter")

// ErrOutOfRangeStartIndex and ErrOutOfRangeEndIndex are returned by the
// range forms for an invalid startIdx/endIdx (TA_OUT_OF_RANGE_START_INDEX /
// TA_OUT_OF_RANGE_END_INDEX).
var (
	ErrOutOfRangeStartIndex = errors.New("ta: start index out of range")
	ErrOutOfRangeEndIndex   = errors.New("ta: end index out of range")
)

// checkRange is the startIdx/endIdx prologue every TA-Lib function runs.
func checkRange(startIdx, endIdx int) error {
	if startIdx < 0 || startIdx > taMaxIdx {
		return ErrOutOfRangeStartIndex
	}
	if endIdx < 0 || endIdx > taMaxIdx || endIdx < startIdx {
		return ErrOutOfRangeEndIndex
	}
	return nil
}

// ParamError describes an invalid argument to an indicator.
type ParamError struct {
	Func   string // indicator name, e.g. "SMA"
	Param  string // parameter name, e.g. "optInTimePeriod" or "inHigh"
	Reason string // human-readable reason
}

func (e *ParamError) Error() string {
	return fmt.Sprintf("ta: %s: %s: %s", e.Func, e.Param, e.Reason)
}

func (e *ParamError) Unwrap() error { return ErrBadParam }

func badParam(fn, param, format string, args ...any) error {
	return &ParamError{Func: fn, Param: param, Reason: fmt.Sprintf(format, args...)}
}

// checkInt validates an integer optional input against its inclusive range
// from the TA-Lib spec (yaml `range:`).
func checkInt(fn, param string, v, lo, hi int) error {
	if v < lo || v > hi {
		return badParam(fn, param, "%d out of range [%d, %d]", v, lo, hi)
	}
	return nil
}

// checkReal validates a real optional input against its inclusive range.
// NaN is always rejected.
func checkReal(fn, param string, v, lo, hi float64) error {
	if v != v || v < lo || v > hi {
		return badParam(fn, param, "%g out of range [%g, %g]", v, lo, hi)
	}
	return nil
}

// checkMAType validates an MAType optional input.
func checkMAType(fn, param string, t MAType) error {
	if t < MATypeSMA || t > MATypeRMA {
		return badParam(fn, param, "unknown MAType %d", int(t))
	}
	return nil
}

// checkInputs verifies that all input series have the same length and
// returns it. A nil series is treated as empty, as usual in Go. names and
// series are parallel.
func checkInputs(fn string, names []string, series ...[]float64) (int, error) {
	n := 0
	for i, s := range series {
		if i == 0 {
			n = len(s)
		} else if len(s) != n {
			return 0, badParam(fn, names[i], "length %d differs from %s length %d", len(s), names[0], n)
		}
	}
	return n, nil
}

// checkBuf verifies, in a range form, that every buffer can be indexed up to
// endIdx. C cannot check this; Go must, or it would panic.
func checkBuf(fn string, endIdx int, names []string, bufs ...[]float64) error {
	for i, b := range bufs {
		if len(b) <= endIdx {
			return badParam(fn, names[i], "length %d too short for endIdx %d", len(b), endIdx)
		}
	}
	return nil
}

// checkIntBuf is checkBuf for an integer output buffer.
func checkIntBuf(fn string, endIdx int, name string, b []int) error {
	if len(b) <= endIdx {
		return badParam(fn, name, "length %d too short for endIdx %d", len(b), endIdx)
	}
	return nil
}
