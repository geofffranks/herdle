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

var _ = Describe("Engine.ticketTable / effective lifecycle", func() {
	var (
		tk  *vcsfakes.FakeTKRunner
		eng dashboard.Engine
	)

	BeforeEach(func() {
		tk = &vcsfakes.FakeTKRunner{}
		eng = dashboard.Engine{TK: tk, Glob: func(string) ([]string, error) { return nil, nil }}
	})

	It("drops closed tickets and keeps the rest", func() {
		tk.TicketsReturns([]vcs.Ticket{
			{ID: "a", Status: "open"},
			{ID: "b", Status: "closed"},
			{ID: "c", Status: "in_progress"},
		}, nil)
		Expect(eng.TicketTableForTest("/r")).To(HaveLen(2))
	})

	It("uses a set lifecycle verbatim", func() {
		tk.TicketsReturns([]vcs.Ticket{{ID: "a", Status: "open", Lifecycle: "validated"}}, nil)
		Expect(eng.TicketTableForTest("/r")[0].EffLifecycle).To(Equal("validated"))
	})

	It("derives planned from a plans file, designed from a specs file", func() {
		tk.TicketsReturns([]vcs.Ticket{
			{ID: "pp", Status: "open", Lifecycle: "-"},
			{ID: "ss", Status: "open", Lifecycle: ""},
		}, nil)
		eng.Glob = func(pattern string) ([]string, error) {
			switch {
			case contains(pattern, "plans") && contains(pattern, "pp"):
				return []string{"x"}, nil
			case contains(pattern, "specs") && contains(pattern, "ss"):
				return []string{"y"}, nil
			}
			return nil, nil
		}
		tbl := eng.TicketTableForTest("/r")
		Expect(tbl[0].EffLifecycle).To(Equal("planned"))
		Expect(tbl[1].EffLifecycle).To(Equal("designed"))
	})

	It("derives planned/designed from the feature-dir layout too", func() {
		tk.TicketsReturns([]vcs.Ticket{
			{ID: "fp", Status: "open", Lifecycle: ""},
			{ID: "fd", Status: "open", Lifecycle: ""},
		}, nil)
		// Feature-dir layout: docs/superpowers/<tkid>-<slug>/{design_spec,plan}.md.
		// Only the feature-dir globs match here; the old {specs,plans}/*id* globs return nil.
		eng.Glob = func(pattern string) ([]string, error) {
			switch {
			case contains(pattern, "*fp*") && contains(pattern, "plan.md"):
				return []string{"/r/docs/superpowers/fp-x/plan.md"}, nil
			case contains(pattern, "*fd*") && contains(pattern, "design_spec.md"):
				return []string{"/r/docs/superpowers/fd-y/design_spec.md"}, nil
			}
			return nil, nil
		}
		tbl := eng.TicketTableForTest("/r")
		Expect(tbl[0].EffLifecycle).To(Equal("planned"))
		Expect(tbl[1].EffLifecycle).To(Equal("designed"))
	})

	It("falls back to '-' for explicit '-' and '?' for an absent field", func() {
		tk.TicketsReturns([]vcs.Ticket{
			{ID: "dash", Status: "open", Lifecycle: "-"},
			{ID: "absent", Status: "open", Lifecycle: ""},
		}, nil)
		tbl := eng.TicketTableForTest("/r")
		Expect(tbl[0].EffLifecycle).To(Equal("-"))
		Expect(tbl[1].EffLifecycle).To(Equal("?"))
	})
})

