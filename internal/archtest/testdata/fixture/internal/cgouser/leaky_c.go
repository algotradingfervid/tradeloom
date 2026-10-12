//go:build libmc || linux

package cgouser

// Allow-listed by name for libmc, but links on linux without libmc too: flagged (TestCgo_Allowance).
import "C"
