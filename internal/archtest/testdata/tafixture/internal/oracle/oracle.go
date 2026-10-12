//go:build oracle || linux

// Package oracle is on the cgo allow-list for the oracle tag, but this constraint links it on linux
// without the tag: C and unsafe are flagged (g0-ta).
package oracle

import "C"
import "unsafe"

var X unsafe.Pointer
