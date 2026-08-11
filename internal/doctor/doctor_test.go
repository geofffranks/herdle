package doctor_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/agent"
	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/doctor"
	"github.com/geofffranks/herdle/internal/initcmd"
	"github.com/geofffranks/herdle/internal/vcs/vcsfakes"
)

// find returns the result with the given Name, failing the spec if absent.
func find(rs []doctor.Result, name string) doctor.Result {
	for _, r := range rs {
		if r.Name == name {
			return r
		}
	}
	Fail("no result named " + name)
	return doctor.Result{}
}

// goodEnv builds an Env in which every check passes. Individual specs mutate one
// aspect to drive a single check to Warn/Fail.
func goodEnv() doctor.Env {
	claude := GinkgoT().TempDir()
	assetsFS := fstest.MapFS{
		"skills/herdle-tk-flow/SKILL.md": {Data: []byte("flow\n")},
		"rules/herdle.md":                {Data: []byte("rule\n")},
	}
	for p, f := range assetsFS {
		dest := filepath.Join(claude, filepath.FromSlash(p))
		Expect(os.MkdirAll(filepath.Dir(dest), 0o750)).To(Succeed())
		Expect(os.WriteFile(dest, f.Data, 0o600)).To(Succeed())
	}
	Expect(os.MkdirAll(filepath.Join(claude, "plugins", "cache", "mk", "superpowers"), 0o750)).To(Succeed())

	cfgPath := filepath.Join(GinkgoT().TempDir(), "config.toml")
	Expect(os.WriteFile(cfgPath, []byte("[[project]]\npath = \"/x\"\n"), 0o600)).To(Succeed())

	settingsPath := filepath.Join(claude, "settings.json")
	Expect(os.WriteFile(settingsPath, []byte(`{"hooks":{"PreToolUse":[{"matcher":"Edit|Write|Bash","hooks":[{"type":"command","command":"/x/herdle hook gatekeeper"}]}]}}`), 0o600)).To(Succeed())

	binDir := GinkgoT().TempDir()

	git := &vcsfakes.FakeGitRunner{}
	git.AvailableReturns(true)
	gh := &vcsfakes.FakeGHRunner{}
	gh.AvailableReturns(true)
	gh.AuthenticatedReturns(true)
	gl := &vcsfakes.FakeGLRunner{}
	gl.AvailableReturns(true)
	gl.AuthenticatedReturns(true)
	tk := &vcsfakes.FakeTKRunner{}
	tk.AvailableReturns(true)

	return doctor.Env{
		Git: git, GH: gh, GL: gl, TK: tk,
		Assets:       assetsFS,
		ClaudeDir:    claude,
		ConfigPath:   cfgPath,
		SettingsPath: settingsPath,
		ExecPath:     filepath.Join(binDir, "herdle"),
		PathDirs:     []string{binDir},
	}
}

func withHealthyPolytoken(env doctor.Env) doctor.Env {
	polytoken := GinkgoT().TempDir()
	polytokenFS := fstest.MapFS{
		"herdle.md":                           {Data: []byte("context\n")},
		"skills/herdle-tk-flow/SKILL.md":      {Data: []byte("flow\n")},
		"skills/herdle-tk-artifacts/SKILL.md": {Data: []byte("artifacts\n")},
	}
	command := filepath.Join(filepath.Dir(env.ExecPath), "herdle") + " hook gatekeeper"
	_, err := initcmd.InstallPolytoken(polytokenFS, polytoken, command, false)
	Expect(err).NotTo(HaveOccurred())
	env.PolytokenAssets = polytokenFS
	env.PolytokenDir = polytoken
	env.PolytokenHooksPath = filepath.Join(polytoken, "hooks.json")
	env.PolytokenCommand = command
	return env
}

