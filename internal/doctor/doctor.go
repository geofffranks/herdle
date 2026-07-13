// Package doctor implements `herdle doctor`: a green/red checklist of the herdle
// dependency and install contract. All external access (tool probes, filesystem,
// PATH, embedded artifacts) is injected via Env, so the logic is unit-tested with
// no real tools, repo, or HOME.
package doctor

import (
	"io/fs"

	"github.com/geofffranks/herdle/internal/agent"
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

	Agents             []agent.Name
	Assets             fs.FS // legacy Claude assets; retained during migration
	ClaudeAssets       fs.FS
	PolytokenAssets    fs.FS
	ClaudeDir          string
	PolytokenDir       string
	PolytokenHooksPath string
	PolytokenCommand   string
	ConfigPath         string
	SettingsPath       string
	ExecPath           string
	HerdleOnPath       string
	PathDirs           []string
}

// Run executes common checks once, then selected harness checks in selection order.
func Run(env Env) []Result {
	common := []func(Env) Result{
		checkGit, checkTK, checkGH, checkGHAuth, checkGLab, checkGLabAuth,
		checkHerdlePath, checkConfig,
	}
	rs := make([]Result, 0, len(common)+6)
	for _, check := range common {
		rs = append(rs, check(env))
	}
	agents := env.Agents
	if len(agents) == 0 {
		agents = []agent.Name{agent.Claude}
	}
	for _, selected := range agents {
		switch selected {
		case agent.Claude:
			rs = append(rs, checkSuperpowers(env), checkClaudeIntegrity(env), checkGate(env))
		case agent.Polytoken:
			rs = append(rs, checkPolytoken(env)...)
		}
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
