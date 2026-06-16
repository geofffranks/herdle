package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/geofffranks/herdle/internal/dashboard"
)

// Column widths match wip's `printf '%-30s %-26s %4s  %s'`.
const (
	colProject = 30
	colBranch  = 26
	colPRs     = 4
	colMerge   = 7 // display width; fits "✗N ✓M"
)

// Summary writes wip's cross-project summary layout for rows to w. The view is
// monochrome (wip's summary() emits no ANSI). fetched selects the cache/fetch
// footer note; ghAbsent appends a note that PR counts are hidden because the gh
// binary was not found.
func Summary(w io.Writer, rows []dashboard.SummaryRow, fetched, ghAbsent bool) error {
	out := &errWriter{w: w}
	out.line(row("PROJECT", "BRANCH", "PRs", "merge", "tk(ip/ready)"))
	out.line(row("-------", "------", "---", "-----", "------------"))
	for _, r := range rows {
		out.line(row(r.Name, headString(r.Head), prCell(r.PR), mergeCell(r.PR), tkCell(r.TK)))
	}
	note := "cached — herdle --fetch to refresh"
	if fetched {
		note = "fetched"
	}
	footer := "(" + note + `)  tk = in-progress/ready · run "herdle <name>" for detail · merge: ✗ need attention / ✓ ready to merge`
	if ghAbsent {
		footer += " · gh not found — PR counts hidden"
	}
	out.line("")
	out.line(footer)
	return out.err
}

// row assembles one line in wip's exact column layout. The merge cell is padded
// by display width (it carries multibyte glyphs); all other columns keep byte
// padding.
func row(project, branch, prs, merge, tk string) string {
	return padRight(project, colProject) + " " +
		padRight(branch, colBranch) + " " +
		padLeft(prs, colPRs) + "  " +
		padRightWidth(merge, colMerge) + " " + tk
}

// headString mirrors wip's git_head assembly: branch (or "(detached)"), a "*"
// when dirty, "  ↑<ahead>" (two leading spaces) when ahead, " ↓<behind>" (one
// leading space) when behind.
func headString(h dashboard.HeadInfo) string {
	br := h.Branch
	if br == "" {
		br = "(detached)"
	}
	s := br
	if h.Dirty {
		s += "*"
	}
	if h.Ahead != 0 {
		s += "  ↑" + strconv.Itoa(h.Ahead)
	}
	if h.Behind != 0 {
		s += " ↓" + strconv.Itoa(h.Behind)
	}
	return s
}

func prCell(p dashboard.PRCell) string {
	switch p.State {
	case dashboard.PRNoSlug:
		return "-"
	case dashboard.PRUnknown:
		return "?"
	default:
		return strconv.Itoa(p.Count)
	}
}

// mergeCell renders the merge column: "✗N ✓M" (zero parts omitted), "-" when
// nothing qualifies or no slug, "?" when gh failed. Monochrome (summary emits no
// ANSI); the glyphs alone carry meaning.
func mergeCell(p dashboard.PRCell) string {
	switch p.State {
	case dashboard.PRNoSlug:
		return "-"
	case dashboard.PRUnknown:
		return "?"
	default:
		if p.Attention == 0 && p.Ready == 0 {
			return "-"
		}
		parts := make([]string, 0, 2)
		if p.Attention > 0 {
			parts = append(parts, "✗"+strconv.Itoa(p.Attention))
		}
		if p.Ready > 0 {
			parts = append(parts, "✓"+strconv.Itoa(p.Ready))
		}
		return strings.Join(parts, " ")
	}
}

func tkCell(t dashboard.TKCell) string {
	if !t.Present {
		return "-"
	}
	return strconv.Itoa(t.InProgress) + "/" + strconv.Itoa(t.Ready)
}

// errWriter collects the first write error so Summary need not check every line.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) line(s string) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintln(e.w, s)
}
