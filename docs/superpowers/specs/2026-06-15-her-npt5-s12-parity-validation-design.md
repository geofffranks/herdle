# S12: Output-Parity Validation Gate (her-npt5)

**Epic:** her-x9jl — herdle: shareable tk-driven workflow tooling
**Depends on:** her-fogw (S4 summary), her-ju9h (S5 drilldown), her-6hgc (S6 degradation/remote)
**Date:** 2026-06-15

## Goal

Run the dashboard go/no-go gate from the epic spec: prove `herdle`'s rendered
output matches the personal `wip` bash dashboard (layout *and* ANSI colors) for
the two reference scenarios, color forced. This is the functional/format
acceptance check that lets the dashboard port be called validated.

This is a **validation story, not a feature**. It produces no shipped code and
no committed harness — only a committed validation document recording the run.

## Nature of the check

The epic spec already designates the `wip`-vs-`herdle` diff as a *bootstrap*
check, explicitly **not** the long-term regression surface. The committed,
long-term tests are the Ginkgo + Counterfeiter specs and the render golden
files already delivered in S4/S5/S6. S12 is a **one-off acceptance test** run
against the author's live environment, where `wip` and `~/workspace` actually
exist. It cannot run in CI (no `wip`, drifting repo state) and is not intended
to.

## Scenarios

Exactly the two named in the epic spec — no edge-mode sweep (those modes are
already Ginkgo-covered):

1. **Summary:** `wip --all` vs `herdle --all` over `~/workspace`.
2. **Drilldown:** `wip` vs `herdle` inside `~/workspace/dcs-retribution`.

## Method

### Color forcing

Both tools honor the same gate — verified: `wip` (`~/bin/wip`, lines 25–26) and
`herdle` (`internal/render/color.go`) each enable color when
`CLICOLOR_FORCE`/`FORCE_COLOR` is set and `NO_COLOR` is unset. So a single
`FORCE_COLOR=1` in the environment forces color identically for both — no pty
trickery (`script`/`unbuffer`) required, even when output is redirected to a
file.

### Throwaway harness

A scratch script (e.g. `/tmp/parity.sh`) — **not committed** — performs four
captures into scratch files (e.g. under `/tmp`):

| Capture | Command (env: `FORCE_COLOR=1`) |
|---|---|
| `wip-summary` | `wip --all` run from `~/workspace` |
| `herdle-summary` | `herdle --all` run from `~/workspace` |
| `wip-drilldown` | `wip` run from `~/workspace/dcs-retribution` |
| `herdle-drilldown` | `herdle` run from `~/workspace/dcs-retribution` |

For each scenario the harness prints the two captures and a `diff -u` of the
pair for reference. The exact commands are reproduced verbatim in the
validation doc so the run is reproducible without committing the script.

### Precondition: equal inputs

The comparison is only fair if both tools see the same project set and config.
`wip` reads `~/.config/wip/projects`; `herdle` reads its own migrated config.
Before running, confirm `herdle`'s config has been migrated/initialized so the
configured project set matches `wip`'s. If the sets diverge, note it in the doc
— a project present in one but not the other is an environment difference, not
a port bug.

## Pass criterion

**Manual visual sign-off.** For each scenario, eyeball layout and colors and
record a pass/fail judgment plus notes in the validation doc. There is no
mechanical assertion gate — the `diff -u` output is an aid to the eye, not the
verdict.

A difference is one of:

- **Port bug** — a layout/color/data divergence that should match `wip`. Triage
  to a fix in `internal/render` or `internal/dashboard` before the gate passes.
- **Intentional divergence** — a difference that follows from de-personalization
  or environment (e.g. config path, a project only one tool knows about).
  Record it with a one-line justification; it does not fail the gate.

## Deliverables

- **Committed:** `docs/superpowers/validation/2026-06-15-her-npt5-s12-parity-validation.md`
  containing:
  - environment header — herdle git SHA, date, host, `FORCE_COLOR=1`, herdle
    version;
  - the exact commands run (the throwaway harness contents inlined);
  - per-scenario verdict (pass/fail) with notes;
  - any differences found, each classified port-bug (→ fix + commit reference)
    or intentional-divergence (→ justification);
  - short captured excerpts as evidence where useful (not the full captures);
  - final go/no-go statement.
- **Not committed:** the harness script and the raw capture files. They are
  scratch artifacts of a one-off run.

## Triage loop

If a scenario fails on a port bug: fix the responsible component
(`internal/render` / `internal/dashboard`), rebuild `herdle`, re-run the
affected capture, and confirm match. Record the fix's commit reference in the
validation doc. Repeat until both scenarios sign off green.

## Close-out

On green, set `her-npt5` `lifecycle: validated` (status closed per the repo's
branch-finish convention). **Leave the epic `her-x9jl` open** — S10 (release
pipeline, her-mg2c) is still open, so the epic is not fully validated until it
is too. S12 validates the dashboard port only.

## Out of scope

- Any committed parity harness or golden capture (the epic's long-term surface
  is the existing Ginkgo specs).
- CI integration of the diff (cannot run without `wip` + the author's
  workspace).
- Edge-mode scenarios beyond the two named (already Ginkgo-covered).
- Closing or validating the epic.
