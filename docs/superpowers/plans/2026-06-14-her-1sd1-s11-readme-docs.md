# S11 — README + install/usage/contributor docs — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the stub README with a lean front door plus a user-facing `docs/` tree and `CONTRIBUTING.md`, guarded by a Ginkgo spec that fails when the docs drift from the live CLI surface.

**Architecture:** Pure documentation plus one test. `docs/usage.md` carries the full command surface and a **Command Reference** table that anchors the drift guard. The guard (`cmd/herdle/docs_drift_test.go`) reflects over `newApp()`, asserts every command/flag is documented (forward), and asserts the Command Reference table names only real surface (reverse). No CLI behavior changes.

**Tech Stack:** Go 1.26, urfave/cli/v2, Ginkgo + Gomega, Markdown.

**Spec:** `docs/superpowers/specs/2026-06-14-her-1sd1-s11-readme-docs-design.md`

**Verified CLI surface (from the built binary):**
- Commands: `version`, `project`, `project add`, `project set`, `project rm`, `project list`, `init`, `doctor`.
- Global flags: `--all` (`-a`), `--fetch` (`-f`).
- `project add` / `project set` flags: `--gh`, `--remote`, `--base`, `--integration`.
- `init` flags: `--force`, `--uninstall`.
- `project rm` / `project list` / `doctor` / `version`: no extra flags.
- Release assets (from `.github/workflows/release.yml`): `herdle-linux-amd64`, `herdle-linux-arm64`, `herdle-darwin-amd64`, `herdle-darwin-arm64`, `herdle-windows-amd64` (each with a `.sha1`).

---

### Task 1: Setup

**Files:** none (branch + ticket bookkeeping).

- [ ] **Step 1: Start the ticket**

Run: `tk start her-1sd1`
Expected: status → in_progress.

- [ ] **Step 2: Create the feature branch off main**

```bash
git checkout main
git checkout -b her-1sd1-s11-readme-docs
```

- [ ] **Step 3: Record the branch + lifecycle on the ticket**

Edit `.tickets/her-1sd1.md` frontmatter: add `branch: her-1sd1-s11-readme-docs` and change `lifecycle: designed` to `lifecycle: in-development`.

- [ ] **Step 4: Commit the bookkeeping**

```bash
git add .tickets/her-1sd1.md docs/superpowers/specs/2026-06-14-her-1sd1-s11-readme-docs-design.md docs/superpowers/plans/2026-06-14-her-1sd1-s11-readme-docs.md
git commit -m "chore(her-1sd1): S11 setup — spec, plan, branch"
```

---

### Task 2: docs/usage.md — full command surface + Command Reference table

This file must exist before the drift guard (Task 3) can go green: it covers the full surface (forward check) and provides the Command Reference table (reverse check).

**Files:**
- Create: `docs/usage.md`

- [ ] **Step 1: Build the binary and capture real samples**

```bash
make build
NO_COLOR=1 ./herdle --help
NO_COLOR=1 ./herdle 2>&1 | head -30        # a real dashboard view (run inside this repo)
./herdle doctor 2>&1 | head -40            # keep for docs/install.md too
```

Keep the captured output to paste as fenced samples.

- [ ] **Step 2: Write `docs/usage.md`**

Sections (prose per the spec outline), in order:

1. **Dashboard** — outside a repo → cross-project summary; inside a repo → drilldown; `herdle --all` forces the summary; `herdle <name>` targets a named project; `herdle --fetch` does a network refresh first (default offline). Paste the `NO_COLOR=1 ./herdle` sample.
2. **Reading a drilldown** — the four sections (open PRs, merged-PR cleanup, work-in-progress, up-next), the sync column, lifecycle, and origin pruning / hidden-branch rules, at orientation depth. Link to `tk-conventions.md` for deeper semantics.
3. **Managing projects** — `herdle project list`, `herdle project add <path>` with `--gh`, `--remote`, `--base`, `--integration`; `herdle project set <name|path>` with the same flags; `herdle project rm <name|path>`.
4. **Diagnostics** — `herdle doctor`, `herdle version`.
5. **Command Reference** — the table below (exact). This heading text (`## Command Reference`) and the first-column format are a contract the drift guard parses; do not rename the heading or move the invocation out of column 1.

