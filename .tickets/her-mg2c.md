---
id: her-mg2c
status: closed
deps: [her-58tk]
links: [her-x9jl]
lifecycle: validated
created: 2026-06-12T20:40:38Z
type: task
priority: 2
assignee: Geoff Franks
---
# S10: release pipeline (release.yml GOOS/GOARCH matrix, codeql, dependabot)

**Epic:** her-x9jl

**Scope:** `release.yml`: on release tag, GOOS/GOARCH matrix `go build` with version ldflags (linux amd64/arm64, darwin amd64/arm64, windows amd64) uploading binaries to the Release. Add codeql-analysis and dependabot-auto-merge, modeled on spruce.

**Acceptance:** A test tag produces all five platform binaries on the Release; codeql + dependabot workflows present and valid.