func names(rs []doctor.Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

func installProjectPolytoken(env doctor.Env, project string) {
	layout := initcmd.PolytokenLayout{
		StandaloneDir:  filepath.Join(project, ".polytoken"),
		HooksPath:      filepath.Join(project, ".polytoken", "hooks.json"),
		ContextPath:    filepath.Join(project, "AGENTS.md"),
		ContextInclude: "@.polytoken/herdle.md",
	}
	_, err := initcmd.InstallPolytokenLayout(env.PolytokenAssets, layout, env.PolytokenCommand, false)
	Expect(err).NotTo(HaveOccurred())
}

func saveDoctorConfig(env doctor.Env, projects ...config.Project) {
	Expect((&config.Config{Projects: projects}).SaveTo(env.ConfigPath)).To(Succeed())
}

var _ = Describe("doctor harness composition", func() {
	It("defaults an empty selection to Claude and runs common checks once", func() {
		Expect(names(doctor.Run(goodEnv()))).To(Equal([]string{
			"git", "tk", "gh", "gh auth", "glab", "glab auth", "herdle on PATH", "config",
			"superpowers", "claude: skills + rule", "claude: lifecycle gatekeeper",
		}))
	})

	It("runs only Polytoken harness checks without scanning Claude plugins", func() {
		env := withHealthyPolytoken(goodEnv())
		env.Agents = []agent.Name{agent.Polytoken}
		Expect(os.RemoveAll(filepath.Join(env.ClaudeDir, "plugins"))).To(Succeed())
		Expect(names(doctor.Run(env))).To(Equal([]string{
			"git", "tk", "gh", "gh auth", "glab", "glab auth", "herdle on PATH", "config",
			"polytoken: skills + context", "polytoken: AGENTS.md link", "polytoken: lifecycle gatekeeper",
			"polytoken: installation scopes",
		}))
	})

	It("appends dual harness rows in selected order while common rows remain singular", func() {
		env := withHealthyPolytoken(goodEnv())
		env.Agents = []agent.Name{agent.Polytoken, agent.Claude}
		Expect(names(doctor.Run(env))).To(Equal([]string{
			"git", "tk", "gh", "gh auth", "glab", "glab auth", "herdle on PATH", "config",
			"polytoken: skills + context", "polytoken: AGENTS.md link", "polytoken: lifecycle gatekeeper",
			"polytoken: installation scopes",
			"superpowers", "claude: skills + rule", "claude: lifecycle gatekeeper",
		}))
	})
})

var _ = Describe("doctor Polytoken diagnostics", func() {
	var env doctor.Env

	BeforeEach(func() {
		env = withHealthyPolytoken(goodEnv())
		env.Agents = []agent.Name{agent.Polytoken}
	})

	It("reports every healthy Polytoken row OK", func() {
		rs := doctor.Run(env)
		for _, name := range []string{"polytoken: installation scopes", "polytoken: skills + context", "polytoken: AGENTS.md link", "polytoken: lifecycle gatekeeper"} {
			Expect(find(rs, name).Status).To(Equal(doctor.OK), name)
		}
	})

	Describe("validates registered Polytoken candidates", func() {
		It("reports scope-qualified rows for every healthy registered project", func() {
			first := GinkgoT().TempDir()
			second := GinkgoT().TempDir()
			installProjectPolytoken(env, first)
			installProjectPolytoken(env, second)
			saveDoctorConfig(env,
				config.Project{Path: first, Polytoken: true},
				config.Project{Path: second, Polytoken: true},
			)

			rs := doctor.Run(env)
			for _, project := range []string{first, second} {
				for _, component := range []string{"skills + context", "AGENTS.md link", "lifecycle gatekeeper"} {
					name := "polytoken: project " + project + ": " + component
					Expect(find(rs, name).Status).To(Equal(doctor.OK), name)
				}
			}
		})

		It("fails all integrity rows for a missing registered project with exact remediation", func() {
			project := filepath.Join(GinkgoT().TempDir(), "missing")
			saveDoctorConfig(env, config.Project{Path: project, Polytoken: true})
			command := "cd '" + project + "' && herdle init --agent polytoken --scope project"

			rs := doctor.Run(env)
			for _, component := range []string{"skills + context", "AGENTS.md link", "lifecycle gatekeeper"} {
				r := find(rs, "polytoken: project "+project+": "+component)
				Expect(r.Status).NotTo(Equal(doctor.OK), component)
				Expect(r.Detail).To(ContainSubstring(project))
				Expect(r.Remediation).To(ContainSubstring(command))
			}
		})

		DescribeTable("recognizes malformed global ownership signatures and reports integrity failure",
			func(path, contents string) {
				Expect(os.RemoveAll(env.PolytokenDir)).To(Succeed())
				Expect(os.MkdirAll(env.PolytokenDir, 0o750)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(env.PolytokenDir, path), []byte(contents), 0o600)).To(Succeed())

				rs := doctor.Run(env)
				Expect(find(rs, "polytoken: skills + context").Detail).NotTo(ContainSubstring("not present"))
				if path == "AGENTS.md" {
					Expect(find(rs, "polytoken: AGENTS.md link").Status).To(Equal(doctor.Fail))
				} else {
					Expect(find(rs, "polytoken: lifecycle gatekeeper").Status).To(Equal(doctor.Fail))
				}
			},
			Entry("context marker", "AGENTS.md", "<!-- herdle:begin -->\n"),
			Entry("named hook", "hooks.json", `[{"name":"herdle-gatekeeper"`),
		)

		DescribeTable("recognizes every partial global signature and reports missing integrity",
			func(signature string) {
				Expect(os.RemoveAll(env.PolytokenDir)).To(Succeed())
				Expect(os.MkdirAll(env.PolytokenDir, 0o750)).To(Succeed())
				switch signature {
				case "standalone":
					Expect(os.WriteFile(filepath.Join(env.PolytokenDir, "herdle.md"), []byte("context\n"), 0o600)).To(Succeed())
				case "context":
					Expect(os.WriteFile(filepath.Join(env.PolytokenDir, "AGENTS.md"), []byte("<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\n"), 0o600)).To(Succeed())
				case "hook":
					contents := `[{"name":"herdle-gatekeeper","event":"pre_tool_use","matcher":"*","handler":{"bash":"` + env.PolytokenCommand + `"}}]`
					Expect(os.WriteFile(env.PolytokenHooksPath, []byte(contents), 0o600)).To(Succeed())
				}

				rs := doctor.Run(env)
				Expect([]doctor.Status{
					find(rs, "polytoken: skills + context").Status,
					find(rs, "polytoken: AGENTS.md link").Status,
					find(rs, "polytoken: lifecycle gatekeeper").Status,
				}).To(ContainElement(doctor.Fail))
			},
			Entry("standalone file", "standalone"),
			Entry("managed context marker", "context"),
			Entry("named hook", "hook"),
		)
	})

	Describe("checks registry-wide Polytoken scope conflicts", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(env.PolytokenDir)).To(Succeed())
		})

		DescribeTable("classifies project scope overlap",
			func(first, second string, conflict bool) {
				root := GinkgoT().TempDir()
				paths := []string{filepath.Join(root, first), filepath.Join(root, second)}
				projects := make([]config.Project, len(paths))
				for i, path := range paths {
					projects[i] = config.Project{Path: path, Polytoken: true}
				}
				saveDoctorConfig(env, projects...)
				r := find(doctor.Run(env), "polytoken: installation scopes")
				if conflict {
					Expect(r.Status).To(Equal(doctor.Fail))
					for _, path := range paths {
						Expect(r.Detail).To(ContainSubstring(path))
						Expect(r.Remediation).To(ContainSubstring("cd '" + path + "' && herdle init --agent polytoken --scope project --uninstall"))
					}
				} else {
					Expect(r.Status).To(Equal(doctor.OK))
				}
			},
			Entry("equal", "same", "same", true),
			Entry("ancestor and descendant", ".", "child", true),
			Entry("siblings", "one", "two", false),
			Entry("prefix lookalikes", "app", "app-two", false),
		)

		It("fails global plus any registered project", func() {
			env = withHealthyPolytoken(env)
			project := filepath.Join(GinkgoT().TempDir(), "registered")
			saveDoctorConfig(env, config.Project{Path: project, Polytoken: true})

			r := find(doctor.Run(env), "polytoken: installation scopes")
			Expect(r.Status).To(Equal(doctor.Fail))
			Expect(r.Detail).To(ContainSubstring(env.PolytokenDir))
			Expect(r.Detail).To(ContainSubstring(project))
			Expect(r.Remediation).To(ContainSubstring("herdle init --agent polytoken --scope global --uninstall"))
			Expect(r.Remediation).To(ContainSubstring("cd '" + project + "' && herdle init --agent polytoken --scope project --uninstall"))
		})
	})

	Describe("diagnoses unresolved registered project paths", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(env.PolytokenDir)).To(Succeed())
		})

		DescribeTable("retains a deterministic best-effort identity",
			func(kind string) {
				root := GinkgoT().TempDir()
				project := filepath.Join(root, "project")
				if kind == "missing parent" {
					project = filepath.Join(root, "missing", "project")
				}
				if kind == "broken symlink" {
					link := filepath.Join(root, "broken")
					Expect(os.Symlink(filepath.Join(root, "absent"), link)).To(Succeed())
					project = filepath.Join(link, "project")
				}
				saveDoctorConfig(env, config.Project{Path: project, Polytoken: true})
				r := find(doctor.Run(env), "polytoken: project "+project+": path identity")
				Expect(r.Status).To(Equal(doctor.Fail))
				Expect(r.Detail).To(ContainSubstring(project))
				Expect(r.Remediation).To(ContainSubstring("cd '" + project + "' && herdle init --agent polytoken --scope project"))
			},
			Entry("missing parent", "missing parent"),
			Entry("missing child", "missing child"),
			Entry("broken symlink", "broken symlink"),
		)

		It("appends an unresolved suffix to the longest physically resolved ancestor", func() {
			physical := GinkgoT().TempDir()
			alias := filepath.Join(GinkgoT().TempDir(), "alias")
			Expect(os.Symlink(physical, alias)).To(Succeed())
			stored := filepath.Join(alias, "missing", "project")
			identity := filepath.Join(physical, "missing", "project")
			saveDoctorConfig(env, config.Project{Path: stored, Polytoken: true})

			r := find(doctor.Run(env), "polytoken: project "+identity+": path identity")
			Expect(r.Detail).To(ContainSubstring(identity))
			Expect(r.Remediation).To(ContainSubstring("cd '" + identity + "'"))
		})

		It("uses an unresolved identity in overlap checks against an existing path", func() {
			root := GinkgoT().TempDir()
			missing := filepath.Join(root, "missing")
			saveDoctorConfig(env,
				config.Project{Path: root, Polytoken: true},
				config.Project{Path: missing, Polytoken: true},
			)
			r := find(doctor.Run(env), "polytoken: installation scopes")
			Expect(r.Status).To(Equal(doctor.Fail))
			Expect(r.Detail).To(ContainSubstring(root))
			Expect(r.Detail).To(ContainSubstring(missing))
		})
	})

	Describe("detects unregistered current-cwd drift", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(env.PolytokenDir)).To(Succeed())
		})

		It("fails a recognizable unregistered current project install", func() {
			project := GinkgoT().TempDir()
			installProjectPolytoken(env, project)
			env.CWD = project
			saveDoctorConfig(env)

			r := find(doctor.Run(env), "polytoken: unregistered project "+project)
			Expect(r.Status).To(Equal(doctor.Fail))
			Expect(r.Detail).To(ContainSubstring(project))
			Expect(r.Remediation).To(ContainSubstring("cd '" + project + "' && herdle init --agent polytoken --scope project"))
			Expect(r.Remediation).To(ContainSubstring("cd '" + project + "' && herdle init --agent polytoken --scope project --uninstall"))
		})

		It("does not inspect recognizable unregistered paths outside cwd", func() {
			elsewhere := GinkgoT().TempDir()
			installProjectPolytoken(env, elsewhere)
			env.CWD = GinkgoT().TempDir()
			saveDoctorConfig(env, config.Project{Path: elsewhere})

			rs := doctor.Run(env)
			Expect(names(rs)).NotTo(ContainElement("polytoken: unregistered project " + elsewhere))
			Expect(doctor.Failed(rs)).To(BeFalse())
		})

		It("does not flag cwd when its canonical identity is registered", func() {
			project := GinkgoT().TempDir()
			alias := filepath.Join(GinkgoT().TempDir(), "alias")
			Expect(os.Symlink(project, alias)).To(Succeed())
			installProjectPolytoken(env, project)
			env.CWD = alias
			saveDoctorConfig(env, config.Project{Path: project, Polytoken: true})

			rs := doctor.Run(env)
			Expect(names(rs)).NotTo(ContainElement("polytoken: unregistered project " + project))
		})
	})

	It("fails missing standalone content with the exact init command", func() {
		Expect(os.Remove(filepath.Join(env.PolytokenDir, "herdle.md"))).To(Succeed())
		r := find(doctor.Run(env), "polytoken: skills + context")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Remediation).To(Equal("herdle init --agent polytoken"))
	})

	It("warns on drifted standalone content with the exact force command", func() {
		Expect(os.WriteFile(filepath.Join(env.PolytokenDir, "herdle.md"), []byte("drifted\n"), 0o600)).To(Succeed())
		r := find(doctor.Run(env), "polytoken: skills + context")
		Expect(r.Status).To(Equal(doctor.Warn))
		Expect(r.Remediation).To(Equal("herdle init --agent polytoken --force"))
	})

	It("fails a missing AGENTS.md link with the exact init command", func() {
		Expect(os.Remove(filepath.Join(env.PolytokenDir, "AGENTS.md"))).To(Succeed())
		r := find(doctor.Run(env), "polytoken: AGENTS.md link")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Remediation).To(Equal("herdle init --agent polytoken"))
	})

	DescribeTable("fails malformed or duplicate AGENTS.md state through the shared inspector",
		func(contents string) {
			path := filepath.Join(env.PolytokenDir, "AGENTS.md")
			Expect(os.WriteFile(path, []byte(contents), 0o600)).To(Succeed())
			r := find(doctor.Run(env), "polytoken: AGENTS.md link")
			Expect(r.Status).To(Equal(doctor.Fail))
			Expect(r.Detail).To(ContainSubstring(path))
			Expect(r.Remediation).To(Equal("repair " + path + ", then run: herdle init --agent polytoken"))
		},
		Entry("malformed", "<!-- herdle:begin -->\n"),
		Entry("duplicate", "<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\n<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\n"),
	)

	It("fails a missing lifecycle hook with the exact init command", func() {
		Expect(os.WriteFile(env.PolytokenHooksPath, []byte("[]\n"), 0o600)).To(Succeed())
		r := find(doctor.Run(env), "polytoken: lifecycle gatekeeper")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Remediation).To(Equal("herdle init --agent polytoken"))
	})

	DescribeTable("fails malformed or duplicate hook state through the shared inspector",
		func(contents string) {
			Expect(os.WriteFile(env.PolytokenHooksPath, []byte(contents), 0o600)).To(Succeed())
			r := find(doctor.Run(env), "polytoken: lifecycle gatekeeper")
			Expect(r.Status).To(Equal(doctor.Fail))
			Expect(r.Detail).To(ContainSubstring(env.PolytokenHooksPath))
			Expect(r.Remediation).To(Equal("repair " + env.PolytokenHooksPath + ", then run: herdle init --agent polytoken"))
		},
		Entry("malformed", "["),
		Entry("duplicate", `[{"name":"herdle-gatekeeper","event":"pre_tool_use","matcher":"*","handler":{"bash":"one"}},{"name":"herdle-gatekeeper","event":"pre_tool_use","matcher":"*","handler":{"bash":"two"}}]`),
	)

	DescribeTable("fails stale hook fields with the exact init command",
		func(event, matcher, command string) {
			contents := `[{"name":"herdle-gatekeeper","event":"` + event + `","matcher":"` + matcher + `","handler":{"bash":"` + command + `"}}]`
			Expect(os.WriteFile(env.PolytokenHooksPath, []byte(contents), 0o600)).To(Succeed())
			r := find(doctor.Run(env), "polytoken: lifecycle gatekeeper")
			Expect(r.Status).To(Equal(doctor.Fail))
			Expect(r.Detail).To(ContainSubstring("stale"))
			Expect(r.Remediation).To(Equal("herdle init --agent polytoken"))
		},
		Entry("event", "PreToolUse", "*", "current"),
		Entry("matcher", "pre_tool_use", "Bash", "current"),
		Entry("command", "pre_tool_use", "*", "stale"),
	)
})

