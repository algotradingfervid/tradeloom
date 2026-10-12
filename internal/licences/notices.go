// internal/licences/notices.go
// Package licences embeds the generated third-party notices for System.Info (FR-SHELL-019 About).
package licences

import _ "embed"

// Notices is the copy of THIRD_PARTY_NOTICES.md refreshed by `make notices`.
//
//go:embed THIRD_PARTY_NOTICES.md
var Notices []byte
