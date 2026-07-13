# Polytoken agent installation design

**Ticket:** `her-kk7g`
**Date:** 2026-07-12
**Status:** approved

## Summary

Extend Herdle's install-once agent setup to support Polytoken alongside Claude
Code. The first Polytoken integration is global-only. It installs
Polytoken-native Herdle skills, a named lifecycle gate hook, and a Herdle-owned
context file linked from Polytoken's global `AGENTS.md`.

The CLI uses a repeatable `--agent` selector. Bare `herdle init` and
`herdle doctor` retain their current Claude-only behavior. Shared installer
primitives handle atomic writes and result reporting, while narrow harness
adapters own schema-specific paths, assets, merges, hook payloads, and integrity
checks.

Polytoken review enforcement uses durable, ticket-correlated review markers in
the validation document rather than relying on undocumented transcript
internals.

## Goals

- Install Herdle's hooks, skills, and standing context for a global Polytoken
  agent configuration.
- Preserve backward compatibility for existing Claude users and commands.
- Give Claude and Polytoken native instructions rather than ambiguous universal
  prose.
- Preserve all unrelated user content in shared configuration files.
- Make install, refresh, uninstall, and doctor idempotent and explicit about
  ownership.
- Preserve the three lifecycle gates for Polytoken using documented interfaces
  and durable evidence.

## Non-goals

- Project-local `.polytoken` installation in this release.
- A public or dynamically loaded harness plugin system.
- Automatic harness detection.
- Installing into every project tracked by Herdle.
- Proving cryptographically that a particular model or subagent performed a
  review.
- Making the Claude and Polytoken skill files byte-identical.

## User experience

### Harness selection

`init` and `doctor` gain a repeatable `--agent` flag:

```text
herdle init                                  # Claude only
herdle init --agent polytoken                # Polytoken only
herdle init --agent claude --agent polytoken # both

herdle doctor                                  # Claude checks
herdle doctor --agent polytoken                # Polytoken checks
herdle doctor --agent claude --agent polytoken # both
```

Accepted values are `claude` and `polytoken`. Repeated duplicate values are
deduplicated. Any unknown value is rejected before files are changed. Existing
`--force` and `--uninstall` flags apply to every selected harness.

Herdle config seeding is harness-independent. It runs once, and only after all
selected harness installers succeed. Uninstall never removes Herdle's project
config.

### Output and partial failure

Output identifies the harness for every installed, overwritten, skipped, or
removed artifact. A multi-harness operation is not globally transactional. If
one harness succeeds and a later harness fails, Herdle reports both outcomes,
returns a non-zero status, and does not roll back valid earlier writes. This is
safer than attempting cross-file rollback over user-owned configuration.

## Architecture

### Narrow harness adapters

The init command resolves selected harnesses and invokes one installer per
harness. The interface is internal and deliberately limited to the two known
harnesses. It is not a generic extension API.

```text
init command
  ├─ resolve agents and destination paths
  ├─ Claude installer
  │    ├─ install Claude assets
  │    └─ merge settings.json gate
  └─ Polytoken installer
       ├─ install Polytoken assets
       ├─ merge hooks.json gate
       └─ merge AGENTS.md transclusion
```

Shared `internal/initcmd` primitives provide:

- atomic standalone-file writes;
- preservation of existing file modes where applicable;
- `Written`, `Overwritten`, `Skipped`, and `Removed` results;
- skip-without-force and replace-with-force behavior;
- owned-file removal and empty-directory pruning.

Harness adapters provide:

- destination resolution;
- embedded asset selection;
- schema-specific shared-file merge and unmerge;
- hook envelope normalization;
- harness-specific doctor checks.

### Embedded assets

Split embedded artifacts by harness:

```text
assets/
  claude/
    skills/herdle-tk-flow/SKILL.md
    skills/herdle-tk-artifacts/SKILL.md
    rules/herdle.md
  polytoken/
    skills/herdle-tk-flow/SKILL.md
    skills/herdle-tk-artifacts/SKILL.md
    herdle.md
```

