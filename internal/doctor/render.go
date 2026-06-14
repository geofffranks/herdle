package doctor

import (
	"fmt"
	"io"
)

const reset = "\x1b[0m"

// styleFor returns the glyph and ANSI color for a status.
func styleFor(s Status) (glyph, color string) {
	switch s {
	case OK:
		return "✓", "\x1b[32m" // green
	case Warn:
		return "⚠", "\x1b[33m" // yellow
	default: // Fail
		return "✗", "\x1b[31m" // red
	}
}

// Render writes the checklist to w. When color is false no ANSI is emitted, so
// output is byte-identical to plain text (mirrors the dashboard palette).
func Render(w io.Writer, rs []Result, color bool) {
	for _, r := range rs {
		glyph, c := styleFor(r.Status)
		if color {
			glyph = c + glyph + reset
		}
		fmt.Fprintf(w, "  %s %-14s %s\n", glyph, r.Name, r.Detail)
		if r.Status != OK && r.Remediation != "" {
			fmt.Fprintf(w, "      → %s\n", r.Remediation)
		}
	}
}
