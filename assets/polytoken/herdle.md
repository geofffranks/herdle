herdle: keep this standing context in the repository's `AGENTS.md`. `tk`
(wedow/ticket) is the source of truth for durable work across projects. Run
`herdle` to see work in progress: a drilldown inside a repo or a cross-project
summary outside one (`herdle --help` lists flags). Track work in tk tickets, link
branches and reviews with `external-ref` or `branch:`, and keep each ticket's
`lifecycle:` current. Load the `herdle-tk-flow` skill for ticket, dashboard, and
session-todo conventions. Load `herdle-tk-artifacts` for spec, plan, review, and
validation conventions. Lifecycle transitions are enforced by
`herdle hook gatekeeper`; satisfy the gates described in `herdle-tk-flow` before
advancing a ticket.
