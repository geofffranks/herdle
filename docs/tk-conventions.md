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
tasks. Code Review always runs two independent passes in order, regardless of
diff size:

1. **Standard review** inspects the full branch diff against its base; address
   every valid finding and verify the fixes.
2. **Deep review** starts with fresh reviewer context and broader scrutiny of
   correctness, regressions, maintainability, and requirement compliance;
   address every valid finding and rerun verification.

Do not reuse the standard pass as the deep pass or replace either with informal
self-review. Record durable evidence in this exact order, leaving each marker
unchecked until that review or findings work is actually complete:

```markdown
## Herdle code review

- [ ] Standard review completed
- [ ] Standard review findings addressed
- [ ] Deep review completed
- [ ] Deep review findings addressed
```

Only after both passes, their fixes, and all four checked markers are on disk may
the ticket move to `pending-validation`. Check automated validation boxes only
for commands actually run; leave human-only boxes open and do not move to
`validated` until a human completes them.

---

## The installed skills are authoritative

Bare `herdle init` is the Claude-compatible default; repeat `--agent` to install
both harnesses. `herdle init --agent polytoken` installs globally only—there is
no project-local mode. For Claude Code, the two skills live under
`~/.claude/skills/` with a rules stub at `~/.claude/rules/herdle.md`. For
Polytoken, they live under
`${XDG_CONFIG_HOME:-$HOME/.config}/polytoken/skills/` with a context file at
`herdle.md` linked from a marked block in `AGENTS.md`. Reload Claude with
`/reload`; start a new Polytoken session or restart its client after changes.
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
