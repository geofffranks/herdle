package render_test

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/dashboard"
	"github.com/geofffranks/herdle/internal/render"
)

// sampleDrilldown exercises every section + both empty-section paths.
var sampleDrilldown = dashboard.Drilldown{
	Name: "herdle", Path: "/home/u/herdle", Fetched: false,
	Head:    dashboard.HeadInfo{Branch: "main"},
	HasSlug: true,
	OpenPRs: []dashboard.PRRow{
		{Number: 7, Head: "fix/7-x", Title: "a fix", TKs: []string{"her-aaaa"}, Note: dashboard.FlagNote{Text: "✓ in sync", Sev: dashboard.SevGreen}},
	},
	MergedCleanup: []dashboard.MergedRow{
		{Number: 6, Head: "old", Title: "merged thing", Flags: dashboard.FlagNote{Text: "⚠ local branch", Sev: dashboard.SevYellow}},
	},
	WIP: []dashboard.WIPRow{
		{Lifecycle: "in-development", Sync: dashboard.SyncBad, TKID: "her-bbbb", Branch: "feature-y", Title: "the feature", Issue: "↑1 unpushed", IssueSev: dashboard.SevRed},
		{Lifecycle: "-", Sync: dashboard.SyncNA, TKID: "", Branch: "orphan", Title: "", Issue: "no tk", IssueSev: dashboard.SevYellow},
	},
	UpNext: []dashboard.UpNextRow{
		{Lifecycle: "planned", TKID: "her-cccc", Title: "next up", Priority: 2},
	},
	Artifacts: []dashboard.ArtifactRow{
		{TKID: "her-aaaa", Kind: "specs", Filename: "2026-06-13-her-aaaa-x-design.md"},
	},
}

func renderDrilldown(color bool) string {
	var buf bytes.Buffer
	Expect(render.Drilldown(&buf, sampleDrilldown, color)).To(Succeed())
	return buf.String()
}

var _ = Describe("render.Drilldown", func() {
	It("matches the full golden (NO_COLOR / color off)", func() {
		matchesGolden("drilldown_full.golden", renderDrilldown(false))
	})

	It("color-on and color-off carry identical plain text once stripped", func() {
		on := renderDrilldown(true)
		Expect(on).To(ContainSubstring("\x1b[")) // drilldown DOES use color (unlike summary)
		Expect(stripANSI(on)).To(Equal(renderDrilldown(false)))
	})

	It("shows '(gh unavailable)' when the slug is set but gh failed and no PRs", func() {
		d := sampleDrilldown
		d.OpenPRs = nil
		d.GHUnavailable = true
		var buf bytes.Buffer
		Expect(render.Drilldown(&buf, d, false)).To(Succeed())
		Expect(buf.String()).To(ContainSubstring("— open PRs —"))
		Expect(buf.String()).To(ContainSubstring("(gh unavailable)"))
	})

	It("hides empty sections (no slug, nothing to show)", func() {
		d := dashboard.Drilldown{Name: "x", Path: "/x", Head: dashboard.HeadInfo{Branch: "main"}}
		var buf bytes.Buffer
		Expect(render.Drilldown(&buf, d, false)).To(Succeed())
		out := buf.String()
		Expect(out).NotTo(ContainSubstring("— open PRs —"))
		Expect(out).NotTo(ContainSubstring("— work in progress"))
		Expect(out).To(ContainSubstring("sync:"))
		Expect(out).To(ContainSubstring("lifecycle:"))
	})
})

// stripANSI removes CSI sequences for the color-parity assertion.
func stripANSI(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b = append(b, s[i])
	}
	return string(b)
}
