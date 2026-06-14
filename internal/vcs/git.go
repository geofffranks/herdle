package vcs

import (
	"fmt"
	"strconv"
	"strings"
)

type gitRunner struct{}

// NewGitRunner returns a GitRunner backed by the real git binary
// (HERDLE_GIT override, else PATH).
func NewGitRunner() GitRunner { return gitRunner{} }

func (gitRunner) git(dir string, args ...string) (result, error) {
	bin, err := resolveBinary("HERDLE_GIT", "git")
	if err != nil {
		return result{}, err
	}
	return run(dir, bin, args...)
}

func (g gitRunner) RepoRoot(path string) (string, error) {
	res, err := g.git(path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	if res.code != 0 {
		return "", ErrNotARepo
	}
	return res.trimmed(), nil
}

func (g gitRunner) CurrentBranch(path string) (string, error) {
	res, err := g.git(path, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	if res.code != 0 {
		return "", fmt.Errorf("git branch --show-current: %s", strings.TrimSpace(res.stderr))
	}
	return res.trimmed(), nil // "" when detached
}

func (g gitRunner) RemoteURL(path, remote string) (string, error) {
	res, err := g.git(path, "remote", "get-url", remote)
	if err != nil {
		return "", err
	}
	if res.code != 0 {
		// Non-zero here means the remote is absent; the caller decides what to
		// try next (e.g. upstream then origin). Surface it as an error.
		return "", fmt.Errorf("git remote get-url %s: %s", remote, strings.TrimSpace(res.stderr))
	}
	return res.trimmed(), nil
}

func (g gitRunner) RemoteHead(path, remote string) (string, error) {
	res, err := g.git(path, "symbolic-ref", "--short", "refs/remotes/"+remote+"/HEAD")
	if err != nil {
		return "", err
	}
	if res.code != 0 {
		// No refs/remotes/<remote>/HEAD set locally — not an error; the caller
		// falls back to main/master. Purely local; no network probe.
		return "", nil
	}
	return strings.TrimPrefix(res.trimmed(), remote+"/"), nil
}

func (g gitRunner) IsDirty(path string) (bool, error) {
	wt, err := g.git(path, "diff", "--quiet")
	if err != nil {
		return false, err
	}
	if wt.code != 0 {
		return true, nil
	}
	idx, err := g.git(path, "diff", "--cached", "--quiet")
	if err != nil {
		return false, err
	}
	return idx.code != 0, nil
}

func (g gitRunner) Divergence(path, leftRef, rightRef string) (int, int, error) {
	res, err := g.git(path, "rev-list", "--left-right", "--count", leftRef+"..."+rightRef)
	if err != nil {
		return 0, 0, err
	}
	if res.code != 0 {
		return 0, 0, nil // no upstream / unknown ref — wip treats as 0/0
	}
	fields := strings.Fields(res.trimmed())
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("git rev-list: unexpected output %q", res.trimmed())
	}
	left, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("git rev-list: parse left %q: %w", fields[0], err)
	}
	right, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("git rev-list: parse right %q: %w", fields[1], err)
	}
	return left, right, nil
}

func (g gitRunner) LocalBranchExists(path, branch string) (bool, error) {
	return g.refExists(path, "refs/heads/"+branch)
}

func (g gitRunner) RemoteBranchExists(path, remote, branch string) (bool, error) {
	return g.refExists(path, "refs/remotes/"+remote+"/"+branch)
}

func (g gitRunner) refExists(path, ref string) (bool, error) {
	res, err := g.git(path, "show-ref", "--verify", "--quiet", ref)
	if err != nil {
		return false, err
	}
	return res.code == 0, nil
}

func (g gitRunner) LocalBranches(path string) ([]Branch, error) {
	res, err := g.git(path, "for-each-ref", "--format=%(refname:short) %(upstream:track)", "refs/heads")
	if err != nil {
		return nil, err
	}
	if res.code != 0 {
		return nil, fmt.Errorf("git for-each-ref refs/heads: %s", strings.TrimSpace(res.stderr))
	}
	var out []Branch
	for _, line := range strings.Split(res.trimmed(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := line
		gone := false
		if i := strings.IndexByte(line, ' '); i >= 0 {
			name = line[:i]
			gone = strings.Contains(line[i+1:], "[gone]")
		}
		out = append(out, Branch{Name: name, UpstreamGone: gone})
	}
	return out, nil
}

func (g gitRunner) RemoteBranches(path, remote string) ([]string, error) {
	res, err := g.git(path, "for-each-ref", "--format=%(refname:short)", "refs/remotes/"+remote)
	if err != nil {
		return nil, err
	}
	if res.code != 0 {
		return nil, fmt.Errorf("git for-each-ref refs/remotes/%s: %s", remote, strings.TrimSpace(res.stderr))
	}
	prefix := remote + "/"
	var out []string
	for _, line := range strings.Split(res.trimmed(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == prefix+"HEAD" {
			continue
		}
		out = append(out, strings.TrimPrefix(line, prefix))
	}
	return out, nil
}

func (g gitRunner) Fetch(path string) error {
	res, err := g.git(path, "fetch", "--all", "--prune")
	if err != nil {
		return err
	}
	if res.code != 0 {
		return fmt.Errorf("git fetch --all --prune: %s", strings.TrimSpace(res.stderr))
	}
	return nil
}

func (gitRunner) Available() bool { return binaryAvailable("HERDLE_GIT", "git") }

func (g gitRunner) PruneRemote(path, remote string) error {
	res, err := g.git(path, "remote", "prune", remote)
	if err != nil {
		return err
	}
	if res.code != 0 {
		// Offline this fails; the dashboard ignores prune errors when not
		// fetching. We report the truth and let the caller decide.
		return fmt.Errorf("git remote prune %s: %s", remote, strings.TrimSpace(res.stderr))
	}
	return nil
}