var _ = Describe("doctor core (git/tk)", func() {
	It("reports git and tk OK when available", func() {
		rs := doctor.Run(goodEnv())
		Expect(find(rs, "git").Status).To(Equal(doctor.OK))
		Expect(find(rs, "tk").Status).To(Equal(doctor.OK))
	})

	It("reports git Fail with remediation when unavailable", func() {
		env := goodEnv()
		env.Git.(*vcsfakes.FakeGitRunner).AvailableReturns(false)
		r := find(doctor.Run(env), "git")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Remediation).To(ContainSubstring("brew install git"))
	})

	It("reports tk Fail with remediation when unavailable", func() {
		env := goodEnv()
		env.TK.(*vcsfakes.FakeTKRunner).AvailableReturns(false)
		r := find(doctor.Run(env), "tk")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Remediation).To(ContainSubstring("ticket"))
	})

	It("Failed is false when every check is OK, true on any non-OK", func() {
		Expect(doctor.Failed(doctor.Run(goodEnv()))).To(BeFalse())
		env := goodEnv()
		env.Git.(*vcsfakes.FakeGitRunner).AvailableReturns(false)
		Expect(doctor.Failed(doctor.Run(env))).To(BeTrue())
	})
})

var _ = Describe("doctor gh + gh auth", func() {
	It("gh OK + auth OK when available and authenticated", func() {
		rs := doctor.Run(goodEnv())
		Expect(find(rs, "gh").Status).To(Equal(doctor.OK))
		Expect(find(rs, "gh auth").Status).To(Equal(doctor.OK))
	})

	It("gh Warn when absent, and auth row skipped-OK (not double-counted)", func() {
		env := goodEnv()
		env.GH.(*vcsfakes.FakeGHRunner).AvailableReturns(false)
		rs := doctor.Run(env)
		Expect(find(rs, "gh").Status).To(Equal(doctor.Warn))
		Expect(find(rs, "gh").Remediation).To(ContainSubstring("brew install gh"))
		auth := find(rs, "gh auth")
		Expect(auth.Status).To(Equal(doctor.OK))
		Expect(auth.Detail).To(ContainSubstring("skipped"))
	})

	It("gh OK but auth Warn when present and not authenticated", func() {
		env := goodEnv()
		env.GH.(*vcsfakes.FakeGHRunner).AuthenticatedReturns(false)
		rs := doctor.Run(env)
		Expect(find(rs, "gh").Status).To(Equal(doctor.OK))
		auth := find(rs, "gh auth")
		Expect(auth.Status).To(Equal(doctor.Warn))
		Expect(auth.Remediation).To(ContainSubstring("gh auth login"))
	})
})

