# tk Conventions

This page orients a human reader to the tk-driven workflow conventions that
herdle encodes. The installed skills are the authoritative, agent-facing source
of truth — see [The installed skills are authoritative](#the-installed-skills-are-authoritative)
at the bottom.

---

## Why

`tk` (wedow/ticket) is the source of truth for work. herdle's value is that it
surfaces `tk` lifecycle state together with git and GitHub on a single dashboard
row — open PRs, branches, and tickets all correlated. That correlation is only
meaningful because of a shared convention layer: consistent lifecycle fields,
linking fields, and artifact filenames. This page describes that convention.

---

## Lifecycle states

Each ticket carries a `lifecycle:` frontmatter field that moves forward as work
progresses:

```
-  →  designed  →  planned  →  in-development  →  pending-validation  →  validated
```

| State | Meaning |
|---|---|
| `-` | not started |
| `designed` | spec written and on disk |
| `planned` | implementation plan written and on disk |
| `in-development` | actively being built |
| `pending-validation` | built; validation not yet done |
| `validated` | verified and ready to merge |

**Convention: a PR is opened only after the work is validated.** An open — or
merged — PR therefore implies `validated`. The lifecycle field is maintained by
hand; treat a stale-looking state as advisory and trust `git`/`gh`/`tk show`
for ground truth.

When `lifecycle:` is unset, herdle *derives* a state from on-disk artifacts: a
matching `plans/*<tkid>*` file implies `planned`; a matching `specs/*<tkid>*`
file implies `designed`. An explicitly set value always wins.

---

## tk ⇄ branch ⇄ PR correlation

herdle links a ticket to its branch and PR via two frontmatter fields:

- **`external-ref`** — an issue, PR, or MR reference such as `gh-123`,
  `github#123`, `gl-123`, `!123`, or a URL containing `/issues/123`, `/pull/123`,
  or `/merge_requests/123`. herdle token-matches the number against open PR/MR
  numbers and branch names, so the convention is identical for GitHub and GitLab.
- **`branch:`** — an explicit, exact branch name. Use this as a fallback when
  the branch carries no issue or PR number (e.g. `jtac-autolase-*`), so
  correlation still works.

When the dashboard shows a ticket with *no external-ref/branch*, or a branch
with *no tk*, that is an unlinked association to fix — add the field or create
the ticket.

### Issue triage via `external-ref`

A ticket's `external-ref` triages a forge issue — removing it from the
un-triaged (`⚑`) list in the dashboard — when it points at an issue number:

- Short forms: `gh-59`, `github#59`, `gl-59`
- Long forms: a URL containing `.../issues/59`

A URL containing `.../pull/59` (or `.../merge_requests/59`) does **not** triage
issue #59 — it is a PR/MR reference, not an issue reference. GitHub shares one
number namespace for issues and PRs, but herdle distinguishes them by path
segment: only `/issues/` paths count as issue refs.

herdle never auto-creates tickets. To triage an issue, create a ticket manually
and set its `external-ref` to the issue number or URL.

---

## Design artifacts

Specs, plans, and validation documents live under `docs/superpowers/` in three
sibling directories and follow a shared filename stem:

```
docs/superpowers/specs/YYYY-MM-DD-<tkid>-<slug>-design.md
docs/superpowers/plans/YYYY-MM-DD-<tkid>-<slug>.md
docs/superpowers/validation/YYYY-MM-DD-<tkid>-<slug>-validation.md
```

The embedded `<tkid>` (e.g. `her-cung`) is how herdle correlates an artifact to
its ticket and lists it in the dashboard. Omit the tkid and the link breaks —
always include it.

---

## Review evidence and lifecycle order

Every implementation plan ends with separate **Code Review** and **Finalize**
tasks. Code Review runs one fresh final integration review of the full branch
diff against its base. Address valid Critical and Important findings as a
complete batch. Incremental rereview is required only after review fixes change
the branch.

Finalize records durable evidence in one ticket-correlated validation document.
Use exactly one of these forms, checking each line only after its work is
complete. When no review fix changed the branch, use the three-line form:

```markdown
## Herdle code review

- [x] Final integration review completed
- [x] Final integration review findings addressed
- [x] Final integration rereview not required
```

When review fixes changed the branch and the resulting rereview is complete, use
the four-line form:

```markdown
## Herdle code review

- [x] Final integration review completed
- [x] Final integration review findings addressed
- [x] Final integration rereview completed
- [x] Final integration rereview findings addressed
```

New validation documents use only one of the final-integration forms above.
Legacy four-line Standard/Deep markers remain accepted indefinitely for existing
validation documents. Herdle does not automatically expire or rewrite legacy
evidence.

Only after review work and one complete supported evidence form are on disk may
the ticket move forward from `in-development` to `pending-validation`. A rollback
from `validated` to `pending-validation`, or an idempotent rewrite already at
`pending-validation`, does not require a new review. For an exceptional forward
transition, add the reason-bearing `[skip-code-review-gate] <reason>` override;
a bare override marker is rejected. Check automated validation boxes only for
commands actually run; leave human-only boxes open and do not move to `validated`
until a human completes them.

---

## The installed skills are authoritative

Bare `herdle init` is the Claude-compatible default; repeat `--agent` to install
both harnesses globally. `herdle init --agent polytoken` also defaults to global
scope. Run `herdle init --agent polytoken --scope project` for the exact current
directory instead. For Claude Code, the two skills live under
`~/.claude/skills/` with a rules stub at `~/.claude/rules/herdle.md`. Global
Polytoken skills live under
`${XDG_CONFIG_HOME:-$HOME/.config}/polytoken/skills/`; project skills live under
`.polytoken/skills/`. Each scope links its `herdle.md` context from a marked
block in the applicable `AGENTS.md`. Reload Claude with `/reload`; start a new
Polytoken session or restart its client after changes.
Both harnesses install the same two skills with harness-native wording:

- **`herdle-tk-flow`** — the lifecycle, correlation, and dashboard-reading
  conventions. Use this skill when tracking work, starting feature work, or
  reading the herdle dashboard.
- **`herdle-tk-artifacts`** — the spec/plan/validation artifact naming, lifecycle
  stamping, and the Setup/Finalize tasks baked into every implementation plan.
  Use this skill when producing design artifacts under the `superpowers:*`
  process skills.
- **Claude** `~/.claude/rules/herdle.md` / **Polytoken** `herdle.md` — a short
  always-on context file that orients an agent toward these two skills without
  spelling out the full convention.

Those skills are the agent-facing source of truth. This page is a human
orientation only — it does not repeat the skills verbatim and will not stay
in sync with every nuance. When in doubt, read the skill.
