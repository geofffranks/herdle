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

		It("is Counted with len(prs) and queries the open state", func() {
			gh.PRListReturns([]vcs.PR{{Number: 1}, {Number: 2}}, nil)
			cfg := &config.Config{Projects: []config.Project{{Path: "/r", GH: "o/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].PR).To(Equal(dashboard.PRCell{State: dashboard.PRCounted, Count: 2}))
			slug, state := gh.PRListArgsForCall(0)
			Expect(slug).To(Equal("o/r"))
			Expect(state).To(Equal("open"))
		})

		It("is Unknown when gh fails", func() {
			gh.PRListReturns(nil, errors.New("gh down"))
			cfg := &config.Config{Projects: []config.Project{{Path: "/r", GH: "o/r"}}}
			res, _ := eng.Summary(cfg, false)
			Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRUnknown))
		})

		It("counts ready and needs-attention open PRs (neutral uncounted)", func() {
			gh.PRListReturns([]vcs.PR{
				{Number: 1, State: "OPEN", Mergeable: "MERGEABLE"},                                      // ready
				{Number: 2, State: "OPEN", Mergeable: "CONFLICTING"},                                    // attention
				{Number: 3, State: "OPEN", Mergeable: "MERGEABLE", ReviewDecision: "CHANGES_REQUESTED"}, // attention
				{Number: 4, State: "OPEN", IsDraft: true},                                               // neutral
			}, nil)
			cfg := &config.Config{Projects: []config.Project{{Path: "/r", GH: "o/r"}}}
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

	It("when gh is absent: no PR call, PR cell '-' (NoSlug), GHAbsent set", func() {
		gh.AvailableReturns(false)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r", GH: "o/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.GHAbsent).To(BeTrue())
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
		Expect(res.GHAbsent).To(BeFalse())
		Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRNoSlug))
		Expect(gh.PRListCallCount()).To(Equal(0))
	})

	It("does not flag GHAbsent when gh is absent but no project is a GitHub remote", func() {
		gh.AvailableReturns(false)
		gh.KnownHostsReturns([]string{"github.com"})
		git.RemoteURLReturns("git@gitlab.com:o/r.git", nil)
		git.RemoteHeadReturns("main", nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.GHAbsent).To(BeFalse()) // no GitHub project -> no spurious "gh not found" note
		Expect(res.Rows[0].PR.State).To(Equal(dashboard.PRNoSlug))
	})

	It("a GitHub Enterprise remote queries gh with a host-prefixed slug", func() {
		gh.AvailableReturns(true)
		gh.KnownHostsReturns([]string{"github.example.com"})
		gh.PRListReturns([]vcs.PR{{Number: 1}}, nil)
		git.RemoteURLReturns("git@github.example.com:o/r.git", nil)
		git.RemoteHeadReturns("main", nil)
		cfg := &config.Config{Projects: []config.Project{{Path: "/r"}}}
		res, _ := eng.Summary(cfg, false)
		Expect(res.Rows[0].PR).To(Equal(dashboard.PRCell{State: dashboard.PRCounted, Count: 1}))
		slug, _ := gh.PRListArgsForCall(0)
		Expect(slug).To(Equal("github.example.com/o/r"))
	})
})
