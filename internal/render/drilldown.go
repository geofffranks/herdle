package render

import (
	"fmt"
	"io"
	"strconv"

	"github.com/geofffranks/herdle/internal/dashboard"
	"github.com/geofffranks/herdle/internal/vcs"
)

// Drilldown writes wip's per-repo drilldown for d to w, mirroring drilldown().
// color (from DetectColor) gates the palette; off => byte-identical to plain.
func Drilldown(w io.Writer, d dashboard.Drilldown, color bool) error {
	p := newPalette(color)
	out := &errWriter{w: w}

	// Name the forge's CLI and change-request noun so GitLab repos read naturally
	// (glab / MR) instead of GitHub-specific copy (gh / PR).
	cli, noun := "gh", "PR"
	if d.Forge == "gitlab" {
		cli, noun = "glab", "MR"
	}

	out.line(fmt.Sprintf("### %s   (%s)", d.Name, d.Path))
	if d.Fetched {
		out.line("(fetched)")
	}
	out.line("")
	out.line("— git —  " + headString(d.Head))

	// open PRs/MRs
	if len(d.OpenPRs) > 0 {
		out.line("")
		out.line(p.hdr("— open " + noun + "s —"))
		for _, r := range d.OpenPRs {
			out.line("  " + p.fpr(r.Number) + " " + p.fbranch(r.Head, 30) + " " +
				p.fdesc(r.Title, 40) + "  " + p.ftklist(r.TKs) + "  " + p.fnotes(r.Notes))
		}
	} else if d.HasSlug && d.ForgeUnavailable {
		out.line("")
		out.line(p.hdr("— open " + noun + "s —"))
		out.line("  (" + cli + " unavailable)")
	}

	// merged PRs/MRs needing cleanup
	if len(d.MergedCleanup) > 0 {
		out.line("")
		out.line(p.hdr("— merged " + noun + "s needing cleanup —"))
		for _, r := range d.MergedCleanup {
			out.line("  " + p.fpr(r.Number) + " " + p.fbranch(r.Head, 30) + " " +
				p.fdesc(r.Title, 40) + "  " + p.fflags(r.Flags))
		}
	}

	// open issues (source-of-truth repos only)
	if d.TrackIssues {
		if d.IssuesUnavailable {
			out.line("")
			out.line(p.hdr("— open issues —"))
			out.line("  (" + cli + " unavailable)")
		} else if len(d.OpenIssues) > 0 || d.TriagedHidden > 0 {
			out.line("")
			hdr := "— open issues —"
			if d.IssuesCapped {
				hdr += fmt.Sprintf(" (showing first %d)", vcs.IssueFetchLimit)
			}
			out.line(p.hdr(hdr))
			for _, r := range d.OpenIssues {
				marker := p.ftklist(r.TKs)
				if r.Untriaged {
					marker = p.sevColor(dashboard.SevYellow) + "⚑ untriaged" + p.rst
				}
				out.line("  " + p.fpr(r.Number) + " " + marker + "  " + p.fdesc(r.Title, 50))
			}
			if d.TriagedHidden > 0 {
				out.line("  " + p.dim + "+ " + strconv.Itoa(d.TriagedHidden) + " triaged" + p.rst)
			}
		}
	}

	// work in progress
	if len(d.WIP) > 0 {
		out.line("")
		out.line(p.hdr("— work in progress (in-flight tk + branches, not in a " + noun + ") —"))
		out.line(p.bld + "  " + padRight("state", 19) + " " + padRight("sync", 4) + " " +
			padRight("tk", 9) + " " + padRight("branch", 30) + " " + padRight("feature", 40) + " problems" + p.rst)
		for _, r := range d.WIP {
			problem := ""
			if r.Problem != "" {
				problem = p.sevColor(r.ProblemSev) + r.Problem + p.rst
			}
			out.line("  " + p.fstate(r.Lifecycle) + " " + p.fsync(r.Sync) + " " +
				p.ftk(r.TKID) + " " + p.fbranch(r.Branch, 30) + " " + p.fdesc(r.Title, 40) + " " + problem)
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
	out.line(fmt.Sprintf("sync: %s✓%s local==remote · %s✗%s differs (see problems) · %s·%s n/a — merged-%s & upstream-gone branches hidden, remote auto-pruned",
		p.grn, p.rst, p.red, p.rst, p.dim, p.rst, noun))
	out.line(noun + " status: " + p.grn + "✓ ready to merge" + p.rst + " · " +
		p.red + "✗ conflicts" + p.rst + " · " + p.red + "✗ checks failing" + p.rst + " · " +
		p.yel + "✎ changes requested" + p.rst + " · " + p.yel + "⚠ blocked" + p.rst + " · " +
		p.dim + "— pending/draft/computing" + p.rst)
	out.line(fmt.Sprintf("lifecycle: %s-%s (not started) → %sdesigned%s → %splanned%s → %sin-development%s → %spending-validation%s → %svalidated%s",
		p.dim, p.rst, p.mag, p.rst, p.blu, p.rst, p.cyn, p.rst, p.yel, p.rst, p.grn, p.rst))
	if d.TrackIssues {
		out.line("issues: " + p.yel + "⚑ untriaged" + p.rst + " (needs a tk) · [tk …] tracked — source-of-truth repos only")
	}
	if d.ForgeAbsent {
		out.line(p.dim + cli + ": not found — " + noun + " sections hidden" + p.rst)
	}

	return out.err
}
