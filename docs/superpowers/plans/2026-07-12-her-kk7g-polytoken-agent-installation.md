# Polytoken Agent Installation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add global Polytoken hook, skill, and standing-context installation to Herdle while preserving Claude behavior and lifecycle-gate parity.

**Architecture:** A small `internal/agent` package owns harness names and path resolution. Existing atomic file primitives remain in `internal/initcmd`; harness-specific installers own Claude settings versus Polytoken hooks/context merges. Both hook envelopes normalize into `gate.HookInput`, while harness adapters gather Claude transcript or Polytoken validation-document review evidence for the shared gate policy.

**Tech Stack:** Go 1.26.0/toolchain 1.26.3, urfave/cli v2, Ginkgo v2/Gomega, `embed.FS`, JSON, Markdown Agent Skills, Polytoken hook/config CLI.

## Global Constraints

- Ticket: `her-kk7g`; approved spec: `docs/superpowers/specs/2026-07-12-her-kk7g-polytoken-agent-installation-design.md` at baseline commit `685b84d`.
- Polytoken support is global-only; project-local `.polytoken` installation is excluded.
- Bare `herdle init` and `herdle doctor` remain Claude-only.
- Accepted repeatable `--agent` values are exactly `claude` and `polytoken`; duplicates deduplicate and unknown values fail before writes.
- Standalone files skip user changes unless `--force`; named hook and managed context wiring self-heal on every install.
- Herdle owns only its standalone paths, hook named `herdle-gatekeeper`, and `<!-- herdle:begin -->`/`<!-- herdle:end -->` block.
- New modes: `hooks.json` `0o600`; docs/skills/context `0o644`; parent directories `0o750`; existing modes are preserved.
- Polytoken hook stdin uses `tool_name` and `input`; project root comes from `POLYTOKEN_PROJECT_DIR`, falling back to `POLYTOKEN_PROJECT_PATH`.
- Polytoken user config resolves to `${XDG_CONFIG_HOME:-$HOME/.config}/polytoken`; no new environment override is invented.
- The four exact review markers in the spec are required once each before a forward `pending-validation` transition.
- No new runtime dependencies.

---

### Task 1: Harness Selection and Global Path Resolution

**Files:**
- Create: `internal/agent/agent.go`
- Create: `internal/agent/agent_test.go`
- Modify: `internal/config/io.go:12-50`
- Modify: `internal/config/io_test.go`

**Interfaces:**
- Produces: `type agent.Name string`; constants `agent.Claude`, `agent.Polytoken`; `agent.Parse(values []string) ([]Name, error)`.
- Produces: `config.PolytokenDir() (string, error)`.
- Parse returns `[agent.Claude]` for nil/empty input, preserves first occurrence order, and rejects unknown names.

- [ ] **Step 1: Write failing parser tests**

```go
var _ = Describe("agent.Parse", func() {
    It("defaults to Claude", func() {
        got, err := agent.Parse(nil)
        Expect(err).NotTo(HaveOccurred())
        Expect(got).To(Equal([]agent.Name{agent.Claude}))
    })
    It("preserves order while deduplicating", func() {
        got, err := agent.Parse([]string{"polytoken", "claude", "polytoken"})
        Expect(err).NotTo(HaveOccurred())
        Expect(got).To(Equal([]agent.Name{agent.Polytoken, agent.Claude}))
    })
    It("rejects an unknown harness", func() {
        _, err := agent.Parse([]string{"cursor"})
        Expect(err).To(MatchError(`unknown agent "cursor" (expected claude or polytoken)`))
    })
})
```

- [ ] **Step 2: Run the parser tests and verify RED**

Run: `go tool ginkgo ./internal/agent`
Expected: FAIL because `internal/agent` and `agent.Parse` do not exist.

- [ ] **Step 3: Implement the parser**

```go
package agent

import "fmt"

type Name string

const (
    Claude    Name = "claude"
    Polytoken Name = "polytoken"
)

func Parse(values []string) ([]Name, error) {
    if len(values) == 0 {
        return []Name{Claude}, nil
    }
    seen := map[Name]bool{}
    out := make([]Name, 0, len(values))
    for _, raw := range values {
        n := Name(raw)
        if n != Claude && n != Polytoken {
            return nil, fmt.Errorf("unknown agent %q (expected claude or polytoken)", raw)
        }
        if !seen[n] {
            seen[n] = true
            out = append(out, n)
        }
    }
    return out, nil
}
```

