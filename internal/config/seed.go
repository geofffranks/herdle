package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/geofffranks/herdle/internal/vcs"
)

// ClaudeProjectsDir returns the Claude Code projects directory:
// ${CLAUDE_CONFIG_DIR:-$HOME/.claude}/projects.
func ClaudeProjectsDir() (string, error) {
	base, err := baseDir("CLAUDE_CONFIG_DIR", ".claude")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "projects"), nil
}

// DiscoverClaudeProjects scans projectsDir, resolving each child directory's
// newest transcript cwd to a git repo root via git.RepoRoot. Non-repos, missing
// transcripts, and unresolvable entries are skipped; roots are deduped. A missing
// projectsDir yields nil and no error. The caller merges via Config.Add.
func DiscoverClaudeProjects(projectsDir string, git vcs.GitRunner) ([]string, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var roots []string
	seen := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cwd := transcriptCwd(filepath.Join(projectsDir, e.Name()))
		if cwd == "" {
			continue
		}
		root, err := git.RepoRoot(cwd)
		if err != nil { // ErrNotARepo / missing path -> skip
			continue
		}
		if seen[root] {
			continue
		}
		seen[root] = true
		roots = append(roots, root)
	}
	return roots, nil
}

// transcriptCwd returns the cwd recorded in any of the dir's *.jsonl transcripts.
// All transcripts under one project dir share the same cwd, so it returns the
// first one found and falls through truncated / cwd-less transcripts.
func transcriptCwd(dir string) string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, m := range matches {
		if cwd := firstCwd(m); cwd != "" {
			return cwd
		}
	}
	return ""
}

// firstCwd returns the first non-empty "cwd" field across the transcript's JSON
// lines (it is on the first event in practice). Transcript lines can be large, so
// the scanner buffer is raised well above the 64 KiB default.
func firstCwd(path string) string {
	f, err := os.Open(path) // #nosec G304 -- reading a Claude Code transcript under the configured projects dir
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var rec struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) == nil && rec.Cwd != "" {
			return rec.Cwd
		}
	}
	return ""
}
