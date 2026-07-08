package config_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
	"github.com/geofffranks/herdle/internal/vcs/vcsfakes"
)

var _ = Describe("Config.Resolve", func() {
	It("prefers explicit project fields over everything", func() {
		git := &vcsfakes.FakeGitRunner{}
		c := &config.Config{DefaultRemote: "origin", DefaultBase: "trunk"}
		r, err := c.Resolve(config.Project{
			Path: "/repo", Remote: "fork", Base: "dev", Integration: "mine", Slug: "o/r",
		}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r).To(Equal(config.Resolved{
			Path: "/repo", Name: "repo", Remote: "fork", Base: "dev", Integration: "mine", Slug: "o/r",
			SlugExplicit: true,
		}))
		// The explicit slug value wins, but the remote host is still probed for
		// forge routing (the fake returns no URL here, so RemoteHost stays "").
		// TrackIssues also probes for the "upstream" remote => two calls total.
		Expect(git.RemoteURLCallCount()).To(Equal(2))
	})

	It("falls back to global defaults when project fields are unset", func() {
		git := &vcsfakes.FakeGitRunner{}
		c := &config.Config{DefaultRemote: "origin", DefaultBase: "trunk"}
		r, err := c.Resolve(config.Project{Path: "/repo"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Remote).To(Equal("origin"))
		Expect(r.Base).To(Equal("trunk"))
		// TrackIssues also probes for the "upstream" remote => two calls total.
		Expect(git.RemoteURLCallCount()).To(Equal(2))
	})

	It("falls back to upstream when origin is absent (base from RemoteHead, slug from the URL)", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLStub = func(_, remote string) (string, error) {
			if remote == "upstream" {
				return "git@github.com:o/r.git", nil
			}
			return "", errors.New("no such remote")
		}
		git.RemoteHeadReturns("develop", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Remote).To(Equal("upstream"))
		Expect(r.Base).To(Equal("develop"))
		Expect(r.Slug).To(Equal("o/r"))
	})

	It("falls back to origin, then main/master when RemoteHead is empty", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLStub = func(_, remote string) (string, error) {
			if remote == "origin" {
				return "https://github.com/o/r", nil // no .git suffix
			}
			return "", errors.New("no such remote")
		}
		git.RemoteHeadReturns("", nil) // no HEAD ref
		git.RemoteBranchExistsStub = func(_, _, branch string) (bool, error) {
			return branch == "master", nil // only master exists
		}
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Remote).To(Equal("origin"))
		Expect(r.Base).To(Equal("master"))
		Expect(r.Slug).To(Equal("o/r"))
	})

	It("degrades to empty remote/slug and base main when there is no remote", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLReturns("", errors.New("no remote"))
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Remote).To(Equal(""))
		Expect(r.Base).To(Equal("main"))
		Expect(r.Slug).To(Equal(""))
		Expect(git.RemoteHeadCallCount()).To(Equal(0)) // no remote -> skipped
	})
})

var _ = Describe("slugFromURL (via Resolve)", func() {
	cases := map[string]string{
		"git@github.com:o/r.git":     "o/r",
		"https://github.com/o/r.git": "o/r",
		"https://github.com/o/r":     "o/r",
		"ssh://git@host/o/r.git":     "o/r",
		"not-a-url":                  "",
		// GitLab nested groups: any depth >= 2 segments is a valid project path.
		"git@gitlab.rivianvw.io:vt/ps/infra/rcs/rivian_crypto_service.git": "vt/ps/infra/rcs/rivian_crypto_service",
		"https://gitlab.com/group/subgroup/project.git":                    "group/subgroup/project",
		"git@host:onlyone":  "", // single segment -> not a slug
		"https://host/a//b": "", // empty segment -> rejected
	}
	for url, want := range cases {
		url, want := url, want
		It("parses "+url, func() {
			git := &vcsfakes.FakeGitRunner{}
			git.RemoteURLReturns(url, nil) // remote resolves to origin via this URL
			git.RemoteHeadReturns("main", nil)
			c := &config.Config{}
			r, err := c.Resolve(config.Project{Path: "/repo", Remote: "origin"}, git)
			Expect(err).NotTo(HaveOccurred())
			Expect(r.Slug).To(Equal(want))
		})
	}
	_ = vcs.ErrNotARepo // keep the vcs import even if unused above
})

func gitWith(remotes map[string]string) *vcsfakes.FakeGitRunner {
	g := &vcsfakes.FakeGitRunner{}
	g.RemoteURLStub = func(_, remote string) (string, error) {
		if u, ok := remotes[remote]; ok {
			return u, nil
		}
		return "", errors.New("no such remote")
	}
	return g
}

