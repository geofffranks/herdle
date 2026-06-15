package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	sort.Strings(missing) // stable, comparable failure messages across runs
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
				isSep := strings.Trim(toks[0], "-:") == ""
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
