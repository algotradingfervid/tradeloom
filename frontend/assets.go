// Package frontend embeds the built web UI (frontend/dist) into the binary.
// `npm run build` (run by `wails build`) fills dist/; dist/.gitkeep keeps the directory present
// so `go build ./...` and `go test ./...` work before the first frontend build.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the contents of dist/ (index.html at the root).
func Assets() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at compile time; Sub cannot fail for a valid directory name
	}
	return sub
}
