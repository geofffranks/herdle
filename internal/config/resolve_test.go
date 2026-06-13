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
			Path: "/repo", Remote: "fork", Base: "dev", Integration: "mine", GH: "o/r",
		}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r).To(Equal(config.Resolved{
			Path: "/repo", Name: "repo", Remote: "fork", Base: "dev", Integration: "mine", Slug: "o/r",
			SlugExplicit: true,
		}))
		Expect(git.RemoteURLCallCount()).To(Equal(0)) // nothing to autodetect
	})

	It("falls back to global defaults when project fields are unset", func() {
		git := &vcsfakes.FakeGitRunner{}
		c := &config.Config{DefaultRemote: "origin", DefaultBase: "trunk"}
		r, err := c.Resolve(config.Project{Path: "/repo"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Remote).To(Equal("origin"))
		Expect(r.Base).To(Equal("trunk"))
		Expect(git.RemoteURLCallCount()).To(Equal(1))
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

	It("marks SlugExplicit when the slug comes from a gh= override", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLReturns("git@github.com:me/fork.git", nil)
		git.RemoteHeadReturns("main", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo", Remote: "origin", GH: "canon/repo"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.Slug).To(Equal("canon/repo"))
		Expect(r.SlugExplicit).To(BeTrue())
		Expect(r.RemoteHost).To(Equal(""))
	})

	It("strips the port from a scheme://host:port remote URL", func() {
		git := &vcsfakes.FakeGitRunner{}
		git.RemoteURLReturns("ssh://git@github.com:22/o/r.git", nil)
		git.RemoteHeadReturns("main", nil)
		c := &config.Config{}
		r, err := c.Resolve(config.Project{Path: "/repo", Remote: "origin"}, git)
		Expect(err).NotTo(HaveOccurred())
		Expect(r.RemoteHost).To(Equal("github.com"))
		Expect(r.Slug).To(Equal("o/r"))
	})
})