```markdown
## Command Reference

| Command | Purpose |
|---|---|
| `herdle` | inside a repo: drilldown; outside: cross-project summary |
| `herdle --all` | force the cross-project summary even inside a repo |
| `herdle --fetch` | `git fetch` each repo first (network; default offline) |
| `herdle <name>` | drilldown for a named project |
| `herdle version` | print the herdle version |
| `herdle project list` | list configured projects |
| `herdle project add <path> --gh <owner/repo> --remote <name> --base <branch> --integration <branch>` | add a project (all flags optional) |
| `herdle project set <name\|path> --gh <owner/repo> --remote <name> --base <branch> --integration <branch>` | update fields on a project |
| `herdle project rm <name\|path>` | remove a project |
| `herdle init --force --uninstall` | write/refresh/remove embedded skills + rules (flags optional) |
| `herdle doctor` | diagnose the herdle setup |
```

- [ ] **Step 3: Sanity-check the surface is fully covered**

Run: `for c in version "project add" "project set" "project rm" "project list" init doctor; do grep -q "$c" docs/usage.md && echo "ok: $c" || echo "MISSING: $c"; done`
Expected: every line prints `ok:`.

- [ ] **Step 4: Commit**

```bash
git add docs/usage.md
git commit -m "docs(her-1sd1): usage guide + command reference table"
```

---

### Task 3: The drift guard (Ginkgo spec)

TDD: synthetic logic first (red on undefined helpers → green), then the real-docs assertions (green against `docs/usage.md`).

**Files:**
- Create: `cmd/herdle/docs_drift_test.go`

- [ ] **Step 1: Write the full spec file**

Write `cmd/herdle/docs_drift_test.go` exactly:

```go
package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/urfave/cli/v2"
)

// docsSurface is the set of command paths and primary flag names a build of
// herdle exposes, with urfave/cli builtins filtered out.
type docsSurface struct {
	commands map[string]bool // e.g. "project add"
	flags    map[string]bool // primary name, e.g. "--gh"
}

var docsBuiltinCommands = map[string]bool{"help": true, "h": true}
var docsBuiltinFlags = map[string]bool{
	"--help": true, "-h": true, "--version": true, "-v": true,
}

// primaryFlagName returns the canonical documented token for a flag: "--long"
// when a long name exists, else "-x" for a single-character-only flag.
func primaryFlagName(f cli.Flag) string {
	names := f.Names()
	if len(names) == 0 {
		return ""
	}
	for _, n := range names {
		if len(n) > 1 {
			return "--" + n
		}
	}
	return "-" + names[0]
}

// collectDocsSurface walks an app's command tree collecting command paths and
// non-builtin flag names.
func collectDocsSurface(app *cli.App) docsSurface {
	s := docsSurface{commands: map[string]bool{}, flags: map[string]bool{}}

	addFlag := func(f cli.Flag) {
		name := primaryFlagName(f)
		if name == "" || docsBuiltinFlags[name] {
			return
		}
		s.flags[name] = true
	}
	for _, f := range app.Flags {
		addFlag(f)
	}

	var walk func(prefix string, cmds []*cli.Command)
	walk = func(prefix string, cmds []*cli.Command) {
		for _, c := range cmds {
			if docsBuiltinCommands[c.Name] {
				continue
			}
			path := strings.TrimSpace(prefix + " " + c.Name)
			s.commands[path] = true
			for _, f := range c.Flags {
				addFlag(f)
			}
			walk(path, c.Subcommands)
		}
	}
	walk("", app.Commands)
	return s
}

// missingFromCorpus returns surface tokens (commands then flags) absent as
// substrings from corpus.
func missingFromCorpus(s docsSurface, corpus string) []string {
	var missing []string
	for c := range s.commands {
		if !strings.Contains(corpus, c) {
			missing = append(missing, "command: "+c)
		}
	}
	for f := range s.flags {
		if !strings.Contains(corpus, f) {
			missing = append(missing, "flag: "+f)
		}
	}
	return missing
}

// commandRefRows extracts the first-column cells of the table under the
// "## Command Reference" heading in usage.md. Returns nil if the heading or a
// table is absent (caller treats that as a failure).
func commandRefRows(usage string) []string {
	lines := strings.Split(usage, "\n")
	var rows []string
	inSection := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if strings.HasPrefix(trimmed, "## ") {
			inSection = strings.Contains(strings.ToLower(trimmed), "command reference")
			continue
		}
		if !inSection || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(trimmed, "|") // ["", col1, col2, ""]
		if len(cells) < 2 {
			continue
		}
		rows = append(rows, strings.TrimSpace(cells[1]))
	}
	return rows
}

// invalidCommandRefTokens parses each first-column cell and returns tokens that
// reference a command or flag the surface does not expose.
func invalidCommandRefTokens(rows []string, s docsSurface) []string {
	var invalid []string
	for _, cell := range rows {
		clean := strings.ReplaceAll(cell, "`", "")
		toks := strings.Fields(clean)
		if len(toks) == 0 || toks[0] != "herdle" {
			// header row ("Command"), separator (e.g. "---"), or a standalone flag row
			if len(toks) > 0 && strings.HasPrefix(toks[0], "-") {
				// skip markdown table separator rows like "---" or ":---:"
				isSep := true
				for _, ch := range toks[0] {
					if ch != '-' && ch != ':' {
						isSep = false
						break
					}
				}
				if !isSep && !s.flags[toks[0]] {
					invalid = append(invalid, "flag: "+toks[0])
				}
			}
			continue
		}
		var cmdWords []string
		for _, t := range toks[1:] {
			switch {
			case strings.HasPrefix(t, "<"): // placeholder like <name>
				continue
			case strings.HasPrefix(t, "-"): // flag
				if !s.flags[t] {
					invalid = append(invalid, "flag: "+t)
				}
			default:
				cmdWords = append(cmdWords, t)
			}
		}
		if len(cmdWords) == 0 {
			continue // bare `herdle` (root) — nothing to verify
		}
		path := strings.Join(cmdWords, " ")
		if !s.commands[path] {
			invalid = append(invalid, "command: "+path)
		}
	}
	return invalid
}

