# S11 — README + install/usage/contributor docs — Design Spec

**Ticket:** her-1sd1 (story) · **Epic:** her-x9jl
**Date:** 2026-06-14
**Depends on:** her-fogw (S4 summary), her-ju9h (S5 drilldown), her-ubn5 (S7 embed+init), her-tj8f (S8 doctor) — all shipped.

## Purpose

Replace the 9-line stub `README.md` with real user-facing documentation that
covers herdle's full command surface, the install/upgrade/uninstall flow, the
dependency contract, the config format, and the tk conventions it encodes — plus
a contributor guide. Add a CI-enforced check so the docs cannot silently drift
out of sync with the CLI.

This is a **docs story plus one test**. It changes no CLI behavior.

## Goals

- A lean `README.md` front door that presents herdle as a normal, ready project
  (no construction/pre-release banner) and links to focused `docs/` pages.
- A user-facing `docs/` tree covering install, usage, configuration, and the tk
  conventions — distinct from the existing `docs/superpowers/` process artifacts.
- A `CONTRIBUTING.md` for building, testing, and the test harness.
- Install steps **accurate against the shipped binary**: documented commands run
  against the built binary; release-asset names match `release.yml`.
- A **drift guard**: a Ginkgo spec that fails when documented commands/flags
  diverge from the live `cli.App`, riding the existing `make test` → `test.yml`
  (no new CI workflow).

## Non-Goals

- **No CLI behavior or feature changes.** The only code added is the drift spec.
- **No generated docs.** A `make docs-gen` generator was considered and rejected;
  docs are hand-written and verified.
- **No man pages or docs website.** Roadmap if ever warranted.
- **No local-LLM delegation docs.** Out of epic scope (her-x9jl).
- `docs/tk-conventions.md` **orients**; it does not re-teach the installed skills
  verbatim. The skills remain the authoritative agent-facing source.
- **No live-release verification.** No tag is cut yet, so asset-name accuracy is
  validated against `.github/workflows/release.yml`, not a published Release.

## Decisions (locked during brainstorming)

1. **Doc layout:** lean `README.md` + a user-facing `docs/` tree + `CONTRIBUTING.md`.
2. **tk conventions:** orient a human reader, then point to the installed skills.
3. **Scope:** docs **plus** a drift guard (not pure docs).
4. **Drift mechanism:** a Ginkgo spec reflecting over `newApp()` — no new workflow.
5. **README status:** drop the "early development" banner; present as ready.

## File Set

```
README.md              # rewritten front door (replaces the 9-line stub)
docs/install.md        # dependency contract + per-platform install/upgrade/uninstall
docs/usage.md          # full command surface + Command Reference table (drift anchor)
docs/configuration.md  # config.toml format, autodetection, wip migration, env vars
docs/tk-conventions.md # orientation, then points to the installed skills
CONTRIBUTING.md        # Go, make targets, ginkgo/counterfeiter, the drift spec, layout
cmd/herdle/docs_drift_test.go   # the drift guard (only code change)
```

`docs/` (user-facing) is intentionally distinct from `docs/superpowers/`
(internal specs/plans/validation). No collision.

## Per-File Content

### README.md

- Tagline + one-paragraph "what herdle is" — lead with the convention-layer value
  (a cross-project tk-driven WIP dashboard, shared as a self-contained binary).
- "What you get": the cross-project dashboard + the skills/rules `herdle init`
  installs.
- Requirements at a glance — required: `git`, `tk`, superpowers; optional: `gh`
  (authenticated) + a GitHub remote. Links to `docs/install.md` for the full
  contract.
- **Quickstart:** download `herdle-<os>-<arch>` for your platform → put on `PATH`
  → `herdle init` → `herdle doctor` → `herdle` / `herdle --all`.
- One annotated sample dashboard view (captured with `NO_COLOR=1` for stable text).
- Links to each `docs/` page and `CONTRIBUTING.md`.
- License.

### docs/install.md

- **Dependency contract** table — each dependency, its kind, and how the user
  gets it:

  | Dependency | Kind | How to get it |
  |---|---|---|
  | `tk` (wedow/ticket) | required | `brew install wedow/tools/ticket` |
  | `git` | required | system / `brew install git` |
  | superpowers plugin | required (for the skills/rules to mean anything) | add its marketplace + `/plugin install` |
  | `gh` (authenticated) | optional | `brew install gh && gh auth login` — enables PR/issue features |
  | GitHub-hosted remote | optional | enables PR/issue features per-project |
  | Go toolchain | dev-only | build from source (see CONTRIBUTING) |

- Per-platform download of the five release assets, with `.sha1` verification and
  `PATH` placement. Asset names (must match `release.yml`):
  `herdle-linux-amd64`, `herdle-linux-arm64`, `herdle-darwin-amd64`,
  `herdle-darwin-arm64`, `herdle-windows-amd64` (each with a `.sha1`).
- What `herdle init` writes: skills → `~/.claude/skills/`, rule stub →
  `~/.claude/rules/`, seed `~/.config/herdle/`, and one-time migrate
  `~/.config/wip/projects` if present.
- Verify the setup with a real `herdle doctor` sample.
- Upgrade: download the new binary, `herdle init --force`.
- Uninstall: `herdle init --uninstall` removes only the artifacts herdle wrote;
  it never edits `CLAUDE.md` and leaves `~/.config/herdle/` in place.

### docs/usage.md

