# herdle

> **Wrangle the herd, spot the hurdles.**

herdle is a self-contained Go binary that gives you a cross-project, tk-driven work-in-progress dashboard across many git repos on GitHub and GitLab (gitlab.com and self-hosted) — open PRs/MRs, cleanup candidates, in-flight branches, and up-next tickets, all in one view. It also installs the Claude Code convention skills and a rules stub into `~/.claude/` so your AI assistant speaks the same workflow language from day one.

## What you get

**Cross-project dashboard** — run `herdle --all` for a one-line summary per configured repo, or `herdle` inside any repo for a full drilldown: open PRs correlated to tickets, merged PRs needing branch cleanup, work-in-progress branches with sync state, and an up-next queue sorted by priority. Design artifacts (specs/plans/validation docs) are listed alongside their tickets.

**Skills + rules installation** — `herdle init` writes the superpowers skills and a Claude Code rules stub into `~/.claude/`, seeds an initial `config.toml`, and gets you ready to run.

## Requirements at a glance

| Dependency | Notes |
|---|---|
| `git` | Required — all repo introspection goes through git |
| `tk` ([wedow/ticket](https://github.com/wedow/ticket)) | Required — herdle reads ticket state and metadata |
| superpowers plugin | Required — installed skills depend on it |
| `gh` (authenticated) + GitHub remote | Optional — enables PR correlation, issue links, and GitHub-aware features |
| `glab` (authenticated) + GitLab remote | Optional — enables MR correlation for gitlab.com and self-hosted GitLab (`glab auth login --hostname <host>` per instance) |

See [docs/install.md](docs/install.md) for the full dependency contract and installation instructions.

## Quickstart

Download the binary for your platform from the [latest GitHub Release](https://github.com/geofffranks/herdle/releases/latest) — assets are named `herdle-<os>-<arch>` (e.g. `herdle-darwin-arm64`, `herdle-linux-amd64`). Put it on your `PATH`, then:

```bash
herdle init      # writes the skills + rules, seeds config
herdle doctor    # verify the setup
herdle --all     # cross-project summary
```

## Sample

Running `herdle` inside its own repo gives you its drilldown — representative of what you see in any project:

```
### herdle   (/Users/gfranks/workspace/herdle)

— git —  her-1sd1-s11-readme-docs*

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
  ...
```

Each section appears only when it has content — empty sections are hidden automatically.

## Docs

- [docs/install.md](docs/install.md) — install + dependency contract
- [docs/usage.md](docs/usage.md) — commands + dashboard reference
- [docs/configuration.md](docs/configuration.md) — config.toml reference
- [docs/tk-conventions.md](docs/tk-conventions.md) — the tk workflow herdle encodes
- [CONTRIBUTING.md](CONTRIBUTING.md) — build from source

## License

MIT — see [LICENSE](LICENSE).
