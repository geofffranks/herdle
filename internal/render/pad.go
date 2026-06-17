package render

import (
	"strings"
	"unicode/utf8"
)

// padRight pads s with spaces to at least w bytes, matching bash printf's
// byte-counted "%-*s". Multibyte runes (e.g. the ↑/↓ arrows in a branch cell)
// therefore consume more than one display column — exactly the layout wip
// produces. Never truncates (wip's summary() does not truncate either).
func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

// padRightWidth pads s to at least w display columns, counting runes not bytes,
// so multibyte glyphs (the ↑/↓ branch arrows, the ✗/✓ merge glyphs) consume one
// column each and the trailing columns stay aligned.
func padRightWidth(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// padLeftWidth right-justifies s to at least w display columns, counting runes
// not bytes (the display-width companion to padLeft).
func padLeftWidth(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return strings.Repeat(" ", w-n) + s
}

// dispWidth is the display width of s in columns: the rune count. herdle's cells
// are ASCII plus a handful of single-column symbols (↑ ↓ ✗ ✓ ✎), none of them
// zero- or double-width, so rune count is the correct display width here.
func dispWidth(s string) int { return utf8.RuneCountInString(s) }
