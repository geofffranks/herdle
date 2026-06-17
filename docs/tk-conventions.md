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

## The installed skills are authoritative

`herdle init` installs two skills under `~/.claude/skills/` and a rule stub
under `~/.claude/rules/`:

- **`herdle-tk-flow`** — the lifecycle, correlation, and dashboard-reading
  conventions. Use this skill when tracking work, starting feature work, or
  reading the herdle dashboard.
- **`herdle-tk-artifacts`** — the spec/plan/validation artifact naming, lifecycle
  stamping, and the Setup/Finalize tasks baked into every implementation plan.
  Use this skill when producing design artifacts under the `superpowers:*`
  process skills.
- **`~/.claude/rules/herdle.md`** — a short always-on rule stub that orients an
  agent toward these two skills without spelling out the full convention.

Those skills are the agent-facing source of truth. This page is a human
orientation only — it does not repeat the skills verbatim and will not stay
in sync with every nuance. When in doubt, read the skill.