var _ = Describe("Engine PR sections", func() {
	var (
		git *vcsfakes.FakeGitRunner
		eng dashboard.Engine
	)
	BeforeEach(func() {
		git = &vcsfakes.FakeGitRunner{}
		eng = dashboard.Engine{Git: git}
		git.LocalBranchExistsReturns(false, nil)
		git.RemoteBranchExistsReturns(false, nil)
	})

	It("builds open-PR rows only for OPEN PRs, with correlated tks + note", func() {
		prs := []vcs.PR{
			{Number: 5, State: "OPEN", HeadRefName: "feat", Title: "a feature"},
			{Number: 6, State: "MERGED", HeadRefName: "old", Title: "done"},
		}
		tickets := eng.TicketsForTest([]vcs.Ticket{{ID: "t1", ExternalRef: "gh-5"}})
		tickets[0] = eng.WithLifecycleForTest(tickets[0], "validated") // validated -> no tk flag, asserts the baseline note set
		rows := eng.OpenPRRowsForTest(prs, tickets, "/r", "origin")
		Expect(rows).To(HaveLen(1))
		Expect(rows[0].Number).To(Equal(5))
		Expect(rows[0].TKs).To(Equal([]string{"t1"}))
		Expect(rows[0].Notes).To(Equal([]dashboard.FlagNote{
			{Text: "—", Sev: dashboard.SevNone},           // neutral: no merge fields set
			{Text: "origin only", Sev: dashboard.SevNone}, // sync note (not green) is appended
		}))
	})

	It("leads the notes with merge status and appends only non-green sync notes", func() {
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(true, nil)
		git.DivergenceReturns(0, 0, nil) // in sync -> green -> dropped
		prs := []vcs.PR{{Number: 7, State: "OPEN", HeadRefName: "feat", Title: "x", Mergeable: "MERGEABLE"}}
		rows := eng.OpenPRRowsForTest(prs, nil, "/r", "origin")
		Expect(rows).To(HaveLen(1))
		Expect(rows[0].Notes).To(Equal([]dashboard.FlagNote{
			{Text: "✓ ready to merge", Sev: dashboard.SevGreen}, // only the merge note; "✓ in sync" dropped
		}))
	})

	It("flags merged PRs needing cleanup (local/origin branch, open tk)", func() {
		git.LocalBranchExistsReturns(true, nil)  // local branch lingers
		git.RemoteBranchExistsReturns(true, nil) // origin branch lingers
		prs := []vcs.PR{{Number: 6, State: "MERGED", HeadRefName: "old", Title: "done"}}
		tickets := eng.TicketsForTest([]vcs.Ticket{{ID: "t6", ExternalRef: "gh-6"}})
		rows := eng.MergedCleanupRowsForTest(prs, tickets, config.Resolved{Path: "/r", Remote: "origin"})
		Expect(rows).To(HaveLen(1))
		Expect(rows[0].Flags).To(Equal(dashboard.FlagNote{
			Text: "⚠ local branch · ⚠ origin branch · ⚠ tk t6 open", Sev: dashboard.SevYellow,
		}))
	})

	It("drops merged PRs with no leftovers", func() {
		prs := []vcs.PR{{Number: 6, State: "MERGED", HeadRefName: "old"}}
		Expect(eng.MergedCleanupRowsForTest(prs, nil, config.Resolved{Path: "/r", Remote: "origin"})).To(BeEmpty())
	})

	It("drops merged PRs whose head is a trunk/base/integration branch", func() {
		// Fork PRs opened from main->main (or base/integration) carry a trunk
		// HeadRefName. That branch is never cleaned up and its tk may belong to
		// ongoing work (a still-open PR off the same branch), so it is not cleanup.
		git.LocalBranchExistsReturns(true, nil)  // trunk always exists locally
		git.RemoteBranchExistsReturns(true, nil) // and on the remote
		tickets := eng.TicketsForTest([]vcs.Ticket{{ID: "t6", ExternalRef: "gh-6"}})
		for _, head := range []string{"main", "master", "dev", "geoff-main", "origin"} {
			prs := []vcs.PR{{Number: 6, State: "MERGED", HeadRefName: head, Title: "done"}}
			r := config.Resolved{Path: "/r", Remote: "origin", Base: "dev", Integration: "geoff-main"}
			Expect(eng.MergedCleanupRowsForTest(prs, tickets, r)).To(BeEmpty(), "head %q should be excluded", head)
		}
	})

	It("threads the configured remote into the open-PR sync check", func() {
		git.LocalBranchExistsReturns(true, nil) // past the "<remote> only" early return
		prs := []vcs.PR{{Number: 5, State: "OPEN", HeadRefName: "feat", Title: "x"}}
		_ = eng.OpenPRRowsForTest(prs, nil, "/r", "fork")
		_, remote, _ := git.RemoteBranchExistsArgsForCall(0)
		Expect(remote).To(Equal("fork"))
	})

	It("skips the remote-branch flag when no remote is configured", func() {
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(true, nil)
		prs := []vcs.PR{{Number: 6, State: "MERGED", HeadRefName: "old", Title: "done"}}
		rows := eng.MergedCleanupRowsForTest(prs, nil, config.Resolved{Path: "/r"})
		Expect(rows).To(HaveLen(1))
		Expect(rows[0].Flags.Text).To(Equal("⚠ local branch")) // no remote-branch flag
		Expect(git.RemoteBranchExistsCallCount()).To(Equal(0))
	})

	It("names the configured remote in the merged-cleanup flag", func() {
		git.LocalBranchExistsReturns(false, nil)
		git.RemoteBranchExistsReturns(true, nil)
		prs := []vcs.PR{{Number: 6, State: "MERGED", HeadRefName: "old", Title: "done"}}
		rows := eng.MergedCleanupRowsForTest(prs, nil, config.Resolved{Path: "/r", Remote: "fork"})
		Expect(rows).To(HaveLen(1))
		Expect(rows[0].Flags.Text).To(Equal("⚠ fork branch"))
	})

	It("appends a yellow tk flag when a correlated ticket is not validated", func() {
		prs := []vcs.PR{{Number: 5, State: "OPEN", HeadRefName: "feat", Title: "x", Mergeable: "MERGEABLE"}}
		tickets := eng.TicketsForTest([]vcs.Ticket{{ID: "t1", ExternalRef: "gh-5"}})
		tickets[0] = eng.WithLifecycleForTest(tickets[0], "pending-validation")
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(true, nil)
		git.DivergenceReturns(0, 0, nil) // in sync -> green sync note dropped
		rows := eng.OpenPRRowsForTest(prs, tickets, "/r", "origin")
		Expect(rows[0].Notes).To(Equal([]dashboard.FlagNote{
			{Text: "✓ ready to merge", Sev: dashboard.SevGreen},
			{Text: "⚠ tk t1 pending-validation", Sev: dashboard.SevYellow},
		}))
	})

	It("adds no tk flag when the correlated ticket is validated", func() {
		prs := []vcs.PR{{Number: 5, State: "OPEN", HeadRefName: "feat", Title: "x", Mergeable: "MERGEABLE"}}
		tickets := eng.TicketsForTest([]vcs.Ticket{{ID: "t1", ExternalRef: "gh-5"}})
		tickets[0] = eng.WithLifecycleForTest(tickets[0], "validated")
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(true, nil)
		git.DivergenceReturns(0, 0, nil)
		rows := eng.OpenPRRowsForTest(prs, tickets, "/r", "origin")
		Expect(rows[0].Notes).To(Equal([]dashboard.FlagNote{
			{Text: "✓ ready to merge", Sev: dashboard.SevGreen},
		}))
	})
})

