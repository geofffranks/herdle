// Package dashboard gathers cross-project and per-repo work state from the vcs
// runners and classifies it into typed rows for the render layer. It performs no
// formatting and no color — that is internal/render's job — so the gather logic
// is unit-testable against the vcs fakes with no real repo, network, or tools.
package dashboard

import (
	"os"

	"github.com/geofffranks/herdle/internal/vcs"
)

// PRState classifies the PR-count cell, mirroring wip's pr_count outcomes.
type PRState int

const (
	PRNoSlug  PRState = iota // no gh slug resolved        -> "-"
	PRUnknown                // slug present, gh failed     -> "?"
	PRCounted                // slug present, gh succeeded  -> Count
)

// HeadInfo is the structured form of wip's git_head: branch (empty => detached),
// dirty flag, and ahead/behind vs the tracking branch.
type HeadInfo struct {
	Branch        string
	Dirty         bool
	Ahead, Behind int
}

// PRCell is the open-PR-count cell. Attention/Ready break the open PRs down by
// merge status for the summary's merge column; both 0 when none qualify.
type PRCell struct {
	State     PRState
	Count     int
	Attention int // Conflicts + ChecksFailing + ChangesRequested
	Ready     int // ready to merge
}

// TKCell is the tk(in-progress/ready) cell. Present is false when there is no
// .tickets dir or HasTickets returns an error (any stat error treated as absent);
// the counts are 0 when the dir exists but tk fails (wip: "0/0").
type TKCell struct {
	Present    bool
	InProgress int
	Ready      int
}

// IssueState classifies the summary iss cell.
type IssueState int

const (
	IssueUntracked IssueState = iota // fork / no forge / no slug -> "-"
	IssueUnknown                     // slug present, IssueList failed -> "?"
	IssueTracked                     // listed -> Open/Untriaged counts
)

// IssueCell is the summary iss cell: open count + un-triaged sub-count, mirroring
// the two-part merge cell. Capped is true when the fetch hit IssueFetchLimit.
type IssueCell struct {
	State     IssueState
	Open      int
	Untriaged int
	Capped    bool
}

// SummaryRow is one project's row in the cross-project summary.
type SummaryRow struct {
	Name     string
	Head     HeadInfo
	PR       PRCell
	TK       TKCell
	Problems int // count of flagged conditions in this repo's drilldown (excl. merge attention)
	Issues   IssueCell
}

// SummaryResult is the cross-project summary plus run-wide degradation state.
// AbsentForges lists the forge CLIs ("gh", "glab") that at least one routed
// project needed but could not be located, in a stable order, so the renderer can
// note (by name) that PR/MR counts are hidden. Empty when every needed CLI is
// present (or no project routes to a forge).
type SummaryResult struct {
	Rows         []SummaryRow
	AbsentForges []string
}

// Engine gathers dashboard state through the vcs runners. DirExists abstracts the
// project-directory check so the engine is testable without touching disk; when
// nil it defaults to an os.Stat-backed check.
type Engine struct {
	Git vcs.GitRunner
	GH  vcs.GHRunner
	// GL is the GitLab (glab) forge client. Optional: when nil, GitLab remotes are
	// treated as having no forge (git+tk only), exactly as before GitLab support.
	GL        vcs.GLRunner
	TK        vcs.TKRunner
	DirExists func(path string) bool
	// Glob abstracts filepath.Glob so disk-touching gather (lifecycle derivation,
	// design-artifact scan) is testable; when nil it defaults to filepath.Glob.
	Glob func(pattern string) ([]string, error)
}

func (e Engine) dirExists(path string) bool {
	if e.DirExists != nil {
		return e.DirExists(path)
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
