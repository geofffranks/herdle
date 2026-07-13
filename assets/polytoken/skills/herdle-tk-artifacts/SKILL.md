---
name: herdle-tk-artifacts
description: Use when Polytoken produces a spec, plan, or validation document, or when baking Setup, Code Review, and Finalize tasks into an implementation plan.
---

# herdle tk artifacts

Design artifacts follow a filename and lifecycle convention so the `herdle`
dashboard can correlate each file to its tk ticket. This skill adds Herdle
bookkeeping to Polytoken's brainstorming, writing-plans, and executing-plans
skills; it does not replace those processes.

## Where artifacts live and how they're named

Artifacts live under `docs/superpowers/` in three sibling directories, sharing a
filename stem:

```
docs/superpowers/specs/YYYY-MM-DD-<tkid>-<slug>-design.md
docs/superpowers/plans/YYYY-MM-DD-<tkid>-<slug>.md
docs/superpowers/validation/YYYY-MM-DD-<tkid>-<slug>-validation.md
```

The embedded **`<tkid>`** (for example, `her-cung`) is how Herdle correlates a
file to its ticket. Omit the tkid and the link breaks.

## Lifecycle stamping

As each artifact is produced, set the ticket's `lifecycle:` field:

- brainstorming writes the spec: record its filename and set
  `lifecycle: designed`;
- writing-plans writes the plan: record its filename and set
  `lifecycle: planned`.

When `lifecycle:` is unset, Herdle derives `designed` or `planned` from matching
artifact filenames. An explicit value always wins, so keep it current.

## Bake Setup, Code Review, and Finalize into every plan

Every implementation plan starts with **Setup** and ends with separate **Code
Review** and **Finalize** tasks, in that order. Track these tasks with
`todo_create`, mark active or blocked work with `todo_update`, and close finished
work with `todo_complete`.

### Setup

- Run `tk start <id>`.
- Create the work branch from the repository's default branch.
- Record `branch:` on the ticket before, or in the same edit as, setting
  `lifecycle: in-development`. `herdle hook gatekeeper` enforces this linkage.

### Code Review

Run two independent passes in order, regardless of diff size:

1. **Standard review:** invoke the `requesting-code-review` skill and dispatch a
   reviewer subagent to inspect the full branch diff against its base. Address
   every valid finding and verify the resulting changes.
2. **Deep review:** invoke `requesting-code-review` again and dispatch fresh
   reviewer subagents with broader scrutiny of correctness, regressions,
   maintainability, and requirement compliance. Address every valid finding and
   re-run verification.

Do not reuse the standard review as the deep review, and do not substitute an
informal self-review for either pass. Fresh reviewer context is part of the deep
pass. Keep review tasks open until both the review and its resulting fixes are
complete.

### Finalize

Create or update the validation document before advancing the ticket. It must
contain this exact durable evidence section, with all four lines checked only
after the corresponding work is complete:

```markdown
## Herdle code review

- [x] Standard review completed
- [x] Standard review findings addressed
- [x] Deep review completed
- [x] Deep review findings addressed
```

Then:

- add concrete **automated** and **human** validation sections;
- run automated validation and check only the steps it actually covered;
- leave human-only steps unchecked;
- only after both review passes, fixes, and the four markers are on disk, set
  `lifecycle: pending-validation`;
- do not set `lifecycle: validated` while any validation box remains open;
- fix defects until automated validation passes, squash as required by the plan,
  and do not open a PR during Finalize.

`herdle hook gatekeeper` enforces forward lifecycle transitions. Its
reason-bearing overrides (`[skip-branch-linkage] <reason>`,
`[skip-code-review-gate] <reason>`, and `[skip-validation-gate] <reason>`) are
exceptional escape hatches, not routine workflow.

## Defer plans that will not be implemented immediately

If a brainstorming-to-spec cycle finishes but implementation will wait, stop at
the approved spec. Add a **Baseline** section recording the branch and commit, a
ready-to-run `git log <baseline>..<branch> -- <load-bearing paths>`, and design
assumptions to re-check if those paths change. Regenerate the plan at
implementation time rather than letting file-specific instructions rot.
