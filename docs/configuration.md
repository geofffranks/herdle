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
gh          = "owner/repo"      # optional; enables PR/issue features
remote      = "upstream"        # optional; autodetect if unset
base        = "dev"             # optional; autodetect if unset
integration = "geoff-main"      # optional; personal integration branch

[[project]]
path = "/path/to/plain"          # no gh -> git+tk view only
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
| `gh` | no | `owner/repo` slug. Enables GitHub PR and issue features. |
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

---

## Environment Variables

| Variable | Effect |
|---|---|
| `HERDLE_GIT` | Override the resolved path to the `git` binary (else found on `PATH`). |
| `HERDLE_GH` | Override the resolved path to the `gh` binary (else found on `PATH`). |
| `HERDLE_TK` | Override the resolved path to the `tk` binary (else found on `PATH`). |
| `HERDLE_CONFIG` | Override the config file location (else the default path above). |
| `NO_COLOR` | Disable ANSI color output. A non-empty value always wins. |
| `FORCE_COLOR` / `CLICOLOR_FORCE` | Force color even when stdout is not a TTY. |
