//go:build libmc || linux

// Allow-listed by name for libmc, but links on linux without libmc too: flagged (TestCgo_Allowance).
package cgouser

import "C"
