package timeuser

import (
	"time"
	tm "time"
)

// Wait has two rule-8 violations (one through an import alias) and one allowed use.
func Wait() time.Duration {
	_ = time.Now()
	tm.Sleep(time.Millisecond)
	return time.Duration(5) * time.Second
}