- [ ] **Step 4: Write failing Polytoken path tests**

Add table cases proving `XDG_CONFIG_HOME=/x` resolves `/x/polytoken`, and unset XDG with `HOME=/h` resolves `/h/.config/polytoken`. Follow the environment cleanup pattern already used by `ClaudeDir` tests.

- [ ] **Step 5: Run config tests and verify RED**

Run: `go tool ginkgo ./internal/config`
Expected: FAIL with undefined `config.PolytokenDir`.

- [ ] **Step 6: Implement `PolytokenDir`**

```go
func PolytokenDir() (string, error) {
    base, err := baseDir("XDG_CONFIG_HOME", ".config")
    if err != nil {
        return "", err
    }
    return filepath.Join(base, "polytoken"), nil
}
```

- [ ] **Step 7: Run focused tests and commit**

Run: `go tool ginkgo ./internal/agent ./internal/config`
Expected: PASS.

```bash
git add internal/agent internal/config/io.go internal/config/io_test.go
git commit -m "feat(her-kk7g): add agent selection and Polytoken paths"
```

### Task 2: Harness-Native Embedded Assets

**Files:**
- Modify: `assets/assets.go`
- Move: `assets/skills/herdle-tk-flow/SKILL.md` → `assets/claude/skills/herdle-tk-flow/SKILL.md`
- Move: `assets/skills/herdle-tk-artifacts/SKILL.md` → `assets/claude/skills/herdle-tk-artifacts/SKILL.md`
- Move: `assets/rules/herdle.md` → `assets/claude/rules/herdle.md`
- Create: `assets/polytoken/skills/herdle-tk-flow/SKILL.md`
- Create: `assets/polytoken/skills/herdle-tk-artifacts/SKILL.md`
- Create: `assets/polytoken/herdle.md`
- Modify: `assets/linthelpers_test.go`
- Modify: `assets/lint_test.go`

**Interfaces:**
- Produces: `assets.ClaudeFS fs.FS` rooted at Claude `skills/` and `rules/`.
- Produces: `assets.PolytokenFS fs.FS` rooted at Polytoken `skills/` and `herdle.md`.
- Existing callers of `assets.FS` migrate in later tasks; keep `var FS = ClaudeFS` temporarily to keep intermediate commits green.

- [ ] **Step 1: Write failing dual-tree asset tests**

```go
It("lints both harness trees", func() {
    Expect(lintSkills(assets.ClaudeFS, "rules/herdle.md")).To(BeEmpty())
    Expect(lintSkills(assets.PolytokenFS, "herdle.md")).To(BeEmpty())
})

It("keeps Polytoken assets harness-native", func() {
    err := fs.WalkDir(assets.PolytokenFS, ".", func(p string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() { return err }
        b, readErr := fs.ReadFile(assets.PolytokenFS, p)
        Expect(readErr).NotTo(HaveOccurred())
        text := string(b)
        Expect(text).NotTo(ContainSubstring("CLAUDE.md"), p)
        Expect(text).NotTo(ContainSubstring("TodoWrite"), p)
        Expect(text).NotTo(ContainSubstring("/code-review"), p)
        return nil
    })
    Expect(err).NotTo(HaveOccurred())
})
```

Change `lintSkills(fsys fs.FS)` to `lintSkills(fsys fs.FS, contextPath string)` and validate the supplied context file is non-empty; apply Claude-only `paths:` checking only when `contextPath == "rules/herdle.md"`.

- [ ] **Step 2: Run asset tests and verify RED**

Run: `go tool ginkgo ./assets`
Expected: FAIL because `ClaudeFS` and `PolytokenFS` do not exist.

- [ ] **Step 3: Split the embed roots**

```go
//go:embed claude
var bundle embed.FS

var ClaudeFS = mustSub(bundle, "claude")

//go:embed polytoken
var polytokenBundle embed.FS

var PolytokenFS = mustSub(polytokenBundle, "polytoken")

// FS is the compatibility alias removed after all callers migrate.
var FS = ClaudeFS

func mustSub(fsys fs.FS, dir string) fs.FS {
    sub, err := fs.Sub(fsys, dir)
    if err != nil { panic(err) }
    return sub
}
```

Use two embed variables if Go forbids overlapping declarations with the chosen layout; the exported contract above must remain unchanged.

