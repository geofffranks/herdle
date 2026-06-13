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

// PRCell is the open-PR-count cell.
type PRCell struct {
	State PRState
	Count int
}

// TKCell is the tk(in-progress/ready) cell. Present is false when there is no
// .tickets dir or HasTickets returns an error (any stat error treated as absent);
// the counts are 0 when the dir exists but tk fails (wip: "0/0").
type TKCell struct {
	Present    bool
	InProgress int
	Ready      int
}

// SummaryRow is one project's row in the cross-project summary.
type SummaryRow struct {
	Name string
	Head HeadInfo
	PR   PRCell
	TK   TKCell
}

// Engine gathers dashboard state through the vcs runners. DirExists abstracts the
// project-directory check so the engine is testable without touching disk; when
// nil it defaults to an os.Stat-backed check.
type Engine struct {
	Git       vcs.GitRunner
	GH        vcs.GHRunner
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