var _ = Describe("Engine WIP section", func() {
	var (
		git *vcsfakes.FakeGitRunner
		eng dashboard.Engine
		r   config.Resolved
	)
	BeforeEach(func() {
		git = &vcsfakes.FakeGitRunner{}
		eng = dashboard.Engine{Git: git}
		r = config.Resolved{Path: "/r", Base: "dev", Integration: "geoff-main", Remote: "origin"}
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(true, nil)
		git.DivergenceReturns(0, 0, nil)
	})

	It("excludes base/integration/main/master/HEAD/origin and backup/*, keeps real work", func() {
		git.LocalBranchesReturns([]vcs.Branch{
			{Name: "dev"}, {Name: "main"}, {Name: "geoff-main"}, {Name: "HEAD"},
			{Name: "backup/old"}, {Name: "feature-x"},
		}, nil)
		git.RemoteBranchesReturns([]string{"feature-y"}, nil)
		rows := eng.WIPRowsForTest(r, nil, nil)
		var names []string
		for _, w := range rows {
			names = append(names, w.Branch)
		}
		Expect(names).To(Equal([]string{"feature-x", "feature-y"})) // sorted, exclusions gone
	})

	It("skips branches already in a PR and upstream-gone branches", func() {
		git.LocalBranchesReturns([]vcs.Branch{{Name: "in-pr"}, {Name: "dead", UpstreamGone: true}, {Name: "live"}}, nil)
		prs := []vcs.PR{{Number: 1, State: "OPEN", HeadRefName: "in-pr"}}
		rows := eng.WIPRowsForTest(r, prs, nil)
		Expect(rows).To(HaveLen(1))
		Expect(rows[0].Branch).To(Equal("live"))
	})

	It("correlates a tk and marks 'no tk' otherwise", func() {
		git.LocalBranchesReturns([]vcs.Branch{{Name: "fix/12-x"}, {Name: "orphan"}}, nil)
		tickets := eng.TicketsForTest([]vcs.Ticket{{ID: "t12", ExternalRef: "gh-12", Title: "the fix"}})
		tickets[0] = eng.WithLifecycleForTest(tickets[0], "in-development")
		rows := eng.WIPRowsForTest(r, nil, tickets)
		byBranch := map[string]dashboardWIP{}
		for _, w := range rows {
			byBranch[w.Branch] = dashboardWIP{w.TKID, w.Lifecycle, w.Problem}
		}
		Expect(byBranch["fix/12-x"]).To(Equal(dashboardWIP{tkid: "t12", lc: "in-development", problem: ""}))
		Expect(byBranch["orphan"]).To(Equal(dashboardWIP{tkid: "", lc: "-", problem: "no tk"}))
	})

	It("appends standalone in-flight tks not matched and not in a PR", func() {
		git.LocalBranchesReturns(nil, nil)
		git.RemoteBranchesReturns(nil, nil)
		tickets := eng.TicketsForTest([]vcs.Ticket{
			{ID: "solo", Status: "in_progress"},
			{ID: "open", Status: "open"},
		})
		rows := eng.WIPRowsForTest(r, nil, tickets)
		Expect(rows).To(HaveLen(1))
		Expect(rows[0]).To(Equal(dashboard.WIPRow{
			Lifecycle: "", Sync: dashboard.SyncNA, TKID: "solo", Branch: "(no branch)",
			Title: "", Problem: "no external-ref / branch", ProblemSev: dashboard.SevRed,
		}))
	})

	It("flags a standalone in-flight tk whose explicit branch is missing", func() {
		git.LocalBranchesReturns(nil, nil)
		git.RemoteBranchesReturns(nil, nil)
		tickets := eng.TicketsForTest([]vcs.Ticket{
			{ID: "solo", Status: "in_progress", Branch: "feat/x"},
		})
		rows := eng.WIPRowsForTest(r, nil, tickets)
		Expect(rows).To(HaveLen(1))
		Expect(rows[0]).To(Equal(dashboard.WIPRow{
			Lifecycle: "", Sync: dashboard.SyncNA, TKID: "solo", Branch: "(no branch)",
			Title: "", Problem: "branch feat/x missing", ProblemSev: dashboard.SevRed,
		}))
	})
})

