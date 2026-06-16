package render

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/geofffranks/herdle/internal/dashboard"
)

// palette holds wip's ANSI codes, all empty when color is off so rendered output
// is byte-identical to plain text.
type palette struct {
	rst, bld, dim, red, grn, yel, blu, mag, cyn string
}

func newPalette(color bool) palette {
	if !color {
		return palette{}
	}
	return palette{
		rst: "\x1b[0m", bld: "\x1b[1m", dim: "\x1b[2m",
		red: "\x1b[31m", grn: "\x1b[32m", yel: "\x1b[33m",
		blu: "\x1b[34m", mag: "\x1b[35m", cyn: "\x1b[36m",
	}
}

// pad mirrors wip's pad(): truncate to w-1 runes + "…" when over w runes, else
// byte-pad to w (padRight). A truncated cell runs wider than w bytes (… is 3
// bytes) — wip does the same; golden fixtures lock the column bytes.
func pad(s string, w int) string {
	if w > 0 && utf8.RuneCountInString(s) > w {
		r := []rune(s)
		return string(r[:w-1]) + "…"
	}
	return padRight(s, w)
}

func (p palette) stateColor(lc string) string {
	switch lc {
	case "validated":
		return p.grn
	case "pending-validation":
		return p.yel
	case "in-development":
		return p.cyn
	case "planned":
		return p.blu
	case "designed":
		return p.mag
	default:
		return p.dim
	}
}

func (p palette) sevColor(sev dashboard.Severity) string {
	switch sev {
	case dashboard.SevGreen:
		return p.grn
	case dashboard.SevYellow:
		return p.yel
	case dashboard.SevRed:
		return p.red
	default:
		return p.dim
	}
}

func (p palette) fstate(lc string) string        { return p.stateColor(lc) + pad(lc, 19) + p.rst }
func (p palette) fdesc(s string, w int) string   { return p.dim + pad(s, w) + p.rst }
func (p palette) fbranch(s string, w int) string { return pad(s, w) }
func (p palette) hdr(s string) string            { return p.bld + s + p.rst }

func (p palette) ftk(id string) string {
	if id == "" {
		id = "-"
	}
	return p.cyn + pad(id, 9) + p.rst
}

func (p palette) ftklist(ids []string) string {
	s := strings.Join(ids, ",")
	if s == "" {
		s = "-"
	}
	return p.cyn + "[" + s + "]" + p.rst
}

func (p palette) fpr(n int) string {
	return p.bld + "#" + padRight(strconv.Itoa(n), 5) + p.rst
}

func (p palette) fsync(s dashboard.SyncState) string {
	switch s {
	case dashboard.SyncOK:
		return p.grn + "✓" + p.rst + "   "
	case dashboard.SyncBad:
		return p.red + "✗" + p.rst + "   "
	default:
		return p.dim + "·" + p.rst + "   "
	}
}

func (p palette) fflags(note dashboard.FlagNote) string {
	if note.Text == "" {
		return ""
	}
	return p.sevColor(note.Sev) + note.Text + p.rst
}

// fnotes joins note segments with " · ", coloring each by its own severity (the
// open-PR cell mixes merge status and a sync warning, which can differ in color).
func (p palette) fnotes(notes []dashboard.FlagNote) string {
	parts := make([]string, 0, len(notes))
	for _, n := range notes {
		if n.Text == "" {
			continue
		}
		parts = append(parts, p.sevColor(n.Sev)+n.Text+p.rst)
	}
	return strings.Join(parts, " · ")
}
