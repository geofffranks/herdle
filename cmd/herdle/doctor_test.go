package main

import (
	"bytes"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"
)

var _ = Describe("herdle doctor", func() {
	var (
		home string
		buf  *bytes.Buffer
		app  *cli.App
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		os.Unsetenv("XDG_CONFIG_HOME")
		os.Unsetenv("CLAUDE_CONFIG_DIR")
		os.Unsetenv("HERDLE_CONFIG")
		DeferCleanup(func() {
			os.Unsetenv("XDG_CONFIG_HOME")
			os.Unsetenv("CLAUDE_CONFIG_DIR")
			os.Unsetenv("HERDLE_CONFIG")
		})
		buf = &bytes.Buffer{}
		app = newApp()
		app.Writer = buf
	})

	It("prints a checklist and returns an error on an unhealthy setup", func() {
		// Point git/tk at missing binaries so the required checks fail.
		GinkgoT().Setenv("HERDLE_GIT", filepath.Join(home, "nope-git"))
		GinkgoT().Setenv("HERDLE_TK", filepath.Join(home, "nope-tk"))

		err := app.Run([]string{"herdle", "doctor"})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("need attention"))
		out := buf.String()
		Expect(out).To(ContainSubstring("git"))
		Expect(out).To(ContainSubstring("tk"))
		Expect(out).To(ContainSubstring("✗")) // at least one failure glyph
	})
})
