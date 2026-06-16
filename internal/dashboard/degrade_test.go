package dashboard_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/dashboard"
	"github.com/geofffranks/herdle/internal/vcs/vcsfakes"
)

var _ = Describe("selectForge (host->forge routing)", func() {
	// An engine wired to both forges. GH knows github.com + github.example.com;
	// GL knows gitlab.com + gitlab.example.com.
	var eng dashboard.Engine
	BeforeEach(func() {
		gh := &vcsfakes.FakeGHRunner{}
		gh.KnownHostsReturns([]string{"github.com", "github.example.com"})
		gl := &vcsfakes.FakeGLRunner{}
		gl.KnownHostsReturns([]string{"gitlab.com", "gitlab.example.com"})
		eng = dashboard.Engine{GH: gh, GL: gl}
	})

	Describe("explicit overrides", func() {
		It("routes a host-less explicit slug (legacy gh=) to GitHub, trusted as-is", func() {
			slug, kind, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "canon/repo", SlugExplicit: true})
			Expect(ok).To(BeTrue())
			Expect(kind).To(Equal("github"))
			Expect(slug).To(Equal("canon/repo"))
		})

		It("routes an explicit slug on a GitLab host to GitLab, trusted as-is", func() {
			slug, kind, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "grp/proj", SlugExplicit: true, RemoteHost: "gitlab.enterprise.io"})
			// gitlab.enterprise.io is not in GL.KnownHosts here, so it is unknown ->
			// defaults to GitHub. A configured self-hosted host (below) routes to GL.
			Expect(ok).To(BeTrue())
			Expect(kind).To(Equal("github"))
			Expect(slug).To(Equal("grp/proj"))
		})

		It("host-qualifies an explicit slug on a known self-hosted GitLab host", func() {
			slug, kind, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "grp/proj", SlugExplicit: true, RemoteHost: "gitlab.example.com"})
			Expect(ok).To(BeTrue())
			Expect(kind).To(Equal("gitlab"))
			// must target the right server, not glab's default host
			Expect(slug).To(Equal("https://gitlab.example.com/grp/proj"))
		})

		It("host-prefixes an explicit slug on a known GitHub Enterprise host", func() {
			slug, kind, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "o/r", SlugExplicit: true, RemoteHost: "github.example.com"})
			Expect(ok).To(BeTrue())
			Expect(kind).To(Equal("github"))
			Expect(slug).To(Equal("github.example.com/o/r"))
		})

		It("does NOT qualify an explicit slug on the canonical host (github.com/gitlab.com)", func() {
			slug, _, _ := eng.SelectForgeForTest(
				config.Resolved{Slug: "grp/proj", SlugExplicit: true, RemoteHost: "gitlab.com"})
			Expect(slug).To(Equal("grp/proj"))
		})

		It("rejects an empty explicit slug", func() {
			_, _, ok := eng.SelectForgeForTest(config.Resolved{SlugExplicit: true})
			Expect(ok).To(BeFalse())
		})
	})

	Describe("derived slugs (host-gated)", func() {
		It("returns a bare owner/repo for a github.com remote", func() {
			slug, kind, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "o/r", RemoteHost: "github.com"})
			Expect(ok).To(BeTrue())
			Expect(kind).To(Equal("github"))
			Expect(slug).To(Equal("o/r"))
		})

		It("host-prefixes a GitHub Enterprise remote", func() {
			slug, kind, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "o/r", RemoteHost: "github.example.com"})
			Expect(ok).To(BeTrue())
			Expect(kind).To(Equal("github"))
			Expect(slug).To(Equal("github.example.com/o/r"))
		})

		It("returns a bare group/project for a gitlab.com remote", func() {
			slug, kind, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "grp/proj", RemoteHost: "gitlab.com"})
			Expect(ok).To(BeTrue())
			Expect(kind).To(Equal("gitlab"))
			Expect(slug).To(Equal("grp/proj"))
		})

		It("uses a full https URL for a self-hosted GitLab remote", func() {
			slug, kind, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "grp/proj", RemoteHost: "gitlab.example.com"})
			Expect(ok).To(BeTrue())
			Expect(kind).To(Equal("gitlab"))
			Expect(slug).To(Equal("https://gitlab.example.com/grp/proj"))
		})

		It("rejects a host belonging to no configured forge", func() {
			slug, _, ok := eng.SelectForgeForTest(
				config.Resolved{Slug: "o/r", RemoteHost: "bitbucket.org"})
			Expect(ok).To(BeFalse())
			Expect(slug).To(Equal(""))
		})
	})

	Describe("without a GitLab runner wired (GL nil)", func() {
		It("treats GitLab remotes as having no forge", func() {
			gh := &vcsfakes.FakeGHRunner{}
			gh.KnownHostsReturns([]string{"github.com"})
			noGL := dashboard.Engine{GH: gh}
			_, _, ok := noGL.SelectForgeForTest(
				config.Resolved{Slug: "grp/proj", RemoteHost: "gitlab.com"})
			Expect(ok).To(BeFalse())
		})
	})
})