// repoRootFromTest locates the repository root relative to this test file.
func repoRootFromTest() string {
	_, file, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	// file = <root>/cmd/herdle/docs_drift_test.go
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// readDocsCorpus concatenates README.md, CONTRIBUTING.md and the top-level
// docs/*.md pages (excluding the internal docs/superpowers/ tree).
func readDocsCorpus(root string) string {
	var b strings.Builder
	read := func(p string) {
		if data, err := os.ReadFile(p); err == nil { // #nosec G304 -- reads repo doc files under the test repo root
			b.Write(data)
			b.WriteString("\n")
		}
	}
	read(filepath.Join(root, "README.md"))
	read(filepath.Join(root, "CONTRIBUTING.md"))
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	for _, m := range matches {
		read(m)
	}
	return b.String()
}

var _ = Describe("docs drift guard", func() {
	Describe("matching logic (synthetic)", func() {
		fakeApp := func() *cli.App {
			return &cli.App{
				Name: "herdle",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "all", Aliases: []string{"a"}},
				},
				Commands: []*cli.Command{
					{Name: "doctor"},
					{
						Name: "project",
						Subcommands: []*cli.Command{
							{Name: "add", Flags: []cli.Flag{&cli.StringFlag{Name: "gh"}}},
						},
					},
				},
			}
		}

		It("collects command paths and non-builtin flags", func() {
			s := collectDocsSurface(fakeApp())
			Expect(s.commands).To(HaveKey("doctor"))
			Expect(s.commands).To(HaveKey("project"))
			Expect(s.commands).To(HaveKey("project add"))
			Expect(s.flags).To(HaveKey("--all"))
			Expect(s.flags).To(HaveKey("--gh"))
			Expect(s.flags).NotTo(HaveKey("--help"))
		})

		It("reports surface tokens absent from the corpus", func() {
			s := collectDocsSurface(fakeApp())
			Expect(missingFromCorpus(s, "nothing here")).To(ContainElement("command: project add"))
			full := "herdle doctor; herdle project add --gh --all"
			Expect(missingFromCorpus(s, full)).To(BeEmpty())
		})

		It("flags command-reference rows that name a nonexistent command or flag", func() {
			s := collectDocsSurface(fakeApp())
			rows := []string{"`herdle project add --gh`", "`herdle bogus`", "`--nope`"}
			invalid := invalidCommandRefTokens(rows, s)
			Expect(invalid).To(ContainElement("command: bogus"))
			Expect(invalid).To(ContainElement("flag: --nope"))
			Expect(invalid).NotTo(ContainElement("command: project add"))
		})
	})

	Describe("real docs vs the live CLI", func() {
		var s docsSurface
		var root string
		BeforeEach(func() {
			s = collectDocsSurface(newApp())
			root = repoRootFromTest()
		})

		It("documents every command and flag herdle exposes", func() {
			missing := missingFromCorpus(s, readDocsCorpus(root))
			Expect(missing).To(BeEmpty(), "undocumented CLI surface: %v", missing)
		})

		It("has a Command Reference table that only names real commands/flags", func() {
			usage, err := os.ReadFile(filepath.Join(root, "docs", "usage.md")) // #nosec G304 -- reads docs/usage.md under the test repo root
			Expect(err).NotTo(HaveOccurred())
			rows := commandRefRows(string(usage))
			Expect(rows).NotTo(BeEmpty(), "no '## Command Reference' table found in docs/usage.md")
			invalid := invalidCommandRefTokens(rows, s)
			Expect(invalid).To(BeEmpty(), "command reference names unknown surface: %v", invalid)
		})
	})
})
```

- [ ] **Step 2: Run the suite**

Run: `PATH=".venv/bin:$PATH" rtk pytest` is **not** applicable — this is Go. Run: `go tool ginkgo -r ./cmd/herdle`
Expected: PASS. The synthetic specs prove the matching logic bites; the real-docs specs pass because `docs/usage.md` covers the full surface.

- [ ] **Step 3: Run lint + full test (the CI gate)**

Run: `make test`
Expected: PASS (gofmt/staticcheck/gosec clean, all suites green).

- [ ] **Step 4: Commit**

```bash
git add cmd/herdle/docs_drift_test.go
git commit -m "test(her-1sd1): docs drift guard over newApp() surface"
```

---

### Task 4: README.md — lean front door

**Files:**
- Modify: `README.md` (replace the 9-line stub entirely)

- [ ] **Step 1: Rewrite `README.md`**

No status/construction banner (present herdle as ready). Sections:

1. Title + tagline (`> **Wrangle the herd, spot the hurdles.**`) + one paragraph on what herdle is — lead with the convention-layer value: a self-contained Go binary that gives a cross-project, tk-driven WIP dashboard plus the convention skills/rules it installs.
2. **What you get** — the cross-project dashboard and the skills + rule stub `herdle init` installs.
3. **Requirements at a glance** — required: `git`, `tk`, superpowers; optional: `gh` (authenticated) + a GitHub remote. Link to `docs/install.md` for the full contract.
4. **Quickstart** — download `herdle-<os>-<arch>` for your platform, put it on `PATH`, then:

```bash
herdle init      # writes skills + rules, seeds config
herdle doctor    # verify the setup
herdle --all     # cross-project summary
```

5. One annotated sample dashboard view (paste the `NO_COLOR=1 ./herdle` capture from Task 2).
6. **Docs** — bullet links to `docs/install.md`, `docs/usage.md`, `docs/configuration.md`, `docs/tk-conventions.md`, and `CONTRIBUTING.md`.
7. License (MIT — link `LICENSE`).

- [ ] **Step 2: Verify drift guard still green**

Run: `make test`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(her-1sd1): rewrite README as lean front door"
```

