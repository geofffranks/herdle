package vcs_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/vcs"
	"github.com/geofffranks/herdle/internal/vcs/vcsfakes"
)

// Compile-time proof the fakes satisfy the interfaces.
var (
	_ vcs.GitRunner = &vcsfakes.FakeGitRunner{}
	_ vcs.GHRunner  = &vcsfakes.FakeGHRunner{}
	_ vcs.TKRunner  = &vcsfakes.FakeTKRunner{}
)

var _ = Describe("vcs fakes", func() {
	It("stubs GitRunner return values and records calls", func() {
		fake := &vcsfakes.FakeGitRunner{}
		fake.CurrentBranchReturns("main", nil)

		br, err := fake.CurrentBranch("/repo")

		Expect(err).NotTo(HaveOccurred())
		Expect(br).To(Equal("main"))
		Expect(fake.CurrentBranchCallCount()).To(Equal(1))
		Expect(fake.CurrentBranchArgsForCall(0)).To(Equal("/repo"))
	})

	It("stubs GHRunner.PRList", func() {
		fake := &vcsfakes.FakeGHRunner{}
		fake.PRListReturns([]vcs.PR{{Number: 7, State: "OPEN"}}, nil)

		prs, err := fake.PRList("owner/repo", "open")

		Expect(err).NotTo(HaveOccurred())
		Expect(prs).To(HaveLen(1))
		Expect(prs[0].Number).To(Equal(7))
	})

	It("stubs TKRunner.Tickets", func() {
		fake := &vcsfakes.FakeTKRunner{}
		fake.TicketsReturns([]vcs.Ticket{{ID: "her-1", Status: "open"}}, nil)

		tk, err := fake.Tickets("/repo")

		Expect(err).NotTo(HaveOccurred())
		Expect(tk).To(HaveLen(1))
		Expect(tk[0].ID).To(Equal("her-1"))
	})
})
