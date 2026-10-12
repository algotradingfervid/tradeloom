// internal/app/errors.go
package app

import (
	"encoding/json"
	"errors"

	"tradeloom/internal/logging"
	"tradeloom/internal/service"
)

// ErrorFormatter turns a Go error into the JSON string the UI parses (ARCH §8.4).
func ErrorFormatter(err error) any {
	m := map[string]string{"code": "internal", "stage": "", "reason": err.Error()}
	var r *service.ErrRefused
	if errors.As(err, &r) {
		m = map[string]string{"code": r.Code, "stage": r.Stage, "reason": r.Reason}
	}
	m["reason"] = logging.Default.RedactString(m["reason"]) // every reason, not only the façade's (Ruling 82 M4)
	b, _ := json.Marshal(m)
	return string(b)
}