var _ = Describe("doctor glab + glab auth", func() {
	It("glab OK + auth OK when available and authenticated", func() {
		rs := doctor.Run(goodEnv())
		Expect(find(rs, "glab").Status).To(Equal(doctor.OK))
		Expect(find(rs, "glab auth").Status).To(Equal(doctor.OK))
	})

	It("glab Warn when absent, and auth row skipped-OK (not double-counted)", func() {
		env := goodEnv()
		env.GL.(*vcsfakes.FakeGLRunner).AvailableReturns(false)
		rs := doctor.Run(env)
		Expect(find(rs, "glab").Status).To(Equal(doctor.Warn))
		Expect(find(rs, "glab").Remediation).To(ContainSubstring("brew install glab"))
		auth := find(rs, "glab auth")
		Expect(auth.Status).To(Equal(doctor.OK))
		Expect(auth.Detail).To(ContainSubstring("skipped"))
	})

	It("glab OK but auth Warn when present and not authenticated", func() {
		env := goodEnv()
		env.GL.(*vcsfakes.FakeGLRunner).AuthenticatedReturns(false)
		rs := doctor.Run(env)
		Expect(find(rs, "glab").Status).To(Equal(doctor.OK))
		auth := find(rs, "glab auth")
		Expect(auth.Status).To(Equal(doctor.Warn))
		Expect(auth.Remediation).To(ContainSubstring("glab auth login"))
	})

	It("treats a nil GL runner as skipped-OK (no panic, no failure)", func() {
		env := goodEnv()
		env.GL = nil
		rs := doctor.Run(env)
		Expect(find(rs, "glab").Status).To(Equal(doctor.OK))
		Expect(find(rs, "glab").Detail).To(ContainSubstring("skipped"))
		Expect(find(rs, "glab auth").Status).To(Equal(doctor.OK))
	})
})