- [ ] **Step 4: Author native Polytoken skill/context content**

Copy policy sections that are truly shared, then make these exact semantic changes:

- standing rules belong in `AGENTS.md`;
- session work uses `todo_create`, `todo_update`, and `todo_complete`;
- standard and deep review passes use `requesting-code-review` plus reviewer subagents, not Claude slash commands;
- Finalize writes the four exact `## Herdle code review` markers before `pending-validation`;
- the context file points agents to the two Herdle skills and `herdle hook gatekeeper`.

Keep frontmatter `name` and `description`; add no `polytoken` key.

- [ ] **Step 5: Run asset and legacy init tests**

Run: `go tool ginkgo ./assets ./internal/initcmd ./cmd/herdle`
Expected: PASS, with legacy callers using the temporary `assets.FS` alias.

- [ ] **Step 6: Commit**

```bash
git add assets
git commit -m "feat(her-kk7g): add native Polytoken agent assets"
```

### Task 3: Safe Polytoken Hook and Context Installation

**Files:**
- Create: `internal/initcmd/polytoken.go`
- Create: `internal/initcmd/polytoken_test.go`
- Modify: `internal/initcmd/install.go`

**Interfaces:**
- Produces: `initcmd.InstallPolytoken(src fs.FS, dir, command string, force bool) ([]Result, error)`.
- Produces: `initcmd.UninstallPolytoken(src fs.FS, dir string) ([]Result, error)`.
- Internal helpers: `mergePolytokenHooks(path, command string) (Result, error)`, `unmergePolytokenHooks(path string) (Result, error)`, `mergeAgentContext(path string) (Result, error)`, `unmergeAgentContext(path string) (Result, error)`.
- Produces read-only diagnostics: `InspectPolytokenHooks(path string) (PolytokenHookInspection, error)` and `InspectAgentContext(path string) (AgentContextInspection, error)`, with the field contracts in Task 5.
- Reuse exported `Result` actions; errors include the shared-file path.

- [ ] **Step 1: Write failing fresh-install and ownership tests**

Use `fstest.MapFS` with both skills and `herdle.md`. Assert:

```go
results, err := initcmd.InstallPolytoken(src, dir, "/bin/herdle hook gatekeeper --agent polytoken", false)
Expect(err).NotTo(HaveOccurred())
Expect(filepath.Join(dir, "skills", "herdle-tk-flow", "SKILL.md")).To(BeAnExistingFile())
Expect(filepath.Join(dir, "herdle.md")).To(BeAnExistingFile())
Expect(readJSON(filepath.Join(dir, "hooks.json"))).To(ContainElement(HaveKeyWithValue("name", "herdle-gatekeeper")))
Expect(read(filepath.Join(dir, "AGENTS.md"))).To(Equal("<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\n"))
Expect(mode(filepath.Join(dir, "hooks.json"))).To(Equal(os.FileMode(0o600)))
```

Also prepopulate foreign hooks and Markdown and assert they survive install/uninstall.

- [ ] **Step 2: Run installer tests and verify RED**

Run: `go tool ginkgo ./internal/initcmd --focus Polytoken`
Expected: FAIL with undefined `InstallPolytoken`.

- [ ] **Step 3: Implement standalone-file install plus JSON merge**

Parse absent/blank hooks as `[]json.RawMessage{}`. Decode each raw entry enough to read `name`; preserve non-Herdle `json.RawMessage` order. Reject more than one matching name. Replace zero/one matching entry with:

```go
type polytokenHook struct {
    Name    string `json:"name"`
    Event   string `json:"event"`
    Matcher string `json:"matcher"`
    Handler struct { Bash string `json:"bash"` } `json:"handler"`
}
```

Write the complete array atomically with indentation and trailing newline. Use `writeAtomic` with existing mode or `0o600`.

- [ ] **Step 4: Implement marked Markdown merge**

Use constants:

```go
const contextBlock = "<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\n"
const contextBegin = "<!-- herdle:begin -->"
const contextEnd = "<!-- herdle:end -->"
```

Count begin/end markers. Accept only `(0,0)` or `(1,1)` in correct order. Insert with exactly one separating newline, or replace the existing complete block. Reject partial, reversed, nested, or duplicate markers. Preserve all text outside the managed block byte-for-byte and preserve mode/default `0o644`.

- [ ] **Step 5: Add exhaustive ambiguity, force, mode, and uninstall tests**

