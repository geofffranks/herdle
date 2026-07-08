package dashboard_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/dashboard"
	"github.com/geofffranks/herdle/internal/vcs"
	"github.com/geofffranks/herdle/internal/vcs/vcsfakes"
)

var _ = Describe("Engine parity — problemCount equals Drilldown flagged-row count", func() {
	It("reports the same count for a mixed fixture with merged cleanup, WIP problem, and open-PR sync flag", func() {
		git := &vcsfakes.FakeGitRunner{}
		gh := &vcsfakes.FakeGHRunner{}
		tk := &vcsfakes.FakeTKRunner{}
		eng := dashboard.Engine{
			Git:  git,
			GH:   gh,
			TK:   tk,
			Glob: func(string) ([]string, error) { return nil, nil },
		}

		r := config.Resolved{
			Path:         "/r",
			Remote:       "origin",
			Slug:         "o/r",
			SlugExplicit: true,
			RemoteHost:   "github.com",
		}

		// Mixed PR list:
		//   PR 1: MERGED, local branch lingers              -> 1 MergedCleanup row (problem)
		//   PR 2: OPEN,   local exists but not pushed       -> SevYellow sync note  (problem)
		//   PR 3: OPEN,   no local checkout (remote-only)   -> SevNone "origin only" (NOT a problem)
		allPRs := []vcs.PR{
			{Number: 1, State: "MERGED", HeadRefName: "feat/old", Title: "done"},
			{Number: 2, State: "OPEN", HeadRefName: "feat/pushed"},
			{Number: 3, State: "OPEN", HeadRefName: "feat/remote"},
		}
		gh.AvailableReturns(true)
		gh.PRListReturns(allPRs, nil)
		tk.TicketsReturns(nil, nil)

		// "feat/old" + "feat/pushed" + "wip/no-tk" exist locally; "feat/remote" does not.
		// This makes feat/old linger as a merged-cleanup row, feat/pushed get a SevYellow
		// "local-only (not pushed)" sync note, and feat/remote get a SevNone "origin only"
		// note that does not count as a problem.
		git.LocalBranchExistsCalls(func(_ string, branch string) (bool, error) {
			return branch != "feat/remote", nil
		})
		// No branch has a remote counterpart: feat/pushed → "local-only (not pushed)";
		// feat/old MergedCleanup has only the local-branch flag; wip/no-tk → SyncBad.
		git.RemoteBranchExistsCalls(func(_, _, _ string) (bool, error) {
			return false, nil
		})

		// One WIP branch not in any PR: no-tk match + local-only sync → problem.
		git.LocalBranchesReturns([]vcs.Branch{{Name: "wip/no-tk"}}, nil)
		git.RemoteBranchesReturns([]string{}, nil)

		// Boilerplate required by head().
		git.CurrentBranchReturns("main", nil)
		git.IsDirtyReturns(false, nil)
		git.DivergenceReturns(0, 0, nil)

		d, err := eng.Drilldown(r, false)
		Expect(err).NotTo(HaveOccurred())

		// Independently count flagged rows from the Drilldown return value.
		// This is the invariant: summary count must equal what the user sees on drilldown.
		drilldownFlagged := len(d.MergedCleanup) // PR 1: local branch lingers -> 1
		for _, w := range d.WIP {
			if w.Problem != "" {
				drilldownFlagged++ // wip/no-tk: "no tk · local only — not pushed" -> +1
			}
		}
		for _, pr := range d.OpenPRs {
			// Notes[0] is always the merge-status note; only non-merge notes (Notes[1:])
			// at SevYellow or above count as problems.
			for _, note := range pr.Notes[1:] {
				if note.Sev >= dashboard.SevYellow {
					drilldownFlagged++ // PR 2: "⚠ local-only (not pushed)" -> +1
					break              // count once per PR, not once per note
				}
			}
		}

		// problemCount must agree with the count derived from Drilldown's own output.
		// The stubs are branch-name-based, not call-count-based, so they behave
		// identically when problemCount re-invokes the same row builders.
		problemCount := eng.ProblemCountForTest(r, allPRs, nil)

		Expect(problemCount).To(Equal(drilldownFlagged))
		// Literal guard: if both paths silently regress to 0, this assertion catches it.
		Expect(drilldownFlagged).To(Equal(3))
	})
})