var _ = Describe("Resolve TrackIssues", func() {
	cfg := &config.Config{}
	It("tracks issues when only origin exists (source of truth)", func() {
		r, _ := cfg.Resolve(config.Project{Path: "/p"}, gitWith(map[string]string{"origin": "git@github.com:me/repo.git"}))
		Expect(r.TrackIssues).To(BeTrue())
	})
	It("does not track when an upstream remote exists (fork)", func() {
		r, _ := cfg.Resolve(config.Project{Path: "/p"}, gitWith(map[string]string{
			"origin": "git@github.com:me/repo.git", "upstream": "git@github.com:orig/repo.git"}))
		Expect(r.TrackIssues).To(BeFalse())
	})
	It("issues=true forces tracking on despite an upstream", func() {
		on := true
		r, _ := cfg.Resolve(config.Project{Path: "/p", Issues: &on}, gitWith(map[string]string{
			"origin": "git@github.com:me/repo.git", "upstream": "git@github.com:orig/repo.git"}))
		Expect(r.TrackIssues).To(BeTrue())
	})
	It("issues=false forces tracking off despite no upstream", func() {
		off := false
		r, _ := cfg.Resolve(config.Project{Path: "/p", Issues: &off}, gitWith(map[string]string{"origin": "git@github.com:me/repo.git"}))
		Expect(r.TrackIssues).To(BeFalse())
	})
	It("skips the upstream remote probe entirely when an issues= override is set", func() {
		on := true
		g := gitWith(map[string]string{"origin": "git@github.com:me/repo.git"})
		cfg.Resolve(config.Project{Path: "/p", Issues: &on}, g)
		for i := 0; i < g.RemoteURLCallCount(); i++ {
			_, remote := g.RemoteURLArgsForCall(i)
			Expect(remote).NotTo(Equal("upstream")) // override short-circuits the probe — no extra git call
		}
	})
})

var _ = Describe("Config.Resolve — S6 additions", func() {
	It("prefers origin over upstream when both remotes exist", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLStub = func(_, remote string) (string, error) {
			switch remote {
			case "origin":
				return "git@github.com:me/fork.git", nil
			case "upstream":
				return "git@github.com:canon/repo.git", nil
			}
			return "", errors.New("no such remote")
		}
		git.RemoteHeadReturns("main", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Remote).To(Equal("origin"))
		Expect(r.Slug).To(Equal("me/fork"))
	})

	It("sets RemoteHost from the derived remote URL and leaves SlugExplicit false", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLReturns("git@github.example.com:o/r.git", nil)
		git.RemoteHeadReturns("main", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo", Remote: "origin"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Slug).To(Equal("o/r"))
		Expect(r.RemoteHost).To(Equal("github.example.com"))
		Expect(r.SlugExplicit).To(BeFalse())
	})

	It("marks SlugExplicit for a slug= override AND still resolves RemoteHost", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLReturns("git@gitlab.enterprise.io:grp/proj.git", nil)
		git.RemoteHeadReturns("main", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo", Remote: "origin", Slug: "grp/override"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Slug).To(Equal("grp/override")) // explicit value wins
		Expect(r.SlugExplicit).To(BeTrue())
		Expect(r.RemoteHost).To(Equal("gitlab.enterprise.io")) // probed, for forge routing
	})

	It("probes the remote host even for an explicit slug= (no legacy probe-skip)", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLReturns("git@github.com:me/fork.git", nil)
		git.RemoteHeadReturns("main", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo", Remote: "fork", Slug: "canon/repo"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Slug).To(Equal("canon/repo")) // explicit value trusted as-is
		Expect(r.SlugExplicit).To(BeTrue())
		Expect(r.RemoteHost).To(Equal("github.com"))  // host resolved for routing
		Expect(git.RemoteURLCallCount()).To(Equal(2)) // host probe + TrackIssues upstream probe
	})

	It("strips the port from RemoteHost but retains it in RemoteHostPort", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLReturns("https://gitlab.internal:8929/grp/proj.git", nil)
		git.RemoteHeadReturns("main", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo", Remote: "origin"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.RemoteHost).To(Equal("gitlab.internal"))          // port stripped for routing
		Expect(r.RemoteHostPort).To(Equal("gitlab.internal:8929")) // port retained for URL rebuild
		Expect(r.Slug).To(Equal("grp/proj"))
	})

	It("leaves RemoteHostPort equal to RemoteHost when the URL has no port", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLReturns("ssh://git@github.com/o/r.git", nil)
		git.RemoteHeadReturns("main", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo", Remote: "origin"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.RemoteHost).To(Equal("github.com"))
		Expect(r.RemoteHostPort).To(Equal("github.com"))
		Expect(r.Slug).To(Equal("o/r"))
	})
})
