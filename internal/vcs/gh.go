package vcs

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ghRunner struct{}

// NewGHRunner returns a GHRunner backed by the real gh binary
// (HERDLE_GH override, else PATH).
func NewGHRunner() GHRunner { return ghRunner{} }

func (ghRunner) gh(args ...string) (result, error) {
	bin, err := resolveBinary("HERDLE_GH", "gh")
	if err != nil {
		return result{}, err
	}
	return run("", bin, args...)
}

func (r ghRunner) PRList(slug, state string) ([]PR, error) {
	args := []string{
		"pr", "list", "-R", slug, "--author", "@me", "--state", state,
		"--limit", "60", "--json", "number,state,headRefName,title",
	}
	// gh is occasionally flaky; retry once. Only accept a real JSON array — a
	// transient failure must NOT look like "no PRs".
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		res, err := r.gh(args...)
		if err != nil {
			lastErr = err
			continue
		}
		if res.code != 0 {
			lastErr = fmt.Errorf("gh pr list -R %s: %s", slug, strings.TrimSpace(res.stderr))
			continue
		}
		out := strings.TrimSpace(res.stdout)
		if !strings.HasPrefix(out, "[") {
			lastErr = fmt.Errorf("gh pr list -R %s: unexpected output %q", slug, out)
			continue
		}
		var prs []PR
		if err := json.Unmarshal([]byte(out), &prs); err != nil {
			lastErr = fmt.Errorf("gh pr list -R %s: parse json: %w", slug, err)
			continue
		}
		return prs, nil
	}
	return nil, lastErr
}

// Available reports whether the gh binary can be located. When HERDLE_GH is set
// it must point at an existing non-directory file; otherwise gh must be on PATH.
func (ghRunner) Available() bool {
	if v := os.Getenv("HERDLE_GH"); v != "" {
		info, err := os.Stat(v) // #nosec G304,G703 -- HERDLE_GH is a deliberate user-supplied override (see resolveBinary)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath("gh")
	return err == nil
}

// KnownHosts returns the GitHub hosts gh is authenticated to (the top-level keys
// in gh's hosts.yml) unioned with github.com. Dependency-free: a top-level YAML
// key is a line with no leading whitespace whose colon is followed by nothing
// (host keys carry no inline value; child keys are indented).
func (ghRunner) KnownHosts() []string {
	hosts := []string{"github.com"}
	seen := map[string]bool{"github.com": true}
	data, err := os.ReadFile(ghHostsPath()) // #nosec G304 -- gh's own config path
	if err != nil {
		return hosts
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		i := strings.IndexByte(line, ':')
		if i <= 0 || strings.TrimSpace(line[i+1:]) != "" {
			continue
		}
		h := strings.TrimSpace(line[:i])
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// ghHostsPath resolves gh's hosts.yml location: GH_CONFIG_DIR, else
// XDG_CONFIG_HOME/gh, else ~/.config/gh.
func ghHostsPath() string {
	if d := os.Getenv("GH_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "hosts.yml")
	}
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "gh", "hosts.yml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gh", "hosts.yml")
}
