package main

import (
	"bytes"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"
)

var _ = Describe("herdle CLI", func() {
	var (
		buf *bytes.Buffer
		app *cli.App
	)

	BeforeEach(func() {
		buf = &bytes.Buffer{}
		app = newApp()
		app.Writer = buf
	})

	Describe("version command", func() {
		It("prints the version", func() {
			err := app.Run([]string{"herdle", "version"})
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring(Version))
		})
	})

	Describe("stub commands", func() {
		It("errors with a not-implemented message", func() {
			err := app.Run([]string{"herdle", "doctor"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not implemented"))
		})

		It("routes the no-arg root action inside a repo to the drilldown view", func() {
			// Pin config to an empty file so the test does not read the developer's
			// real ~/.config/herdle/config.toml. The herdle repo (this test's cwd)
			// has no configured entry, so the drilldown synthesizes one for the repo
			// root and renders its header.
			GinkgoT().Setenv("HERDLE_CONFIG", filepath.Join(GinkgoT().TempDir(), "none.toml"))
			err := app.Run([]string{"herdle"})
			Expect(err).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("### herdle"))
		})
	})
})
