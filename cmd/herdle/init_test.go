package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"
)

var _ = Describe("herdle init", func() {
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

	skill := func() string { return filepath.Join(home, ".claude", "skills", "herdle-tk-flow", "SKILL.md") }
	skill2 := func() string { return filepath.Join(home, ".claude", "skills", "herdle-tk-artifacts", "SKILL.md") }
	rule := func() string { return filepath.Join(home, ".claude", "rules", "herdle.md") }
	configFile := func() string { return filepath.Join(home, ".config", "herdle", "config.toml") }
	polytokenDir := func() string { return filepath.Join(home, ".config", "polytoken") }
	polytokenSkill := func() string {
		return filepath.Join(polytokenDir(), "skills", "herdle-tk-flow", "SKILL.md")
	}
	polytokenSkill2 := func() string {
		return filepath.Join(polytokenDir(), "skills", "herdle-tk-artifacts", "SKILL.md")
	}
	polytokenContext := func() string { return filepath.Join(polytokenDir(), "herdle.md") }
	polytokenHooks := func() string { return filepath.Join(polytokenDir(), "hooks.json") }
	polytokenAgents := func() string { return filepath.Join(polytokenDir(), "AGENTS.md") }

	// freshApp builds a new app+writer for a follow-up Run in the same scratch HOME.
	freshApp := func() *cli.App {
		buf = &bytes.Buffer{}
		a := newApp()
		a.Writer = buf
		return a
	}

	It("installs skills + rules and seeds an (empty) config on first run", func() {
		Expect(app.Run([]string{"herdle", "init"})).To(Succeed())
		Expect(skill()).To(BeAnExistingFile())
		Expect(skill2()).To(BeAnExistingFile()) // both embedded skills land, not just the first
		Expect(rule()).To(BeAnExistingFile())
		Expect(configFile()).To(BeAnExistingFile())
		Expect(buf.String()).To(ContainSubstring("written"))
		Expect(buf.String()).To(ContainSubstring("seeded"))
	})

	It("is idempotent: a second run skips existing artifacts and seeding", func() {
		Expect(app.Run([]string{"herdle", "init"})).To(Succeed())
		a := freshApp()
		Expect(a.Run([]string{"herdle", "init"})).To(Succeed())
		Expect(buf.String()).To(ContainSubstring("skipped"))
		Expect(buf.String()).To(ContainSubstring("skipped seeding"))
	})

	It("--force overwrites a user-edited artifact", func() {
		Expect(app.Run([]string{"herdle", "init"})).To(Succeed())
		Expect(os.WriteFile(skill(), []byte("user edit"), 0o600)).To(Succeed())
		a := freshApp()
		Expect(a.Run([]string{"herdle", "init", "--force"})).To(Succeed())
		Expect(buf.String()).To(ContainSubstring("overwritten"))
		data, err := os.ReadFile(skill()) // #nosec G304 -- test reads the file it just wrote
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(Equal("user edit")) // re-laid from embed
	})

	It("--uninstall removes artifacts but leaves config and CLAUDE.md", func() {
		Expect(app.Run([]string{"herdle", "init"})).To(Succeed())
		claudeMd := filepath.Join(home, ".claude", "CLAUDE.md")
		Expect(os.WriteFile(claudeMd, []byte("user rules"), 0o600)).To(Succeed())

		a := freshApp()
		Expect(a.Run([]string{"herdle", "init", "--uninstall"})).To(Succeed())
		Expect(skill()).NotTo(BeAnExistingFile())
		Expect(skill2()).NotTo(BeAnExistingFile()) // both embedded skills removed
		Expect(rule()).NotTo(BeAnExistingFile())
		Expect(configFile()).To(BeAnExistingFile()) // config untouched
		Expect(claudeMd).To(BeAnExistingFile())     // CLAUDE.md untouched
	})

	settings := func() string { return filepath.Join(home, ".claude", "settings.json") }

	It("wires the gatekeeper hook into settings.json", func() {
		Expect(app.Run([]string{"herdle", "init"})).To(Succeed())
		b, err := os.ReadFile(settings())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(b)).To(ContainSubstring("hook gatekeeper"))
	})

	It("--uninstall removes the gate from settings.json", func() {
		Expect(app.Run([]string{"herdle", "init"})).To(Succeed())
		a := freshApp()
		Expect(a.Run([]string{"herdle", "init", "--uninstall"})).To(Succeed())
		b, err := os.ReadFile(settings())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(b)).NotTo(ContainSubstring("gatekeeper"))
	})

	It("installs only Polytoken artifacts at the XDG config path", func() {
		xdg := filepath.Join(home, "xdg")
		GinkgoT().Setenv("XDG_CONFIG_HOME", xdg)
		Expect(app.Run([]string{"herdle", "init", "--agent", "polytoken"})).To(Succeed())

		base := filepath.Join(xdg, "polytoken")
		Expect(filepath.Join(base, "skills", "herdle-tk-flow", "SKILL.md")).To(BeAnExistingFile())
		Expect(filepath.Join(base, "skills", "herdle-tk-artifacts", "SKILL.md")).To(BeAnExistingFile())
		Expect(filepath.Join(base, "herdle.md")).To(BeAnExistingFile())
		Expect(filepath.Join(base, "hooks.json")).To(BeAnExistingFile())
		Expect(filepath.Join(base, "AGENTS.md")).To(BeAnExistingFile())
		Expect(skill()).NotTo(BeAnExistingFile())
		Expect(buf.String()).To(ContainSubstring("polytoken: written"))
	})

	It("installs dual harnesses in selected order and seeds config once", func() {
		Expect(app.Run([]string{"herdle", "init", "--agent", "polytoken", "--agent", "claude"})).To(Succeed())
		Expect(polytokenSkill()).To(BeAnExistingFile())
		Expect(skill()).To(BeAnExistingFile())
		Expect(configFile()).To(BeAnExistingFile())
		out := buf.String()
		Expect(strings.Index(out, "polytoken:")).To(BeNumerically("<", strings.Index(out, "claude:")))
		Expect(strings.Count(out, "seeded ")).To(Equal(1))
	})

	It("deduplicates repeated agents", func() {
		Expect(app.Run([]string{"herdle", "init", "--agent", "polytoken", "--agent", "polytoken"})).To(Succeed())
		Expect(polytokenSkill()).To(BeAnExistingFile())
		Expect(strings.Count(buf.String(), "polytoken: written "+polytokenSkill())).To(Equal(1))
	})

	It("rejects every unknown agent before resolving paths or writing", func() {
		GinkgoT().Setenv("HOME", "")
		err := app.Run([]string{"herdle", "init", "--agent", "claude", "--agent", "unknown"})
		Expect(err).To(MatchError(`unknown agent "unknown" (expected claude or polytoken)`))
		Expect(filepath.Join(home, ".claude")).NotTo(BeADirectory())
		Expect(filepath.Join(home, ".config")).NotTo(BeADirectory())
	})

	It("applies force and uninstall to Polytoken without reseeding config", func() {
		Expect(app.Run([]string{"herdle", "init", "--agent", "polytoken"})).To(Succeed())
		Expect(os.WriteFile(polytokenSkill(), []byte("user edit"), 0o600)).To(Succeed())
		a := freshApp()
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--force"})).To(Succeed())
		data, err := os.ReadFile(polytokenSkill())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(Equal("user edit"))

		Expect(os.Remove(configFile())).To(Succeed())
		a = freshApp()
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--uninstall"})).To(Succeed())
		Expect(polytokenSkill()).NotTo(BeAnExistingFile())
		Expect(polytokenSkill2()).NotTo(BeAnExistingFile())
		Expect(polytokenContext()).NotTo(BeAnExistingFile())
		Expect(polytokenHooks()).To(BeAnExistingFile())
		Expect(polytokenAgents()).To(BeAnExistingFile())
		Expect(configFile()).NotTo(BeAnExistingFile())
	})

	It("keeps an earlier install but does not seed config after a later harness fails", func() {
		Expect(os.MkdirAll(filepath.Dir(polytokenDir()), 0o750)).To(Succeed())
		Expect(os.WriteFile(polytokenDir(), []byte("not a directory"), 0o600)).To(Succeed())
		err := app.Run([]string{"herdle", "init", "--agent", "claude", "--agent", "polytoken"})
		Expect(err).To(HaveOccurred())
		Expect(skill()).To(BeAnExistingFile())
		Expect(configFile()).NotTo(BeAnExistingFile())
	})
})
