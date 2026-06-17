package vcs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type glRunner struct{}

// NewGLRunner returns a GLRunner backed by the real glab binary
// (HERDLE_GLAB override, else PATH).
func NewGLRunner() GLRunner { return glRunner{} }

func (glRunner) glab(args ...string) (result, error) {
	bin, err := resolveBinary("HERDLE_GLAB", "glab")
	if err != nil {
		return result{}, err
	}
	return run("", bin, args...)
}

// glMR is the subset of a `glab mr list -F json` element herdle consumes. glab
// emits the raw GitLab API shape (snake_case fields, lowercase states), which
// toPR maps onto the forge-neutral PR type.
type glMR struct {
	IID                 int    `json:"iid"`
	State               string `json:"state"` // opened | closed | merged | locked
	SourceBranch        string `json:"source_branch"`
	Title               string `json:"title"`
	Draft               bool   `json:"draft"`
	WorkInProgress      bool   `json:"work_in_progress"`
	HasConflicts        bool   `json:"has_conflicts"`
	DetailedMergeStatus string `json:"detailed_merge_status"`
}

// toPR maps a GitLab merge request onto the forge-neutral PR, translating
// detailed_merge_status into the GitHub-shaped Mergeable/ReviewDecision fields
// classifyMerge already understands. GitLab's `mr list` does not include pipeline
// detail (the pipeline field is null), so StatusCheckRollup is left empty and CI
// state does not factor into merge classification for GitLab MRs.
func (m glMR) toPR() PR {
	pr := PR{
		Number:      m.IID,
		HeadRefName: m.SourceBranch,
		Title:       m.Title,
		IsDraft:     m.Draft || m.WorkInProgress,
	}
	switch m.State {
	case "merged":
		pr.State = "MERGED"
	case "closed", "locked":
		pr.State = "CLOSED"
	default: // opened
		pr.State = "OPEN"
	}
	switch {
	case m.HasConflicts, m.DetailedMergeStatus == "conflict":
		pr.Mergeable = "CONFLICTING"
	case m.DetailedMergeStatus == "mergeable":
		pr.Mergeable = "MERGEABLE"
	}
	// NOTE: detailed_merge_status == "broken_status" is deliberately NOT mapped to
	// CONFLICTING. It signals GitLab could not compute mergeability (a transient
	// failure), not an actual conflict; mapping it would show a false "✗ conflicts"
	// on a cleanly-mergeable MR. Left unset, it classifies as neutral.
	//
	// GitLab only returns "mergeable" once ALL merge requirements pass, so the
	// not-yet-ready states below name the specific blocker. Unlike GitHub (whose
	// `mergeable` is blind to branch protection, rendering such PRs "ready"), we
	// surface them as a "blocked" attention marker via BlockReason. "ci_still_running"
	// / "checking" / "unchecked" are deliberately left neutral — they are transient
	// "still computing" states, not a settled blocker. "ci_must_pass" reads as
	// amber "checks not passed" rather than a hard red "checks failing": `mr list`
	// returns no pipeline detail, so we cannot tell a failed pipeline from one that
	// simply has not run.
	switch m.DetailedMergeStatus {
	case "requested_changes":
		pr.ReviewDecision = "CHANGES_REQUESTED"
	case "not_approved":
		pr.BlockReason = "needs approval"
	case "discussions_not_resolved":
		pr.BlockReason = "unresolved threads"
	case "need_rebase":
		pr.BlockReason = "needs rebase"
	case "ci_must_pass":
		pr.BlockReason = "checks not passed"
	}
	return pr
}

func (r glRunner) PRList(slug, state string) ([]PR, error) {
	args := []string{"mr", "list", "-R", slug, "--author", "@me", "-F", "json", "--per-page", "60"}
	if state == "all" {
		args = append(args, "--all")
	}
	// glab is occasionally flaky; retryJSONFetch retries once and only trusts a
	// real JSON array, so a transient failure never looks like "no MRs". glab emits
	// the raw GitLab API shape, so decode into glMR and map onto the neutral PR.
	mrs, err := retryJSONFetch[glMR](fmt.Sprintf("glab mr list -R %s", slug), func() (result, error) {
		return r.glab(args...)
	})
	if err != nil {
		return nil, err
	}
	prs := make([]PR, len(mrs))
	for i, m := range mrs {
		prs[i] = m.toPR()
	}
	return prs, nil
}

// Available reports whether the glab binary can be located (HERDLE_GLAB override,
// else PATH). It does not check auth — see Authenticated.
func (glRunner) Available() bool { return binaryAvailable("HERDLE_GLAB", "glab") }

// Authenticated reports whether `glab auth status` exits 0. False when glab cannot
// be run at all (absent binary / bad override).
func (r glRunner) Authenticated() bool {
	res, err := r.glab("auth", "status")
	if err != nil {
		return false
	}
	return res.code == 0
}

// KnownHosts returns the GitLab hosts glab is configured for (the keys under the
// top-level `hosts:` map in glab's config.yml) unioned with gitlab.com. A missing
// or unreadable config yields just {"gitlab.com"}.
func (glRunner) KnownHosts() []string {
	hosts := []string{"gitlab.com"}
	seen := map[string]bool{"gitlab.com": true}
	data, err := os.ReadFile(glabConfigPath()) // #nosec G304 -- glab's own config path
	if err != nil {
		return hosts
	}
	for _, h := range parseGlabHosts(string(data)) {
		if h != "" && !seen[h] {
			seen[h] = true
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// parseGlabHosts extracts the host keys nested under the top-level `hosts:` map of
// glab's YAML config. Dependency-free: it finds the unindented `hosts:` line, then
// collects keys at the first child-indent level whose colon carries no inline
// value (a host map key; deeper keys are that host's own settings). Hosts are
// lowercased for case-insensitive matching against a remote URL host.
func parseGlabHosts(cfg string) []string {
	var hosts []string
	inHosts := false
	childIndent := -1
	for _, raw := range strings.Split(cfg, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(trimmed)
		if !inHosts {
			if indent == 0 && trimmed == "hosts:" {
				inHosts = true
			}
			continue
		}
		if indent == 0 {
			break // a new top-level key ends the hosts block
		}
		if childIndent == -1 {
			childIndent = indent
		}
		if indent != childIndent {
			continue // deeper (a host's own settings) — skip
		}
		i := strings.IndexByte(trimmed, ':')
		if i <= 0 || strings.TrimSpace(trimmed[i+1:]) != "" {
			continue // not a bare "host:" key
		}
		hosts = append(hosts, strings.ToLower(strings.TrimSpace(trimmed[:i])))
	}
	return hosts
}

// glabConfigPath resolves glab's config.yml location, mirroring glab's own
// resolution order: GLAB_CONFIG_DIR, else the OS user-config dir (macOS:
// ~/Library/Application Support; Linux: $XDG_CONFIG_HOME or ~/.config) under
// glab-cli, else ~/.config/glab-cli. It returns the first candidate that exists,
// falling back to the last candidate when none do.
func glabConfigPath() string {
	if d := os.Getenv("GLAB_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "config.yml")
	}
	var candidates []string
	if d, err := os.UserConfigDir(); err == nil && d != "" {
		candidates = append(candidates, filepath.Join(d, "glab-cli", "config.yml"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".config", "glab-cli", "config.yml"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if len(candidates) > 0 {
		return candidates[len(candidates)-1]
	}
	return ""
}
