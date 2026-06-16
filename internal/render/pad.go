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

// padLeft right-justifies s to at least w bytes, matching bash printf's "%*s".
func padLeft(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return strings.Repeat(" ", w-len(s)) + s
}

// padRightWidth pads s to at least w display columns, counting runes not bytes,
// so the multibyte ✗/✓ glyphs in the merge column keep the trailing tk column
// aligned. (The other columns keep wip's byte-counted padRight.)
func padRightWidth(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}
