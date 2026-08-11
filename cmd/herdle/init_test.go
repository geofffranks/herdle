package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/internal/config"
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

	appWithInitDeps := func(deps initDependencies) *cli.App {
		buf = &bytes.Buffer{}
		a := newApp()
		a.Writer = buf
		for i, command := range a.Commands {
			if command.Name == "init" {
				a.Commands[i] = initCommandWithDependencies(deps)
				break
			}
		}
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

	It("keeps explicit global Polytoken behavior", func() {
		Expect(app.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "global"})).To(Succeed())
		Expect(polytokenSkill()).To(BeAnExistingFile())
		Expect(configFile()).To(BeAnExistingFile())
	})

	It("defines installation scope metadata", func() {
		command := initCommand()
		var scope *cli.StringFlag
		for _, flag := range command.Flags {
			if candidate, ok := flag.(*cli.StringFlag); ok && candidate.Name == "scope" {
				scope = candidate
				break
			}
		}
		Expect(scope).NotTo(BeNil())
		Expect(scope.Value).To(Equal("global"))
		Expect(scope.Usage).To(MatchRegexp(`(?i)installation scope`))
		Expect(scope.Usage).To(MatchRegexp(`global.*project|project.*global`))
	})

	DescribeTable("rejects unsupported project scope before path resolution or writes",
		func(args []string, expected string) {
			called := false
			deps := defaultInitDependencies()
			deps.getwd = func() (string, error) {
				called = true
				return "", errors.New("cwd sentinel")
			}
			deps.loadConfig = func() (*config.Config, error) {
				called = true
				return nil, errors.New("load sentinel")
			}
			a := appWithInitDeps(deps)
			err := a.Run(append([]string{"herdle", "init"}, args...))
			Expect(err).To(MatchError(expected))
			Expect(called).To(BeFalse())
			Expect(filepath.Join(home, ".claude")).NotTo(BeADirectory())
			Expect(filepath.Join(home, ".config")).NotTo(BeADirectory())
		},
		Entry("unknown scope", []string{"--agent", "polytoken", "--scope", "workspace"}, `unknown scope "workspace" (expected global or project)`),
		Entry("implicit Claude", []string{"--scope", "project"}, `project scope requires explicit exclusive --agent polytoken`),
		Entry("explicit Claude", []string{"--agent", "claude", "--scope", "project"}, `project scope requires explicit exclusive --agent polytoken`),
		Entry("multiple agents", []string{"--agent", "polytoken", "--agent", "claude", "--scope", "project"}, `project scope requires explicit exclusive --agent polytoken`),
	)

	It("accepts repeated explicit Polytoken selection for project scope", func() {
		project := GinkgoT().TempDir()
		deps := defaultInitDependencies()
		deps.getwd = func() (string, error) { return project, nil }
		a := appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--agent", "polytoken", "--scope", "project"})).To(Succeed())
		Expect(strings.Count(buf.String(), "polytoken: written "+filepath.Join(project, ".polytoken", "skills", "herdle-tk-flow", "SKILL.md"))).To(Equal(1))
	})

	It("installs project Polytoken into the canonical exact cwd and registers it", func() {
		physical := GinkgoT().TempDir()
		project := filepath.Join(physical, "project")
		Expect(os.Mkdir(project, 0o750)).To(Succeed())
		alias := filepath.Join(GinkgoT().TempDir(), "alias")
		Expect(os.Symlink(project, alias)).To(Succeed())
		var canonicalized string
		deps := defaultInitDependencies()
		deps.getwd = func() (string, error) { return filepath.Join(alias, "."), nil }
		deps.canonicalProjectPath = func(path string) (string, error) {
			var err error
			canonicalized, err = config.CanonicalProjectPath(path)
			return canonicalized, err
		}
		a := appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project"})).To(Succeed())
		Expect(canonicalized).To(Equal(project))
		Expect(filepath.Join(project, ".polytoken", "skills", "herdle-tk-flow", "SKILL.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, ".polytoken", "skills", "herdle-tk-artifacts", "SKILL.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, ".polytoken", "herdle.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, ".polytoken", "hooks.json")).To(BeAnExistingFile())
		agents, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(agents)).To(Equal("<!-- herdle:begin -->\n@.polytoken/herdle.md\n<!-- herdle:end -->\n"))
		cfg, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Projects).To(Equal([]config.Project{{Path: project, Polytoken: true}}))
		Expect(polytokenSkill()).NotTo(BeAnExistingFile())
	})

	It("fails project init before writes when cwd canonicalization fails", func() {
		deps := defaultInitDependencies()
		deps.getwd = func() (string, error) { return "/missing", nil }
		deps.canonicalProjectPath = func(string) (string, error) { return "", errors.New("canonical sentinel") }
		a := appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project"})).To(MatchError("canonical sentinel"))
		Expect(configFile()).NotTo(BeAnExistingFile())
		Expect(polytokenSkill()).NotTo(BeAnExistingFile())
	})

	It("repeats project install idempotently and forces owned refresh", func() {
		project := GinkgoT().TempDir()
		deps := defaultInitDependencies()
		deps.getwd = func() (string, error) { return project, nil }
		a := appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project"})).To(Succeed())
		docPath := filepath.Join(project, ".polytoken", "herdle.md")
		agentsPath := filepath.Join(project, "AGENTS.md")
		Expect(os.WriteFile(docPath, []byte("stale"), 0o600)).To(Succeed())
		Expect(os.Chmod(docPath, 0o600)).To(Succeed())
		foreign := "# Mine\n\n<!-- herdle:begin -->\n@.polytoken/herdle.md\n<!-- herdle:end -->\n"
		Expect(os.WriteFile(agentsPath, []byte(foreign), 0o640)).To(Succeed())
		Expect(os.Chmod(agentsPath, 0o640)).To(Succeed())

		a = appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project"})).To(Succeed())
		contents, err := os.ReadFile(docPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("stale"))
		Expect(strings.Count(buf.String(), "polytoken: skipped "+agentsPath)).To(Equal(1))

		a = appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project", "--force"})).To(Succeed())
		contents, err = os.ReadFile(docPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).NotTo(Equal("stale"))
		docInfo, err := os.Stat(docPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(docInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))
		agents, err := os.ReadFile(agentsPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(agents)).To(Equal(foreign))
		agentsInfo, err := os.Stat(agentsPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(agentsInfo.Mode().Perm()).To(Equal(os.FileMode(0o640)))
		cfg, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Projects).To(Equal([]config.Project{{Path: project, Polytoken: true}}))
	})

	It("does not register after project artifact failure", func() {
		project := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(project, ".polytoken"), []byte("not a directory"), 0o600)).To(Succeed())
		deps := defaultInitDependencies()
		deps.getwd = func() (string, error) { return project, nil }
		a := appWithInitDeps(deps)
		err := a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project"})
		Expect(err).To(MatchError(ContainSubstring(filepath.Join(project, ".polytoken"))))
		Expect(err).NotTo(MatchError("project-scoped Polytoken artifacts are not implemented"))
		Expect(configFile()).NotTo(BeAnExistingFile())
	})

	It("reports project config save failure after artifacts and repairs on rerun", func() {
		project := GinkgoT().TempDir()
		deps := defaultInitDependencies()
		deps.getwd = func() (string, error) { return project, nil }
		deps.saveConfig = func(*config.Config) error { return errors.New("save sentinel") }
		a := appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project"})).To(MatchError("save sentinel"))
		Expect(filepath.Join(project, ".polytoken", "herdle.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, "AGENTS.md")).To(BeAnExistingFile())
		Expect(configFile()).NotTo(BeAnExistingFile())

		deps = defaultInitDependencies()
		deps.getwd = func() (string, error) { return project, nil }
		a = appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project"})).To(Succeed())
		cfg, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg.Projects).To(Equal([]config.Project{{Path: project, Polytoken: true}}))
	})

	DescribeTable("uninstalls registered project Polytoken state",
		func(existing config.Project, preservesEntry bool) {
			project := GinkgoT().TempDir()
			existing.Path = project
			cfg := &config.Config{Projects: []config.Project{existing}}
			Expect(cfg.Save()).To(Succeed())
			deps := defaultInitDependencies()
			deps.getwd = func() (string, error) { return project, nil }
			a := appWithInitDeps(deps)
			Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project"})).To(Succeed())
			agentsPath := filepath.Join(project, "AGENTS.md")
			managed := "<!-- herdle:begin -->\n@.polytoken/herdle.md\n<!-- herdle:end -->\n"
			Expect(os.WriteFile(agentsPath, []byte("# Mine\n\n"+managed), 0o640)).To(Succeed())

			a = appWithInitDeps(deps)
			Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project", "--uninstall"})).To(Succeed())
			Expect(filepath.Join(project, ".polytoken", "herdle.md")).NotTo(BeAnExistingFile())
			contents, err := os.ReadFile(agentsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("# Mine\n\n"))
			loaded, err := config.Load()
			Expect(err).NotTo(HaveOccurred())
			if preservesEntry {
				Expect(loaded.Projects).To(Equal([]config.Project{{Path: project, Slug: existing.Slug}}))
			} else {
				Expect(loaded.Projects).To(BeEmpty())
			}
		},
		Entry("removes an empty path-only entry", config.Project{}, false),
		Entry("preserves a metadata-bearing entry", config.Project{Slug: "owner/app"}, true),
	)

	DescribeTable("unregistered project uninstall with missing config does not create config",
		func(withArtifacts bool) {
			project := GinkgoT().TempDir()
			if withArtifacts {
				Expect(os.Mkdir(filepath.Join(project, ".polytoken"), 0o750)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(project, ".polytoken", "herdle.md"), []byte("recognizable"), 0o600)).To(Succeed())
			}
			deps := defaultInitDependencies()
			deps.getwd = func() (string, error) { return project, nil }
			a := appWithInitDeps(deps)
			Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project", "--uninstall"})).To(Succeed())
			Expect(filepath.Join(project, ".polytoken", "herdle.md")).NotTo(BeAnExistingFile())
			Expect(configFile()).NotTo(BeAnExistingFile())
		},
		Entry("without artifacts", false),
		Entry("with recognizable artifacts", true),
	)

	It("unregistered project uninstall without a matching entry does not rewrite config", func() {
		project := GinkgoT().TempDir()
		original := []byte("[[project]]\npath = '/other/app'\npolytoken = true\n")
		Expect(os.MkdirAll(filepath.Dir(configFile()), 0o750)).To(Succeed())
		Expect(os.WriteFile(configFile(), original, 0o600)).To(Succeed())
		saves := 0
		deps := defaultInitDependencies()
		deps.getwd = func() (string, error) { return project, nil }
		deps.saveConfig = func(*config.Config) error { saves++; return nil }
		a := appWithInitDeps(deps)
		Expect(a.Run([]string{"herdle", "init", "--agent", "polytoken", "--scope", "project", "--uninstall"})).To(Succeed())
		contents, err := os.ReadFile(configFile())
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(Equal(original))
		Expect(saves).To(BeZero())
	})

	Describe("project config persistence", func() {
		It("upserts and reports injected save failure", func() {
			cfg := &config.Config{}
			deps := defaultInitDependencies()
			deps.loadConfig = func() (*config.Config, error) { return cfg, nil }
			deps.saveConfig = func(*config.Config) error { return errors.New("save sentinel") }
			Expect(updateProjectConfig("/work/app", false, deps)).To(MatchError("save sentinel"))
			Expect(cfg.Projects).To(Equal([]config.Project{{Path: "/work/app", Polytoken: true}}))
		})

		It("clears metadata-bearing state and saves once", func() {
			cfg := &config.Config{Projects: []config.Project{{Path: "/work/app", Slug: "owner/app", Polytoken: true}}}
			saves := 0
			deps := defaultInitDependencies()
			deps.loadConfig = func() (*config.Config, error) { return cfg, nil }
			deps.saveConfig = func(*config.Config) error { saves++; return nil }
			Expect(updateProjectConfig("/work/app", true, deps)).To(Succeed())
			Expect(cfg.Projects).To(Equal([]config.Project{{Path: "/work/app", Slug: "owner/app"}}))
			Expect(saves).To(Equal(1))
		})

		DescribeTable("unregistered uninstall does not save config",
			func(cfg *config.Config) {
				saves := 0
				deps := defaultInitDependencies()
				deps.loadConfig = func() (*config.Config, error) { return cfg, nil }
				deps.saveConfig = func(*config.Config) error { saves++; return nil }
				Expect(updateProjectConfig("/work/app", true, deps)).To(Succeed())
				Expect(saves).To(Equal(0))
			},
			Entry("empty config", &config.Config{}),
			Entry("config without exact match", &config.Config{Projects: []config.Project{{Path: "/other/app", Polytoken: true}}}),
			Entry("matching project without state", &config.Config{Projects: []config.Project{{Path: "/work/app", Slug: "owner/app"}}}),
		)
	})
})