---

### Task 5: docs/install.md — dependency contract + install/upgrade/uninstall

**Files:**
- Create: `docs/install.md`

- [ ] **Step 1: Capture the doctor sample**

Run: `./herdle doctor 2>&1`
Keep the output to paste as the "verify" sample.

- [ ] **Step 2: Write `docs/install.md`**

Sections:

1. **Dependency contract** — paste this table exactly:

```markdown
| Dependency | Kind | How to get it |
|---|---|---|
| `tk` (wedow/ticket) | required | `brew install wedow/tools/ticket` |
| `git` | required | system / `brew install git` |
| superpowers plugin | required (for the skills/rules to mean anything) | add its marketplace + `/plugin install` |
| `gh` (authenticated) | optional | `brew install gh && gh auth login` — enables PR/issue features |
| GitHub-hosted remote | optional | enables PR/issue features per-project |
| Go toolchain | dev-only | build from source (see CONTRIBUTING) |
```

2. **Download** — the five release assets and `.sha1` verification:

```markdown
| Platform | Asset |
|---|---|
| Linux x86-64 | `herdle-linux-amd64` |
| Linux arm64 | `herdle-linux-arm64` |
| macOS Intel | `herdle-darwin-amd64` |
| macOS Apple Silicon | `herdle-darwin-arm64` |
| Windows x86-64 | `herdle-windows-amd64` |
```