var _ = Describe("Engine.Drilldown", func() {
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
		eng = dashboard.Engine{Git: git, GH: gh, TK: tk, Glob: func(string) ([]string, error) { return nil, nil }}
		git.CurrentBranchReturns("main", nil)
		gh.AvailableReturns(true)
	})

	It("prunes the configured remote when not fetching and sets Fetched=false", func() {
		d, err := eng.Drilldown(config.Resolved{Name: "r", Path: "/r", Remote: "origin"}, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(d.Fetched).To(BeFalse())
		Expect(git.PruneRemoteCallCount()).To(Equal(1))
		p, remote := git.PruneRemoteArgsForCall(0)
		Expect(p).To(Equal("/r"))
		Expect(remote).To(Equal("origin"))
		Expect(git.FetchCallCount()).To(Equal(0))
	})

	It("skips prune when there is no configured remote", func() {
		_, err := eng.Drilldown(config.Resolved{Path: "/r"}, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(git.PruneRemoteCallCount()).To(Equal(0))
	})

	It("fetches when asked and sets Fetched=true", func() {
		d, _ := eng.Drilldown(config.Resolved{Path: "/r"}, true)
		Expect(d.Fetched).To(BeTrue())
		Expect(git.FetchCallCount()).To(Equal(1))
		Expect(git.PruneRemoteCallCount()).To(Equal(0))
	})

	It("marks ForgeUnavailable when PRList errors (and skips gh without a slug)", func() {
		gh.PRListReturns(nil, errors.New("gh down"))
		d, _ := eng.Drilldown(config.Resolved{Path: "/r", Slug: "o/r", SlugExplicit: true, RemoteHost: "github.com"}, false)
		Expect(d.HasSlug).To(BeTrue())
		Expect(d.ForgeUnavailable).To(BeTrue())
		Expect(d.ForgeAbsent).To(BeFalse())

		gh2 := &vcsfakes.FakeGHRunner{}
		gh2.AvailableReturns(true)
		eng.GH = gh2
		d2, _ := eng.Drilldown(config.Resolved{Path: "/r"}, false)
		Expect(d2.HasSlug).To(BeFalse())
		Expect(gh2.PRListCallCount()).To(Equal(0))
	})

	It("populates Name/Path/Head", func() {
		d, _ := eng.Drilldown(config.Resolved{Name: "herdle", Path: "/r"}, false)
		Expect(d.Name).To(Equal("herdle"))
		Expect(d.Path).To(Equal("/r"))
		Expect(d.Head.Branch).To(Equal("main"))
	})

	It("sets ForgeAbsent and skips gh entirely when gh is unavailable", func() {
		gh.AvailableReturns(false)
		d, _ := eng.Drilldown(config.Resolved{Path: "/r", Slug: "o/r", SlugExplicit: true, RemoteHost: "github.com"}, false)
		Expect(d.ForgeAbsent).To(BeTrue())
		Expect(d.ForgeUnavailable).To(BeFalse())
		Expect(gh.PRListCallCount()).To(Equal(0))
	})

	It("does not flag ForgeAbsent for a non-GitHub repo when gh is unavailable", func() {
		gh.AvailableReturns(false)
		gh.KnownHostsReturns([]string{"github.com"})
		d, _ := eng.Drilldown(config.Resolved{Path: "/r", Slug: "o/r", RemoteHost: "gitlab.com"}, false)
		Expect(d.HasSlug).To(BeFalse())
		Expect(d.ForgeAbsent).To(BeFalse()) // non-GitHub repo -> no spurious "gh not found" note
		Expect(gh.PRListCallCount()).To(Equal(0))
	})

	It("queries gh with a host-prefixed slug for a GitHub Enterprise remote", func() {
		gh.KnownHostsReturns([]string{"github.example.com"})
		gh.PRListReturns(nil, nil)
		_, _ = eng.Drilldown(config.Resolved{Path: "/r", Slug: "o/r", RemoteHost: "github.example.com"}, false)
		Expect(gh.PRListCallCount()).To(Equal(1))
		slug, state := gh.PRListArgsForCall(0)
		Expect(slug).To(Equal("github.example.com/o/r"))
		Expect(state).To(Equal("all"))
	})

	Context("open issue gather", func() {
		var (
			resolvedTrackIssues config.Resolved
			resolvedForked      config.Resolved
		)
		BeforeEach(func() {
			resolvedTrackIssues = config.Resolved{
				Path:         "/r",
				Slug:         "o/r",
				SlugExplicit: true,
				RemoteHost:   "github.com",
				TrackIssues:  true,
			}
			resolvedForked = config.Resolved{
				Path:         "/r",
				Slug:         "o/r",
				SlugExplicit: true,
				RemoteHost:   "github.com",
				TrackIssues:  false,
			}
			// ticket her-x2b tracks issue #59 via ExternalRef "gh-59"
			tk.TicketsReturns([]vcs.Ticket{
				{ID: "her-x2b", Status: "open", ExternalRef: "gh-59"},
			}, nil)
		})

		It("lists open issues with untriaged first and triaged tk correlation", func() {
			gh.IssueListReturns([]vcs.Issue{
				{Number: 59, Title: "Tracked", State: "OPEN"},
				{Number: 61, Title: "Untriaged", State: "OPEN"},
			}, nil)
			d, _ := eng.Drilldown(resolvedTrackIssues, false)
			Expect(d.TrackIssues).To(BeTrue())
			Expect(d.OpenIssues).To(Equal([]dashboard.IssueRow{
				{Number: 61, Title: "Untriaged", Untriaged: true},
				{Number: 59, Title: "Tracked", TKs: []string{"her-x2b"}},
			}))
		})

		It("gathers no issues for a fork (TrackIssues false)", func() {
			d, _ := eng.Drilldown(resolvedForked, false)
			Expect(d.TrackIssues).To(BeFalse())
			Expect(d.OpenIssues).To(BeEmpty())
			Expect(gh.IssueListCallCount()).To(BeZero())
		})
	})
})

