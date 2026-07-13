---
name: herdle-tk-flow
description: Use when tracking work across projects, starting feature work, checking "what's in progress", reading the herdle dashboard, or running any superpowers process skill.
---

# herdle tk flow

herdle wraps a tk-driven, multi-project workflow. `tk` (wedow/ticket) is the
source of truth for work; the `herdle` dashboard reads tk together with git and
gh to show what's in progress across every configured project. This skill is the
convention layer: apply it alongside Polytoken's process skills rather than
replacing them.

## tk is the source of truth

Active work — status, design decisions, and sub-tasks that outlive a single
session — lives in tk tickets, not in scattered notes. If it matters beyond this
session and isn't code, it belongs in a ticket (or an artifact the ticket links).

## Where each kind of information belongs

Route work-state to the right home; don't default everything to one place:

- **Durable rule, convention, or gotcha** → the repo's `AGENTS.md`. It is the
  project's standing instruction to every future Polytoken session.
- **Active work** (status, design decisions, multi-session sub-tasks) → a **tk
  ticket**. This is the unit `herdle` tracks.
- **Branch existence, commit history, PR state** → **never store these; query
  `git` and `gh` live.** Recorded status rots the moment a branch merges or a PR
  closes.
- **A full spec, plan, or validation artifact** → a **doc file the ticket links**
  (see the `herdle-tk-artifacts` skill for naming and lifecycle).
- **Ephemeral, within-session sub-tasks** → Polytoken's todo tools: create work
  with `todo_create`, reflect active or blocked state with `todo_update`, and
  finish it with `todo_complete`.

> **Polytoken todos are session state, not repository files.** Do not create or
> edit a `TODO`, `TODO.md`, or similar file to track session work. Promote an item
> to a tk ticket only if it must survive the session.

## tk ⇄ branch ⇄ PR correlation

herdle links a ticket to its branch and PR so the dashboard can show them on one
row. Maintain these ticket frontmatter fields:

- **`external-ref`** — an issue/PR/MR reference (`gh-123`, `github#123`,
  `gl-123`, `!123`, `.../issues/123`, `.../pull/123`, `.../merge_requests/123`).
  herdle token-matches the number against open PR/MR numbers and branch names, so
  the convention is the same whether the forge is GitHub or GitLab.
- **`branch:`** — an exact branch name. Use this fallback when the branch carries
  no issue or PR number, so it still correlates.

When the dashboard shows a ticket with **no external-ref/branch**, or a branch
with **no tk**, that's an unlinked association to fix — add the field or create
the ticket.

## Lifecycle states

Each ticket carries a `lifecycle:` field tracking where the work is:

```
-  →  designed  →  planned  →  in-development  →  pending-validation  →  validated
```

- `-` not started · `designed` spec on disk · `planned` plan on disk ·
  `in-development` being built · `pending-validation` built, awaiting validation ·
  `validated` verified.
- **Convention: open a PR only after the work is validated** — so an open (or
  merged) PR implies `validated`.
- Lifecycle is maintained by hand. Treat a stale-looking state as advisory and
  trust `git` / `gh` / `tk show` for ground truth.
- When `lifecycle:` is unset, herdle *derives* `designed`/`planned` from matching
  artifacts on disk (see `herdle-tk-artifacts`); an explicitly set value always
  wins.

## Lifecycle gates

herdle installs a PreToolUse hook (`herdle hook gatekeeper`) that **mechanically
blocks** three forward transitions until their preconditions hold. Do the
precondition first and the gate is invisible; the blocks below are the backstop,
not the place to learn the rule.

- **→ in-development** — set `branch:` (or `external-ref`) on the ticket *first*,
  so the dashboard can correlate it.
- **→ pending-validation** — complete standard and deep review passes with the
  `requesting-code-review` skill and reviewer subagents, address their findings,
  then record the four durable review markers in the validation document.
- **→ validated** — the ticket must already be at `pending-validation` (never jump
  straight from in-development), **and** every `- [ ]` in its validation doc must
  be checked. Automated steps you ran get checked off by you; human-only steps stay
  open until a human checks them — so leave the ticket at `pending-validation` and
  let the human flip it to `validated`.

Each gate has an explicit, reason-bearing override for the rare legitimate case:
`[skip-branch-linkage] <reason>`, `[skip-code-review-gate] <reason>`,
`[skip-validation-gate] <reason>`. Overrides are exceptional — prefer satisfying
the precondition.

## Reading the herdle dashboard

- Run **`herdle`** inside a repo for that repo's **drilldown**; run it **outside**
  any repo for the **cross-project summary**.
- **`herdle --all`** forces the summary even inside a repo · **`herdle <name>`**
  drills into a named project · **`herdle --fetch`** runs `git fetch` first
  (network; default is offline) · **`herdle --help`** lists every flag.

The drilldown has four sections, **each hidden when it's empty**:

1. **open PRs** — open pull requests, with the correlated tk ticket and a
   local↔origin **sync** indicator.
2. **merged PRs needing cleanup** — merged PRs whose local branch still lingers.
3. **work in progress** — non-PR branches plus in-flight `in_progress` tickets,
   each with a sync column and its lifecycle state.
4. **up next** — open / not-started tickets, priority-sorted.

Notes on what you're seeing:

- The **sync** column shows local↔origin divergence (ahead/behind).
- herdle **auto-prunes the remote** each run and **hides merged-PR and
  upstream-gone branches**, so the board shows only live work.
- **Recently-closed work is not shown.** For "what just shipped," use
  `tk closed`, `gh pr list --state merged`, or `git log`.

## Working with Polytoken process skills

These conventions are an **additive layer** over Polytoken's process skills,
including brainstorming, writing-plans, executing-plans, and
`requesting-code-review`. They never replace those procedures.

- When you run a process skill, apply the Herdle conventions alongside it: make
  sure a tk ticket exists, keep its `lifecycle:` current, and link any
  spec/plan/validation artifacts to it.
- To extend a process with a local convention, author a thin wrapper skill that
  invokes the original skill and adds only the convention. Do not reimplement
  the underlying process.
