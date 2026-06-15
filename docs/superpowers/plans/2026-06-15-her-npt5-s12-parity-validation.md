# S12: Output-Parity Validation Gate Implementation Plan

> **For agentic workers:** This is a **validation story**, not a feature. It produces no
> production code unless a port bug is found in triage (Task 5). Several steps require
> **interactive human visual sign-off** and the author's live environment (`wip`,
> `~/workspace`, `~/workspace/dcs-retribution`) — they cannot be delegated to a headless
> subagent or run in CI. Execute **inline** with the user. Steps use checkbox (`- [ ]`)
> syntax for tracking.

**Goal:** Prove `herdle`'s rendered dashboard matches the personal `wip` bash dashboard
(layout + ANSI colors) for the two reference scenarios, and record the go/no-go in a
committed validation document.

**Architecture:** One-off acceptance test. A throwaway script forces color (`FORCE_COLOR=1`,
honored identically by both tools) and captures `wip` vs `herdle` output for the summary
(`--all` over `~/workspace`) and drilldown (`dcs-retribution`) scenarios into scratch
files. The human eyeballs each pair for a pass/fail verdict. Only the validation doc is
committed — never the harness or captures.

**Tech Stack:** Go (`make build`), bash (throwaway harness), `wip` (`~/bin/wip`), `tk`.

**Spec:** `docs/superpowers/specs/2026-06-15-her-npt5-s12-parity-validation-design.md`

**Sandbox note (critical):** `herdle init` writes skills/rules into the real `~/.claude`,
which would clobber the user's live personal skills. To avoid that, every herdle invocation
in this plan overrides both `HERDLE_CONFIG` (config file) and `CLAUDE_CONFIG_DIR` (where
skills/rules land) to scratch paths under `/tmp/herdle-parity/`. `wip` projects are read
from the real `$HOME/.config/wip/projects` (unaffected by those overrides), so the migrated
project set matches what `wip` sees.

---

### Task 1: Setup

**Files:**
- Modify: `.tickets/her-npt5.md` (frontmatter)

- [ ] **Step 1: Start the ticket**

Run: `tk start her-npt5`
Expected: status → `in_progress`.

- [ ] **Step 2: Branch off main**

```bash
git switch -c her-npt5-s12-parity-validation
```

- [ ] **Step 3: Record branch + lifecycle on the ticket**

Edit `.tickets/her-npt5.md` frontmatter: change `lifecycle: designed` → `lifecycle: in-development`
and add a line `branch: her-npt5-s12-parity-validation`.

- [ ] **Step 4: Build a fresh herdle binary at HEAD**

Run: `make build`
Expected: `go build` succeeds; `./herdle` is rebuilt. Confirm it is current:
`./herdle version` (should reflect the current dirty/SHA build, not the stale `4cee549`).

- [ ] **Step 5: Confirm preconditions**

```bash
test -x ~/bin/wip && echo "wip present"
test -d ~/workspace && echo "workspace present"
test -d ~/workspace/dcs-retribution && echo "dcs present"
test -f ~/.config/wip/projects && echo "wip projects present"
```
Expected: all four print. If any fails, stop — the gate cannot run.

- [ ] **Step 6: Commit the lifecycle/branch bookkeeping**

```bash
git add .tickets/her-npt5.md
git commit -m "chore(her-npt5): start S12 parity validation branch"
```

---

### Task 2: Capture parity outputs (throwaway harness)

**Files:**
- Create (scratch, NOT committed): `/tmp/herdle-parity/parity.sh` and capture files under `/tmp/herdle-parity/`

- [ ] **Step 1: Write the throwaway harness**

Write the following to `/tmp/herdle-parity/parity.sh`. It seeds a sandboxed herdle config by
migrating the real `wip` projects (writing skills only into the scratch `CLAUDE_CONFIG_DIR`,
never `~/.claude`), then captures all four outputs with color forced and prints a reference
`diff` per scenario.

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

- [ ] **Step 2: Run the harness**

```bash
bash /tmp/herdle-parity/parity.sh
```
Expected: prints the seeded config (project set matching `~/.config/wip/projects`), then the
two `diff -u` blocks. Note: ANSI escapes will be visible in the diff — that is intended
(we are validating colors too).

