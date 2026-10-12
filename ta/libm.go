//go:build !libmc

package ta

import "math"

// The transcendental functions every indicator calls. They differ from the C
// library's libm in the last ulp. Tests built with the libmc tag (libm_c.go)
// route them to libm instead, so the oracle can demand bit-exact output
// everywhere: go test -tags oracle,libmc ./...

func libmAtan(x float64) float64  { return math.Atan(x) }
func libmSin(x float64) float64   { return math.Sin(x) }
func libmCos(x float64) float64   { return math.Cos(x) }
func libmTan(x float64) float64   { return math.Tan(x) }
func libmAsin(x float64) float64  { return math.Asin(x) }
func libmAcos(x float64) float64  { return math.Acos(x) }
func libmLog(x float64) float64   { return math.Log(x) }
func libmLog10(x float64) float64 { return math.Log10(x) }
func libmTanh(x float64) float64  { return math.Tanh(x) }

// libmOverflowStart is just below log(math.MaxFloat64) (~709.78). Go's
// math.Cosh/Sinh compute Exp(|x|)*0.5 for |x| > 21, so they return ±Inf for
// |x| in (709.78, 710.48] where libm (and the true value) is still finite, up
// to ~1.8e308. In that band libmCosh/libmSinh use (exp(|x|/2)/2)*exp(|x|/2),
// fdlibm's split, which overflows only where the function really does.
const libmOverflowStart = 709.0

func libmCosh(x float64) float64 {
	if a := math.Abs(x); a > libmOverflowStart && !math.IsInf(a, 0) {
		w := math.Exp(0.5 * a)
		return (0.5 * w) * w
	}
	return math.Cosh(x)
}

func libmSinh(x float64) float64 {
	if a := math.Abs(x); a > libmOverflowStart && !math.IsInf(a, 0) {
		w := math.Exp(0.5 * a)
		return math.Copysign((0.5*w)*w, x)
	}
	return math.Sinh(x)
}

// libmExpSubnormalStart is just above log(math.SmallestNormal) (~-708.396): below
// it exp(x) is subnormal.
const libmExpSubnormalStart = -708.0

// libmExp is C's exp(). math.Exp is used for normal results, where it agrees
// with libm to the last ulp or so. On arm64 (and amd64) math.Exp dispatches to
// assembly whose subnormal path returns 0 for x in about [-745.133, -745.0],
// where libm (and the true value, rounded) gives math.SmallestNonzeroFloat64.
// For subnormal results use libmExpFdlibm, the portable fdlibm algorithm (Go's
// own pure-Go math.exp), which gradually underflows like libm.
func libmExp(x float64) float64 {
	if x < libmExpSubnormalStart {
		return libmExpFdlibm(x)
	}
	return math.Exp(x)
}

// libmExpFdlibm is Go's pure-Go math.exp (fdlibm e_exp.c), with every
// product rounded before it is added, as in C without contraction.
func libmExpFdlibm(x float64) float64 {
	const (
		ln2Hi = 6.93147180369123816490e-01
		ln2Lo = 1.90821492927058770002e-10
		log2e = 1.44269504088896338700e+00

		overflow  = 7.09782712893383973096e+02
		underflow = -7.45133219101941108420e+02
		nearZero  = 1.0 / (1 << 28) // 2**-28

		p1 = 1.66666666666666657415e-01  /* 0x3FC55555; 0x55555555 */
		p2 = -2.77777777770155933842e-03 /* 0xBF66C16C; 0x16BEBD93 */
		p3 = 6.61375632143793436117e-05  /* 0x3F11566A; 0xAF25DE2C */
		p4 = -1.65339022054652515390e-06 /* 0xBEBBBD41; 0xC5D26BF1 */
		p5 = 4.13813679705723846039e-08  /* 0x3E663769; 0x72BEA4D0 */
	)

	switch {
	case math.IsNaN(x) || math.IsInf(x, 1):
		return x
	case math.IsInf(x, -1):
		return 0
	case x > overflow:
		return math.Inf(1)
	case x < underflow:
		return 0
	case -nearZero < x && x < nearZero:
		return 1 + x
	}

	// Reduce; computed as r = hi - lo for extra precision.
	var k int
	switch {
	case x < 0:
		k = int(float64(log2e*x) - 0.5)
	case x > 0:
		k = int(float64(log2e*x) + 0.5)
	}
	hi := x - float64(float64(k)*ln2Hi)
	lo := float64(float64(k) * ln2Lo)

	r := hi - lo
	t := r * r
	c := r - float64(t*(p1+float64(t*(p2+float64(t*(p3+float64(t*(p4+float64(t*p5)))))))))
	y := 1 - ((lo - float64(r*c)/(2-c)) - hi)
	return math.Ldexp(y, k)
}