Table-test malformed JSON, object top-level, duplicate named hooks, partial/duplicate markers, stale command self-healing without force, standalone skip/overwrite, existing modes, absent uninstall, and foreign-content survival.

- [ ] **Step 6: Run focused tests and commit**

Run: `go tool ginkgo ./internal/initcmd`
Expected: PASS.

```bash
git add internal/initcmd/install.go internal/initcmd/polytoken.go internal/initcmd/polytoken_test.go
git commit -m "feat(her-kk7g): install Polytoken hooks and context safely"
```

### Task 4: Polytoken Hook Adapter and Durable Review Gate

**Files:**
- Modify: `internal/gate/gate.go:340-397,442-497`
- Modify: `internal/gate/gate_test.go`
- Modify: `cmd/herdle/hook.go`
- Modify: `cmd/herdle/hook_test.go`

**Interfaces:**
- Replace transcript-only pending evidence in `gate.Env` with `ReviewEvidence gate.ReviewEvidence`.
- Produce:

```go
type ReviewEvidence struct {
    ReadOK       bool
    Required     []string
    Present      map[string]bool
    Unreadable   string
    BlockedIntro string
}

func ClaudeReviewEvidence(r io.Reader, ticketPath string) ReviewEvidence
func PolytokenReviewEvidence(docs []string, found bool) ReviewEvidence
```

- Required Claude keys remain `medium`, `high`; required Polytoken keys are `standard-completed`, `standard-addressed`, `deep-completed`, `deep-addressed`.
- `runGatekeeper(r io.Reader, harness agent.Name, projectDir string) gate.Decision`; CLI defaults hidden hook `--agent` to `claude`. For Polytoken pass `firstNonEmpty(os.Getenv("POLYTOKEN_PROJECT_DIR"), os.Getenv("POLYTOKEN_PROJECT_PATH"))`.

- [ ] **Step 1: Write failing pure evidence tests**

```go
It("requires each Polytoken marker exactly once", func() {
    doc := "## Herdle code review\n\n" +
        "- [x] Standard review completed\n" +
        "- [x] Standard review findings addressed\n" +
        "- [x] Deep review completed\n" +
        "- [x] Deep review findings addressed\n"
    ev := gate.PolytokenReviewEvidence([]string{doc}, true)
    Expect(ev.ReadOK).To(BeTrue())
    Expect(ev.Present).To(HaveLen(4))
})
```

Add table cases removing, unchecking, altering, and duplicating each line. A duplicate must make that key false. Add a test proving fenced examples do not count.

- [ ] **Step 2: Run gate tests and verify RED**

Run: `go tool ginkgo ./internal/gate`
Expected: FAIL with undefined `ReviewEvidence`/`PolytokenReviewEvidence`.

- [ ] **Step 3: Refactor pending policy around abstract evidence**

Implement exact-line scanning outside fenced blocks. `ClaudeReviewEvidence` wraps existing `EffortsFromTranscript` and fills `Required` with `medium, high`, `Unreadable` with the existing `failClosedReason`, and `BlockedIntro` with the existing Claude block prefix. `PolytokenReviewEvidence` fills the four marker keys plus Polytoken-specific unreadable/block prefixes. `decidePending` keeps rollback/idempotent/override logic, denies with `Unreadable` when `ReadOK` is false, computes missing keys in `Required` order, and builds the denial from `BlockedIntro` plus the missing list.

Keep all existing Claude tests passing and reason text unchanged for Claude.

- [ ] **Step 4: Write failing Polytoken envelope tests**

Test `file_edit_search_replace`, `file_write`, and `shell_exec` payloads shaped as:

```json
{"tool_name":"file_edit_search_replace","input":{"path":".tickets/her-x.md","new_string":"lifecycle: in-development\nbranch: feat/x\n"}}
```

Assert relative paths resolve using the supplied project directory. Test malformed payload and irrelevant tool fail open. For pending validation, create a ticket and ticket-correlated validation doc under the temporary project.

- [ ] **Step 5: Implement CLI adapter selection**

Add a hidden `cli.StringFlag{Name: "agent", Value: "claude"}`. Keep Claude parsing in a dedicated function. Add Polytoken raw input with `ToolName string` and `Input` fields `Path`, `NewString`, `Content`, `Command`. Normalize names:

```go
switch raw.ToolName {
case "file_edit_search_replace": in.ToolName = "Edit"
case "file_write": in.ToolName = "Write"
case "shell_exec": in.ToolName = "Bash"
default: return gate.Decision{Allow: true}
}
```

Gather validation docs for `ToPendingValidation` only in the Polytoken branch; Claude continues opening its transcript. Both branches gather ticket/validation evidence needed by the other transitions.

- [ ] **Step 6: Run hook and gate tests and commit**

Run: `go tool ginkgo ./internal/gate ./cmd/herdle`
Expected: PASS.

```bash
git add internal/gate/gate.go internal/gate/gate_test.go cmd/herdle/hook.go cmd/herdle/hook_test.go
git commit -m "feat(her-kk7g): gate Polytoken lifecycle with durable reviews"
```

### Task 5: Harness-Aware Doctor Checks

**Files:**
- Modify: `internal/doctor/doctor.go`
- Modify: `internal/doctor/checks.go`
- Create: `internal/doctor/polytoken.go`
- Modify: `internal/doctor/doctor_test.go`
- Modify: `internal/doctor/export_test.go`

**Interfaces:**
- Add `Agents []agent.Name`, `ClaudeAssets fs.FS`, `PolytokenAssets fs.FS`, `PolytokenDir string`, and `PolytokenHooksPath string` to `doctor.Env`; retain current Claude fields during migration.
- `doctor.Run` executes common checks once, then selected harness checks in selection order.
- Row names are exact spec labels: `claude: skills + rule`, `claude: lifecycle gatekeeper`, `polytoken: skills + context`, `polytoken: AGENTS.md link`, `polytoken: lifecycle gatekeeper`.

- [ ] **Step 1: Write failing doctor composition tests**

Construct healthy Claude-only, Polytoken-only, and dual environments. Assert common `git`, `tk`, and `config` rows occur once; harness rows occur only when selected and in selected order.

- [ ] **Step 2: Run doctor tests and verify RED**

Run: `go tool ginkgo ./internal/doctor`
Expected: FAIL because `Env.Agents` and Polytoken checks do not exist.

- [ ] **Step 3: Split common and harness-specific checks**

Build result slices explicitly rather than one fixed function array. Default empty `Env.Agents` to Claude for compatibility in tests. Prefix existing integrity/gate names and update test lookups.

- [ ] **Step 4: Implement Polytoken diagnostics**

Reuse standalone integrity comparison against `PolytokenAssets`. In Task 3 export two read-only inspectors from `initcmd`: `InspectPolytokenHooks(path string) (PolytokenHookInspection, error)` with `Count int`, `Event`, `Matcher`, and `Command` fields; and `InspectAgentContext(path string) (AgentContextInspection, error)` with `Count int` and `Exact bool`. Both reuse the install parser and ambiguity validation. Doctor calls only these inspectors, verifies event `pre_tool_use`, matcher `*`, current command, and one exact context block, then returns missing=Fail, drift=Warn, malformed/duplicate=Fail with the spec's remediation commands.

- [ ] **Step 5: Add healthy/missing/drifted/malformed/duplicate/stale tests**

Cover every Polytoken row and confirm Claude superpowers scanning does not run or emit a row for Polytoken-only selection.

- [ ] **Step 6: Run doctor tests and commit**

Run: `go tool ginkgo ./internal/doctor`
Expected: PASS.

```bash
git add internal/doctor
git commit -m "feat(her-kk7g): diagnose selected agent installations"
```

### Task 6: CLI Orchestration, Documentation, and End-to-End Verification

**Files:**
- Modify: `cmd/herdle/init.go`
- Modify: `cmd/herdle/init_test.go`
- Modify: `cmd/herdle/doctor.go`
- Modify: `cmd/herdle/doctor_test.go`
- Modify: `cmd/herdle/docs_drift_test.go` only if command-reference parsing needs repeated flags handled differently
- Modify: `assets/assets.go` (remove compatibility `FS` alias)
- Modify: `README.md`
- Modify: `docs/install.md`
- Modify: `docs/usage.md`
- Modify: `docs/tk-conventions.md`
- Create: `docs/superpowers/validation/2026-07-12-her-kk7g-polytoken-agent-installation-validation.md`
- Modify: `.tickets/her-kk7g.md`