- [ ] **Step 3: Sanity-check the project sets match**

Confirm the seeded `config.toml` covers the same projects `wip` lists. If a project appears
in one tool but not the other, note it — that is an environment difference to call out in
the validation doc, not necessarily a port bug.

*(No commit — these are scratch artifacts.)*

---

### Task 3: Summary scenario sign-off

**Files:** none committed in this task.

- [ ] **Step 1: Display the summary pair to the user**

```bash
cd ~/workspace
echo "=== wip --all ===";    cat /tmp/herdle-parity/wip-summary.txt
echo "=== herdle --all ==="; cat /tmp/herdle-parity/herdle-summary.txt
```
(Both already color-forced; the terminal renders the ANSI.)

- [ ] **Step 2: Eyeball layout + colors with the user**

Compare, top to bottom: section headers, column widths/alignment, row ordering, state
labels and their colors, sync indicators, truncation/padding. The `diff -u` from Task 2 is
the aid; the verdict is the human's.

- [ ] **Step 3: Record the summary verdict**

Note PASS or FAIL for the summary scenario plus any specific differences observed (to be
written into the validation doc in Task 6). If FAIL on a port bug, it is fixed in Task 5.

---

### Task 4: Drilldown scenario sign-off

**Files:** none committed in this task.

- [ ] **Step 1: Display the drilldown pair to the user**

```bash
echo "=== wip (dcs-retribution) ===";    cat /tmp/herdle-parity/wip-drilldown.txt
echo "=== herdle (dcs-retribution) ==="; cat /tmp/herdle-parity/herdle-drilldown.txt
```

- [ ] **Step 2: Eyeball layout + colors with the user**

Compare the four drilldown sections (open PRs, merged-needing-cleanup, work-in-progress,
up-next): presence, column alignment, tk⇄branch⇄PR correlation, lifecycle/sync columns,
colors, pruning behavior, and the hidden-when-empty rule.

- [ ] **Step 3: Record the drilldown verdict**

Note PASS or FAIL plus specific differences. FAIL on a port bug → Task 5.

---

### Task 5: Triage port bugs (only if a scenario failed)

**Files:**
- Modify (as diagnosis dictates): `internal/render/*.go` and/or `internal/dashboard/*.go`

Skip this task entirely if both scenarios passed in Tasks 3–4.

- [ ] **Step 1: Classify each difference**

For every difference, decide: **port bug** (herdle should match `wip`) or **intentional
divergence** (follows from de-personalization or environment — e.g. config path, a project
only one tool knows about). Intentional divergences need only a justification (recorded in
Task 6); port bugs proceed below.

- [ ] **Step 2: Locate and fix the responsible code**

Layout/column/color issues live in `internal/render`; data/classification/ordering issues
(branch state, correlation, pruning) live in `internal/dashboard`. Use
`superpowers:systematic-debugging` to find root cause before editing. Where a render fix is
made, add or update a golden-file case in `internal/render/testdata/` so the regression is
captured in the committed Ginkgo suite (the long-term surface).

- [ ] **Step 3: Rebuild and re-run the affected capture**

```bash
make build && bash /tmp/herdle-parity/parity.sh
```
Expected: the previously-failing scenario now matches under the human's eye.

- [ ] **Step 4: Run the unit suite**

Run: `PATH=".venv/bin:$PATH" rtk go test ./...` is N/A here (Go, no venv) — use:
`rtk go test ./...`
Expected: all Ginkgo specs pass (including any new golden case).

- [ ] **Step 5: Commit each fix**

```bash
git add internal/render internal/dashboard
git commit -m "fix(her-npt5): <specific parity divergence fixed>"
```
Repeat Tasks 3–4 sign-off for the fixed scenario until green.

---

### Task 6: Write and commit the validation document

**Files:**
- Create: `docs/superpowers/validation/2026-06-15-her-npt5-s12-parity-validation.md`
- Modify: `.tickets/her-npt5.md` (frontmatter)

- [ ] **Step 1: Write the validation doc**

Create `docs/superpowers/validation/2026-06-15-her-npt5-s12-parity-validation.md` with these
sections (fill from the actual run — no placeholders):

