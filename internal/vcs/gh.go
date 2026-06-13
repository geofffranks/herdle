package vcs

import (
	"encoding/json"
	"fmt"
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