var _ = Describe("doctor superpowers (best-effort)", func() {
	It("OK when a superpowers dir exists under plugins", func() {
		Expect(find(doctor.Run(goodEnv()), "superpowers").Status).To(Equal(doctor.OK))
	})

	It("Warn when plugins exists but no superpowers", func() {
		env := goodEnv()
		// remove the superpowers dir goodEnv created, leaving the plugins tree
		Expect(os.RemoveAll(filepath.Join(env.ClaudeDir, "plugins", "cache", "mk", "superpowers"))).To(Succeed())
		r := find(doctor.Run(env), "superpowers")
		Expect(r.Status).To(Equal(doctor.Warn))
		Expect(r.Remediation).To(ContainSubstring("plugin install"))
	})

	It("OK (indeterminate, never failing) when there is no plugins dir", func() {
		env := goodEnv()
		Expect(os.RemoveAll(filepath.Join(env.ClaudeDir, "plugins"))).To(Succeed())
		r := find(doctor.Run(env), "superpowers")
		Expect(r.Status).To(Equal(doctor.OK))
		Expect(r.Detail).To(ContainSubstring("could not verify"))
	})
})

var _ = Describe("doctor herdle on PATH", func() {
	It("OK when the running binary's dir is on PATH", func() {
		Expect(find(doctor.Run(goodEnv()), "herdle on PATH").Status).To(Equal(doctor.OK))
	})

	It("Fail with remediation when the binary's dir is not on PATH", func() {
		env := goodEnv()
		env.PathDirs = []string{"/somewhere/else"}
		r := find(doctor.Run(env), "herdle on PATH")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Remediation).To(ContainSubstring(filepath.Dir(env.ExecPath)))
	})

	It("OK via a symlink on PATH that targets the running binary", func() {
		env := goodEnv()
		realDir := GinkgoT().TempDir()
		real := filepath.Join(realDir, "herdle")
		Expect(os.WriteFile(real, []byte("bin"), 0o755)).To(Succeed()) // #nosec G306 -- test binary
		linkDir := GinkgoT().TempDir()
		link := filepath.Join(linkDir, "herdle")
		Expect(os.Symlink(real, link)).To(Succeed())
		env.ExecPath = real              // os.Executable resolves to the real path
		env.HerdleOnPath = link          // exec.LookPath finds the symlink on PATH
		env.PathDirs = []string{linkDir} // real dir is NOT on PATH
		r := find(doctor.Run(env), "herdle on PATH")
		Expect(r.Status).To(Equal(doctor.OK))
		Expect(r.Detail).To(ContainSubstring(link))
	})

	It("Warn when a different herdle is on PATH than the running binary", func() {
		env := goodEnv()
		aDir := GinkgoT().TempDir()
		running := filepath.Join(aDir, "herdle")
		Expect(os.WriteFile(running, []byte("running"), 0o755)).To(Succeed()) // #nosec G306 -- test binary
		bDir := GinkgoT().TempDir()
		other := filepath.Join(bDir, "herdle")
		Expect(os.WriteFile(other, []byte("other"), 0o755)).To(Succeed()) // #nosec G306 -- test binary
		env.ExecPath = running
		env.HerdleOnPath = other
		env.PathDirs = []string{bDir} // running binary's dir not on PATH; a different herdle is
		r := find(doctor.Run(env), "herdle on PATH")
		Expect(r.Status).To(Equal(doctor.Warn))
		Expect(r.Detail).To(ContainSubstring(other))
	})
})

