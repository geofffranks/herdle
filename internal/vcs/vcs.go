// Package vcs wraps the git, gh, and tk binaries behind typed interfaces so the
// dashboard can be exercised against fakes with no real repo, network, or tools.
// Binary paths are overridable via HERDLE_GIT / HERDLE_GH / HERDLE_TK.
package vcs

import "errors"

//go:generate go tool counterfeiter -generate

// ErrNotARepo is returned by GitRunner.RepoRoot when path is not in a git repo.
var ErrNotARepo = errors.New("not a git repository")

// Branch is a local branch and whether its upstream was deleted on the remote
// (git for-each-ref %(upstream:track) == "[gone]").
type Branch struct {
	Name         string
	UpstreamGone bool
}

// PR is an authored pull request as reported by `gh pr list --json`.
type PR struct {
	Number      int    `json:"number"`
	State       string `json:"state"` // OPEN | MERGED | CLOSED
	HeadRefName string `json:"headRefName"`
	Title       string `json:"title"`
}

// Ticket is a tk ticket. Lifecycle is the raw frontmatter value ("-" or "" when
// unset); the designed/planned-from-disk derivation is the consumer's job.
type Ticket struct {
	ID          string
	Status      string // open | in_progress | closed
	Lifecycle   string
	Title       string // first "# " heading in .tickets/<id>.md
	Branch      string // frontmatter branch: (optional)
	ExternalRef string // frontmatter external-ref: (optional)
	Type        string
	Assignee    string
	Priority    int
}

//counterfeiter:generate -o vcsfakes/fake_git_runner.go . GitRunner

// GitRunner wraps read-mostly git queries plus fetch/prune. remote is always a
// parameter (de-personalization): callers supply the configured remote name.
type GitRunner interface {
	RepoRoot(path string) (string, error)                                   // rev-parse --show-toplevel; ErrNotARepo outside a repo
	CurrentBranch(path string) (string, error)                              // branch --show-current; "" when detached
	IsDirty(path string) (bool, error)                                      // diff --quiet || diff --cached --quiet
	Divergence(path, leftRef, rightRef string) (left, right int, err error) // rev-list --left-right --count "L...R"
	LocalBranchExists(path, branch string) (bool, error)                    // show-ref --verify refs/heads/<b>
	RemoteBranchExists(path, remote, branch string) (bool, error)           // show-ref --verify refs/remotes/<remote>/<b>
	LocalBranches(path string) ([]Branch, error)                            // for-each-ref refs/heads (+ upstream:track)
	RemoteBranches(path, remote string) ([]string, error)                   // for-each-ref refs/remotes/<remote> (stripped)
	RemoteURL(path, remote string) (string, error)                          // remote get-url <remote>
	Fetch(path string) error                                                // fetch --all --prune
	PruneRemote(path, remote string) error                                  // remote prune <remote>
}

//counterfeiter:generate -o vcsfakes/fake_gh_runner.go . GHRunner

// GHRunner wraps `gh pr list`. PRList returns a non-nil error on gh failure
// (missing binary, auth, network) — never a silent empty slice.
type GHRunner interface {
	// PRList returns the @me-authored PRs for slug in the given state
	// ("open" | "all"). The impl retries once and validates a JSON-array shape
	// before trusting an empty result.
	PRList(slug, state string) ([]PR, error)
}

//counterfeiter:generate -o vcsfakes/fake_tk_runner.go . TKRunner

// TKRunner wraps tk. Tickets returns fully-populated tickets (incl. Title);
// Ready defers to `tk ready`; HasTickets gates whether tk output renders.
type TKRunner interface {
	Tickets(path string) ([]Ticket, error) // tk query + heading read for Title
	Ready(path string) ([]string, error)   // tk ready -> ready ticket ids
	HasTickets(path string) (bool, error)  // .tickets/ dir present
}
