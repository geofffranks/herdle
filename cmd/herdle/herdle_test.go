package main

import (
	"bytes"

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

		It("errors for the root action", func() {
			err := app.Run([]string{"herdle"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not implemented"))
		})
	})
})