Show: download the asset + its `.sha1` from the latest GitHub Release, verify with `sha1sum -c herdle-<os>-<arch>.sha1`, `chmod +x`, and move onto `PATH` (e.g. `~/bin` or `/usr/local/bin`).

3. **`herdle init`** — writes the skills → `~/.claude/skills/`, the rule stub → `~/.claude/rules/`, seeds `~/.config/herdle/`, and migrates `~/.config/wip/projects` if present. Idempotent.
4. **Verify** — run `herdle doctor`; paste the captured sample; note non-zero exit on missing required deps.
5. **Upgrade** — download the new binary, then `herdle init --force` to re-lay the skills/rules.
6. **Uninstall** — `herdle init --uninstall` removes only the skills/rule herdle wrote; it never edits `CLAUDE.md` and leaves `~/.config/herdle/` in place.

- [ ] **Step 3: Verify asset names match release.yml**

Run: `for a in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64; do grep -q "herdle-$a" docs/install.md && grep -rq "$a" .github/workflows/release.yml && echo "ok: $a" || echo "MISMATCH: $a"; done`
Expected: every line prints `ok:`.

- [ ] **Step 4: Verify drift guard still green, then commit**

```bash
make test
git add docs/install.md
git commit -m "docs(her-1sd1): install guide + dependency contract"
```

---

### Task 6: docs/configuration.md — config format, autodetection, migration, env

**Files:**
- Create: `docs/configuration.md`

- [ ] **Step 1: Write `docs/configuration.md`**

Sections:

1. **Location & format** — `~/.config/herdle/config.toml`, hand-editable, managed by `herdle project`. Paste this example:

```toml
default_remote = "upstream"   # optional global defaults
default_base   = "main"

[[project]]
path        = "/path/to/repo"
gh          = "owner/repo"      # optional; enables PR/issue features
remote      = "upstream"        # optional; autodetect if unset
base        = "dev"             # optional; autodetect if unset
integration = "geoff-main"      # optional; personal integration branch

[[project]]
path = "/path/to/plain"          # no gh -> git+tk view only
```

2. **Field reference** — `path`, `gh`, `remote`, `base`, `integration` (one line each).
3. **Autodetection** — remote: `upstream` else `origin`; base: remote `HEAD` else `main`/`master`.
4. **Migration from `wip`** — one-time import of the legacy `~/.config/wip/projects` line format on first run / `herdle init`.
5. **Environment variables** — `HERDLE_GIT` / `HERDLE_GH` / `HERDLE_TK` (override the resolved binary paths); `NO_COLOR` / `FORCE_COLOR` (rendering).

- [ ] **Step 2: Verify drift guard still green, then commit**

```bash
make test
git add docs/configuration.md
git commit -m "docs(her-1sd1): configuration reference"
```

---

### Task 7: docs/tk-conventions.md — orient, then point to the skills

**Files:**
- Create: `docs/tk-conventions.md`

- [ ] **Step 1: Write `docs/tk-conventions.md`**

