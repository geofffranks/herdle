package render

import (
	"fmt"
	"io"
	"strconv"

	"github.com/geofffranks/herdle/internal/dashboard"
)

// Drilldown writes wip's per-repo drilldown for d to w, mirroring drilldown().
// color (from DetectColor) gates the palette; off => byte-identical to plain.
func Drilldown(w io.Writer, d dashboard.Drilldown, color bool) error {
	p := newPalette(color)
	out := &errWriter{w: w}

	out.line(fmt.Sprintf("### %s   (%s)", d.Name, d.Path))
	if d.Fetched {
		out.line("(fetched)")
	}
	out.line("")
	out.line("— git —  " + headString(d.Head))

	// open PRs
	if len(d.OpenPRs) > 0 {
		out.line("")
		out.line(p.hdr("— open PRs —"))
		for _, r := range d.OpenPRs {
			out.line("  " + p.fpr(r.Number) + " " + p.fbranch(r.Head, 30) + " " +
				p.fdesc(r.Title, 40) + "  " + p.ftklist(r.TKs) + "  " + p.fflags(r.Note))
		}
	} else if d.HasSlug && d.GHUnavailable {
		out.line("")
		out.line(p.hdr("— open PRs —"))
		out.line("  (gh unavailable)")
	}

	// merged PRs needing cleanup
	if len(d.MergedCleanup) > 0 {
		out.line("")
		out.line(p.hdr("— merged PRs needing cleanup —"))
		for _, r := range d.MergedCleanup {
			out.line("  " + p.fpr(r.Number) + " " + p.fbranch(r.Head, 30) + " " +
				p.fdesc(r.Title, 40) + "  " + p.fflags(r.Flags))
		}
	}

	// work in progress
	if len(d.WIP) > 0 {
		out.line("")
		out.line(p.hdr("— work in progress (in-flight tk + branches, not in a PR) —"))
		out.line(p.bld + "  " + padRight("state", 19) + " " + padRight("sync", 4) + " " +
			padRight("tk", 9) + " " + padRight("branch", 30) + " " + padRight("feature", 40) + " issues" + p.rst)
		for _, r := range d.WIP {
			issue := ""
			if r.Issue != "" {
				issue = p.sevColor(r.IssueSev) + r.Issue + p.rst
			}
			out.line("  " + p.fstate(r.Lifecycle) + " " + p.fsync(r.Sync) + " " +
				p.ftk(r.TKID) + " " + p.fbranch(r.Branch, 30) + " " + p.fdesc(r.Title, 40) + " " + issue)
		}
	}

	// up next
	if len(d.UpNext) > 0 {
		out.line("")
		out.line(p.hdr("— up next (open tk, not started) —"))
		for _, r := range d.UpNext {
			out.line("  " + p.fstate(r.Lifecycle) + " " + p.ftk(r.TKID) + " " +
				p.dim + "P" + strconv.Itoa(r.Priority) + p.rst + " " + r.Title)
		}
	}

	// design artifacts
	if len(d.Artifacts) > 0 {
		out.line("")
		out.line(p.hdr("— design artifacts (specs / plans / validation) —"))
		for _, r := range d.Artifacts {
			out.line("  " + p.ftk(r.TKID) + " " + padRight(r.Kind, 6) + " " + p.dim + r.Filename + p.rst)
		}
	}

	// legends (always)
	out.line("")
	out.line(fmt.Sprintf("sync: %s✓%s local==remote · %s✗%s differs (see issues) · %s·%s n/a — merged-PR & upstream-gone branches hidden, remote auto-pruned",
		p.grn, p.rst, p.red, p.rst, p.dim, p.rst))
	out.line(fmt.Sprintf("lifecycle: %s-%s (not started) → %sdesigned%s → %splanned%s → %sin-development%s → %spending-validation%s → %svalidated%s",
		p.dim, p.rst, p.mag, p.rst, p.blu, p.rst, p.cyn, p.rst, p.yel, p.rst, p.grn, p.rst))
	if d.GHAbsent {
		out.line(p.dim + "gh: not found — PR sections hidden" + p.rst)
	}

	return out.err
}
