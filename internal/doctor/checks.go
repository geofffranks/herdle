package doctor

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/geofffranks/herdle/internal/config"
)

func checkGit(env Env) Result {
	if env.Git.Available() {
		return Result{Name: "git", Status: OK, Detail: "found"}
	}
	return Result{Name: "git", Status: Fail, Detail: "not found",
		Remediation: "install git (system package or: brew install git)"}
}

func checkTK(env Env) Result {
	if env.TK.Available() {
		return Result{Name: "tk", Status: OK, Detail: "found"}
	}
	return Result{Name: "tk", Status: Fail, Detail: "not found",
		Remediation: "install tk: brew install wedow/tools/ticket"}
}

func checkGH(env Env) Result {
	if env.GH.Available() {
		return Result{Name: "gh", Status: OK, Detail: "found"}
	}
	return Result{Name: "gh", Status: Warn, Detail: "not found (optional)",
		Remediation: "install gh to enable PR features: brew install gh"}
}

func checkGHAuth(env Env) Result {
	if !env.GH.Available() {
		return Result{Name: "gh auth", Status: OK, Detail: "skipped (gh not installed)"}
	}
	if env.GH.Authenticated() {
		return Result{Name: "gh auth", Status: OK, Detail: "authenticated"}
	}
	return Result{Name: "gh auth", Status: Warn, Detail: "not authenticated",
		Remediation: "authenticate gh: gh auth login"}
}

func checkGLab(env Env) Result {
	if env.GL == nil {
		return Result{Name: "glab", Status: OK, Detail: "skipped (not configured)"}
	}
	if env.GL.Available() {
		return Result{Name: "glab", Status: OK, Detail: "found"}
	}
	return Result{Name: "glab", Status: Warn, Detail: "not found (optional)",
		Remediation: "install glab to enable GitLab MR features: brew install glab"}
}

func checkGLabAuth(env Env) Result {
	if env.GL == nil || !env.GL.Available() {
		return Result{Name: "glab auth", Status: OK, Detail: "skipped (glab not installed)"}
	}
	if env.GL.Authenticated() {
		return Result{Name: "glab auth", Status: OK, Detail: "authenticated"}
	}
	return Result{Name: "glab auth", Status: Warn, Detail: "not authenticated",
		Remediation: "authenticate glab: glab auth login (use --hostname for self-hosted GitLab)"}
}

func checkSuperpowers(env Env) Result {
	dir := filepath.Join(env.ClaudeDir, "plugins")
	found, scanned := scanForDir(dir, "superpowers", 6)
	switch {
	case !scanned:
		return Result{Name: "superpowers", Status: OK,
			Detail: "could not verify (no plugins dir at " + dir + ")"}
	case found:
		return Result{Name: "superpowers", Status: OK, Detail: "found under " + dir}
	default:
		return Result{Name: "superpowers", Status: Warn,
			Detail:      "not found under " + dir,
			Remediation: "add the superpowers marketplace, then: /plugin install superpowers"}
	}
}

func checkHerdlePath(env Env) Result {
	const name = "herdle on PATH"
	execDir := filepath.Dir(env.ExecPath)
	for _, d := range env.PathDirs {
		if d != "" && filepath.Clean(d) == execDir {
			return Result{Name: name, Status: OK, Detail: "on PATH (" + execDir + ")"}
		}
	}
	// The running binary's dir isn't literally a PATH entry, but `herdle` may still
	// resolve on PATH to *this same* binary via a symlink (os.Executable resolves
	// symlinks, so execDir points at the real, off-PATH location). os.SameFile
	// compares identity (inode / Windows file ID), so it is case- and symlink-correct.
	if env.HerdleOnPath != "" && sameFile(env.HerdleOnPath, env.ExecPath) {
		return Result{Name: name, Status: OK, Detail: "on PATH as " + env.HerdleOnPath}
	}
	// A herdle is on PATH, but it is a *different* binary than the one running —
	// the user's `herdle` would invoke that other (possibly stale) copy.
	if env.HerdleOnPath != "" {
		return Result{Name: name, Status: Warn,
			Detail:      "PATH has a different herdle (" + env.HerdleOnPath + ") than the running binary (" + env.ExecPath + ")",
			Remediation: "put " + execDir + " ahead of the other herdle on PATH, or reinstall"}
	}
	return Result{Name: name, Status: Fail,
		Detail:      "running from " + execDir + ", which is not on PATH",
		Remediation: "add " + execDir + " to your PATH"}
}

