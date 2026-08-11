---
name: herdle-tk-artifacts
description: Use when a spec, plan, or validation document is produced under superpowers:brainstorming, writing-plans, or executing-plans, or when baking Setup, Code Review, and Finalize tasks into an implementation plan.
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

## Bake Setup, Code Review, and Finalize tasks into every plan

Every implementation plan wraps its work in three fixed tasks: **Setup** first,
then **Code Review** and **Finalize** as the last two, in that order. Code Review
is its own standalone task — never folded into Finalize as a bullet, where it gets
silently skipped.

**Setup (first task):**

- Mark the ticket in progress (`tk start <id>`).
- Create the work branch off the repo's default branch.
- Record the branch on the ticket (`branch:` frontmatter) and set
  `lifecycle: in-development`.
  The gatekeeper blocks the `in-development` bump unless `branch:`/`external-ref`
  is set — record the branch in the same edit or before it.

**Code Review (second-to-last task):**

After all implementation tasks are approved, run **one fresh final integration
review** of the full branch diff against its base. Defer the review process to
`superpowers:requesting-code-review`; Herdle does not require a harness-specific
review command.

Collect all valid Critical and Important findings before changing the branch,
then address those **Critical and Important findings** as **one complete fixer
batch**. Do not alternate between fixing and rereviewing one finding at a time.
Rereview the final integration only after branch-changing fixes. If review
requires no branch-changing fixes, record that rereview was not required. If a
rereview finds Critical or Important issues, collect and fix them as another
complete batch, then rereview **only after branch-changing fixes**.

Keep the Code Review task open until the review, required fixes, and any required
rereview are complete. The gatekeeper's other transitions have their own
reason-bearing overrides — `[skip-branch-linkage] <reason>` (in-development) and
`[skip-validation-gate] <reason>` (validated) — exceptional escape hatches, not
routine.

**Finalize (last task):**

Write the validation doc (`docs/superpowers/validation/...-validation.md`) with
concrete acceptance steps. Emit **exactly one of these mutually exclusive forms**,
checking its lines only after the corresponding work is complete.

When the final integration review required no branch-changing fixes:

```markdown
## Herdle code review

- [x] Final integration review completed
- [x] Final integration review findings addressed
- [x] Final integration rereview not required
```

When branch-changing fixes were made and final integration rereview completed:

```markdown
## Herdle code review

- [x] Final integration review completed
- [x] Final integration review findings addressed
- [x] Final integration rereview completed
- [x] Final integration rereview findings addressed
```

Then:

- Set `lifecycle: pending-validation` only after Code Review is complete and the
  matching durable evidence form is on disk.
- Structure the validation doc into **automated** and **human** sections. Write a
  script that exercises as much of the automated section as it can, run it, and
  check off only the steps it actually covered. **Human-only steps stay `- [ ]`.**
- Do **not** set `lifecycle: validated` while any box is open. The gatekeeper
  blocks it (and blocks skipping straight from `in-development`). Leave the ticket
  at `pending-validation`; the human checks the remaining boxes, then sets
  `validated`.
- Fix bugs as needed until the validation script passes.
- Squash the branch's commits into one.
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