var _ = Describe("doctor install integrity", func() {
	It("OK when every embedded artifact is present and current", func() {
		Expect(find(doctor.Run(goodEnv()), "claude: skills + rule").Status).To(Equal(doctor.OK))
	})

	It("Fail listing a missing artifact", func() {
		env := goodEnv()
		Expect(os.Remove(filepath.Join(env.ClaudeDir, "rules", "herdle.md"))).To(Succeed())
		r := find(doctor.Run(env), "claude: skills + rule")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Detail).To(ContainSubstring("herdle.md"))
		Expect(r.Remediation).To(ContainSubstring("herdle init"))
	})

	It("Warn when an artifact has drifted from the embedded copy", func() {
		env := goodEnv()
		Expect(os.WriteFile(filepath.Join(env.ClaudeDir, "rules", "herdle.md"),
			[]byte("locally edited\n"), 0o600)).To(Succeed())
		r := find(doctor.Run(env), "claude: skills + rule")
		Expect(r.Status).To(Equal(doctor.Warn))
		Expect(r.Remediation).To(ContainSubstring("--force"))
	})

	It("fails clearly when selected Claude assets are unavailable", func() {
		env := goodEnv()
		env.Assets = nil
		env.ClaudeAssets = nil
		r := find(doctor.Run(env), "claude: skills + rule")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Detail).To(ContainSubstring("asset filesystem is unavailable"))
		Expect(r.Remediation).To(Equal("run: herdle init"))
	})

	It("fails clearly when selected Polytoken assets are unavailable", func() {
		env := withHealthyPolytoken(goodEnv())
		env.Agents = []agent.Name{agent.Polytoken}
		env.PolytokenAssets = nil
		r := find(doctor.Run(env), "polytoken: skills + context")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Detail).To(ContainSubstring("asset filesystem is unavailable"))
		Expect(r.Remediation).To(Equal("herdle init --agent polytoken"))
	})
})

