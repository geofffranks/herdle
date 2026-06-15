# S12: Output-Parity Validation Results (her-npt5)

**Epic:** her-x9jl — herdle: shareable tk-driven workflow tooling
**Spec:** `docs/superpowers/specs/2026-06-15-her-npt5-s12-parity-validation-design.md`
**Plan:** `docs/superpowers/plans/2026-06-15-her-npt5-s12-parity-validation.md`
**Date:** 2026-06-15
**Verdict:** ✅ **GO** — dashboard port validated. No port bugs found.

## Environment

| Field | Value |
|---|---|
| herdle git SHA | `94f3e274fd834c2b276ec798d2442e2c911712be` (branch `her-npt5-s12-parity-validation`) |
| herdle version | `herdle 62c015b-dirty` (built via `make build` at HEAD) |
| wip | `~/bin/wip` (md5 `f362984d20bd06356f93a4f4dec59900`) |
| Host / OS | Darwin 25.5.0 arm64 |
| Color | `FORCE_COLOR=1` (honored identically by both: wip `~/bin/wip:25-26`, herdle `internal/render/color.go`) |
| Sandbox config | `HERDLE_CONFIG=/tmp/herdle-parity/config.toml` |
| Sandbox claude dir | `CLAUDE_CONFIG_DIR=/tmp/herdle-parity/claude` (kept `herdle init` away from the real `~/.claude`) |

The sandbox overrides ensured `herdle init` never wrote into the live `~/.claude`;
`wip` projects were read from the real `~/.config/wip/projects`, so both tools saw
the same project set.

## Commands run

Throwaway harness (`/tmp/herdle-parity/parity.sh`, not committed — reproduced here):

```bash
#!/usr/bin/env bash
set -euo pipefail

SB=/tmp/herdle-parity
mkdir -p "$SB/claude"
export FORCE_COLOR=1
export HERDLE_CONFIG="$SB/config.toml"
export CLAUDE_CONFIG_DIR="$SB/claude"     # sandbox: keep init away from real ~/.claude

HERDLE="$(git -C "$HOME/workspace/herdle" rev-parse --show-toplevel)/herdle"

# Seed sandbox config from the real wip projects (idempotent).
"$HERDLE" init >/dev/null
echo "== seeding done; config: =="; cat "$HERDLE_CONFIG"

# --- Summary scenario ---
( cd "$HOME/workspace" && wip --all )    > "$SB/wip-summary.txt"     2>&1 || true
( cd "$HOME/workspace" && "$HERDLE" --all ) > "$SB/herdle-summary.txt" 2>&1 || true

# --- Drilldown scenario ---
( cd "$HOME/workspace/dcs-retribution" && wip )    > "$SB/wip-drilldown.txt"     2>&1 || true
( cd "$HOME/workspace/dcs-retribution" && "$HERDLE" ) > "$SB/herdle-drilldown.txt" 2>&1 || true

echo; echo "===== SUMMARY diff (wip -> herdle) ====="
diff -u "$SB/wip-summary.txt" "$SB/herdle-summary.txt" || true
echo; echo "===== DRILLDOWN diff (wip -> herdle) ====="
diff -u "$SB/wip-drilldown.txt" "$SB/herdle-drilldown.txt" || true
```

Follow-up demonstrating the designed remedy for the personal-branch divergence:

```bash
export FORCE_COLOR=1 HERDLE_CONFIG=/tmp/herdle-parity/config.toml CLAUDE_CONFIG_DIR=/tmp/herdle-parity/claude
herdle project set /Users/gfranks/workspace/dcs-retribution --integration geoff-main
( cd ~/workspace/dcs-retribution && herdle ) > /tmp/herdle-parity/herdle-drilldown-cfg.txt 2>&1
diff -u /tmp/herdle-parity/wip-drilldown.txt /tmp/herdle-parity/herdle-drilldown-cfg.txt
```

## Project set

The sandbox config seeded from `~/.config/wip/projects` covered exactly the 7
projects `wip` lists (dcs-retribution, dcs-retribution-remote,
DCS-World-Dedicated-Server-Docker, dcs-kneeboards, ha-configs, rookies-bot,
lm-studio-mcp-server). No divergence in the project set.

## Scenario 1 — Summary (`wip --all` vs `herdle --all` over `~/workspace`)

**Verdict: ✅ PASS.** All project rows, columns, alignment, and colors identical.
The only diff is the footer self-reference:

```diff
-(cached — wip --fetch to refresh)  tk = in-progress/ready · run "wip <name>" for detail
+(cached — herdle --fetch to refresh)  tk = in-progress/ready · run "herdle <name>" for detail
```

→ **Intentional divergence.** herdle correctly names its own command.

## Scenario 2 — Drilldown (`wip` vs `herdle` in `~/workspace/dcs-retribution`)

**Verdict: ✅ PASS.** Open-PRs, work-in-progress, up-next, and design-artifacts
sections all render with identical rows, columns, correlation, lifecycle/sync
indicators, and ANSI colors. Two diffs, both intentional de-personalization:

1. **Legend wording** — `local==origin` / `origin auto-pruned` →
   `local==remote` / `remote auto-pruned`.
   ```diff
   -sync: ✓ local==origin · ✗ differs (see issues) · · n/a — merged-PR & upstream-gone branches hidden, origin auto-pruned
   +sync: ✓ local==remote · ✗ differs (see issues) · · n/a — merged-PR & upstream-gone branches hidden, remote auto-pruned
   ```
   → **Intentional divergence.** wip hardcodes the remote name `origin`; herdle
   uses generic "remote" wording because the remote name is config-driven.

2. **Extra `geoff-main` WIP row** — herdle initially listed the checked-out
   `geoff-main` branch as a "no tk" work-in-progress row; wip omits it.
   ```diff
   +  -                   ✓    -         geoff-main                                                              no tk
   ```
   → **Intentional divergence (de-personalization).** wip excludes the author's
   personal branches via a hardcoded `grep -vxE 'dev|main|master|geoff-main|origin|HEAD'`
   (`~/bin/wip:284`). herdle deliberately does **not** hardcode personal branch
   names — `internal/config/resolve.go`: `// integration: explicit only (personal
   branch; never autodetected)`; `internal/dashboard/drilldown.go`: `(De-personalized:
   no hardcoded dev/geoff-main.)`. This is the master design's central
   de-personalization precondition working as specified.

   **Remedy demonstrated:** running
   `herdle project set <dcs-retribution> --integration geoff-main` and re-rendering
   makes the row disappear, leaving diff #1 (legend wording) as the *only*
   remaining difference — i.e. the drilldown **body is byte-identical** once the
   personal branch is configured per-project.

## Differences summary

| # | Scenario | Difference | Classification |
|---|---|---|---|
| 1 | Summary | footer `wip`→`herdle` self-reference | Intentional (tool name) |
| 2 | Drilldown | legend `origin`→`remote` | Intentional (de-personalization: configurable remote) |
| 3 | Drilldown | `geoff-main` WIP row | Intentional (de-personalization: no hardcoded personal branches); resolved by `--integration geoff-main` |

**Port bugs: none.** No `internal/render` or `internal/dashboard` changes were
required.

## Go / No-Go

✅ **GO.** `herdle` reproduces `wip`'s dashboard output — layout and ANSI colors —
for both reference scenarios. Every observed difference is the designed
de-personalization behavior, not a port defect. The dashboard port (S4/S5/S6) is
validated.

The epic `her-x9jl` remains **open**: S10 (release pipeline, her-mg2c) is still
open and must also be validated before the epic is closed. S12 validates the
dashboard port only.