- **Environment:** herdle git SHA (`git rev-parse HEAD`), `herdle version`, date, host/OS,
  `FORCE_COLOR=1`, sandbox paths (`HERDLE_CONFIG`, `CLAUDE_CONFIG_DIR`).
- **Commands run:** the full `/tmp/herdle-parity/parity.sh` contents inlined (so the run is
  reproducible without the committed script) plus the display commands from Tasks 3–4.
- **Project set:** confirmation the seeded config matched `~/.config/wip/projects`, or a
  note of any divergence.
- **Summary scenario:** PASS/FAIL verdict + notes + a short captured excerpt as evidence.
- **Drilldown scenario:** PASS/FAIL verdict + notes + a short captured excerpt.
- **Differences:** each one classified — port bug (→ fixing commit SHA) or intentional
  divergence (→ one-line justification).
- **Go / No-Go:** final statement. Green only when both scenarios sign off.

- [ ] **Step 2: Commit the validation doc (force-add past .git/info/exclude)**

`docs/superpowers/` is listed in `.git/info/exclude`, so a plain `git add` is refused — use
`-f`, consistent with every prior spec/plan in this repo.

```bash
git add -f docs/superpowers/validation/2026-06-15-her-npt5-s12-parity-validation.md
git commit -m "docs(her-npt5): S12 parity validation results"
```

- [ ] **Step 3: Mark the ticket validated (only on a green go)**

Edit `.tickets/her-npt5.md` frontmatter: `lifecycle: in-development` → `lifecycle: validated`.
Then close it: `tk close her-npt5`.
**Do NOT touch the epic `her-x9jl`** — it stays open until S10 (her-mg2c) is also validated.

```bash
git add .tickets/her-npt5.md
git commit -m "chore(her-npt5): mark S12 validated"
```

---

### Task 7: Finalize

- [ ] **Step 1: Code review (only if Task 5 changed production code)**

If any `internal/render` / `internal/dashboard` code was modified in triage, dispatch a
subagent running `/code-review her-npt5-s12-parity-validation medium --fix`, then a second
running `/code-review her-npt5-s12-parity-validation high --fix`. If no production code
changed (doc-only outcome), **skip this step** and note "no code changed; review N/A" in the
finalize summary.

- [ ] **Step 2: Squash the branch commits into one**

```bash
git reset --soft $(git merge-base HEAD main)
git commit -m "feat(her-npt5): S12 output-parity validation gate"
```
(Body: summarize the go/no-go and any port-bug fixes.)

- [ ] **Step 3: Confirm the tree is clean and tests pass**

Run: `rtk go test ./...`
Expected: green.

- [ ] **Step 4: Hand off the merge decision**

This story is already validated by its own nature (the gate sign-off), so lifecycle stays
`validated` (set in Task 6) rather than `pending-validation`. **Do NOT open a PR.** Per this
repo's CLAUDE.md, finishing a branch means merging back to `main` without disrupting any
in-progress work — invoke `superpowers:finishing-a-development-branch` to handle the merge.

---

## Self-Review

- **Spec coverage:** two scenarios (Tasks 3–4) ✓; color-forced via `FORCE_COLOR=1` (Task 2)
  ✓; manual visual sign-off (Tasks 3–4) ✓; equal-inputs precondition via sandboxed migrate
  (Task 2) ✓; nothing committed but the validation doc (Tasks 2 scratch, 6 commit) ✓;
  port-bug vs intentional-divergence triage (Task 5) ✓; her-npt5 → validated, epic left
  open (Task 6 Step 3) ✓; no PR (Task 7) ✓.
- **Placeholders:** none — `<specific parity divergence fixed>` and the validation-doc
  field values are run-dependent by design, not unspecified plan content.
- **Type consistency:** N/A (no new code types); env var names (`HERDLE_CONFIG`,
  `CLAUDE_CONFIG_DIR`, `FORCE_COLOR`) verified against `internal/config/io.go` and
  `internal/render/color.go`.
- **Deviation from TDD template:** intentional — a validation story writes no production
  code on the happy path; the only test work is golden-file updates *if* a render port bug
  surfaces (Task 5 Step 2).
