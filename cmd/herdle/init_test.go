package main

import (
	"bytes"
	"os"
	"path/filepath"

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

	It("wires the code-review gate into settings.json", func() {
		Expect(app.Run([]string{"herdle", "init"})).To(Succeed())
		b, err := os.ReadFile(settings())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(b)).To(ContainSubstring("hook code-review-gate"))
	})

	It("--uninstall removes the gate from settings.json", func() {
		Expect(app.Run([]string{"herdle", "init"})).To(Succeed())
		a := freshApp()
		Expect(a.Run([]string{"herdle", "init", "--uninstall"})).To(Succeed())
		b, err := os.ReadFile(settings())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(b)).NotTo(ContainSubstring("code-review-gate"))
	})
})