Orientation depth only — do not re-teach the skills verbatim. Sections:

1. **Why** — the tk convention layer is the value herdle shares.
2. **Lifecycle** — `-` → `designed` → `planned` → `in-development` → `pending-validation` → `validated`, plus "an open (or merged) PR ⇒ `validated`".
3. **Correlation** — tk ⇄ branch ⇄ PR via `external-ref` (`gh-N`/`#N`) or an explicit `branch:` field — briefly.
4. **Design artifacts** — the filename pattern `docs/superpowers/{specs,plans,validation}/YYYY-MM-DD-<tkid>-<slug>` and how `herdle` correlates them to tickets by the embedded `<tkid>`.
5. **The installed skills are authoritative** — point to `herdle-tk-flow` and `herdle-tk-artifacts` (installed under `~/.claude/skills/` by `herdle init`) as the agent-facing source of truth, and note the always-on `~/.claude/rules/herdle.md` stub that orients toward them.

- [ ] **Step 2: Verify drift guard still green, then commit**

```bash
make test
git add docs/tk-conventions.md
git commit -m "docs(her-1sd1): tk conventions orientation"
```

---

### Task 8: CONTRIBUTING.md — build, test, the drift guard, layout

**Files:**
- Create: `CONTRIBUTING.md`

- [ ] **Step 1: Write `CONTRIBUTING.md`**

Sections:

1. **Prerequisites** — Go 1.26.x.
2. **Build & install** — `make build`; `go install ./cmd/herdle` for a dev install; `make all` runs `vet lint test build`.
3. **Testing** — Ginkgo specs + Counterfeiter fakes; `make test`; `go generate ./...` to regenerate fakes.
4. **Docs stay in sync** — the `cmd/herdle/docs_drift_test.go` guard fails when a command/flag is undocumented or when `docs/usage.md`'s Command Reference table names surface that no longer exists. To fix a failure: document the new command/flag in `docs/usage.md` (and update the Command Reference table), or correct/remove the stale table row.
5. **Repo layout** — `cmd/herdle/` (CLI wiring), `internal/` (engine/render/vcs/config/initcmd/doctor), `assets/` (embedded skills + rule), `docs/` (user docs), `docs/superpowers/` (internal specs/plans/validation).
6. **Lint stack** — `gofmt`, `staticcheck`, `gosec` (run by `make lint`).

- [ ] **Step 2: Verify drift guard still green, then commit**

```bash
make test
git add CONTRIBUTING.md
git commit -m "docs(her-1sd1): contributor guide"
```

---

### Task 9: Finalize

**Files:** validation doc + script.

- [ ] **Step 1: First code-review pass (medium, auto-fix)**

Dispatch a subagent running `/code-review her-1sd1-s11-readme-docs medium --fix`. Review and accept the applied fixes; re-run `make test`.

- [ ] **Step 2: Second code-review pass (high, auto-fix)**

Dispatch a subagent running `/code-review her-1sd1-s11-readme-docs high --fix`. Review and accept; re-run `make test`.

- [ ] **Step 3: Squash the branch into one commit**

```bash
git reset --soft $(git merge-base main HEAD)
git commit -m "feat(her-1sd1): S11 README + install/usage/contributor docs + drift guard"
```

- [ ] **Step 4: Set lifecycle to pending-validation**

Edit `.tickets/her-1sd1.md`: change `lifecycle: in-development` to `lifecycle: pending-validation`. (Amend the squash commit to include it: `git add .tickets/her-1sd1.md && git commit --amend --no-edit`.)

- [ ] **Step 5: Write the validation doc**

Create `docs/superpowers/validation/2026-06-14-her-1sd1-s11-readme-docs-validation.md` with concrete acceptance steps:
- `make test` passes (drift guard green).
- Every command in `docs/usage.md`'s Command Reference table runs (`herdle <cmd> --help` exits 0).
- Install asset names in `docs/install.md` match `.github/workflows/release.yml`.
- Internal Markdown links in `README.md` resolve to existing files.
- README has no construction/pre-release banner; tagline present.
- Manual read-through: install → init → doctor → dashboard flow is coherent.