**Interfaces:**
- Both commands expose `&cli.StringSliceFlag{Name: "agent", Usage: "agent harness to configure: claude or polytoken (repeatable)"}`.
- Init validates selection before path resolution/writes, runs installers in selected order, prefixes output with harness, and seeds once only after complete success.
- Doctor builds one environment with selected agents and renders one result list.

- [ ] **Step 1: Write failing init CLI tests**

Add cases for bare Claude-only, explicit Polytoken-only, dual install, duplicate agent, unknown-before-write, force/uninstall applying to Polytoken, and partial dual failure preventing config seeding. Assert exact destination paths under temporary `HOME`/`XDG_CONFIG_HOME`.

- [ ] **Step 2: Run init tests and verify RED**

Run: `go tool ginkgo ./cmd/herdle --focus 'herdle init'`
Expected: FAIL because `--agent` is undefined and Polytoken files are absent.

- [ ] **Step 3: Implement init orchestration**

Parse `c.StringSlice("agent")` first. Resolve `os.Executable()` once. Dispatch:

```go
switch name {
case agent.Claude:
    results, err = initcmd.Install(assets.ClaudeFS, claudeDir, force)
    // merge or unmerge Claude settings
case agent.Polytoken:
    command := exe + " hook gatekeeper --agent polytoken"
    results, err = initcmd.InstallPolytoken(assets.PolytokenFS, polytokenDir, command, force)
}
```

Use the corresponding uninstall functions. Prefix result lines with `claude:`/`polytoken:`. Only execute `SeedConfig` after the loop succeeds and skip seeding for uninstall.

- [ ] **Step 4: Write and implement doctor CLI selection tests**

Verify bare, Polytoken-only, dual, duplicate, and unknown selection. Populate `buildDoctorEnv(selected []agent.Name)` with both asset filesystems and paths while keeping common dependencies singletons.

Run: `go tool ginkgo ./cmd/herdle --focus 'herdle doctor'`
Expected after implementation: PASS.

- [ ] **Step 5: Remove the asset compatibility alias and run all focused suites**

Remove `assets.FS`; update every caller to `assets.ClaudeFS` or `assets.PolytokenFS`.

Run: `go tool ginkgo ./assets ./internal/agent ./internal/config ./internal/initcmd ./internal/gate ./internal/doctor ./cmd/herdle`
Expected: PASS.

- [ ] **Step 6: Update user documentation**

Document exact command forms, `${XDG_CONFIG_HOME:-$HOME/.config}/polytoken`, installed paths, managed ownership, `--force`, uninstall, `/reload`/session restart expectations, broad named hook behavior, review markers/order, doctor rows/remediation, and global-only scope. Keep unqualified examples explicitly described as Claude-compatible defaults.

- [ ] **Step 7: Run docs drift tests**

Run: `go tool ginkgo ./cmd/herdle --focus 'docs drift guard'`
Expected: PASS with `--agent` present in the docs corpus and only real commands/flags in the usage table.

- [ ] **Step 8: Validate generated Polytoken configuration with the installed CLI**

Build Herdle and install into a temporary Polytoken config directory:

```bash
go build -o /tmp/herdle-her-kk7g ./cmd/herdle
TMP_HOME=$(mktemp -d)
XDG_CONFIG_HOME="$TMP_HOME/config" HOME="$TMP_HOME" /tmp/herdle-her-kk7g init --agent polytoken
polytoken --config-dir "$TMP_HOME/config/polytoken" config validate --user
polytoken validate skill "$TMP_HOME/config/polytoken/skills/herdle-tk-flow/SKILL.md"
polytoken validate skill "$TMP_HOME/config/polytoken/skills/herdle-tk-artifacts/SKILL.md"
```

Expected: init exits 0; config validation and both skill validations exit 0. Capture the actual temporary path in the validation document without committing temporary files.

- [ ] **Step 9: Run full repository verification**

Run: `make all`
Expected: `go vet`, gofmt check, staticcheck, gosec, all race-enabled Ginkgo suites, and build exit 0 with no pending/empty suite failures.

- [ ] **Step 10: Write validation evidence and stamp lifecycle**

Create the validation file with this structure and fill command outputs from Steps 8–9:

```markdown
# Polytoken agent installation validation

## Herdle code review

- [ ] Standard review completed
- [ ] Standard review findings addressed
- [ ] Deep review completed
- [ ] Deep review findings addressed

## Automated

- [x] Focused Go suites pass
- [x] `make all` passes
- [x] Polytoken user configuration validates
- [x] Both Polytoken skills validate

## Human

- [ ] Start/reload a Polytoken session and confirm the Herdle context is visible
- [ ] Attempt each lifecycle transition and confirm the displayed denial/remediation is clear
```

