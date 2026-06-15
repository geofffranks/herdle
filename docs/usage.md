# herdle Usage Guide

herdle is a cross-project work-in-progress dashboard that surfaces open PRs,
in-flight branches, `tk` tickets, and design artifacts across all your configured
repositories. Run it anywhere — inside or outside a git repo — to see what's on
your plate.

---

## Dashboard

The default invocation (`herdle`) detects context automatically:

- **Inside a repo** — shows a drilldown for that specific project: open PRs, branches
  not yet in a PR, unstarted tickets, and design artifacts.
- **Outside a repo** — shows a cross-project summary across every configured project,
  one row per project, with aggregate counts of open PRs, branches, and tickets.

Two global flags change that detection:

- `herdle --all` (alias `-a`) — forces the cross-project summary even when you are
  sitting inside a git repo. Useful when you want a birds-eye view without `cd`-ing
  away first.
- `herdle --fetch` (alias `-f`) — runs `git fetch` against each repo before
  generating output. By default herdle works entirely offline; pass `--fetch` when
  you want origin-sync data to be current (costs a network round-trip per repo).

A positional argument also changes detection:

- `herdle <name>` — targets a single named configured project by name (or an
  unambiguous prefix) and shows its drilldown, regardless of your current directory.

### Sample output (drilldown)

The following was captured with `NO_COLOR=1 ./herdle` from inside the herdle repo
itself:

```
### herdle   (/Users/gfranks/workspace/herdle)

— git —  her-1sd1-s11-readme-docs

— work in progress (in-flight tk + branches, not in a PR) —
  state               sync tk        branch                         feature                                  issues
  in-development      ✗    her-1sd1  her-1sd1-s11-readme-docs       S11: README + install/usage/contributor… local only — not pushed

— up next (open tk, not started) —
  designed            her-x9jl  P2 herdle: shareable tk-driven workflow tooling (wip dashboard + skills + rules)
  -                   her-npt5  P2 S12: output-parity validation gate (wip vs herdle over ~/workspace + dcs-retribution)
  pending-validation  her-mg2c  P2 S10: release pipeline (release.yml GOOS/GOARCH matrix, codeql, dependabot)

— design artifacts (specs / plans / validation) —
  her-15ji  specs  2026-06-12-her-15ji-s2-vcs-abstraction-design.md
  her-58tk  specs  2026-06-12-her-58tk-s1-scaffold-design.md
  her-me5d  specs  2026-06-12-her-me5d-s3-config-design.md
  her-x9jl  specs  2026-06-12-her-x9jl-herdle-design.md
  her-6hgc  specs  2026-06-13-her-6hgc-s6-degradation-remote-design.md
  her-fogw  specs  2026-06-13-her-fogw-s4-dashboard-summary-design.md
  her-ju9h  specs  2026-06-13-her-ju9h-s5-dashboard-drilldown-design.md
  her-tj8f  specs  2026-06-13-her-tj8f-s8-doctor-design.md
  her-ubn5  specs  2026-06-13-her-ubn5-s7-embed-init-design.md
  her-1sd1  specs  2026-06-14-her-1sd1-s11-readme-docs-design.md
  her-cung  specs  2026-06-14-her-cung-s9-skills-rules-content-design.md
  her-mg2c  specs  2026-06-14-her-mg2c-s10-release-pipeline-design.md
  her-15ji  plans  2026-06-12-her-15ji-s2-vcs-abstraction.md
  her-58tk  plans  2026-06-12-her-58tk-s1-scaffold.md
  her-me5d  plans  2026-06-12-her-me5d-s3-config-subsystem.md
  her-6hgc  plans  2026-06-13-her-6hgc-s6-degradation-remote.md
```

---

## Reading a drilldown

A repo drilldown has up to five sections, each hidden when it has nothing to show:

**Open PRs** — pull requests currently open on the configured remote. Each row
includes the PR number and title, the correlated `tk` ticket (if any), and whether
the local branch is in sync with origin.

**Merged-PR cleanup** — branches that correspond to already-merged PRs and can be
safely deleted. herdle surfaces these so you can prune stale local branches without
having to cross-reference GitHub manually.

**Work in progress** — branches (and their correlated `tk` tickets) that are not
yet in a PR. The `sync` column (`✓`/`✗`) shows whether the local branch is
in sync with its origin tracking ref. The `state` column shows the ticket's
lifecycle state (see below). Branches with no upstream tracking ref appear as
`local only`.

**Up next** — open `tk` tickets that are not yet `in_progress` and are not
associated with an open PR. These are your backlog. Rows include the ticket ID,
priority, title, and derived lifecycle state.

