package ta

// Test files are not linked into the binary: exempt.
import "net/http"

var _ = http.Get
