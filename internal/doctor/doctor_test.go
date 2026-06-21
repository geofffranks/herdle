package doctor_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/geofffranks/herdle/internal/doctor"
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
		Expect(find(doctor.Run(goodEnv()), "skills + rule").Status).To(Equal(doctor.OK))
	})

	It("Fail listing a missing artifact", func() {
		env := goodEnv()
		Expect(os.Remove(filepath.Join(env.ClaudeDir, "rules", "herdle.md"))).To(Succeed())
		r := find(doctor.Run(env), "skills + rule")
		Expect(r.Status).To(Equal(doctor.Fail))
		Expect(r.Detail).To(ContainSubstring("herdle.md"))
		Expect(r.Remediation).To(ContainSubstring("herdle init"))
	})

	It("Warn when an artifact has drifted from the embedded copy", func() {
		env := goodEnv()
		Expect(os.WriteFile(filepath.Join(env.ClaudeDir, "rules", "herdle.md"),
			[]byte("locally edited\n"), 0o600)).To(Succeed())
		r := find(doctor.Run(env), "skills + rule")
		Expect(r.Status).To(Equal(doctor.Warn))
		Expect(r.Remediation).To(ContainSubstring("--force"))
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
