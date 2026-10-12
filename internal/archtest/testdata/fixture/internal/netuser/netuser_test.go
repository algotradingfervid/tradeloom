package netuser

import (
	"net"
	"testing"

	_ "fixture/internal/kite/mock"
)

// Test files are not loaded: no import rule applies.
func TestNet(t *testing.T) { _ = net.IPv4len }