The Claude assets retain current behavior and wording. The Polytoken assets use
Polytoken-native concepts, including:

- `AGENTS.md` for standing project instructions;
- Polytoken `todo_*` tools rather than `TodoWrite`;
- Polytoken skills and subagents rather than Claude slash-command assumptions;
- the durable review evidence contract in this specification.

Both skills remain valid Agent Skills directories with a `SKILL.md` and required
`description` frontmatter. The directory remains the skill name. The Polytoken
variants use no `polytoken` frontmatter key, tags, or templating in this release;
their bodies are delivered verbatim.

## Polytoken destinations and ownership

Resolve the global Polytoken config directory using Polytoken's platform/XDG
convention. The resolver is injected in tests so no test writes to a real home or
config directory.

Herdle owns these standalone paths under that directory:

```text
skills/herdle-tk-flow/SKILL.md
skills/herdle-tk-artifacts/SKILL.md
herdle.md
```

Herdle also owns exactly:

- one hook entry named `herdle-gatekeeper` in `hooks.json`;
- one marked transclusion block in `AGENTS.md`.

The transclusion block is:

```markdown
<!-- herdle:begin -->
@herdle.md
<!-- herdle:end -->
```

Herdle does not own either complete shared file.

### Install and refresh

A normal install writes missing standalone files and preserves an existing file
at an owned path. `--force` refreshes standalone skill and context files from the
embedded versions.

Shared-file wiring self-heals on every install, including without `--force`:

- the named hook is replaced with the current event, matcher, and absolute Herdle
  command;
- a single well-formed managed context block is normalized to the exact block
  above.

This matches the current Claude hook behavior: `--force` controls substantive
standalone content, not whether required wiring points at the current binary.

### Uninstall

Uninstall removes only:

- the exact named hook entry;
- the exact managed context block;
- Herdle-owned standalone files;
- directories made empty by removal of those standalone files.

It leaves unrelated hooks, context, skills, Polytoken settings, and Herdle project
config untouched. Already-absent owned content is a successful skipped result.

## Polytoken hook design

### Installed entry

Install one global hook entry equivalent to:

```json
{
  "name": "herdle-gatekeeper",
  "event": "pre_tool_use",
  "matcher": "*",
  "handler": {
    "bash": "/absolute/path/to/herdle hook gatekeeper --agent polytoken"
  }
}
```

The broad matcher is intentional. Polytoken documents glob matching, not Claude's
alternation syntax. Herdle cheaply allows irrelevant tool calls after parsing the
event. A single named entry also gives install, doctor, migration, and uninstall
one unambiguous ownership unit.

Polytoken passes event JSON on stdin. The hidden command uses exit `0` for allow
and exit `2` with a reason for deny, matching Polytoken's documented blocking-hook
shorthand.

### Envelope normalization

Add an explicit hook adapter selector. Both adapters normalize their event into
the existing policy input:

```text
Claude envelope ─┐
                 ├─> gate.HookInput -> gate.ShouldEvaluate -> gate.Decide
Polytoken event ─┘
```

The Polytoken adapter maps:

- event tool name to `ToolName`;
- `input.path` to the edited file path;
- `input.new_string` or `input.content` to written text;
- `input.command` to the shell command;
- relative paths against the event's documented working/project directory.

Implementation must confirm these exact event keys against the current official
Polytoken hook schema before coding the parser. If the official schema differs,
the adapter follows the documented keys while preserving the normalized internal
contract.

Malformed, irrelevant, or unrecognized envelopes fail open because Herdle cannot
establish that they represent a gated edit. Once a lifecycle transition is
recognized, unreadable required on-disk evidence fails closed through the shared
gate policy.

## Durable review evidence

### Artifact and markers