// sameFile reports whether two paths resolve to the same on-disk file (following
// symlinks). False if either cannot be stat'd.
func sameFile(a, b string) bool {
	ai, err := os.Stat(a) // #nosec G304 -- diagnostic stat of resolved herdle paths
	if err != nil {
		return false
	}
	bi, err := os.Stat(b) // #nosec G304 -- diagnostic stat of resolved herdle paths
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

func checkIntegrity(env Env) Result {
	const name = "skills + rule"
	var missing, drifted []string
	err := fs.WalkDir(env.Assets, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		want, readErr := fs.ReadFile(env.Assets, p)
		if readErr != nil {
			return readErr
		}
		dest := filepath.Join(env.ClaudeDir, filepath.FromSlash(p))
		got, statErr := os.ReadFile(dest) // #nosec G304 -- dest is under ClaudeDir, derived from the embedded artifact tree
		if statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				missing = append(missing, dest)
				return nil
			}
			return statErr
		}
		if !bytes.Equal(got, want) {
			drifted = append(drifted, dest)
		}
		return nil
	})
	if err != nil {
		return Result{Name: name, Status: Fail, Detail: "could not check: " + err.Error(),
			Remediation: "run: herdle init"}
	}
	switch {
	case len(missing) > 0:
		return Result{Name: name, Status: Fail,
			Detail:      fmt.Sprintf("%d missing: %s", len(missing), strings.Join(missing, ", ")),
			Remediation: "run: herdle init"}
	case len(drifted) > 0:
		return Result{Name: name, Status: Warn,
			Detail:      fmt.Sprintf("%d out of date: %s", len(drifted), strings.Join(drifted, ", ")),
			Remediation: "refresh after an upgrade: herdle init --force"}
	default:
		return Result{Name: name, Status: OK, Detail: "present and current"}
	}
}

func checkConfig(env Env) Result {
	const name = "config"
	if _, statErr := os.Stat(env.ConfigPath); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return Result{Name: name, Status: Fail, Detail: "not found at " + env.ConfigPath,
				Remediation: "run: herdle init"}
		}
		return Result{Name: name, Status: Fail,
			Detail:      "cannot read " + env.ConfigPath + ": " + statErr.Error(),
			Remediation: "check permissions on " + env.ConfigPath}
	}
	cfg, err := config.LoadFrom(env.ConfigPath)
	if err != nil {
		return Result{Name: name, Status: Fail, Detail: "invalid config: " + err.Error(),
			Remediation: "fix or remove " + env.ConfigPath + ", then run: herdle init"}
	}
	if len(cfg.Projects) == 0 {
		return Result{Name: name, Status: Warn, Detail: "present but no projects",
			Remediation: "add one: herdle project add <path>"}
	}
	return Result{Name: name, Status: OK,
		Detail: fmt.Sprintf("present (%d project(s))", len(cfg.Projects))}
}

// checkGate verifies the lifecycle gatekeeper is wired into settings.json. A
// substring check is intentional: robust to formatting, not coupled to the
// settings schema. The pre-rename marker (code-review-gate) is reported as stale
// so an upgraded install is told to re-run init.
func checkGate(env Env) Result {
	const name = "lifecycle gatekeeper"
	data, err := os.ReadFile(env.SettingsPath) // #nosec G304 -- SettingsPath is under ClaudeDir
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Result{Name: name, Status: Fail, Detail: "settings.json not found at " + env.SettingsPath,
				Remediation: "run: herdle init"}
		}
		return Result{Name: name, Status: Fail, Detail: "cannot read " + env.SettingsPath + ": " + err.Error(),
			Remediation: "check permissions on " + env.SettingsPath}
	}
	if bytes.Contains(data, []byte("gatekeeper")) {
		return Result{Name: name, Status: OK, Detail: "wired in settings.json"}
	}
	if bytes.Contains(data, []byte("code-review-gate")) {
		return Result{Name: name, Status: Fail, Detail: "stale gate wiring (pre-gatekeeper)",
			Remediation: "run: herdle init"}
	}
	return Result{Name: name, Status: Fail, Detail: "not wired in " + env.SettingsPath,
		Remediation: "run: herdle init"}
}

// scanForDir walks root (bounded to maxDepth) for a directory named target.
// scanned is false when root is absent or not a directory — the caller treats
// that as indeterminate, never a failure.
func scanForDir(root, target string, maxDepth int) (found, scanned bool) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return false, false
	}
	rootDepth := strings.Count(filepath.Clean(root), string(os.PathSeparator))
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable subtrees
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == target {
			found = true
			return filepath.SkipAll
		}
		// Stop descending past maxDepth, but only after the name check above so a
		// target sitting exactly at the boundary is still found.
		if strings.Count(filepath.Clean(p), string(os.PathSeparator))-rootDepth > maxDepth {
			return fs.SkipDir
		}
		return nil
	})
	return found, true
}
