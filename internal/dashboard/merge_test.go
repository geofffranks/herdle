package dashboard_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/dashboard"
	"github.com/geofffranks/herdle/internal/vcs"
)

var _ = Describe("classifyMerge", func() {
	pass := vcs.CheckRun{Typename: "CheckRun", Status: "COMPLETED", Conclusion: "SUCCESS"}
	fail := vcs.CheckRun{Typename: "CheckRun", Status: "COMPLETED", Conclusion: "FAILURE"}
	running := vcs.CheckRun{Typename: "CheckRun", Status: "IN_PROGRESS"}
	ctxFail := vcs.CheckRun{Typename: "StatusContext", State: "FAILURE"}

	DescribeTable("maps a PR to its merge status",
		func(pr vcs.PR, want dashboard.MergeStatus) {
			Expect(dashboard.ClassifyMergeForTest(pr)).To(Equal(want))
		},
		Entry("draft wins over everything",
			vcs.PR{IsDraft: true, Mergeable: "CONFLICTING", StatusCheckRollup: []vcs.CheckRun{fail}},
			dashboard.MergeNeutral),
		Entry("conflicts",
			vcs.PR{Mergeable: "CONFLICTING"}, dashboard.MergeConflicts),
		Entry("checks failing (CheckRun) beats changes-requested",
			vcs.PR{Mergeable: "MERGEABLE", ReviewDecision: "CHANGES_REQUESTED", StatusCheckRollup: []vcs.CheckRun{pass, fail}},
			dashboard.MergeChecksFailing),
		Entry("checks failing (StatusContext)",
			vcs.PR{Mergeable: "MERGEABLE", StatusCheckRollup: []vcs.CheckRun{ctxFail}},
			dashboard.MergeChecksFailing),
		Entry("changes requested when checks pass",
			vcs.PR{Mergeable: "MERGEABLE", ReviewDecision: "CHANGES_REQUESTED", StatusCheckRollup: []vcs.CheckRun{pass}},
			dashboard.MergeChangesRequested),
		Entry("ready: mergeable + checks pass",
			vcs.PR{Mergeable: "MERGEABLE", ReviewDecision: "REVIEW_REQUIRED", StatusCheckRollup: []vcs.CheckRun{pass}},
			dashboard.MergeReady),
		Entry("ready: mergeable + no checks at all",
			vcs.PR{Mergeable: "MERGEABLE"}, dashboard.MergeReady),
		Entry("neutral: checks still running",
			vcs.PR{Mergeable: "MERGEABLE", StatusCheckRollup: []vcs.CheckRun{running}},
			dashboard.MergeNeutral),
		Entry("neutral: mergeability unknown",
			vcs.PR{Mergeable: "UNKNOWN", StatusCheckRollup: []vcs.CheckRun{pass}},
			dashboard.MergeNeutral),
	)
})

var _ = Describe("mergeNote", func() {
	It("renders each status as a colored note segment", func() {
		Expect(dashboard.MergeNoteForTest(dashboard.MergeReady)).To(Equal(dashboard.FlagNote{Text: "✓ ready to merge", Sev: dashboard.SevGreen}))
		Expect(dashboard.MergeNoteForTest(dashboard.MergeConflicts)).To(Equal(dashboard.FlagNote{Text: "✗ conflicts", Sev: dashboard.SevRed}))
		Expect(dashboard.MergeNoteForTest(dashboard.MergeChecksFailing)).To(Equal(dashboard.FlagNote{Text: "✗ checks failing", Sev: dashboard.SevRed}))
		Expect(dashboard.MergeNoteForTest(dashboard.MergeChangesRequested)).To(Equal(dashboard.FlagNote{Text: "✎ changes requested", Sev: dashboard.SevYellow}))
		Expect(dashboard.MergeNoteForTest(dashboard.MergeNeutral)).To(Equal(dashboard.FlagNote{Text: "—", Sev: dashboard.SevNone}))
	})
})
