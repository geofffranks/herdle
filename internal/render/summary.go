package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/geofffranks/herdle/internal/dashboard"
)

// Minimum column widths (display columns). The layout starts here — matching
// wip's `printf '%-30s %-26s %4s  %s'` for the common case — and grows any column
// whose content is wider, so a long branch name no longer shoves the trailing
// columns out of alignment.
const (
	colProject  = 30
	colBranch   = 26
	colPRs      = 4
	colMerge    = 7 // fits "✗N ✓M"
	colProblems = 8 // fits "problems"
)

// Summary writes the cross-project summary layout for rows to w. The view is
// monochrome (wip's summary() emits no ANSI). fetched selects the cache/fetch
// footer note; absentForges names the forge CLIs (e.g. "gh", "glab") that a
// routed project needed but could not be found, appended to the footer so the
// hidden PR/MR counts are explained.
//
// Column widths are computed from the widest cell in each column (display width,
// so the multibyte ↑/↓ branch arrows and ✗/✓ merge glyphs are counted as one
// column each), clamped to the minimums above. Every row — header, separator, and
// data — is laid out at the same widths, so the table stays aligned regardless of
// branch-name length.
func Summary(w io.Writer, rows []dashboard.SummaryRow, fetched bool, absentForges []string) error {
	out := &errWriter{w: w}

	// Render every cell up front so the widths can be measured before emitting.
	type cells struct{ project, branch, prs, merge, problems, tk string }
	body := make([]cells, len(rows))
	wp, wb, wpr, wm, wprob := colProject, colBranch, colPRs, colMerge, colProblems
	for i, r := range rows {
		c := cells{r.Name, headString(r.Head), prCell(r.PR), mergeCell(r.PR), problemsCell(r.Problems), tkCell(r.TK)}
		body[i] = c
		wp = max(wp, dispWidth(c.project))
		wb = max(wb, dispWidth(c.branch))
		wpr = max(wpr, dispWidth(c.prs))
		wm = max(wm, dispWidth(c.merge))
		wprob = max(wprob, dispWidth(c.problems))
	}

	emit := func(project, branch, prs, merge, problems, tk string) {
		out.line(padRightWidth(project, wp) + " " +
			padRightWidth(branch, wb) + " " +
			padLeftWidth(prs, wpr) + "  " +
			padRightWidth(merge, wm) + " " +
			padLeftWidth(problems, wprob) + "  " + tk)
	}
	dashes := func(s string) string { return strings.Repeat("-", dispWidth(s)) }

	emit("PROJECT", "BRANCH", "PRs", "merge", "problems", "tk(ip/ready)")
	emit(dashes("PROJECT"), dashes("BRANCH"), dashes("PRs"), dashes("merge"), dashes("problems"), dashes("tk(ip/ready)"))
	for _, c := range body {
		emit(c.project, c.branch, c.prs, c.merge, c.problems, c.tk)
	}
	note := "cached — herdle --fetch to refresh"
	if fetched {
		note = "fetched"
	}
	footer := "(" + note + `)  tk = in-progress/ready · problems = local/cleanup problems · run "herdle <name>" for detail · merge: ✗ need attention / ✓ ready to merge`
	if len(absentForges) > 0 {
		footer += " · " + strings.Join(absentForges, "/") + " not found — PR/MR counts hidden"
	}
	out.line("")
	out.line(footer)
	return out.err
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

// problemsCell renders the per-project problem count; 0 -> "-" (calm, matching the
// merge cell's "nothing to show" dash).
func problemsCell(n int) string {
	if n == 0 {
		return "-"
	}
	return strconv.Itoa(n)
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
