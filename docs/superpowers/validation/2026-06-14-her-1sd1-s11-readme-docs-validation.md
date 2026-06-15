# S11 — README + install/usage/contributor docs — Validation

**Ticket:** her-1sd1 · **Branch:** her-1sd1-s11-readme-docs
**Spec:** docs/superpowers/specs/2026-06-14-her-1sd1-s11-readme-docs-design.md
**Plan:** docs/superpowers/plans/2026-06-14-her-1sd1-s11-readme-docs.md

## Acceptance criteria (from the ticket)

1. Docs cover the full command surface.
2. Docs cover the dependency contract.
3. Install steps are accurate against the shipped binary.

## Automated checks — `docs/superpowers/validation/validate-her-1sd1-s11.sh`

Run: `bash docs/superpowers/validation/validate-her-1sd1-s11.sh` → **ALL VALIDATION CHECKS PASSED** (2026-06-14).

| Check | Result | Covers |
|---|---|---|
| `make test` (drift guard + full suite, 263 specs) | ✅ ok | criterion 1 — the drift guard fails if any command/flag is undocumented |
| Every Command Reference entry runs (`herdle <cmd> --help` exit 0) | ✅ ok (version, project add/set/rm/list, init, doctor) | criterion 1, 3 |
| Install asset names match `release.yml` matrix (template + 5 tuples) | ✅ ok (linux/darwin/windows × arch) | criterion 3 |
| README internal links resolve (install, usage, configuration, tk-conventions, CONTRIBUTING, LICENSE) | ✅ ok | doc integrity |
| README has no construction/pre-release banner | ✅ ok | design decision (present as ready) |
| No stray `config.yaml` reference (real path is `config.toml`) | ✅ ok | criterion 3 |

## How each criterion is met

1. **Full command surface** — `docs/usage.md` documents every command and flag and carries the `## Command Reference` table. The drift guard (`cmd/herdle/docs_drift_test.go`) mechanically enforces, on every `make test`, that every command/flag exposed by `newApp()` is documented and that the table names only real surface. Green ⇒ complete coverage.
2. **Dependency contract** — `docs/install.md` carries the full dependency table (`tk`, `git`, superpowers required; `gh` + GitHub remote optional; Go dev-only) with how-to-get for each, including the superpowers link.
3. **Install steps accurate against the shipped binary** — commands were run against the built binary while writing; the `herdle doctor` sample is real captured output; the five release-asset names are verified against `.github/workflows/release.yml`; cross-platform checksum/Windows notes added.

## Manual confirmation (recommended for a human reviewer)

- Read `README.md` → `docs/install.md` → `docs/usage.md` end-to-end: the download → `herdle init` → `herdle doctor` → dashboard flow reads coherently.
- `docs/configuration.md` autodetection is origin-first (matches `internal/config/resolve.go`).
- `docs/tk-conventions.md` orients to the lifecycle/correlation model and defers to the installed skills as authoritative.

## Outcome

All automated acceptance checks pass. Lifecycle → `pending-validation` pending human read-through; no PR opened (per repo convention, `finishing-a-development-branch` owns the merge decision).
