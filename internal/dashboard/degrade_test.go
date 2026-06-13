package dashboard_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/dashboard"
)

var _ = Describe("effectiveSlug", func() {
	known := map[string]bool{"github.com": true, "github.example.com": true}

	It("trusts a gh= override regardless of host", func() {
		slug, ok := dashboard.EffectiveSlugForTest(
			config.Resolved{Slug: "canon/repo", SlugExplicit: true, RemoteHost: "gitlab.com"}, known)
		Expect(ok).To(BeTrue())
		Expect(slug).To(Equal("canon/repo"))
	})

	It("returns a bare owner/repo for a github.com remote", func() {
		slug, ok := dashboard.EffectiveSlugForTest(
			config.Resolved{Slug: "o/r", RemoteHost: "github.com"}, known)
		Expect(ok).To(BeTrue())
		Expect(slug).To(Equal("o/r"))
	})

	It("host-prefixes a GitHub Enterprise remote", func() {
		slug, ok := dashboard.EffectiveSlugForTest(
			config.Resolved{Slug: "o/r", RemoteHost: "github.example.com"}, known)
		Expect(ok).To(BeTrue())
		Expect(slug).To(Equal("github.example.com/o/r"))
	})

	It("rejects a non-GitHub host (no PR features)", func() {
		slug, ok := dashboard.EffectiveSlugForTest(
			config.Resolved{Slug: "o/r", RemoteHost: "gitlab.com"}, known)
		Expect(ok).To(BeFalse())
		Expect(slug).To(Equal(""))
	})

	It("rejects an empty slug", func() {
		_, ok := dashboard.EffectiveSlugForTest(config.Resolved{RemoteHost: "github.com"}, known)
		Expect(ok).To(BeFalse())
	})
})
