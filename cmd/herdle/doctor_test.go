package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/assets"
	"github.com/geofffranks/herdle/internal/agent"
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
		Expect(out).To(ContainSubstring("claude: skills + rule"))
		Expect(out).NotTo(ContainSubstring("polytoken: skills + context"))
	})

	It("builds the exact selected-harness environment", func() {
		xdg := filepath.Join(home, "xdg")
		GinkgoT().Setenv("XDG_CONFIG_HOME", xdg)
		env, err := buildDoctorEnv([]agent.Name{agent.Polytoken, agent.Claude})
		Expect(err).NotTo(HaveOccurred())
		exe, err := os.Executable()
		Expect(err).NotTo(HaveOccurred())
		Expect(env.Agents).To(Equal([]agent.Name{agent.Polytoken, agent.Claude}))
		Expect(env.ClaudeAssets).To(Equal(assets.ClaudeFS))
		Expect(env.PolytokenAssets).To(Equal(assets.PolytokenFS))
		Expect(env.ClaudeDir).To(Equal(filepath.Join(home, ".claude")))
		Expect(env.PolytokenDir).To(Equal(filepath.Join(xdg, "polytoken")))
		Expect(env.PolytokenHooksPath).To(Equal(filepath.Join(xdg, "polytoken", "hooks.json")))
		Expect(env.PolytokenCommand).To(Equal(exe + " hook gatekeeper --agent polytoken"))
	})

	It("renders only Polytoken harness rows when selected", func() {
		err := app.Run([]string{"herdle", "doctor", "--agent", "polytoken"})
		Expect(err).To(HaveOccurred())
		out := buf.String()
		Expect(out).To(ContainSubstring("polytoken: skills + context"))
		Expect(out).NotTo(ContainSubstring("claude: skills + rule"))
		Expect(out).NotTo(ContainSubstring("superpowers"))
	})

	It("renders dual harness rows in order and common rows once", func() {
		err := app.Run([]string{"herdle", "doctor", "--agent", "polytoken", "--agent", "claude"})
		Expect(err).To(HaveOccurred())
		out := buf.String()
		Expect(strings.Index(out, "polytoken: skills + context")).To(BeNumerically("<", strings.Index(out, "claude: skills + rule")))
		Expect(strings.Count(out, "git ")).To(Equal(1))
	})

	It("deduplicates repeated doctor agents", func() {
		err := app.Run([]string{"herdle", "doctor", "--agent", "polytoken", "--agent", "polytoken"})
		Expect(err).To(HaveOccurred())
		Expect(strings.Count(buf.String(), "polytoken: skills + context")).To(Equal(1))
	})

	It("rejects an unknown doctor agent before rendering checks", func() {
		err := app.Run([]string{"herdle", "doctor", "--agent", "unknown"})
		Expect(err).To(MatchError(`unknown agent "unknown" (expected claude or polytoken)`))
		Expect(buf.String()).To(BeEmpty())
	})
})
