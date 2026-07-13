// Package assets bundles herdle's installable convention artifacts into the
// binary via go:embed; internal/initcmd lays them on disk.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed claude
var bundle embed.FS

// ClaudeFS holds the Claude skills/ and rules/ trees.
var ClaudeFS = mustSub(bundle, "claude")

//go:embed polytoken
var polytokenBundle embed.FS

// PolytokenFS holds the Polytoken skills/ tree and herdle.md context fragment.
var PolytokenFS = mustSub(polytokenBundle, "polytoken")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
