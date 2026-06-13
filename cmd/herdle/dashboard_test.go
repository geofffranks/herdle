package main

import (
	"bytes"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("route", func() {
	// Truth table over (all, name, inRepo). A name always wins (drilldown by
	// name); else --all or being outside a repo => summary; else drilldown here.
	DescribeTable("view selection",
		func(all bool, name string, inRepo bool, want routeKind) {
			Expect(route(all, name, inRepo)).To(Equal(want))
		},
		Entry("named project, outside repo", false, "foo", false, routeDrilldownName),
		Entry("named project beats --all", true, "foo", true, routeDrilldownName),
		Entry("--all inside repo -> summary", true, "", true, routeSummary),
		Entry("outside repo, no name -> summary", false, "", false, routeSummary),
		Entry("inside repo, no name, no --all -> drilldown here", false, "", true, routeDrilldownHere),
	)
})

var _ = Describe("herdle --all (summary)", func() {
	It("prints the summary header + footer with an empty config", func() {
		// HERDLE_CONFIG points at a missing file -> empty config -> zero rows ->
		// no runner calls. Fully hermetic.
		GinkgoT().Setenv("HERDLE_CONFIG", filepath.Join(GinkgoT().TempDir(), "none.toml"))
		buf := &bytes.Buffer{}
		app := newApp()
		app.Writer = buf
		Expect(app.Run([]string{"herdle", "--all"})).To(Succeed())
		Expect(buf.String()).To(ContainSubstring("PROJECT"))
		Expect(buf.String()).To(ContainSubstring(`run "herdle <name>" for detail`))
	})

	It("accepts --fetch alongside --all (footer reflects a fetch)", func() {
		GinkgoT().Setenv("HERDLE_CONFIG", filepath.Join(GinkgoT().TempDir(), "none.toml"))
		buf := &bytes.Buffer{}
		app := newApp()
		app.Writer = buf
		Expect(app.Run([]string{"herdle", "--all", "--fetch"})).To(Succeed())
		Expect(buf.String()).To(ContainSubstring("(fetched)"))
	})

	It("errors on an unknown named project", func() {
		GinkgoT().Setenv("HERDLE_CONFIG", filepath.Join(GinkgoT().TempDir(), "none.toml"))
		buf := &bytes.Buffer{}
		app := newApp()
		app.Writer = buf
		err := app.Run([]string{"herdle", "someproject"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`no project named "someproject"`))
	})
})
