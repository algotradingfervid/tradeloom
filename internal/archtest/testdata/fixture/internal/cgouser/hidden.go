//go:build cgohidden

package cgouser

// A cgo file hidden behind a tag no build sets (IgnoredFiles), in a grouped import. Flagged (g0-cgo).
import (
	"C"
	"fmt"
)

var _ = fmt.Sprint
