// Package doctor implements `herdle doctor`: a green/red checklist of the herdle
// dependency and install contract. All external access (tool probes, filesystem,
// PATH, embedded artifacts) is injected via Env, so the logic is unit-tested with
// no real tools, repo, or HOME.
package doctor

import (
	"io/fs"

	"github.com/geofffranks/herdle/internal/vcs"
)

// Status is a check outcome. OK passes; Warn and Fail both make the command exit
// non-zero (strict exit). Render colors them green/yellow/red.
type Status int

const (
	OK Status = iota
	Warn
	Fail
)

// Result is one checklist row. Remediation prints under the row when Status != OK.
type Result struct {
	Name        string
	Status      Status
	Detail      string
	Remediation string
}

// Env is everything doctor inspects. The cli action fills it from the real
// environment; tests fill it with fakes, temp dirs, and a synthetic PATH.
type Env struct {
	Git vcs.GitRunner
	GH  vcs.GHRunner
	GL  vcs.GLRunner
	TK  vcs.TKRunner

	Assets       fs.FS    // embedded artifacts (assets.FS) — drives integrity
	ClaudeDir    string   // ~/.claude: skills/, rules/, plugins/ live here
	ConfigPath   string   // config.Path(): the herdle config.toml
	SettingsPath string   // config.SettingsPath(): ~/.claude/settings.json
	ExecPath     string   // os.Executable(): the running herdle binary
	HerdleOnPath string   // exec.LookPath("herdle"); "" when not found
	PathDirs     []string // PATH split on os.PathListSeparator
}

// Run executes every check in fixed display order.
func Run(env Env) []Result {
	checks := []func(Env) Result{
		checkGit,
		checkTK,
		checkGH,
		checkGHAuth,
		checkGLab,
		checkGLabAuth,
		checkSuperpowers,
		checkHerdlePath,
		checkIntegrity,
		checkConfig,
		checkGate,
	}
	rs := make([]Result, 0, len(checks))
	for _, c := range checks {
		rs = append(rs, c(env))
	}
	return rs
}

// FailCount returns how many results are not OK (Warn or Fail).
func FailCount(rs []Result) int {
	n := 0
	for _, r := range rs {
		if r.Status != OK {
			n++
		}
	}
	return n
}

// Failed reports whether the command should exit non-zero: true if any result is
// not OK (strict — Warn counts).
func Failed(rs []Result) bool { return FailCount(rs) > 0 }
