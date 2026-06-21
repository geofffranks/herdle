herdle: `tk` (wedow/ticket) is the source of truth for work across projects. Run
`herdle` to see work in progress — a drilldown inside a repo, a cross-project
summary outside one (`herdle --help` for flags). Track work in tk tickets, link
branches/PRs with the `external-ref`/`branch:` fields, and keep each ticket's
`lifecycle:` current. See the `herdle-tk-flow` skill for tracking and dashboard
conventions and `herdle-tk-artifacts` for spec/plan/validation conventions.
Lifecycle transitions are gated by `herdle hook gatekeeper`; see the
`herdle-tk-flow` skill ("Lifecycle gates") for the expected path before bumping a
ticket's `lifecycle:`.