- [ ] **Step 6: Write and run a validation script**

Create `docs/superpowers/validation/validate-her-1sd1-s11.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
fail=0

echo "== make test =="
make test

echo "== command reference entries run =="
make build
for c in version "project add" "project set" "project rm" "project list" init doctor; do
  if ./herdle $c --help >/dev/null 2>&1; then echo "ok: herdle $c"; else echo "FAIL: herdle $c"; fail=1; fi
done

echo "== install asset names match release.yml matrix =="
# release.yml builds the asset as herdle-${{ matrix.goos }}-${{ matrix.goarch }};
# confirm the naming template plus each documented platform tuple (goos + goarch).
grep -q 'herdle-${{ matrix.goos }}-${{ matrix.goarch }}' .github/workflows/release.yml \
  || { echo "FAIL: asset naming template missing in release.yml"; fail=1; }
for tuple in linux:amd64 linux:arm64 darwin:amd64 darwin:arm64 windows:amd64; do
  goos=${tuple%:*}; goarch=${tuple#*:}
  if grep -q "herdle-$goos-$goarch" docs/install.md \
     && grep -q "goos: $goos" .github/workflows/release.yml \
     && grep -q "goarch: $goarch" .github/workflows/release.yml; then
    echo "ok: $goos-$goarch"
  else echo "FAIL: $goos-$goarch"; fail=1; fi
done

echo "== README internal links resolve =="
grep -oE '\]\(([^)]+\.md)\)' README.md | sed -E 's/\]\(([^)]+)\)/\1/' | while read -r link; do
  [ -f "$link" ] && echo "ok: $link" || { echo "FAIL: $link"; fail=1; }
done

echo "== README has no construction banner =="
if grep -qi "Early development\|Not yet usable\|🚧" README.md; then echo "FAIL: banner present"; fail=1; else echo "ok: no banner"; fi

exit $fail
```

Run: `bash docs/superpowers/validation/validate-her-1sd1-s11.sh`
Expected: exits 0; every line prints `ok:`.

- [ ] **Step 7: Record results + commit validation artifacts**

Tick off the validation-doc steps the script covered. Then:

```bash
git add docs/superpowers/validation/2026-06-14-her-1sd1-s11-readme-docs-validation.md docs/superpowers/validation/validate-her-1sd1-s11.sh
git commit -m "docs(her-1sd1): S11 validation plan + script"
```

**Do NOT open a PR.** Hand the merge/PR decision to `finishing-a-development-branch` (this repo's CLAUDE.md: merge back to main, no PR, no origin push).

---

## Self-Review

**Spec coverage:**
- Lean README + `docs/` + CONTRIBUTING layout → Tasks 4–8. ✓
- README drops the banner → Task 4 + validation check (Step 6). ✓
- Dependency contract table → Task 5. ✓
- Install/upgrade/uninstall accurate vs binary → Task 5 (+ asset-name check). ✓
- Full command surface documented → Task 2 + drift guard (Task 3). ✓
- Config format/autodetection/migration/env → Task 6. ✓
- tk conventions orient + point to skills → Task 7. ✓
- Drift guard reflecting `newApp()`, forward + reverse, no new workflow → Task 3 (runs in `make test`). ✓
- Verification against shipped binary → sample captures (Tasks 2,5), asset-name check (Task 5), validation script (Task 9). ✓
- Bookkeeping (branch, lifecycle, validation doc) → Tasks 1 & 9. ✓

**Placeholder scan:** Doc tasks specify exact sections + the exact tables/examples to paste; the one code file (drift guard) is given in full. No "TBD"/"handle edge cases"/undefined references. ✓

**Type consistency:** Helper names (`collectDocsSurface`, `missingFromCorpus`, `commandRefRows`, `invalidCommandRefTokens`, `primaryFlagName`, `repoRootFromTest`, `readDocsCorpus`) and the `docsSurface` struct are used consistently across the synthetic and real-docs specs. The `## Command Reference` heading and first-column format used in Task 2 match what `commandRefRows`/`invalidCommandRefTokens` parse in Task 3. ✓