Polytoken uses the existing ticket-correlated validation document rather than a
fourth artifact type:

```text
docs/superpowers/validation/YYYY-MM-DD-<tkid>-<slug>-validation.md
```

It contains this fixed section:

```markdown
## Herdle code review

- [x] Standard review completed
- [x] Standard review findings addressed
- [x] Deep review completed
- [x] Deep review findings addressed
```

All four exact checked marker lines are required before a forward transition to
`lifecycle: pending-validation`. Missing, unchecked, duplicated, or textually
altered markers do not satisfy the gate. The filename must correlate to the
transitioned ticket ID using Herdle's existing artifact convention.

Reviewers may add free-form details after the markers, including reviewer,
scope, findings, and resolutions. The gate does not parse or require those
details. The markers are an explicit, auditable attestation, not proof that a
particular implementation mechanism ran.

### Workflow ordering

The Polytoken artifact skill requires this order:

1. Run the standard review pass and address its findings.
2. Run the deep review pass and address its findings.
3. Create or update the validation document with all four checked review markers
   and concrete automated and human validation steps.
4. Set `lifecycle: pending-validation`.
5. Run automated validation and check only steps actually completed.
6. Leave human-only steps unchecked.
7. Set `lifecycle: validated` only when every validation checkbox is checked.

For validation completion, the four review markers are treated as review evidence,
not acceptance steps. Every other unchecked `- [ ]` in the validation document
continues to block `validated`.

### Gate policy boundary

Refactor the shared pending-validation policy to consume an abstract review
result rather than directly understanding a Claude transcript. Evidence gathering
stays in the harness adapter:

- Claude reads and scans the current transcript exactly as it does today.
- Polytoken locates and parses the ticket-correlated validation document.

The shared policy still owns transition classification, fail-open/fail-closed
boundaries, reason text, and reason-bearing overrides. Existing overrides remain
available and exceptional.

## Shared-file safety

Before changing shared files, the Polytoken installer validates them:

- absent or whitespace-only `hooks.json` is treated as an empty array;
- present `hooks.json` must parse as a JSON array;
- non-Herdle entries are preserved as parsed values;
- zero or one `herdle-gatekeeper` entry is accepted;
- duplicate Herdle hook names cause refusal rather than guessed cleanup;
- `AGENTS.md` may contain zero or one complete managed block;
- partial, nested, or duplicate Herdle marker blocks cause refusal.

Writes are atomic and preserve an existing shared file's permissions. A new
`hooks.json` is mode `0o600` because configuration may eventually contain
sensitive commands or values. A new `AGENTS.md` and standalone skills/context are
mode `0o644`; created parent directories are mode `0o750`.

Uninstall applies the same ambiguity checks. It refuses malformed shared state
rather than deleting content it cannot identify safely.

## Doctor behavior

Common dependency and Herdle config checks run once. Harness-specific rows are
prefixed to keep dual-agent output clear:

```text
claude: skills + rule
claude: lifecycle gatekeeper
polytoken: skills + context
polytoken: AGENTS.md link
polytoken: lifecycle gatekeeper
```

Polytoken doctor verifies:

- every standalone owned file exists and matches the embedded content;
- the managed transclusion block exists exactly once and is well formed;
- the named hook exists exactly once with the expected event, matcher, and current
  absolute command;
- shared JSON and marker structure are parseable and unambiguous.

Missing required wiring is a failure. Drifted standalone content is a warning
with remediation `herdle init --agent polytoken --force`. Missing content is a
failure with remediation `herdle init --agent polytoken`. Malformed shared state
is a failure that names the file and explains what must be repaired before init
can safely proceed.

The existing superpowers-plugin diagnostic remains Claude-specific. Polytoken
checks should report the availability needed by the Polytoken-native workflow
without scanning Claude plugin directories.

## Error handling

