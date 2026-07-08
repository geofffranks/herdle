package dashboard_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/dashboard"
	"github.com/geofffranks/herdle/internal/vcs"
	"github.com/geofffranks/herdle/internal/vcs/vcsfakes"
)

var _ = Describe("Engine.Summary", func() {
	var (
		git *vcsfakes.FakeGitRunner
		gh  *vcsfakes.FakeGHRunner
		tk  *vcsfakes.FakeTKRunner
		eng dashboard.Engine
	)

	BeforeEach(func() {
		git = &vcsfakes.FakeGitRunner{}
		gh = &vcsfakes.FakeGHRunner{}
		tk = &vcsfakes.FakeTKRunner{}
		eng = dashboard.Engine{Git: git, GH: gh, TK: tk, DirExists: func(string) bool { return true }}
		gh.AvailableReturns(true)
		git.CurrentBranchReturns("main", nil)
		git.IsDirtyReturns(false, nil)
		git.DivergenceReturns(0, 0, nil)
		git.RemoteURLReturns("", errors.New("no remote"))
		tk.HasTicketsReturns(false, nil)
	})

	It("skips projects whose path does not exist", func() {
		eng.DirExists = func(p string) bool { return p == "/exists" }
		cfg := &config.Config{Projects: []config.Project{{Path: "/gone"}, {Path: "/exists"}}}
		res, err := eng.Summary(cfg, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Rows).To(HaveLen(1))
		Expect(res.Rows[0].Name).To(Equal("exists"))
	})

	It("preserves config file order", func() {
		cfg := &config.Config{Projects: []config.Project{{Path: "/a"}, {Path: "/b"}, {Path: "/c"}}}
		res, _ := eng.Summary(cfg, false)
		Expect([]string{res.Rows[0].Name, res.Rows[1].Name, res.Rows[2].Name}).To(Equal([]string{"a", "b", "c"}))
	})

	It("reports detached HEAD, dirty, and behind/ahead in HeadInfo", func() {
		git.CurrentBranchReturns("", nil)
		git.IsDirtyReturns(true, nil)
		git.DivergenceReturns(3, 2, nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.Rows[0].Head).To(Equal(dashboard.HeadInfo{Branch: "", Dirty: true, Behind: 3, Ahead: 2}))
		p, left, right := git.DivergenceArgsForCall(0)
		Expect(p).To(Equal("/r"))
		Expect(left).To(Equal("@{upstream}"))
		Expect(right).To(Equal("HEAD"))
	})

	It("treats an IsDirty error as dirty (wip's `|| dirty=*` quirk)", func() {
		git.IsDirtyReturns(false, errors.New("not a repo"))
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.Rows[0].Head.Dirty).To(BeTrue())
	})

	It("shows no ahead/behind when there is no upstream (Divergence errors)", func() {
		git.DivergenceReturns(0, 0, errors.New("no upstream"))
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.Rows[0].Head.Ahead).To(Equal(0))
		Expect(res.Rows[0].Head.Behind).To(Equal(0))
	})

	Describe("PR cell", func() {
		It("is NoSlug (and skips gh) when no slug resolves", func() {
			cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRNoSlug))
			Expect(gh.PRListCallCount()).To(Equal(0))
		})

		It("is Counted with len(open prs) and queries the all state", func() {
			gh.PRListReturns([]vcs.PR{{Number: 1, State: "OPEN"}, {Number: 2, State: "OPEN"}}, nil)
			git.RemoteURLReturns("git@github.com:o/r.git", nil) // GitHub host -> routes to gh
			cfg := &config.Config{Projects: []config.Project{{Path: "/r", Slug: "o/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].PR).To(Equal(dashboard.PRCell{State: dashboard.PRCounted, Count: 2}))
			slug, state := gh.PRListArgsForCall(0)
			Expect(slug).To(Equal("o/r"))
			Expect(state).To(Equal("all"))
		})

		It("is Unknown when gh fails", func() {
			gh.PRListReturns(nil, errors.New("gh down"))
			git.RemoteURLReturns("git@github.com:o/r.git", nil) // GitHub host -> routes to gh
			cfg := &config.Config{Projects: []config.Project{{Path: "/r", Slug: "o/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRUnknown))
		})

		It("counts a ready PR as attention when its tk is not validated", func() {
			gh.PRListReturns([]vcs.PR{
				{Number: 1, State: "OPEN", Mergeable: "MERGEABLE"}, // would be ready
			}, nil)
			git.RemoteURLReturns("git@github.com:o/r.git", nil)
			tk.HasTicketsReturns(true, nil)
			tk.TicketsReturns([]vcs.Ticket{
				{ID: "a", Status: "open", ExternalRef: "gh-1", Lifecycle: "pending-validation"},
			}, nil)
			cfg := &config.Config{Projects: []config.Project{{Path: "/r", Slug: "o/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].PR).To(Equal(dashboard.PRCell{State: dashboard.PRCounted, Count: 1, Attention: 1}))
		})

		It("still counts a ready PR as ready when its tk is validated", func() {
			gh.PRListReturns([]vcs.PR{{Number: 1, State: "OPEN", Mergeable: "MERGEABLE"}}, nil)
			git.RemoteURLReturns("git@github.com:o/r.git", nil)
			tk.HasTicketsReturns(true, nil)
			tk.TicketsReturns([]vcs.Ticket{
				{ID: "a", Status: "open", ExternalRef: "gh-1", Lifecycle: "validated"},
			}, nil)
			cfg := &config.Config{Projects: []config.Project{{Path: "/r", Slug: "o/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].PR).To(Equal(dashboard.PRCell{State: dashboard.PRCounted, Count: 1, Ready: 1}))
		})

		It("counts ready and needs-attention open PRs (neutral uncounted)", func() {
			gh.PRListReturns([]vcs.PR{
				{Number: 1, State: "OPEN", Mergeable: "MERGEABLE"},                                      // ready
				{Number: 2, State: "OPEN", Mergeable: "CONFLICTING"},                                    // attention
				{Number: 3, State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "CHANGES_REQUESTED"}, // attention
				{Number: 4, State: "OPEN", IsDraft: true},                                               // neutral
			}, nil)
			git.RemoteURLReturns("git@github.com:o/r.git", nil) // GitHub host -> routes to gh
			cfg := &config.Config{Projects: []config.Project{{Path: "/r", Slug: "o/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].PR).To(Equal(dashboard.PRCell{State: dashboard.PRCounted, Count: 4, Attention: 2, Ready: 1}))
		})
	})

	Describe("tk cell", func() {
		It("is absent when there is no .tickets dir", func() {
			tk.HasTicketsReturns(false, nil)
			cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].TK.Present).To(BeFalse())
		})

		It("counts in_progress tickets and ready-and-open tickets", func() {
			tk.HasTicketsReturns(true, nil)
			tk.TicketsReturns([]vcs.Ticket{
				{ID: "a", Status: "in_progress"},
				{ID: "b", Status: "open"},
				{ID: "c", Status: "open"},
				{ID: "d", Status: "in_progress"},
			}, nil)
			tk.ReadyReturns([]string{"b", "d"}, nil)
			cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].TK).To(Equal(dashboard.TKCell{Present: true, InProgress: 2, Ready: 1}))
		})

		It("shows 0/0 when the dir exists but the tk commands fail", func() {
			tk.HasTicketsReturns(true, nil)
			tk.TicketsReturns(nil, errors.New("tk boom"))
			cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].TK).To(Equal(dashboard.TKCell{Present: true}))
		})

		It("treats a HasTickets error as absent", func() {
			tk.HasTicketsReturns(false, errors.New("permission denied"))
			cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].TK.Present).To(BeFalse())
		})
	})

	It("git-fetches each surviving project when fetch is true", func() {
		cfg := &config.Config{Projects: []config.Project{{Path: "/a"}, {Path: "/b"}}}
		_, _ = eng.Summary(cfg, true)
		Expect(git.FetchCallCount()).To(Equal(2))
	})

	It("does not fetch when fetch is false", func() {
		cfg := &config.Config{Projects: []config.Project{{Path: "/a"}}}
		_, _ = eng.Summary(cfg, false)
		Expect(git.FetchCallCount()).To(Equal(0))
	})

	It("does not fetch projects that are skipped for not existing", func() {
		eng.DirExists = func(p string) bool { return p == "/exists" }
		cfg := &config.Config{Projects: []config.Project{{Path: "/gone"}, {Path: "/exists"}}}
		_, _ = eng.Summary(cfg, true)
		Expect(git.FetchCallCount()).To(Equal(1))
		Expect(git.FetchArgsForCall(0)).To(Equal("/exists"))
	})
})

var _ = Describe("Engine.ProblemCount", func() {
	var (
		git      *vcsfakes.FakeGitRunner
		eng      dashboard.Engine
		resolved config.Resolved
	)

	BeforeEach(func() {
		git = &vcsfakes.FakeGitRunner{}
		eng = dashboard.Engine{Git: git}
		resolved = config.Resolved{Path: "/r", Remote: "origin"}
	})

	It("counts cleanup + WIP problems + open-PR non-merge notes, excluding merge attention", func() {
		// 1 merged branch still present locally (cleanup), 1 local-only WIP branch
		// (no tk + not pushed), 1 open PR mergeable but branch unpushed (non-merge sync note).
		git.LocalBranchesReturns([]vcs.Branch{{Name: "wip/thing"}}, nil)
		git.RemoteBranchesReturns([]string{}, nil)
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(false, nil)
		allPRs := []vcs.PR{
			{Number: 1, State: "MERGED", HeadRefName: "feat/old"},
			{Number: 2, State: "OPEN", HeadRefName: "feat/new", Mergeable: "MERGEABLE"},
		}
		n := eng.ProblemCountForTest(resolved, allPRs, nil)
		Expect(n).To(Equal(3))
	})

	It("does not count a conflicting PR whose only problem is merge status", func() {
		// one open PR, Mergeable CONFLICTING, branch in sync, no tk issue
		git.LocalBranchesReturns(nil, nil)
		git.RemoteBranchesReturns(nil, nil)
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(true, nil)
		git.DivergenceReturns(0, 0, nil)
		conflictingInSyncPR := vcs.PR{Number: 3, State: "OPEN", HeadRefName: "feat/conflict", Mergeable: "CONFLICTING"}
		n := eng.ProblemCountForTest(resolved, []vcs.PR{conflictingInSyncPR}, nil)
		Expect(n).To(Equal(0)) // merge attention lives in the merge column, not problems
	})

	It("counts an open PR whose only non-merge note is a tk-validation issue (SevYellow)", func() {
		// Branch is in sync so syncNote is SevGreen and NOT appended; prTKIssue
		// returns bad=true (EffLifecycle blank → unvalidated), producing a SevYellow
		// tk-validation note at Notes[1]. problemCount must count it.
		git.LocalBranchesReturns(nil, nil)
		git.RemoteBranchesReturns(nil, nil)
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(true, nil)
		git.DivergenceReturns(0, 0, nil)
		allPRs := []vcs.PR{
			{Number: 1, State: "OPEN", HeadRefName: "feat/needs-validation", Mergeable: "MERGEABLE"},
		}
		// TicketsForTest sets EffLifecycle=""; correlates to PR #1 via ExternalRef "gh-1"
		// → prTKIssue bad=true ("tk a unvalidated ()") → SevYellow note at Notes[1]
		tickets := eng.TicketsForTest([]vcs.Ticket{
			{ID: "a", Status: "in_progress", ExternalRef: "gh-1"},
		})
		n := eng.ProblemCountForTest(resolved, allPRs, tickets)
		Expect(n).To(Equal(1))
	})

	It("does not count an open PR whose sync note is SevNone (branch not checked out locally)", func() {
		// Branch does NOT exist locally -> syncNote returns SevNone "origin only".
		// openPRRows appends it, but problemCount must NOT count it as a problem.
		git.LocalBranchesReturns(nil, nil)
		git.RemoteBranchesReturns(nil, nil)
		git.LocalBranchExistsReturns(false, nil) // branch not checked out locally
		remotePR := vcs.PR{Number: 4, State: "OPEN", HeadRefName: "feat/remote-only", Mergeable: "MERGEABLE"}
		n := eng.ProblemCountForTest(resolved, []vcs.PR{remotePR}, nil)
		Expect(n).To(Equal(0)) // SevNone "origin only" is informational, not a problem
	})
})

var _ = Describe("Engine.issueCell", func() {
	var (
		gh  *vcsfakes.FakeGHRunner
		eng dashboard.Engine
	)

	BeforeEach(func() {
		gh = &vcsfakes.FakeGHRunner{}
		eng = dashboard.Engine{GH: gh}
	})

	It("builds a tracked issue cell with an untriaged sub-count", func() {
		gh.IssueListReturns([]vcs.Issue{
			{Number: 59, State: "OPEN"}, // triaged by gh-59 ticket
			{Number: 61, State: "OPEN"}, // untriaged
		}, nil)
		// ticketsWithGh59: one ticket tracking issue #59 via ExternalRef "gh-59"
		ticketsWithGh59 := eng.TicketsForTest([]vcs.Ticket{
			{ID: "her-x2b", Status: "open", ExternalRef: "gh-59"},
		})
		cell := eng.IssueCellForTest(gh, "o/r", true, ticketsWithGh59)
		Expect(cell).To(Equal(dashboard.IssueCell{State: dashboard.IssueTracked, Open: 2, Untriaged: 1}))
	})
	It("is untracked and makes no forge call when query is false (fork/no forge)", func() {
		Expect(eng.IssueCellForTest(gh, "o/r", false, nil)).To(Equal(dashboard.IssueCell{State: dashboard.IssueUntracked}))
		Expect(gh.IssueListCallCount()).To(BeZero()) // spec Cost: no IssueList call for a fork/forge-less repo
	})
	It("is unknown when IssueList errors", func() {
		gh.IssueListReturns(nil, errors.New("boom"))
		Expect(eng.IssueCellForTest(gh, "o/r", true, nil)).To(Equal(dashboard.IssueCell{State: dashboard.IssueUnknown}))
	})
})

var _ = Describe("Engine.prCell", func() {
	var eng dashboard.Engine

	BeforeEach(func() {
		eng = dashboard.Engine{}
	})

	It("passes PRNoSlug straight through with empty list", func() {
		cell := eng.PrCellForTest(dashboard.PRNoSlug, nil, nil)
		Expect(cell).To(Equal(dashboard.PRCell{State: dashboard.PRNoSlug}))
	})

	It("passes PRUnknown straight through", func() {
		cell := eng.PrCellForTest(dashboard.PRUnknown, nil, nil)
		Expect(cell).To(Equal(dashboard.PRCell{State: dashboard.PRUnknown}))
	})

	It("counts only OPEN PRs from a mixed all-state list", func() {
		prs := []vcs.PR{
			{Number: 1, State: "OPEN", Mergeable: "MERGEABLE"},   // ready
			{Number: 2, State: "OPEN", Mergeable: "CONFLICTING"}, // attention
			{Number: 3, State: "MERGED", HeadRefName: "old"},     // ignored by prCell
		}
		cell := eng.PrCellForTest(dashboard.PRCounted, prs, nil)
		Expect(cell).To(Equal(dashboard.PRCell{State: dashboard.PRCounted, Count: 2, Attention: 1, Ready: 1}))
	})
})

var _ = Describe("Engine.Summary — graceful degradation", func() {
	var (
		git *vcsfakes.FakeGitRunner
		gh  *vcsfakes.FakeGHRunner
		tk  *vcsfakes.FakeTKRunner
		eng dashboard.Engine
	)
	BeforeEach(func() {
		git = &vcsfakes.FakeGitRunner{}
		gh = &vcsfakes.FakeGHRunner{}
		tk = &vcsfakes.FakeTKRunner{}
		eng = dashboard.Engine{Git: git, GH: gh, TK: tk, DirExists: func(string) bool { return true }}
		git.CurrentBranchReturns("main", nil)
		tk.HasTicketsReturns(false, nil)
	})

	It("when gh is absent: no PR call, PR cell '-' (NoSlug), forge listed absent", func() {
		gh.AvailableReturns(false)
		git.RemoteURLReturns("git@github.com:o/r.git", nil) // GitHub host -> routes to gh
		cfg := &config.Config{Projects: []config.Project{{Path: "/r", Slug: "o/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.AbsentForges).To(ContainElement("gh"))
		Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRNoSlug))
		Expect(gh.PRListCallCount()).To(Equal(0))
	})

	It("a non-GitHub remote yields '-' and no gh call (no false '?')", func() {
		gh.AvailableReturns(true)
		gh.KnownHostsReturns([]string{"github.com"})
		git.RemoteURLReturns("git@gitlab.com:o/r.git", nil)
		git.RemoteHeadReturns("main", nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.AbsentForges).To(BeEmpty())
		Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRNoSlug))
		Expect(gh.PRListCallCount()).To(Equal(0))
	})

	It("does not flag the forge absent when gh is absent but no project is a GitHub remote", func() {
		gh.AvailableReturns(false)
		gh.KnownHostsReturns([]string{"github.com"})
		git.RemoteURLReturns("git@gitlab.com:o/r.git", nil)
		git.RemoteHeadReturns("main", nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.AbsentForges).To(BeEmpty()) // no GitHub project -> no spurious "gh not found" note
		Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRNoSlug))
	})

	It("a GitHub Enterprise remote queries gh with a host-prefixed slug", func() {
		gh.AvailableReturns(true)
		gh.KnownHostsReturns([]string{"github.example.com"})
		gh.PRListReturns([]vcs.PR{{Number: 1, State: "OPEN"}}, nil)
		git.RemoteURLReturns("git@github.example.com:o/r.git", nil)
		git.RemoteHeadReturns("main", nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.Rows[0].PR).To(Equal(dashboard.PRCell{State: dashboard.PRCounted, Count: 1}))
		slug, _ := gh.PRListArgsForCall(0)
		Expect(slug).To(Equal("github.example.com/o/r"))
	})

	It("when gh CLI is absent: PR cell '-' (NoSlug) and Problems is 0 even with a local WIP branch", func() {
		// Forge repo whose CLI is unavailable: isForge=true but avail=false, so
		// allPRs is nil and prState remains PRNoSlug. The nil PR list would cause
		// wipRows to misclassify the local branch as orphaned WIP (no tk, not in any
		// seen PR) and inflate Problems. Fix 1's !isForge||PRCounted guard must catch
		// this and degrade Problems to 0.
		gh.AvailableReturns(false)
		git.RemoteURLReturns("git@github.com:o/r.git", nil) // GitHub host -> routes to gh
		git.LocalBranchesReturns([]vcs.Branch{{Name: "wip/thing"}}, nil)
		git.RemoteBranchesReturns([]string{}, nil)
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(false, nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r", Slug: "o/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRNoSlug))
		Expect(res.Rows[0].Problems).To(Equal(0))
	})

	It("when gh fails: PR cell is Unknown and Problems is 0, even with a local WIP branch", func() {
		// Arrange: a forge repo whose PRList returns an error. A local WIP branch
		// exists that would otherwise surface as a problem, but PR data is unreliable
		// so Problems must be zeroed rather than inflated.
		gh.AvailableReturns(true)
		gh.PRListReturns(nil, errors.New("gh flap"))
		git.RemoteURLReturns("git@github.com:o/r.git", nil) // GitHub host -> routes to gh
		git.LocalBranchesReturns([]vcs.Branch{{Name: "wip/thing"}}, nil)
		git.RemoteBranchesReturns([]string{}, nil)
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(false, nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r", Slug: "o/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRUnknown))
		Expect(res.Rows[0].Problems).To(Equal(0))
	})

	It("a no-forge repo (PRNoSlug) still counts local WIP problems", func() {
		// PRNoSlug is a repo without a configured forge; its local problems are
		// real and must not be suppressed by the PRUnknown guard.
		git.RemoteURLReturns("", errors.New("no remote")) // no forge slug resolves
		git.LocalBranchesReturns([]vcs.Branch{{Name: "wip/thing"}}, nil)
		git.RemoteBranchesReturns([]string{}, nil)
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(false, nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRNoSlug))
		Expect(res.Rows[0].Problems).To(BeNumerically(">", 0))
	})
})
