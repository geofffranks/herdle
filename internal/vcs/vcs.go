// Package vcs wraps the git, gh, glab, and tk binaries behind typed interfaces so
// the dashboard can be exercised against fakes with no real repo, network, or
// tools. Binary paths are overridable via HERDLE_GIT / HERDLE_GH / HERDLE_GLAB /
// HERDLE_TK.
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

	// Merge-readiness fields (all from the same `gh pr list --json` call).
	Mergeable         string     `json:"mergeable"`      // MERGEABLE | CONFLICTING | UNKNOWN
	ReviewDecision    string     `json:"reviewDecision"` // APPROVED | CHANGES_REQUESTED | REVIEW_REQUIRED | ""
	IsDraft           bool       `json:"isDraft"`
	StatusCheckRollup []CheckRun `json:"statusCheckRollup"`

	// BlockReason is a short, human-readable reason an open PR/MR is not ready to
	// merge despite having no conflicts, no failing checks, and no requested
	// changes — e.g. "needs approval" or "needs rebase". It is forge-neutral but
	// only GitLab populates it today: GitHub's `mergeable` field is blind to branch
	// protection, so gh reports such PRs as mergeable, whereas GitLab's
	// detailed_merge_status names the specific blocker. Empty for an unblocked PR.
	BlockReason string `json:"-"`
}

// CheckRun is one element of a PR's statusCheckRollup. A single flat struct
// covers both gh element shapes: a CheckRun carries Status/Conclusion, a
// StatusContext carries State. Absent fields unmarshal to "".
type CheckRun struct {
	Typename   string `json:"__typename"`
	Status     string `json:"status"`     // CheckRun: QUEUED | IN_PROGRESS | COMPLETED
	Conclusion string `json:"conclusion"` // CheckRun: SUCCESS | FAILURE | NEUTRAL | ...
	State      string `json:"state"`      // StatusContext: SUCCESS | FAILURE | PENDING | ERROR | EXPECTED
	Name       string `json:"name"`       // CheckRun label
	Context    string `json:"context"`    // StatusContext label
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
	RemoteHead(path, remote string) (string, error)                         // symbolic-ref --short refs/remotes/<remote>/HEAD; "" when unset
	Fetch(path string) error                                                // fetch --all --prune
	PruneRemote(path, remote string) error                                  // remote prune <remote>
	Available() bool                                                        // git binary locatable (HERDLE_GIT override, else PATH)
}

//counterfeiter:generate -o vcsfakes/fake_gh_runner.go . GHRunner

// GHRunner wraps `gh`. PRList returns a non-nil error on gh failure (missing
// binary, auth, network) — never a silent empty slice. Available and KnownHosts
// support graceful degradation and GitHub-Enterprise detection.
type GHRunner interface {
	// PRList returns the @me-authored PRs for slug in the given state
	// ("open" | "all"). slug is gh's [HOST/]OWNER/REPO. The impl retries once and
	// validates a JSON-array shape before trusting an empty result.
	PRList(slug, state string) ([]PR, error)
	// Available reports whether the gh binary can be located (HERDLE_GH override,
	// else PATH). It does not check auth — that is herdle doctor's job (S8).
	Available() bool
	// KnownHosts returns the GitHub hosts gh is authenticated to — the top-level
	// keys of gh's hosts.yml — always unioned with "github.com". A missing or
	// unreadable file yields just {"github.com"}.
	KnownHosts() []string
	// Authenticated reports whether `gh auth status` exits 0 (logged into at
	// least one host). False when gh is absent — callers gate on Available first.
	Authenticated() bool
}

//counterfeiter:generate -o vcsfakes/fake_gl_runner.go . GLRunner

// GLRunner wraps `glab` (the GitLab CLI), supporting gitlab.com and self-hosted
// instances. Its method set mirrors GHRunner so both satisfy the dashboard's
// forge client, but it targets merge requests rather than pull requests and maps
// them onto the same forge-neutral PR type. Binary path overridable via
// HERDLE_GLAB.
type GLRunner interface {
	// PRList returns the @me-authored merge requests for slug in the given state
	// ("open" | "all"), normalized onto the shared PR type. slug is glab's -R
	// value: GROUP/PROJECT (or nested GROUP/SUBGROUP/PROJECT) for gitlab.com, or a
	// full https URL for a self-hosted host. The impl retries once and validates a
	// JSON-array shape before trusting an empty result.
	PRList(slug, state string) ([]PR, error)
	// Available reports whether the glab binary can be located (HERDLE_GLAB
	// override, else PATH). It does not check auth — see Authenticated.
	Available() bool
	// KnownHosts returns the GitLab hosts glab is configured for — the keys under
	// the top-level `hosts:` map in glab's config.yml — always unioned with
	// "gitlab.com". A missing or unreadable config yields just {"gitlab.com"}.
	KnownHosts() []string
	// Authenticated reports whether `glab auth status` exits 0 (logged into at
	// least one host). False when glab is absent — callers gate on Available first.
	Authenticated() bool
}

//counterfeiter:generate -o vcsfakes/fake_tk_runner.go . TKRunner

// TKRunner wraps tk. Tickets returns fully-populated tickets (incl. Title);
// Ready defers to `tk ready`; HasTickets gates whether tk output renders.
type TKRunner interface {
	Tickets(path string) ([]Ticket, error) // tk query + heading read for Title
	Ready(path string) ([]string, error)   // tk ready -> ready ticket ids
	HasTickets(path string) (bool, error)  // .tickets/ dir present
	Available() bool                       // tk binary locatable (HERDLE_TK override, else PATH)
}