var _ = Describe("doctor config", func() {
	It("OK when config is present with at least one project", func() {
		r := find(doctor.Run(goodEnv()), "config")
		Expect(r.Status).To(Equal(doctor.OK))
		Expect(r.Detail).To(ContainSubstring("1 project"))
	})

	It("Fail when config is absent", func() {
		env := goodEnv()
		Expect(os.Remove(env.ConfigPath)).To(Succeed())
		r := find(doctor.Run(env), "config")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Remediation).To(ContainSubstring("herdle init"))
	})

	It("Warn when config is present but has no projects", func() {
		env := goodEnv()
		Expect(os.WriteFile(env.ConfigPath, []byte(""), 0o600)).To(Succeed())
		r := find(doctor.Run(env), "config")
		Expect(r.Status).To(Equal(doctor.Warn))
		Expect(r.Remediation).To(ContainSubstring("herdle project add"))
	})
})

var _ = Describe("doctor lifecycle gatekeeper", func() {
	It("reports the lifecycle gatekeeper as wired", func() {
		dir := GinkgoT().TempDir()
		sp := filepath.Join(dir, "settings.json")
		Expect(os.WriteFile(sp, []byte(`{"hooks":{"PreToolUse":[{"matcher":"Edit|Write|Bash","hooks":[{"type":"command","command":"/x/herdle hook gatekeeper"}]}]}}`), 0o600)).To(Succeed())
		r := doctor.CheckGateForTest(doctor.Env{SettingsPath: sp})
		Expect(r.Status).To(Equal(doctor.OK))
	})

	It("flags a missing gatekeeper (not wired at all)", func() {
		dir := GinkgoT().TempDir()
		sp := filepath.Join(dir, "settings.json")
		Expect(os.WriteFile(sp, []byte(`{}`), 0o600)).To(Succeed())
		r := doctor.CheckGateForTest(doctor.Env{SettingsPath: sp})
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Detail).To(ContainSubstring("not wired"))
		Expect(r.Remediation).To(ContainSubstring("herdle init"))
	})

	It("flags stale code-review-gate wiring distinctly from not-wired (pre-rename)", func() {
		dir := GinkgoT().TempDir()
		sp := filepath.Join(dir, "settings.json")
		Expect(os.WriteFile(sp, []byte(`{"hooks":{"PreToolUse":[{"matcher":"Edit|Write|Bash","hooks":[{"type":"command","command":"/x/herdle hook code-review-gate"}]}]}}`), 0o600)).To(Succeed())
		r := doctor.CheckGateForTest(doctor.Env{SettingsPath: sp})
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Detail).To(ContainSubstring("stale")) // distinguishes the stale branch from the generic not-wired Fail
		Expect(r.Remediation).To(ContainSubstring("herdle init"))
	})
})

