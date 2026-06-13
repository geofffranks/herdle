package render

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/dashboard"
)

// White-box (package render) — formatters are unexported.
var _ = Describe("palette + formatters", func() {
	Describe("pad", func() {
		It("byte-pads short strings to width", func() {
			Expect(pad("ab", 5)).To(Equal("ab   "))
		})
		It("truncates over-width strings to w-1 runes + ellipsis", func() {
			Expect(pad("validation", 6)).To(Equal("valid…"))
		})
	})

	Describe("color gating", func() {
		It("emits no codes when color is off", func() {
			p := newPalette(false)
			Expect(p.fstate("validated")).To(Equal(pad("validated", 19)))
			Expect(p.hdr("x")).To(Equal("x"))
			Expect(p.fflags(dashboard.FlagNote{Text: "note", Sev: dashboard.SevRed})).To(Equal("note"))
		})
		It("wraps with codes when color is on (padding inside the codes)", func() {
			p := newPalette(true)
			Expect(p.fstate("planned")).To(Equal("\x1b[34m" + pad("planned", 19) + "\x1b[0m")) // blue
			Expect(p.fsync(dashboard.SyncOK)).To(Equal("\x1b[32m✓\x1b[0m   "))                 // green + 3 spaces
			Expect(p.fpr(7)).To(Equal("\x1b[1m#7    \x1b[0m"))                                 // #%-5
		})
		It("returns empty for an empty flag note", func() {
			Expect(newPalette(true).fflags(dashboard.FlagNote{})).To(Equal(""))
		})
	})
})
