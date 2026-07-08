package vcs

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
		"--limit", "60", "--json", "number,state,headRefName,title,mergeable,reviewDecision,isDraft,statusCheckRollup",
	}
	// gh is occasionally flaky; retryJSONFetch retries once and only trusts a real
	// JSON array, so a transient failure never looks like "no PRs".
	return retryJSONFetch[PR](fmt.Sprintf("gh pr list -R %s", slug), func() (result, error) {
		return r.gh(args...)
	})
}

func (r ghRunner) IssueList(slug, state string) ([]Issue, error) {
	args := []string{
		"issue", "list", "-R", slug, "--state", state,
		"--limit", strconv.Itoa(IssueFetchLimit), "--json", "number,title,state",
	}
	return retryJSONFetch[Issue](fmt.Sprintf("gh issue list -R %s", slug), func() (result, error) {
		return r.gh(args...)
	})
}

// Available reports whether the gh binary can be located (HERDLE_GH override,
// else PATH). It does not check auth — see Authenticated.
func (ghRunner) Available() bool { return binaryAvailable("HERDLE_GH", "gh") }

// Authenticated reports whether `gh auth status` exits 0. False when gh cannot be
// run at all (absent binary / bad override).
func (r ghRunner) Authenticated() bool {
	res, err := r.gh("auth", "status")
	if err != nil {
		return false
	}
	return res.code == 0
}

// KnownHosts returns the GitHub hosts gh is authenticated to (the top-level keys
// in gh's hosts.yml) unioned with github.com. Host parsing is shared with glab via
// yamlBareKeys; gh's hosts are top-level keys, so the parent is "".
func (ghRunner) KnownHosts() []string {
	hosts := []string{"github.com"}
	seen := map[string]bool{"github.com": true}
	data, err := os.ReadFile(ghHostsPath()) // #nosec G304 -- gh's own config path
	if err != nil {
		return hosts
	}
	for _, h := range yamlBareKeys(string(data), "") {
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
