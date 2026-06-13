// Package render turns the dashboard engine's typed rows into wip-identical text.
// Data gathering lives in internal/dashboard; this package only formats and (in
// later stories) colors, so it can be golden-tested in isolation.
package render

import (
	"io"
	"os"

	"golang.org/x/term"
)

// DetectColor reports whether ANSI color should be emitted on w, mirroring wip's
// gate exactly:
//
//	(isatty(stdout) || CLICOLOR_FORCE || FORCE_COLOR) && !NO_COLOR
//
// wip uses non-empty tests ([ -n ] / [ -z ]), so a variable set to "" counts as
// unset. NO_COLOR (non-empty) always wins; then an explicit force; otherwise color
// is on only when w is a terminal-backed *os.File.
//
// The summary view (S4) emits no color regardless; DetectColor is the foundation
// a later drilldown palette consumes.
func DetectColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") != "" || os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}