type dashboardWIP struct{ tkid, lc, problem string }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

var _ = Describe("Engine up-next + artifacts", func() {
	var eng dashboard.Engine
	BeforeEach(func() { eng = dashboard.Engine{} })

	It("orders open tickets by readiness then priority", func() {
		// Build annotated tickets via the existing shims (Tasks 5 + 6).
		ts := eng.TicketsForTest([]vcs.Ticket{
			{ID: "a", Status: "open", Priority: 1},
			{ID: "b", Status: "open", Priority: 3},
			{ID: "c", Status: "open", Priority: 2},
			{ID: "d", Status: "in_progress", Priority: 1}, // not open -> excluded
		})
		ts[0] = eng.WithLifecycleForTest(ts[0], "designed")
		ts[1] = eng.WithLifecycleForTest(ts[1], "planned")
		ts[2] = eng.WithLifecycleForTest(ts[2], "planned")
		rows := eng.UpNextRowsForTest(ts)
		var ids []string
		for _, r := range rows {
			ids = append(ids, r.TKID)
		}
		Expect(ids).To(Equal([]string{"c", "b", "a"})) // planned(2) < planned(3) < designed
	})

	It("ranks unset and unknown lifecycles after planned/designed", func() {
		// Equal priority, so ordering is purely readinessRank:
		// planned(0) < designed(1) < "-"(2) < unknown/default(3).
		ts := eng.TicketsForTest([]vcs.Ticket{
			{ID: "unknown", Status: "open", Priority: 1},
			{ID: "unset", Status: "open", Priority: 1},
			{ID: "designed", Status: "open", Priority: 1},
			{ID: "planned", Status: "open", Priority: 1},
		})
		ts[0] = eng.WithLifecycleForTest(ts[0], "?") // hits the default arm
		ts[1] = eng.WithLifecycleForTest(ts[1], "-")
		ts[2] = eng.WithLifecycleForTest(ts[2], "designed")
		ts[3] = eng.WithLifecycleForTest(ts[3], "planned")
		rows := eng.UpNextRowsForTest(ts)
		var ids []string
		for _, r := range rows {
			ids = append(ids, r.TKID)
		}
		Expect(ids).To(Equal([]string{"planned", "designed", "unset", "unknown"}))
	})

	It("tags artifacts with a real tk id and leaves slug-only files untagged", func() {
		eng.Glob = func(pattern string) ([]string, error) {
			switch {
			case contains(pattern, ".tickets"):
				return []string{"/r/.tickets/her-ju9h.md", "/r/.tickets/dr-o833.md"}, nil
			case contains(pattern, "specs"):
				return []string{
					"/r/docs/superpowers/specs/2026-06-13-her-ju9h-x-design.md",
					"/r/docs/superpowers/specs/2026-05-28-movable-ships-design.md", // no real id
				}, nil
			case contains(pattern, "plans"):
				return []string{"/r/docs/superpowers/plans/2026-06-13-dr-o833-y.md"}, nil
			}
			return nil, nil
		}
		rows := eng.ArtifactRowsForTest("/r")
		Expect(rows).To(ContainElement(dashboard.ArtifactRow{TKID: "her-ju9h", Kind: "specs", Filename: "2026-06-13-her-ju9h-x-design.md"}))
		Expect(rows).To(ContainElement(dashboard.ArtifactRow{TKID: "dr-o833", Kind: "plans", Filename: "2026-06-13-dr-o833-y.md"}))
		// A pre-convention filename that merely looks id-shaped must NOT be tagged.
		Expect(rows).To(ContainElement(dashboard.ArtifactRow{TKID: "", Kind: "specs", Filename: "2026-05-28-movable-ships-design.md"}))
	})

	It("lists feature-dir artifacts alongside legacy subdirs without double-listing", func() {
		eng.Glob = func(pattern string) ([]string, error) {
			switch {
			case contains(pattern, ".tickets"):
				return []string{"/r/.tickets/her-ju9h.md"}, nil
			case contains(pattern, "docs/superpowers/specs"):
				return []string{"/r/docs/superpowers/specs/2026-06-13-her-ju9h-x-design.md"}, nil
			case contains(pattern, "docs/superpowers/*/*.md"):
				// real filepath.Glob also surfaces the legacy specs/ file here;
				// the feature scan must skip the fixed subdirs to avoid a dup.
				return []string{
					"/r/docs/superpowers/specs/2026-06-13-her-ju9h-x-design.md",
					"/r/docs/superpowers/her-ju9h-x/design_spec.md",
					"/r/docs/superpowers/her-ju9h-x/plan.md",
					"/r/docs/superpowers/her-ju9h-x/validation.md",
				}, nil
			}
			return nil, nil
		}
		rows := eng.ArtifactRowsForTest("/r")
		// legacy specs file listed exactly once (by the kind loop, not the feature scan)
		Expect(rows).To(ContainElement(dashboard.ArtifactRow{TKID: "her-ju9h", Kind: "specs", Filename: "2026-06-13-her-ju9h-x-design.md"}))
		count := 0
		for _, r := range rows {
			if r.Filename == "2026-06-13-her-ju9h-x-design.md" {
				count++
			}
		}
		Expect(count).To(Equal(1))
		// feature-dir files: tkid from the dir name, kind derived from the
		// filename; the dir prefixes the filename so each feature's rows are
		// distinguishable (otherwise every feature shows bare design_spec.md etc.)
		Expect(rows).To(ContainElement(dashboard.ArtifactRow{TKID: "her-ju9h", Kind: "specs", Filename: "her-ju9h-x/design_spec.md"}))
		Expect(rows).To(ContainElement(dashboard.ArtifactRow{TKID: "her-ju9h", Kind: "plans", Filename: "her-ju9h-x/plan.md"}))
		Expect(rows).To(ContainElement(dashboard.ArtifactRow{TKID: "her-ju9h", Kind: "validation", Filename: "her-ju9h-x/validation.md"}))
	})
})

