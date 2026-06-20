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

**Code Review (second-to-last task):**

Two passes, in order. The **controller invokes the `/code-review` Skill directly**
(it spawns its own finder agents) — do **not** delegate the invocation to a
dispatched subagent. A subagent's `/code-review` call is recorded only in that
subagent's transcript, never in the main session transcript, so a delegated pass
both loses the evidence trail and trips the code-review gate (a PreToolUse hook
that blocks the `lifecycle: pending-validation` bump unless both passes appear in
this session's transcript). The invocation is fixed — run it exactly:

1. `/code-review <branch> medium --fix`
2. `/code-review <branch> high --fix`

Each pass means **one `code-review` Skill-tool invocation per effort level.** A
trimmed fan-out, a single hand-dispatched review subagent, the
`subagent-driven-development` whole-branch review, or "I already reviewed
thoroughly" do **not** count — they are a different mechanism and miss a different
bug class. The mandate is **unconditional on diff size**: a small or clean diff is
when you are most tempted to skip and most likely to be wrong.

`--fix` is mandatory: without it the pass prints findings and changes nothing,
which reads as "reviewed" but isn't. Do **not** add `--comment` — that posts to a
PR, and no PR exists yet. Address every finding from both passes. Defer the review
*process* to `superpowers:requesting-code-review`. Record both invocations (the
`medium` and `high` runs) in the SDD progress ledger as the Finalize evidence.

<HARD-GATE>
You MUST complete both passes before Finalize advances the lifecycle. A clean-
looking diff does not exempt you — "looks fine" is not a review. Skipping or
weakening either pass is a defect, not a judgment call.
</HARD-GATE>

**Finalize (last task):**

- Set `lifecycle: pending-validation` — **only after the Code Review task is
  complete.** Both passes done and their findings addressed is the precondition
  for this bump.
- Write the validation doc (`docs/superpowers/validation/...-validation.md`) with
  concrete acceptance steps.
- Where possible, write a script that exercises as much of the validation doc as
  it can, run it, and mark off the steps it covers before handing off. If those
  validations all pass, set `lifecycle: validated`.
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
