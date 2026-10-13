//go:build libmc

// On the cgo allow-list (ta/libm_c.go, libmc) and links only with libmc: C and unsafe are allowed.
package ta

import "C"
import "unsafe"

var _ unsafe.Pointer
