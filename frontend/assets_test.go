package frontend

import (
	"io/fs"
	"testing"
)

func TestAssets_RootIsDist(t *testing.T) {
	if _, err := fs.Stat(Assets(), ".gitkeep"); err != nil {
		t.Fatalf("dist/.gitkeep must be embedded at the FS root: %v", err)
	}
}
