package render_test

import (
	"bytes"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/dashboard"
	"github.com/geofffranks/herdle/internal/render"
)

// sampleRows exercises every cell variant: no-slug PR, counted PR, unknown PR,
// detached head, dirty + ahead/behind (multibyte arrows), present and absent tk.
var sampleRows = []dashboard.SummaryRow{
	{
		Name:     "herdle",
		Head:     dashboard.HeadInfo{Branch: "main"},
		PR:       dashboard.PRCell{State: dashboard.PRNoSlug},
		TK:       dashboard.TKCell{Present: true, InProgress: 1, Ready: 4},
		Problems: 0,
	},
	{
		Name:     "dcs-retribution",
		Head:     dashboard.HeadInfo{Branch: "fix/123", Dirty: true, Ahead: 2, Behind: 1},
		PR:       dashboard.PRCell{State: dashboard.PRCounted, Count: 3, Attention: 2, Ready: 1},
		TK:       dashboard.TKCell{Present: true},
		Problems: 3,
	},
	{
		Name:     "plain",
		Head:     dashboard.HeadInfo{Branch: ""},
		PR:       dashboard.PRCell{State: dashboard.PRUnknown},
		TK:       dashboard.TKCell{Present: false},
		Problems: 0,
	},
	{
		Name:     "quiet",
		Head:     dashboard.HeadInfo{Branch: "main"},
		PR:       dashboard.PRCell{State: dashboard.PRCounted, Count: 2}, // all neutral -> merge "-"
		TK:       dashboard.TKCell{Present: true, Ready: 1},
		Problems: 0,
	},
}

func renderSummary(fetched bool) string {
	var buf bytes.Buffer
	Expect(render.Summary(&buf, sampleRows, fetched, nil)).To(Succeed())
	return buf.String()
}

// matchesGolden compares got to testdata/<name>, regenerating the file first when
// UPDATE_GOLDEN is set (run once, eyeball-verify against wip, then run normally).
// NOTE: in regenerate mode (UPDATE_GOLDEN != "") the assertion passes by construction —
// UPDATE_GOLDEN must not be set in CI or before committing; always unset it and rerun.
func matchesGolden(name, got string) {
	path := "testdata/" + name // #nosec G304 -- test-controlled fixture path
	if os.Getenv("UPDATE_GOLDEN") != "" {
		Expect(os.MkdirAll("testdata", 0o750)).To(Succeed())
		Expect(os.WriteFile(path, []byte(got), 0o600)).To(Succeed())
	}
	want, err := os.ReadFile(path) // #nosec G304 -- test-controlled fixture path
	Expect(err).NotTo(HaveOccurred())
	Expect(got).To(Equal(string(want)))
}

var _ = Describe("render.Summary", func() {
	It("with zero rows emits the header, separator, and footer only", func() {
		var buf bytes.Buffer
		Expect(render.Summary(&buf, nil, false, nil)).To(Succeed())
		matchesGolden("summary_empty.golden", buf.String())
	})

	It("matches the cached-footer golden file", func() {
		matchesGolden("summary_cached.golden", renderSummary(false))
	})

	It("matches the fetched-footer golden file", func() {
		matchesGolden("summary_fetched.golden", renderSummary(true))
	})

	It("names the absent forge CLIs in the footer (one or many)", func() {
		var buf bytes.Buffer
		Expect(render.Summary(&buf, sampleRows, false, []string{"glab"})).To(Succeed())
		Expect(buf.String()).To(ContainSubstring("glab not found — PR/MR counts hidden"))

		buf.Reset()
		Expect(render.Summary(&buf, sampleRows, false, []string{"gh", "glab"})).To(Succeed())
		Expect(buf.String()).To(ContainSubstring("gh/glab not found — PR/MR counts hidden"))
	})

	It("omits the forge note when nothing is absent", func() {
		Expect(renderSummary(false)).NotTo(ContainSubstring("not found"))
	})

	It("emits byte-identical output under forced color and NO_COLOR (no leak)", func() {
		GinkgoT().Setenv("NO_COLOR", "")
		GinkgoT().Setenv("FORCE_COLOR", "1")
		forced := renderSummary(false)
		GinkgoT().Setenv("FORCE_COLOR", "")
		GinkgoT().Setenv("NO_COLOR", "1")
		nocolor := renderSummary(false)
		Expect(forced).To(Equal(nocolor))
		Expect(forced).NotTo(ContainSubstring("\x1b[")) // summary never emits an escape code
	})
})
