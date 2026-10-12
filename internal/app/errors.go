// internal/app/errors.go
package app

import (
	"encoding/json"
	"errors"

	"tradeloom/internal/service"
)

// ErrorFormatter turns a Go error into the JSON string the UI parses (ARCH §8.4).
func ErrorFormatter(err error) any {
	m := map[string]string{"code": "internal", "stage": "", "reason": err.Error()}
	var r *service.ErrRefused
	if errors.As(err, &r) {
		m = map[string]string{"code": r.Code, "stage": r.Stage, "reason": r.Reason}
	}
	b, _ := json.Marshal(m)
	return string(b)
}