var _ = Describe("Engine sync helpers", func() {
	var (
		git *vcsfakes.FakeGitRunner
		eng dashboard.Engine
	)
	BeforeEach(func() {
		git = &vcsfakes.FakeGitRunner{}
		eng = dashboard.Engine{Git: git}
	})

	Describe("syncNote (open PR head)", func() {
		It("is 'origin only' when there is no local branch", func() {
			git.LocalBranchExistsReturns(false, nil)
			Expect(eng.SyncNoteForTest("/r", "origin", "feat")).To(Equal(dashboard.FlagNote{Text: "origin only", Sev: dashboard.SevNone}))
		})
		It("warns local-only when pushed nowhere", func() {
			git.LocalBranchExistsReturns(true, nil)
			git.RemoteBranchExistsReturns(false, nil)
			Expect(eng.SyncNoteForTest("/r", "origin", "feat")).To(Equal(dashboard.FlagNote{Text: "⚠ local-only (not pushed)", Sev: dashboard.SevYellow}))
		})
		It("is in-sync when both present and not diverged", func() {
			git.LocalBranchExistsReturns(true, nil)
			git.RemoteBranchExistsReturns(true, nil)
			git.DivergenceReturns(0, 0, nil)
			Expect(eng.SyncNoteForTest("/r", "origin", "feat")).To(Equal(dashboard.FlagNote{Text: "✓ in sync", Sev: dashboard.SevGreen}))
		})
		It("warns with the divergence text when diverged", func() {
			git.LocalBranchExistsReturns(true, nil)
			git.RemoteBranchExistsReturns(true, nil)
			git.DivergenceReturns(2, 3, nil) // behind=2, ahead=3
			Expect(eng.SyncNoteForTest("/r", "origin", "feat")).To(Equal(dashboard.FlagNote{Text: "⚠ diverged ↑3↓2", Sev: dashboard.SevYellow}))
		})
		It("reports no remote configured when remote is empty", func() {
			git.LocalBranchExistsReturns(true, nil)
			Expect(eng.SyncNoteForTest("/r", "", "feat")).To(Equal(dashboard.FlagNote{Text: "⚠ no remote configured", Sev: dashboard.SevYellow}))
		})
	})

	Describe("wipSync (WIP branch)", func() {
		It("is OK when both present and equal", func() {
			git.LocalBranchExistsReturns(true, nil)
			git.RemoteBranchExistsReturns(true, nil)
			git.DivergenceReturns(0, 0, nil)
			s, reason := eng.WipSyncForTest("/r", "origin", "b")
			Expect(s).To(Equal(dashboard.SyncOK))
			Expect(reason).To(BeEmpty())
		})
		It("is Bad/local-only when not pushed", func() {
			git.LocalBranchExistsReturns(true, nil)
			git.RemoteBranchExistsReturns(false, nil)
			s, reason := eng.WipSyncForTest("/r", "origin", "b")
			Expect(s).To(Equal(dashboard.SyncBad))
			Expect(reason).To(Equal("local only — not pushed"))
		})
		It("is NA when neither side has the branch", func() {
			git.LocalBranchExistsReturns(false, nil)
			git.RemoteBranchExistsReturns(false, nil)
			s, _ := eng.WipSyncForTest("/r", "origin", "b")
			Expect(s).To(Equal(dashboard.SyncNA))
		})
		It("is Bad/remote-only when the branch exists on the remote but not locally", func() {
			git.LocalBranchExistsReturns(false, nil)
			git.RemoteBranchExistsReturns(true, nil)
			s, reason := eng.WipSyncForTest("/r", "origin", "b")
			Expect(s).To(Equal(dashboard.SyncBad))
			Expect(reason).To(Equal("remote only — no local branch"))
		})
		It("uses the configured remote for the remote check", func() {
			_, _ = eng.WipSyncForTest("/r", "fork", "b")
			_, remote, _ := git.RemoteBranchExistsArgsForCall(0)
			Expect(remote).To(Equal("fork"))
		})
		It("is NA immediately when remote is empty (no git calls)", func() {
			s, reason := eng.WipSyncForTest("/r", "", "b")
			Expect(s).To(Equal(dashboard.SyncNA))
			Expect(reason).To(BeEmpty())
			Expect(git.LocalBranchExistsCallCount()).To(Equal(0)) // proves the early return
		})
	})
})

