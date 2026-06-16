# Configuration Reference

## Location & Format

herdle stores its configuration at:

```
${XDG_CONFIG_HOME:-$HOME/.config}/herdle/config.toml
```

By default this resolves to `~/.config/herdle/config.toml`.

The file can be managed with `herdle project add`, `herdle project set`,
`herdle project rm`, and `herdle project list`, or edited by hand.

Only explicitly-set fields are written to the file. Unset fields are resolved
at read time against the live repository, so the config stays pure user-intent
and never bakes in autodetected values.

### Example

```toml
default_remote = "upstream"   # optional global defaults
default_base   = "main"

[[project]]
path        = "/path/to/repo"
slug        = "owner/repo"      # optional; enables PR/MR features (GitHub or GitLab)
remote      = "upstream"        # optional; autodetect if unset
base        = "dev"             # optional; autodetect if unset
integration = "geoff-main"      # optional; personal integration branch

[[project]]
path = "/path/to/gitlab/repo"    # gitlab.com or self-hosted GitLab — forge detected from the remote host

[[project]]
path = "/path/to/plain"          # no forge remote -> git+tk view only
```

---

## Field Reference

### Global Keys

| Key | Description |
|---|---|
| `default_remote` | Remote name applied to every project when its own `remote` is unset. |
| `default_base` | Trunk branch applied to every project when its own `base` is unset. |

### Per-Project Keys (`[[project]]`)

| Key | Required | Description |
|---|---|---|
| `path` | yes | Absolute path to the repository. |
| `slug` | no | Forge-agnostic `[group/]owner/repo` slug. Enables PR/MR features. The forge (GitHub via `gh`, GitLab via `glab`) is selected from the remote host, so this works for github.com, GitHub Enterprise, gitlab.com, and self-hosted GitLab alike. |
| `gh` | no | Legacy GitHub-only `owner/repo` override. Still works and always means GitHub; prefer `slug` for new config. |
| `remote` | no | Git remote to treat as canonical. Autodetected if unset. |
| `base` | no | Trunk branch. Autodetected if unset. |
| `integration` | no | A personal integration branch (e.g. your long-running merge target). |

---

## Autodetection

When a field is unset, herdle resolves it at read time using the following
priority order.

### `remote`

1. Explicit value in the project block.
2. `default_remote` global key.
3. `origin` — if that remote exists in the repo.
4. `upstream` — if it exists.
5. None.

Origin is checked before upstream because branches typically live on your push
remote.

### `base`

1. Explicit value in the project block.
2. `default_base` global key.
3. The remote's HEAD branch.
4. `main` — if that branch exists on the remote.
5. `master`.
6. Falls back to `"main"`.

### `slug` and forge selection

1. Explicit `gh` value (always GitHub).
2. Explicit `slug` value (forge chosen by the remote host).
3. Derived from the canonical remote's URL (`owner/repo`), gated by host.
4. None — the project shows only the git + tk view.

The **forge** for a project is chosen from the host of its canonical remote:

- A host that `gh` is authenticated to (plus `github.com`) → **GitHub**, queried
  with `gh pr list`.
- A host that `glab` is authenticated to (plus `gitlab.com`) → **GitLab**,
  queried with `glab mr list`. This covers both gitlab.com and self-hosted
  instances such as a corporate `gitlab.example.com` — authenticate each with
  `glab auth login --hostname <host>`.
- Any other host → no forge; the project still shows git + tk state.

GitLab merge requests are surfaced through the same PR columns and sections as
GitHub pull requests (open MRs, merged-MR branch cleanup, merge-readiness).

---

## Environment Variables

| Variable | Effect |
|---|---|
| `HERDLE_GIT` | Override the resolved path to the `git` binary (else found on `PATH`). |
| `HERDLE_GH` | Override the resolved path to the `gh` binary (else found on `PATH`). |
| `HERDLE_GLAB` | Override the resolved path to the `glab` binary (else found on `PATH`). |
| `HERDLE_TK` | Override the resolved path to the `tk` binary (else found on `PATH`). |
| `HERDLE_CONFIG` | Override the config file location (else the default path above). |
| `NO_COLOR` | Disable ANSI color output. A non-empty value always wins. |
| `FORCE_COLOR` / `CLICOLOR_FORCE` | Force color even when stdout is not a TTY. |
