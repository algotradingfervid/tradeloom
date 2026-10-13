// Package httpvia reaches net/http identities without the http qualifier (g0-httpuse, matched by
// identity): a method on a *http.Client another package returns, with no net/http import, and a dot-import.
package httpvia

import "fixture/internal/kite"

func Send() { _, _ = kite.Shared().Post("https://api.kite.trade/user/profile", "text/plain", nil) }
