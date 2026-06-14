---
name: herdle-tk-artifacts
description: Use when a spec, plan, or validation document is produced under superpowers:brainstorming, writing-plans, or executing-plans, or when baking Setup/Finalize tasks into an implementation plan.
---

# herdle tk artifacts

Design artifacts (specs, plans, validation docs) follow a filename and lifecycle
convention so the `herdle` dashboard can correlate each file to its tk ticket.
This skill covers that convention. It does **not** replace
`superpowers:brainstorming`, `writing-plans`, or `executing-plans` — it adds the
herdle bookkeeping on top of them.

## Where artifacts live and how they're named

Artifacts live under `docs/superpowers/` in three sibling directories, sharing a
filename stem:

```
docs/superpowers/specs/YYYY-MM-DD-<tkid>-<slug>-design.md
docs/superpowers/plans/YYYY-MM-DD-<tkid>-<slug>.md
docs/superpowers/validation/YYYY-MM-DD-<tkid>-<slug>-validation.md
```

The embedded **`<tkid>`** (e.g. `her-cung`) is how herdle correlates a file to
its ticket and lists it in the dashboard. **Omit the tkid and the link breaks** —
always include it.

## Lifecycle stamping

As each artifact is produced, set the ticket's `lifecycle:` field:

- **brainstorming** writes the spec → set the spec filename and
  `lifecycle: designed`.
- **writing-plans** writes the plan → set the plan filename and
  `lifecycle: planned`.

When `lifecycle:` is unset, herdle *derives* `designed`/`planned` from a matching
`specs/*<tkid>*` / `plans/*<tkid>*` file on disk — but an explicitly set value
always wins, so prefer to set it.

## Bake Setup and Finalize tasks into every plan

Every implementation plan wraps its work in a fixed first and last task:

**Setup (first task):**

- Mark the ticket in progress (`tk start <id>`).
- Create the work branch off the repo's default branch.
- Record the branch on the ticket (`branch:` frontmatter) and set
  `lifecycle: in-development`.

**Finalize (last task):**

- Request a code review — defer to `superpowers:requesting-code-review` — and
  address its findings.
- Squash the branch's commits into one.
- Set `lifecycle: pending-validation`.
- Write the validation doc (`docs/superpowers/validation/...-validation.md`) with
  concrete acceptance steps.
- Do **not** open a PR here — opening a PR signals validated work. Leave that to
  `superpowers:finishing-a-development-branch`.

## Defer the plan when implementation won't follow immediately

If a brainstorming → spec cycle finishes but you won't implement right away,
**stop at the approved spec** instead of writing a plan that will rot. Add a
**Baseline** section to the spec recording:

- the branch and commit the spec was written against,
- a ready-to-run `git log <baseline>..<branch> -- <load-bearing paths>` to see
  what moved, and
- a checklist of design assumptions to re-verify if those files change.

Regenerate the plan from the spec at implementation time. Specs describe intent
and age gracefully; plans reference exact files and line numbers and rot as the
repo moves.

## Defers to superpowers

This skill only adds the herdle filename, lifecycle, and task-baking conventions.
The actual design dialogue, plan structure, and execution come from
`superpowers:brainstorming`, `writing-plans`, and `executing-plans` — invoke
those for the process.
