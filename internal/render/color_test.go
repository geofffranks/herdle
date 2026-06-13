package render_test

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/render"
)

var _ = Describe("DetectColor", func() {
	var buf bytes.Buffer

	BeforeEach(func() {
		buf.Reset()
		// Clean baseline (auto-restored by GinkgoT().Setenv). Empty == not-forcing,
		// matching wip's `[ -n ... ]` / `[ -z ... ]` non-empty semantics.
		GinkgoT().Setenv("NO_COLOR", "")
		GinkgoT().Setenv("FORCE_COLOR", "")
		GinkgoT().Setenv("CLICOLOR_FORCE", "")
	})

	It("is off for a non-terminal writer with no force env", func() {
		Expect(render.DetectColor(&buf)).To(BeFalse())
	})

	It("is on when FORCE_COLOR is set", func() {
		GinkgoT().Setenv("FORCE_COLOR", "1")
		Expect(render.DetectColor(&buf)).To(BeTrue())
	})

	It("is on when CLICOLOR_FORCE is set", func() {
		GinkgoT().Setenv("CLICOLOR_FORCE", "1")
		Expect(render.DetectColor(&buf)).To(BeTrue())
	})

	It("lets NO_COLOR win over a force variable", func() {
		GinkgoT().Setenv("FORCE_COLOR", "1")
		GinkgoT().Setenv("NO_COLOR", "1")
		Expect(render.DetectColor(&buf)).To(BeFalse())
	})

	It("treats an empty NO_COLOR as not-set (wip uses -n, not set-ness)", func() {
		GinkgoT().Setenv("FORCE_COLOR", "1")
		GinkgoT().Setenv("NO_COLOR", "") // set but empty -> does not disable
		Expect(render.DetectColor(&buf)).To(BeTrue())
	})
})
