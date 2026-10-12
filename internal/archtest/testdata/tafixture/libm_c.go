//go:build libmc

package ta

// On the cgo allow-list (ta/libm_c.go, libmc) and links only with libmc: C and unsafe are allowed.
import "C"
import "unsafe"

var _ unsafe.Pointer
