# Installing herdle

herdle is a self-contained CLI binary. Download a prebuilt release, run
`herdle init`, and you're done. No package manager required.

---

## Dependency contract

| Dependency | Kind | How to get it |
|---|---|---|
| `tk` (wedow/ticket) | required | `brew install wedow/tools/ticket` |
| `git` | required | system / `brew install git` |
| [superpowers](https://github.com/obra/superpowers) plugin | required (for the skills/rules to mean anything) | add its marketplace + `/plugin install` |
| `gh` (authenticated) | optional | `brew install gh && gh auth login` — enables PR/issue features |
| GitHub-hosted remote | optional | enables PR/issue features per-project |
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

Run once after download (and after each upgrade):

```sh
herdle init
```

`herdle init` is idempotent — safe to run multiple times. It:

- Writes convention skills → `~/.claude/skills/` (`herdle-tk-flow/`,
  `herdle-tk-artifacts/`)
- Writes a rule stub → `~/.claude/rules/herdle.md`
- Seeds `~/.config/herdle/config.toml` on first run
- Migrates `~/.config/wip/projects` if present (imports existing project list)

Existing files are left untouched unless `--force` is passed.

---

## Verify

Run `herdle doctor` to confirm all dependencies and files are in order:

```sh
herdle doctor
```

Sample output:

```
herdle 734e250-dirty

  ✓ git            found
  ✓ tk             found
  ✓ gh             found
  ✓ gh auth        authenticated
  ✓ superpowers    found under /Users/gfranks/.claude/plugins
  ✓ herdle on PATH on PATH as herdle
  ✗ skills + rule  3 missing: /Users/gfranks/.claude/rules/herdle.md, /Users/gfranks/.claude/skills/herdle-tk-artifacts/SKILL.md, /Users/gfranks/.claude/skills/herdle-tk-flow/SKILL.md
      → run: herdle init
  ✗ config         not found at /Users/gfranks/.config/herdle/config.toml
      → run: herdle init
herdle doctor: 2 check(s) need attention
```

`herdle doctor` exits non-zero if any required dependency is missing or
configuration is incomplete. Run `herdle init` to resolve the items flagged with
`✗`.

---

## Upgrade

1. Download the new binary for your platform (see [Download](#download)) and replace the
   existing binary on your `PATH`.
2. Re-lay the skills and rules:

```sh
herdle init --force
```

`--force` overwrites previously installed skill and rule files with the versions
embedded in the new binary. Config (`~/.config/herdle/`) is not affected.

---

## Uninstall

```sh
herdle init --uninstall
```

This removes only the skill and rule files herdle installed
(`~/.claude/skills/herdle-*/` and `~/.claude/rules/herdle.md`). It never edits
`CLAUDE.md` and leaves `~/.config/herdle/` in place.
