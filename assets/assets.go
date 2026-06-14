// Package assets bundles herdle's installable convention artifacts (Claude Code
// skills and the rules stub) into the binary via go:embed; internal/initcmd lays
// them on disk. The files under skills/ and rules/ are placeholders until S9
// (her-cung) ships the real, de-personalized content.
package assets

import "embed"

// FS holds the embedded skills/ and rules/ trees, mirrored onto disk by
// internal/initcmd. Paths use forward slashes (io/fs convention).
//
//go:embed skills rules
var FS embed.FS
