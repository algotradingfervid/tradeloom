//go:build sometag

package ta

// Hidden behind a tag, still checked: every import here is flagged (g0-ta).
import (
	"net/http"
	"os/exec"
	"plugin"
	"syscall"
	"unsafe"

	"github.com/example/dep"
	"golang.org/x/sys/unix"
)