- Dashboard modes: outside a repo → cross-project summary; inside a repo →
  drilldown; `--all` forces the summary; `herdle <name>` targets a named project;
  `--fetch` does a network refresh first (default is offline).
- How to read a drilldown at orientation depth: the four sections (open PRs,
  merged-PR cleanup, work-in-progress, up-next), the sync column, lifecycle, and
  origin pruning / hidden-branch rules. Deeper semantics live in
  `docs/tk-conventions.md`.
- `herdle project add/set/rm/list` with flags `--gh`, `--remote`, `--base`,
  `--integration`.
- `herdle doctor`, `herdle version`.
- Real sample outputs.
- **Command Reference table** — one row per command and flag, column 1 holding the
  literal `herdle …` invocation or the `--flag` token. This table is the
  structured anchor the drift spec parses for its reverse check; its format is a
  load-bearing contract (see Drift Guard).

### docs/configuration.md

- `~/.config/herdle/config.toml` location and format: global defaults plus
  `[[project]]` entries.
- Field reference: `path`, `gh`, `remote`, `base`, `integration`.
- Autodetection rules: remote `upstream` else `origin`; base = remote `HEAD` else
  `main`/`master`.
- One-time migration from the legacy `~/.config/wip/projects` line format.
- Environment variables: `HERDLE_GIT` / `HERDLE_GH` / `HERDLE_TK` (binary paths),
  `NO_COLOR` / `FORCE_COLOR` (rendering).

### docs/tk-conventions.md

- Why the convention layer is the value being shared.
- Lifecycle states: `-` → `designed` → `planned` → `in-development` →
  `pending-validation` → `validated`, plus the "open PR ⇒ validated" rule.
- tk ⇄ branch ⇄ PR correlation (`external-ref` / `branch:`) — briefly.
- The design-artifact filename pattern
  `docs/superpowers/{specs,plans,validation}/YYYY-MM-DD-<tkid>-<slug>`.
- Then **point to** the installed skills (`herdle-tk-flow`,
  `herdle-tk-artifacts`) as the authoritative, agent-facing source, and note the
  always-on `rules/herdle.md` stub that orients toward them.

### CONTRIBUTING.md

- Prereqs: Go 1.26.x.
- `make` targets: `vet`, `lint`, `test`, `build` (and `all`); `go install
  ./cmd/herdle` for a dev install.
- Test harness: Ginkgo specs + Counterfeiter fakes; `make test`;
  `go generate ./...` to regenerate fakes.
- The docs-drift spec: what it enforces and how to fix a failure (document the new
  command/flag, or update the Command Reference table when a flag is renamed/removed).
- Repo layout (`cmd/`, `internal/`, `assets/`, `docs/`).
- Lint stack: `gofmt`, `staticcheck`, `gosec`.

## Drift Guard

`cmd/herdle/docs_drift_test.go` (package `main`, on the existing
`herdle_suite_test.go` suite):

- **Extract the true surface:** recursively walk `newApp().Commands`, recording
  each full command path (e.g. `project add`) and each flag's primary name (e.g.
  `--gh`, `--all`), including the app-level global flags.
- **Ignore urfave/cli builtins:** command `help` (alias `h`); flags `--help`/`-h`
  and `--version`/`-v`.
- **Locate docs** via `runtime.Caller(0)` → repo root. Corpus for the forward
  check = `README.md` + the **top-level** `docs/*.md` user pages (non-recursive
  glob; the internal `docs/superpowers/` tree is excluded so internal specs can't
  satisfy coverage) + `CONTRIBUTING.md`.
- **Forward check (binary → docs):** every non-builtin command path and primary
  flag name must appear somewhere in the corpus. Catches new/undocumented
  surface; failure names the missing token.
- **Reverse check (docs → binary):** parse the Command Reference table in
  `docs/usage.md` (column 1 = literal `herdle …` invocation or `--flag`); strip
  backticks and the `herdle ` prefix; assert every token exists in the live app
  surface. Catches removed/renamed surface — robustly, off a structured table
  rather than prose.
- Short flag aliases (e.g. `-a`, `-f`) are acceptable in docs but not required;
  the forward check keys off each flag's primary (long) name.

## Verification / Accuracy

How the "accurate against the shipped binary" acceptance is met at write-time:

1. `make build`; run every documented command against the fresh binary.
2. Paste representative real samples (a `herdle doctor` run; a `NO_COLOR=1`
   dashboard view) into the docs.
3. Cross-check the five release-asset names against
   `.github/workflows/release.yml` (the naming contract) — no tag is cut yet.
4. The drift spec then enforces command/flag coverage mechanically on every CI
   run going forward.

## Testing Strategy

- The **drift spec** is the committed regression surface and the only code change.
  It runs under `make test` → `test.yml`, so it guards every push/PR.
- Initial accuracy is established by the manual write-time verification above.
- No other automated tests are needed; the docs themselves are prose.

## Acceptance Mapping

- *"Docs cover the full command surface"* → the forward check guarantees every
  command/flag is documented.
- *"…and the dependency contract"* → the contract table lives in
  `docs/install.md`.
- *"Install steps are accurate against the shipped binary"* → commands are
  verified against the built binary; asset names are verified against
  `release.yml`.

## Bookkeeping

Standard project flow: feature branch; `her-1sd1` lifecycle
`-` → `in-development` → `pending-validation`; write the validation doc at
finalize. The implementation plan's Setup/Finalize tasks own this.
