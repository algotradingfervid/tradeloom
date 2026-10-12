// Package ta mirrors tradeloom/ta for CheckTASources (Ruling 99): only the standard library and its own
// packages. Not compiled; the rule parses imports only.
package ta

import (
	"math"

	"tradeloom/ta/internal/oracle"
)

var _ = math.Pi
var _ = oracle.X
