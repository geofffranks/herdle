# Contributing to herdle

## Prerequisites

- **Go 1.26.x** (`go.mod` declares `go 1.26.0`, toolchain `go1.26.3`).
- All dev tools (Ginkgo, Counterfeiter, staticcheck, gosec) are declared as Go tool dependencies in `go.mod` and run via `go tool …` — no separate global installs are needed.

## Build & Install

```bash
make build          # vet + lint + build the herdle binary
make all            # vet lint test build — the full local gate
go install ./cmd/herdle  # install herdle to your GOBIN for local use
```

## Testing

```bash
make test
```

Runs lint, then the Ginkgo suite:

```
go tool ginkgo -r --race --fail-on-pending --keep-going --fail-on-empty --require-suite ./...
```

Tests use **Ginkgo v2** specs with **Counterfeiter** fakes behind the `git`/`gh`/`tk` runner interfaces (`internal/vcs/`).

Regenerate the fakes with:

```bash
go generate ./...   # or: make generate
```

Driven by the `//go:generate go tool counterfeiter -generate` directive in `internal/vcs/vcs.go`.

## Docs Stay in Sync (the Drift Guard)

`cmd/herdle/docs_drift_test.go` is a Ginkgo spec that fails when the docs drift from the CLI. It fails when:

- **(a)** a command or flag exposed by the binary is not documented anywhere in `README.md`, `docs/*.md`, or `CONTRIBUTING.md`, or
- **(b)** the `## Command Reference` table in `docs/usage.md` names a command or flag that no longer exists.

**To fix a failure:** document the new command/flag in `docs/usage.md` (and update the Command Reference table), or correct/remove the stale table row.

## Repo Layout

```
cmd/herdle/      CLI wiring (urfave/cli commands, version ldflag)
internal/
  dashboard/     gather + classify engine
  render/        column/ANSI rendering, golden-tested
  vcs/           git/gh/tk runner interfaces + counterfeiter fakes
  config/        TOML model, CRUD, migration
  initcmd/       embed + write/uninstall artifacts
  doctor/        diagnostics
assets/          embedded skills + rule stub written by herdle init
docs/            user-facing documentation
docs/superpowers/  internal specs/plans/validation artifacts
```

## Lint Stack

`make lint` runs:

1. A `gofmt` check over all `.go` files.
2. `go tool staticcheck ./...`
3. `go tool gosec ./...`

Justified file reads carry a `// #nosec G304 -- <reason>` annotation (gosec scans test files too).
