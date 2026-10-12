//go:build libmc

package ta

// Test-only: the C library's libm, so differential tests can require
// bit-exact agreement with TA-Lib C (see libm.go).

/*
#cgo LDFLAGS: -lm
#include <math.h>
*/
import "C"

func libmAtan(x float64) float64  { return float64(C.atan(C.double(x))) }
func libmSin(x float64) float64   { return float64(C.sin(C.double(x))) }
func libmCos(x float64) float64   { return float64(C.cos(C.double(x))) }
func libmTan(x float64) float64   { return float64(C.tan(C.double(x))) }
func libmAsin(x float64) float64  { return float64(C.asin(C.double(x))) }
func libmAcos(x float64) float64  { return float64(C.acos(C.double(x))) }
func libmExp(x float64) float64   { return float64(C.exp(C.double(x))) }
func libmLog(x float64) float64   { return float64(C.log(C.double(x))) }
func libmLog10(x float64) float64 { return float64(C.log10(C.double(x))) }
func libmSinh(x float64) float64  { return float64(C.sinh(C.double(x))) }
func libmCosh(x float64) float64  { return float64(C.cosh(C.double(x))) }
func libmTanh(x float64) float64  { return float64(C.tanh(C.double(x))) }
