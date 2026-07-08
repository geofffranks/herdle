package dashboard

import (
	"fmt"

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
	mustText := func(text string, _ bool) string { return text }

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

	Describe("prTKIssue", func() {
		dl := func(id, ref, lc string) dticket {
			return dticket{Ticket: vcs.Ticket{ID: id, ExternalRef: ref}, EffLifecycle: lc}
		}
		It("is silent when the repo has no tk and the PR has no ticket", func() {
			text, bad := prTKIssue(nil, 5, "feat")
			Expect(bad).To(BeFalse())
			Expect(text).To(Equal(""))
		})
		It("flags 'no tk' when the repo uses tk but the PR has none", func() {
			tickets := []dticket{dl("a", "gh-99", "validated")} // unrelated to PR 5
			text, bad := prTKIssue(tickets, 5, "feat")
			Expect(bad).To(BeTrue())
			Expect(text).To(Equal("no tk"))
		})
		It("is silent when the only correlated ticket is validated", func() {
			tickets := []dticket{dl("a", "gh-5", "validated")}
			_, bad := prTKIssue(tickets, 5, "feat")
			Expect(bad).To(BeFalse())
		})
		It("names a real non-validated state", func() {
			tickets := []dticket{dl("a", "gh-5", "pending-validation")}
			text, bad := prTKIssue(tickets, 5, "feat")
			Expect(bad).To(BeTrue())
			Expect(text).To(Equal("tk a pending-validation"))
		})
		It("renders ? and - as unvalidated(<lc>)", func() {
			Expect(mustText(prTKIssue([]dticket{dl("a", "gh-5", "?")}, 5, "feat"))).To(Equal("tk a unvalidated (?)"))
			Expect(mustText(prTKIssue([]dticket{dl("a", "gh-5", "-")}, 5, "feat"))).To(Equal("tk a unvalidated (-)"))
		})
		It("lists only the non-validated tickets when several correlate", func() {
			tickets := []dticket{dl("a", "gh-5", "validated"), dl("b", "gh-5", "in-development")}
			text, bad := prTKIssue(tickets, 5, "feat")
			Expect(bad).To(BeTrue())
			Expect(text).To(Equal("tk b in-development"))
		})
		It("joins multiple non-validated tickets with a comma", func() {
			tickets := []dticket{dl("a", "gh-5", "pending-validation"), dl("b", "gh-5", "?")}
			text, _ := prTKIssue(tickets, 5, "feat")
			Expect(text).To(Equal("tk a pending-validation, tk b unvalidated (?)"))
		})
	})

	Describe("issueTKs / issueTriaged", func() {
		It("triages an issue when a tk external-ref resolves to its number", func() {
			tks := []dticket{{Ticket: vcs.Ticket{ID: "her-x2b", ExternalRef: "gh-59"}}}
			Expect(issueTriaged(tks, 59)).To(BeTrue())
			Expect(issueTKs(tks, 59)).To(Equal([]string{"her-x2b"}))
		})
		It("triages via an explicit /issues/ URL ref", func() {
			tks := []dticket{{Ticket: vcs.Ticket{ID: "her-x2b", ExternalRef: "https://github.com/o/r/issues/59"}}}
			Expect(issueTriaged(tks, 59)).To(BeTrue())
		})
		It("does NOT triage issue #59 from a /pull/59 ref (that ref is a PR)", func() {
			tks := []dticket{{Ticket: vcs.Ticket{ID: "her-x2b", ExternalRef: "https://github.com/o/r/pull/59"}}}
			Expect(issueTriaged(tks, 59)).To(BeFalse())
		})
		It("reports an un-triaged issue when no ref matches", func() {
			tks := []dticket{{Ticket: vcs.Ticket{ID: "her-x2b", ExternalRef: "gh-12"}}}
			Expect(issueTriaged(tks, 59)).To(BeFalse())
			Expect(issueTKs(tks, 59)).To(BeNil())
		})
	})

	Describe("issueRows collapse", func() {
		// build n triaged issues (#1..#n, each tracked by a gh-N ticket) plus one
		// un-triaged issue (#99).
		build := func(n int) ([]vcs.Issue, []dticket) {
			var issues []vcs.Issue
			var tks []dticket
			for i := 1; i <= n; i++ {
				issues = append(issues, vcs.Issue{Number: i, State: "OPEN"})
				tks = append(tks, dticket{Ticket: vcs.Ticket{ID: fmt.Sprintf("tk-%d", i), ExternalRef: fmt.Sprintf("gh-%d", i)}})
			}
			issues = append(issues, vcs.Issue{Number: 99, State: "OPEN"})
			return issues, tks
		}

		It("lists untriaged first, then the first triagedListLimit triaged, collapsing only the excess", func() {
			issues, tks := build(triagedListLimit + 1) // 11 triaged
			rows, hidden, capped := issueRows(issues, tks)
			Expect(capped).To(BeFalse())
			Expect(hidden).To(Equal(1)) // 11 triaged - 10 limit = 1 collapsed
			Expect(rows[0].Number).To(Equal(99))
			Expect(rows[0].Untriaged).To(BeTrue())
			triagedShown := rows[1:]
			Expect(triagedShown).To(HaveLen(triagedListLimit))
			for i, r := range triagedShown {
				Expect(r.Untriaged).To(BeFalse())
				Expect(r.TKs).NotTo(BeEmpty())
				Expect(r.Number).To(Equal(i + 1)) // the FIRST triagedListLimit (#1..#10), in order — pins against an off-by-one slice
			}
			// the (triagedListLimit+1)-th triaged issue (#11) is the collapsed one, not shown
			for _, r := range rows {
				Expect(r.Number).NotTo(Equal(triagedListLimit + 1))
			}
		})

		It("lists all triaged individually at the limit (no collapse)", func() {
			issues, tks := build(triagedListLimit) // 10 triaged
			rows, hidden, _ := issueRows(issues, tks)
			Expect(hidden).To(BeZero())
			Expect(rows).To(HaveLen(triagedListLimit + 1)) // 10 triaged + 1 untriaged
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