Do not check review markers until the dedicated Code Review task during execution completes. After both reviews and findings are addressed, check all four markers, then set ticket `lifecycle: pending-validation`. Leave human boxes open and do not set `validated`.

- [ ] **Step 11: Commit implementation/docs/validation**

```bash
git add cmd/herdle assets README.md docs/install.md docs/usage.md docs/tk-conventions.md
git add -f docs/superpowers/validation/2026-07-12-her-kk7g-polytoken-agent-installation-validation.md .tickets/her-kk7g.md
git commit -m "feat(her-kk7g): integrate Polytoken agent setup"
```

### Task 7: Code Review

**Files:**
- Modify: any Task 1–6 implementation or test file named by a confirmed review finding.
- Modify: `docs/superpowers/validation/2026-07-12-her-kk7g-polytoken-agent-installation-validation.md`

**Interfaces:**
- Consumes the complete implementation branch and validation document from Tasks 1–6.
- Produces two completed review passes and addressed findings, recorded with the four exact Herdle markers.

- [ ] **Step 1: Invoke the requesting-code-review skill for the standard pass**

Review the complete branch against the approved spec and this plan. Dispatch a fresh reviewer with the branch base, head, acceptance criteria, and full changed-file list. Fix every confirmed finding and rerun affected tests.

- [ ] **Step 2: Invoke the requesting-code-review skill for the deep pass**

Use a fresh reviewer and explicitly scrutinize shared-file corruption handling, uninstall ownership, partial failures, hook fail-open/fail-closed boundaries, review-marker parsing, file modes, and Claude regressions. Fix every confirmed finding and rerun affected tests.

- [ ] **Step 3: Record durable review evidence**

Change exactly these lines in the validation document:

```markdown
- [x] Standard review completed
- [x] Standard review findings addressed
- [x] Deep review completed
- [x] Deep review findings addressed
```

Add reviewer/finding notes below the fixed markers without changing marker text.

- [ ] **Step 4: Re-run full verification and commit review fixes**

Run: `make all`
Expected: PASS.

Run the temporary Polytoken validation commands from Task 6 Step 8 again.
Expected: all exit 0.

```bash
git add -A
git add -f .tickets/her-kk7g.md docs/superpowers/validation/2026-07-12-her-kk7g-polytoken-agent-installation-validation.md
git commit -m "fix(her-kk7g): address Polytoken integration review"
```

### Task 8: Finalize

**Files:**
- Modify: `.tickets/her-kk7g.md`
- Modify: `docs/superpowers/validation/2026-07-12-her-kk7g-polytoken-agent-installation-validation.md` if automated evidence changed.

**Interfaces:**
- Consumes reviewed, verified implementation and checked review markers.
- Produces a ticket at `pending-validation` with human validation intentionally open.

- [ ] **Step 1: Run final evidence commands**

Run: `make all`
Expected: PASS.

Set `TMP_HOME` to the temporary home created by Task 6 Step 8. Run `polytoken --config-dir "$TMP_HOME/config/polytoken" config validate --user`, `polytoken validate skill "$TMP_HOME/config/polytoken/skills/herdle-tk-flow/SKILL.md"`, and `polytoken validate skill "$TMP_HOME/config/polytoken/skills/herdle-tk-artifacts/SKILL.md"`.
Expected: all three validation commands exit 0.

- [ ] **Step 2: Set lifecycle to pending validation**

Confirm the validation file contains all four exact checked review markers. Edit `.tickets/her-kk7g.md` to `lifecycle: pending-validation`. The installed/current gate must allow the transition based on the durable document.

- [ ] **Step 3: Keep human checks open and prepare branch completion**

Do not set `validated`, do not open a PR, and do not push. Confirm the two human validation boxes remain unchecked. Use the `finishing-a-development-branch` skill; per repository policy, merge back to `main` without interrupting the primary repository's branch/worktree and do not offer a PR or push.

- [ ] **Step 4: Commit final lifecycle evidence**

```bash
git add -f .tickets/her-kk7g.md docs/superpowers/validation/2026-07-12-her-kk7g-polytoken-agent-installation-validation.md
git commit -m "chore(her-kk7g): mark Polytoken setup pending validation"
```