// Added by the high-effort review pass: cover the divFlag ahead/behind-only arms
// and the WIP branch-row issue-severity (SevRed when out of sync, SevYellow when
// merely untracked) — both were exercised only indirectly before.
var _ = Describe("Engine drilldown — review coverage", func() {
	var (
		git *vcsfakes.FakeGitRunner
		eng dashboard.Engine
		r   config.Resolved
	)
	BeforeEach(func() {
		git = &vcsfakes.FakeGitRunner{}
		eng = dashboard.Engine{Git: git}
		r = config.Resolved{Path: "/r", Remote: "origin"}
		git.LocalBranchExistsReturns(true, nil)
		git.RemoteBranchExistsReturns(true, nil)
	})

	Describe("divFlag arms (via syncNote)", func() {
		It("reports ahead-only as unpushed", func() {
			git.DivergenceReturns(0, 2, nil) // behind=0, ahead=2
			Expect(eng.SyncNoteForTest("/r", "origin", "b")).To(Equal(dashboard.FlagNote{Text: "⚠ ↑2 unpushed", Sev: dashboard.SevYellow}))
		})
		It("reports behind-only as behind", func() {
			git.DivergenceReturns(3, 0, nil) // behind=3, ahead=0
			Expect(eng.SyncNoteForTest("/r", "origin", "b")).To(Equal(dashboard.FlagNote{Text: "⚠ ↓3 behind", Sev: dashboard.SevYellow}))
		})
	})

	Describe("WIP branch-row issue severity", func() {
		It("colors a no-tk in-sync branch yellow", func() {
			git.LocalBranchesReturns([]vcs.Branch{{Name: "loner"}}, nil)
			git.DivergenceReturns(0, 0, nil)
			rows := eng.WIPRowsForTest(r, nil, nil)
			Expect(rows).To(HaveLen(1))
			Expect(rows[0].Problem).To(Equal("no tk"))
			Expect(rows[0].ProblemSev).To(Equal(dashboard.SevYellow))
		})
		It("colors a no-tk out-of-sync branch red", func() {
			git.LocalBranchesReturns([]vcs.Branch{{Name: "loner"}}, nil)
			git.RemoteBranchExistsReturns(false, nil) // local only -> SyncBad
			rows := eng.WIPRowsForTest(r, nil, nil)
			Expect(rows).To(HaveLen(1))
			Expect(rows[0].Problem).To(Equal("no tk · local only — not pushed"))
			Expect(rows[0].ProblemSev).To(Equal(dashboard.SevRed))
		})
	})
})
