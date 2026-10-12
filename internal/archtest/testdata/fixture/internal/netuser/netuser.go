// Package netuser imports packages Ruling 79 confines. Violations (CheckImports): g0-net for net,
// crypto/tls, unsafe and os/exec; g0-mock for internal/kite/mock; g0-http for net/http.
package netuser

import (
	_ "crypto/tls"
	_ "net"
	_ "net/http"
	_ "os/exec"
	_ "unsafe"

	_ "fixture/internal/kite/mock"
)
