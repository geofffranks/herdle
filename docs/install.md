# Installing herdle

herdle is a self-contained CLI binary. Download a prebuilt release, run
`herdle init`, and you're done. No package manager required.

---

## Dependency contract

| Dependency | Kind | How to get it |
|---|---|---|
| `tk` (wedow/ticket) | required | `brew install wedow/tools/ticket` |
| `git` | required | system / `brew install git` |
| [superpowers](https://github.com/obra/superpowers) plugin | required for Claude Code skills (not needed for Polytoken) | add its marketplace + `/plugin install` |
| `gh` (authenticated) | optional | `brew install gh && gh auth login` — enables GitHub PR/issue features |
| `glab` (authenticated) | optional | `brew install glab && glab auth login` — enables GitLab MR features (gitlab.com and self-hosted; run `glab auth login --hostname <host>` per instance) |
| GitHub- or GitLab-hosted remote | optional | enables PR/MR features per-project (forge detected from the remote host) |
| Go toolchain | dev-only | build from source (see CONTRIBUTING) |

herdle and `herdle doctor` detect each dependency and print remediation steps;
nothing is auto-installed.

---

## Download

Download the binary for your platform from the [latest GitHub Release](https://github.com/geofffranks/herdle/releases/latest):

| Platform | Asset |
|---|---|
| Linux x86-64 | `herdle-linux-amd64` |
| Linux arm64 | `herdle-linux-arm64` |
| macOS Intel | `herdle-darwin-amd64` |
| macOS Apple Silicon | `herdle-darwin-arm64` |
| Windows x86-64 | `herdle-windows-amd64` |

Each asset ships with a matching `.sha1` checksum file.

### Install steps

```sh
# 1. Download the binary and its checksum (replace <os>-<arch> with your platform)
curl -LO https://github.com/geofffranks/herdle/releases/latest/download/herdle-<os>-<arch>
curl -LO https://github.com/geofffranks/herdle/releases/latest/download/herdle-<os>-<arch>.sha1

# 2. Verify the checksum (macOS: use `shasum -a 1 -c` instead of `sha1sum -c`)
sha1sum -c herdle-<os>-<arch>.sha1

# 3. Make it executable
chmod +x herdle-<os>-<arch>

# 4. Move onto PATH (example: rename to herdle and place in ~/bin or /usr/local/bin)
mv herdle-<os>-<arch> ~/bin/herdle
```

On **Windows**, rename the downloaded `herdle-windows-amd64` to `herdle.exe`
before placing it on your `PATH` — Windows needs the `.exe` extension to run it.
The Unix steps above also work as-is under Git Bash or WSL.

---

## `herdle init`

Run once after download (and after each upgrade), selecting one or both agent
harnesses:

```sh
herdle init                                   # Claude-compatible default
herdle init --agent claude                    # explicit Claude-only setup
herdle init --agent polytoken                 # Polytoken-only setup
herdle init --agent claude --agent polytoken  # both; --agent is repeatable
```

Repeated names are deduplicated, and selected harnesses are installed in the
order supplied. Bare `herdle init` is intentionally equivalent to
`--agent claude` for backward compatibility. Selection is global only:
`--agent polytoken` writes the user's Polytoken configuration and does not offer
or create a project-local installation.

`herdle init` is idempotent — safe to run multiple times. Claude setup writes:

- `~/.claude/skills/herdle-tk-flow/SKILL.md`
- `~/.claude/skills/herdle-tk-artifacts/SKILL.md`
- `~/.claude/rules/herdle.md`
- a managed lifecycle hook in `~/.claude/settings.json`

Polytoken setup uses `${XDG_CONFIG_HOME:-$HOME/.config}/polytoken` (called
`$POLYTOKEN_CONFIG` below) and writes or merges:

- `$POLYTOKEN_CONFIG/skills/herdle-tk-flow/SKILL.md`
- `$POLYTOKEN_CONFIG/skills/herdle-tk-artifacts/SKILL.md`
- `$POLYTOKEN_CONFIG/herdle.md`
- one named `herdle-gatekeeper` entry in `$POLYTOKEN_CONFIG/hooks.json`
- one `<!-- herdle:begin -->` … `<!-- herdle:end -->` block in
  `$POLYTOKEN_CONFIG/AGENTS.md` that includes `@herdle.md`

The two skills and `herdle.md` are Herdle-owned standalone files. `hooks.json`
and `AGENTS.md` are shared, user-owned files: Herdle updates only its named hook
and marked block and preserves all unrelated entries and bytes. The Polytoken
hook intentionally uses the broad `pre_tool_use` matcher `*`; the gatekeeper
inspects each operation and acts only on relevant lifecycle edits.

After every selected harness succeeds, init seeds
`${XDG_CONFIG_HOME:-$HOME/.config}/herdle/config.toml` once on first run and
migrates `~/.config/wip/projects` if present. A failed multi-harness install does
not seed config. Existing standalone files are left untouched unless `--force`
is passed.

Reload the harness after install or upgrade: use `/reload` in Claude Code, and
start a new Polytoken session (or restart the current Polytoken client) so its
global skills, `AGENTS.md` context, and hooks are reread.

---

## Verify

Run `herdle doctor` with the same selection forms as init:

```sh
herdle doctor                                   # Claude-compatible default
herdle doctor --agent polytoken                 # Polytoken rows only
herdle doctor --agent claude --agent polytoken  # both harnesses
```

Common dependency/config rows are rendered once. Claude adds `superpowers`,
`claude: skills + rule`, and `claude: lifecycle gatekeeper`. Polytoken adds:

- `polytoken: skills + context`
- `polytoken: AGENTS.md link`
- `polytoken: lifecycle gatekeeper`

Doctor verifies the exact installed content, managed context markers, and hook
command. A missing row points to `herdle init --agent polytoken`; stale standalone
content points to `herdle init --agent polytoken --force`; malformed shared files
must be repaired before rerunning init. `herdle doctor` exits non-zero if any
required dependency is missing or configuration is incomplete, making it useful
in scripts and CI.

---

## Upgrade

1. Download the new binary for your platform (see [Download](#download)) and replace the
   existing binary on your `PATH`.
2. Refresh each installed harness:

```sh
herdle init --force                                   # Claude default
herdle init --agent polytoken --force                 # Polytoken only
herdle init --agent claude --agent polytoken --force  # both
```

`--force` overwrites Herdle-owned standalone skill/rule/context files with the
versions embedded in the new binary. Managed shared-file entries are refreshed
surgically. Herdle config (`${XDG_CONFIG_HOME:-$HOME/.config}/herdle`) and unrelated
harness content are not affected. Reload/restart each selected harness afterward.

---

## Uninstall

```sh
herdle init --uninstall                                   # Claude default
herdle init --agent polytoken --uninstall                 # Polytoken only
herdle init --agent claude --agent polytoken --uninstall  # both
```

Claude uninstall removes Herdle's skills/rule and lifecycle hook, never edits
`CLAUDE.md`, and leaves Herdle config in place. Polytoken uninstall removes the
two Herdle skill files and `herdle.md`, removes only the named hook from
`hooks.json`, and removes only the marked block from `AGENTS.md`; the shared
files and unrelated user content remain. Uninstall never reseeds or deletes
`${XDG_CONFIG_HOME:-$HOME/.config}/herdle`.
