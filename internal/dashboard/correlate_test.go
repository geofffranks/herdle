package dashboard

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/vcs"
)

// These are white-box tests (package dashboard) because the helpers are unexported.
var _ = Describe("correlation helpers", func() {
	Describe("ghNum", func() {
		DescribeTable("last digit-run of an external-ref",
			func(ref, want string) { Expect(ghNum(ref)).To(Equal(want)) },
			Entry("gh-682", "gh-682", "682"),
			Entry("issues path", "https://x/issues/59", "59"),
			Entry("pull path", "/pull/767", "767"),
			Entry("empty", "", ""),
			Entry("no digits", "gh-", ""),
		)
	})

	Describe("branchHasNum", func() {
		DescribeTable("n as a whole-number token",
			func(branch, n string, want bool) { Expect(branchHasNum(branch, n)).To(Equal(want)) },
			Entry("token match", "fix/59-x", "59", true),
			Entry("not a substring of a longer number", "fix/590-x", "59", false),
			Entry("trailing", "feat-59", "59", true),
			Entry("leading", "59-feat", "59", true),
			Entry("empty n never matches", "anything", "", false),
		)
	})

	dt := func(id, ref, branch string) dticket {
		return dticket{Ticket: vcs.Ticket{ID: id, ExternalRef: ref, Branch: branch}}
	}

	Describe("tksForPR", func() {
		// a: ghNum==12. c: explicit branch == prHead. b: its branch carries "12" but
		// it has no external-ref and its branch != prHead, so it does NOT correlate —
		// wip's tks_for_pr never number-matches a ticket's own branch against the PR.
		tickets := []dticket{dt("a", "gh-12", ""), dt("b", "", "fix/12-x"), dt("c", "", "feature")}
		It("matches by ghNum==prNum, head carrying ghNum, or explicit branch", func() {
			Expect(tksForPR(tickets, 12, "feature")).To(Equal([]string{"a", "c"}))
		})
		It("matches a ticket whose ghNum appears in the PR head branch", func() {
			Expect(tksForPR(tickets, 99, "fix/12-y")).To(Equal([]string{"a"}))
		})
		It("returns nil when nothing correlates", func() {
			Expect(tksForPR(tickets, 99, "other")).To(BeNil())
		})
	})

	Describe("tkForBranch", func() {
		tickets := []dticket{dt("a", "gh-12", ""), dt("b", "", "exact")}
		It("matches an explicit branch first", func() {
			t, ok := tkForBranch(tickets, "exact")
			Expect(ok).To(BeTrue())
			Expect(t.ID).To(Equal("b"))
		})
		It("matches a branch carrying the ghNum", func() {
			t, ok := tkForBranch(tickets, "fix/12-y")
			Expect(ok).To(BeTrue())
			Expect(t.ID).To(Equal("a"))
		})
		It("reports no match", func() {
			_, ok := tkForBranch(tickets, "none")
			Expect(ok).To(BeFalse())
		})
	})

	Describe("tkInAnyPR", func() {
		prs := []vcs.PR{{Number: 12, HeadRefName: "fix/12"}}
		It("is false when the ticket has neither ghNum nor branch", func() {
			Expect(tkInAnyPR(dt("x", "", ""), prs)).To(BeFalse())
		})
		It("is true when a PR correlates by number", func() {
			Expect(tkInAnyPR(dt("x", "gh-12", ""), prs)).To(BeTrue())
		})
		It("is true when a ref-less ticket's explicit branch equals a PR head", func() {
			// The jtac-autolase case: no external-ref, only a branch: field.
			Expect(tkInAnyPR(dt("x", "", "fix/12"), prs)).To(BeTrue())
		})
		It("is false when the explicit branch matches no PR head", func() {
			Expect(tkInAnyPR(dt("x", "", "other"), prs)).To(BeFalse())
		})
	})
})