- Validate all `--agent` values before filesystem changes.
- Include the harness and path in merge, parse, permission, and write errors.
- Do not silently replace a shared file with an unexpected top-level type.
- Preserve the existing skip behavior for user-modified standalone files unless
  `--force` is present.
- Report completed targets if a later selected target fails.
- Seed config only after every selected target succeeds.
- Keep uninstall idempotent for absent, unambiguous owned content.

## Testing

### Selection and orchestration

- Bare init and doctor remain Claude-only.
- Explicit Polytoken and dual-agent selection work.
- Duplicate values are deduplicated.
- Unknown values fail before writes.
- Flags apply to all selected agents.
- Config seeding runs once and only after successful installation.
- Partial multi-agent failure is reported without rollback.

### Installer unit tests

For each harness, cover fresh install, idempotent reinstall, force refresh,
permission preservation, uninstall, empty-directory pruning, and foreign-content
preservation.

For Polytoken shared files, cover:

- absent, empty, and populated valid files;
- hook insertion, self-healing replacement, and removal;
- context block insertion, normalization, and removal;
- malformed JSON and wrong top-level JSON type;
- duplicate named hooks;
- partial, nested, and duplicate context markers;
- foreign hook entries preserving their parsed JSON values and order, and
  non-managed Markdown surviving byte-for-byte;
- atomic-write cleanup on failure.

### Hook and gate tests

- Representative Polytoken edit, write, and shell event payloads normalize
  correctly.
- Irrelevant and malformed payloads fail open.
- Recognized transitions with unreadable evidence fail closed.
- Branch-link and validated gates retain current behavior.
- Every combination missing one review marker is denied.
- Exactly one copy of all four checked markers is accepted.
- Unchecked, duplicated, or altered review markers are denied.
- Review markers are not mistaken for outstanding validation steps.
- Overrides remain reason-bearing and functional.
- Existing Claude transcript behavior remains unchanged.

### Doctor and asset tests

- Healthy, missing, drifted, malformed, duplicate, and stale-command states are
  covered per harness.
- Every Polytoken skill has valid Agent Skills frontmatter.
- Polytoken assets contain no accidental Claude-only terms such as `CLAUDE.md`,
  `TodoWrite`, or the Claude-specific `/code-review` command.
- Documentation command tables and help output remain synchronized.

### Verification

Run the repository's full Go gate. When an installed `polytoken` CLI exposes an
applicable noninteractive validation or temporary-config loading path, use it to
prove the generated hooks, skills, and context load successfully. If no such
command exists, record startup/reload acceptance as a residual manual validation
step rather than claiming it was automated.

## Documentation changes

Update README and install, usage, and tk-convention documentation to cover:

- the `--agent` command forms and backward-compatible default;
- platform-specific Polytoken config destinations;
- installed paths and exact ownership boundaries;
- refresh and uninstall behavior;
- reload/restart requirements after changing Polytoken hooks or skills;
- the Polytoken review markers and ordering;
- doctor output and remediation;
- the absence of project-local support in this release.

## Baseline and implementation assumptions

This design was written against `main` at commit `ae73c50` (tag `0.7.0`). Before
planning implementation, inspect changes with:

```sh
git log ae73c50..main -- cmd/herdle/init.go cmd/herdle/doctor.go cmd/herdle/hook.go internal/initcmd internal/doctor internal/gate internal/config assets docs
```

Re-verify these assumptions if the listed paths move:

- init still mirrors one embedded asset tree into `~/.claude`;
- Claude hook wiring still lives in `internal/initcmd/settings.go`;
- doctor still has a single Claude-oriented environment and fixed check list;
- gate decisions still normalize through `gate.HookInput`;
- Polytoken still discovers global `skills/`, `hooks.json`, and `AGENTS.md` from its
  config directory;
- Polytoken still supports `@` context transclusion and exit-code shorthand for
  blocking hooks;
- the current official Polytoken hook event keys and config-directory discovery
  rules have been fetched and confirmed before implementation.