**Design artifacts** — specs, plans, and validation documents found under
`docs/superpowers/` that herdle correlates to tickets by embedded ticket ID in the
filename.

### The sync column

`✓` means the local branch is in sync with (or ahead of) its origin tracking
branch. `✗` means local and origin have diverged, or the branch has no upstream
set at all.

### The lifecycle column

Lifecycle states flow from `-` (not started) through `designed`, `planned`,
`in-development`, `pending-validation`, to `validated`. herdle derives the state
from the `tk` ticket's frontmatter; a set lifecycle always wins. When no state is
set, herdle derives `planned` when a plan file for the ticket exists, and falls
back to `designed` when only a spec file exists. A row showing `?` means the
ticket has no lifecycle field and no derivable artifact.

### Origin pruning and hidden branches

herdle auto-prunes stale remote-tracking refs each run (equivalent to
`git remote prune`), so branches whose upstream was deleted on origin disappear
from the sync column rather than accumulating as phantom entries. Branches
associated with already-merged PRs are hidden from the WIP section and surfaced
instead under merged-PR cleanup.

For deeper semantics on how tickets, branches, and PRs correlate, see
[tk-conventions.md](tk-conventions.md).

---

## Managing projects

herdle keeps a list of configured projects in `~/.config/herdle/config.toml`.
The `project` subcommand manages that list.

### List projects

```
herdle project list
```

Prints every configured project with its name, path, and metadata fields.

### Add a project

```
herdle project add <path> [--gh <owner/repo>] [--remote <name>] [--base <branch>] [--integration <branch>]
```

Registers the repository at `<path>` with herdle. The path is the only required
argument; all flags are optional and can be auto-detected:

- `--gh <owner/repo>` — GitHub owner/repo slug for PR and issue features. herdle
  auto-detects this from the git remote URL when the flag is omitted.
- `--remote <name>` — which git remote to treat as the canonical upstream (e.g.
  `origin`, `upstream`). Auto-detected from the repository when omitted.
- `--base <branch>` — the trunk/main branch (e.g. `main`, `master`). Auto-detected
  when omitted.
- `--integration <branch>` — a personal integration branch, if your workflow uses
  one. Optional; leave unset if not applicable.

### Update a project

```
herdle project set <name|path> [--gh <owner/repo>] [--remote <name>] [--base <branch>] [--integration <branch>]
```

Updates one or more fields on an already-configured project. The identifier can
be the project's name or its path. Accepts the same four flags as `project add`;
only the flags you pass are changed — omitted flags are left as-is.

### Remove a project

```
herdle project rm <name|path>
```

Removes the project from herdle's configuration. The repository itself is not
touched; only the herdle config entry is deleted.

---

## Diagnostics

### `herdle doctor`

```
herdle doctor
```

Inspects the herdle setup and reports on required and optional dependencies
(e.g. `git`, `gh`, `tk`). Required dependencies that are missing or misconfigured
cause a non-zero exit code, making `herdle doctor` safe to use in scripts or CI
checks. Optional dependencies that are absent produce warnings but do not affect
the exit code.

### `herdle version`

```
herdle version
```

Prints the herdle version string (embedded at build time from the git commit SHA
or tag).

### `herdle init`

```
herdle init [--force] [--uninstall]
```

Writes herdle's embedded Claude Code skills and rules into your Claude Code
configuration (`~/.claude/skills/` and `~/.claude/rules/`) and seeds the herdle
config (`~/.config/herdle/`) if it does not already exist. Flags:

- `--force` — overwrite existing skill/rule files even if they are already
  present. Use after upgrading herdle to refresh the embedded content.
- `--uninstall` — remove the skills and rules that herdle installed, leaving
  the rest of the project untouched.

---

## Command Reference

| Command | Purpose |
|---|---|
| `herdle` | inside a repo: drilldown; outside: cross-project summary |
| `herdle --all` | force the cross-project summary even inside a repo |
| `herdle --fetch` | `git fetch` each repo first (network; default offline) |
| `herdle <name>` | drilldown for a named project |
| `herdle version` | print the herdle version |
| `herdle project list` | list configured projects |
| `herdle project add <path>` | add a project (flags: `--gh`, `--remote`, `--base`, `--integration`) |
| `herdle project set <name>` | update a project (flags: `--gh`, `--remote`, `--base`, `--integration`) |
| `herdle project rm <name>` | remove a project |
| `herdle init` | write/refresh embedded skills + rules (`--force` overwrites after an upgrade; `--uninstall` removes them) |
| `herdle doctor` | diagnose the herdle setup |