var _ = Describe("doctor.Render", func() {
	It("renders a row per result, remediation under non-OK, no ANSI when color off", func() {
		var buf bytes.Buffer
		doctor.Render(&buf, []doctor.Result{
			{Name: "git", Status: doctor.OK, Detail: "found"},
			{Name: "tk", Status: doctor.Fail, Detail: "not found", Remediation: "install tk"},
		}, false)
		out := buf.String()
		Expect(out).To(ContainSubstring("✓ git"))
		Expect(out).To(ContainSubstring("✗ tk"))
		Expect(out).To(ContainSubstring("→ install tk"))
		Expect(out).NotTo(ContainSubstring("\x1b["))
	})

	It("emits ANSI when color is on", func() {
		var buf bytes.Buffer
		doctor.Render(&buf, []doctor.Result{{Name: "git", Status: doctor.OK, Detail: "found"}}, true)
		Expect(buf.String()).To(ContainSubstring("\x1b[32m"))
	})

	It("renders the Warn glyph and remediation, with yellow ANSI when color is on", func() {
		res := []doctor.Result{{Name: "gh", Status: doctor.Warn, Detail: "absent", Remediation: "install gh"}}

		var plain bytes.Buffer
		doctor.Render(&plain, res, false)
		Expect(plain.String()).To(ContainSubstring("⚠ gh"))
		Expect(plain.String()).To(ContainSubstring("→ install gh")) // remediation shown for non-OK
		Expect(plain.String()).NotTo(ContainSubstring("\x1b["))

		var color bytes.Buffer
		doctor.Render(&color, res, true)
		Expect(color.String()).To(ContainSubstring("\x1b[33m")) // yellow
	})
})
