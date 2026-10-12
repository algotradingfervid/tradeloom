//go:build libmc

package cgouser

// Links only with the libmc tag: allowed when the allowance names this file and libmc (TestCgo_Allowance).
import "C"
