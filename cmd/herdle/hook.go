package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/internal/agent"
	"github.com/geofffranks/herdle/internal/gate"
)

// hookCommand is the hidden parent for Claude Code hook handlers.
func hookCommand() *cli.Command {
	return &cli.Command{
		Name:   "hook",
		Usage:  "internal Claude Code hook handlers (not for direct use)",
		Hidden: true,
		Subcommands: []*cli.Command{
			{
				Name: "gatekeeper",
				// code-review-gate is the pre-rename name: an upgraded binary whose
				// settings.json still wires the old command keeps working until the
				// user re-runs `herdle init` (which migrates the entry; `herdle
				// doctor` flags the stale wiring meanwhile). Aliases are invisible to
				// the docs-drift surface, so only `gatekeeper` needs documenting.
				Aliases: []string{"code-review-gate"},
				Usage:   "PreToolUse gate enforcing herdle lifecycle transitions",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "agent", Value: string(agent.Claude), Hidden: true},
				},
				Action: func(c *cli.Context) error {
					harness := agent.Name(c.String("agent"))
					projectDir := ""
					if harness == agent.Polytoken {
						projectDir = firstNonEmpty(os.Getenv("POLYTOKEN_PROJECT_DIR"), os.Getenv("POLYTOKEN_PROJECT_PATH"))
					}
					d := runGatekeeper(os.Stdin, harness, projectDir)
					if d.Allow {
						return nil // exit 0
					}
					_, _ = io.WriteString(c.App.ErrWriter, d.Reason+"\n")
					return cli.Exit("", 2) // exit 2 blocks the tool call
				},
			},
		},
	}
}

type rawClaudeHookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath  string `json:"file_path"`
		NewString string `json:"new_string"`
		Content   string `json:"content"`
		Command   string `json:"command"`
	} `json:"tool_input"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
}

type rawPolytokenHookInput struct {
	ToolName string `json:"tool_name"`
	Input    struct {
		Path      string `json:"path"`
		NewString string `json:"new_string"`
		Content   string `json:"content"`
		Command   string `json:"command"`
	} `json:"input"`
}

func parseClaudeHook(r io.Reader) (gate.HookInput, rawClaudeHookInput, bool) {
	var raw rawClaudeHookInput
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return gate.HookInput{}, raw, false
	}
	return gate.HookInput{
		ToolName:    raw.ToolName,
		FilePath:    raw.ToolInput.FilePath,
		WrittenText: firstNonEmpty(raw.ToolInput.NewString, raw.ToolInput.Content),
		Command:     raw.ToolInput.Command,
	}, raw, true
}

func resolvePolytokenTicketPath(path, projectDir string) (resolved string, readable bool) {
	if filepath.IsAbs(path) {
		return path, true
	}
	if projectDir != "" {
		return filepath.Join(projectDir, path), true
	}
	return filepath.Join(string(filepath.Separator), path), false
}

func parsePolytokenHook(r io.Reader) (gate.HookInput, bool) {
	var raw rawPolytokenHookInput
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return gate.HookInput{}, false
	}
	in := gate.HookInput{
		FilePath:    raw.Input.Path,
		WrittenText: firstNonEmpty(raw.Input.NewString, raw.Input.Content),
		Command:     raw.Input.Command,
	}
	switch raw.ToolName {
	case "file_edit_search_replace":
		in.ToolName = "Edit"
	case "file_write":
		in.ToolName = "Write"
	case "shell_exec":
		in.ToolName = "Bash"
	default:
		return gate.HookInput{}, false
	}
	return in, true
}

// runGatekeeper parses the harness envelope, classifies the lifecycle
// transition, gathers only the evidence that transition needs, and returns the
// gate decision. Malformed and irrelevant envelopes fail open. Once a gating
// transition is recognized, unreadable required evidence fails closed.
func runGatekeeper(r io.Reader, harness agent.Name, projectDir string) gate.Decision {
	var in gate.HookInput
	var claudeRaw rawClaudeHookInput
	var ok bool
	switch harness {
	case agent.Polytoken:
		in, ok = parsePolytokenHook(r)
	case agent.Claude:
		in, claudeRaw, ok = parseClaudeHook(r)
	default:
		return gate.Decision{Allow: true}
	}
	if !ok {
		return gate.Decision{Allow: true}
	}
	pathReadable := true
	if harness == agent.Polytoken && in.ToolName != "Bash" {
		in.FilePath, pathReadable = resolvePolytokenTicketPath(in.FilePath, projectDir)
	}
	ticketPath, t := gate.ShouldEvaluate(in)
	if t == gate.None {
		return gate.Decision{Allow: true}
	}
	env := gate.Env{Transition: t, TicketPath: ticketPath}
	root := projectDir
	if harness == agent.Claude {
		root = claudeRaw.Cwd
	}
	abs := resolvePath(ticketPath, root)
	if in.ToolName == "Bash" {
		pathReadable = filepath.IsAbs(ticketPath) || root != ""
	}

	switch t {
	case gate.ToPendingValidation:
		if pathReadable {
			env.TicketContent, env.TicketReadOK = readTicket(abs)
		}
		if harness == agent.Polytoken {
			var docs []string
			var found, allReadable bool
			if pathReadable {
				docs, found, allReadable = readValidationDocs(abs)
			}
			env.ReviewEvidence = gate.PolytokenReviewEvidence(docs, found && allReadable)
		} else {
			var transcript io.Reader
			if claudeRaw.TranscriptPath != "" {
				if f, err := os.Open(claudeRaw.TranscriptPath); err == nil { // #nosec G304 -- path is supplied by Claude Code
					defer func() { _ = f.Close() }()
					transcript = f
				}
			}
			env.ReviewEvidence = gate.ClaudeReviewEvidence(transcript, ticketPath)
		}
	case gate.ToValidated:
		if pathReadable {
			env.TicketContent, env.TicketReadOK = readTicket(abs)
			// Invariant: when ValidationReadOK is true, ValidationDocs holds the
			// contents of every matched validation doc and every one was readable,
			// and ValidationFound is true. decideValidated relies on this — it only
			// runs OpenItemCount over ValidationDocs when ValidationFound &&
			// ValidationReadOK — so an unreadable match must surface as
			// ValidationReadOK == false (and an empty-but-matched set is forbidden
			// by readValidationDocs returning found == false). Never append a doc
			// that could not be read.
			env.ValidationDocs, env.ValidationFound, env.ValidationReadOK = readValidationDocs(abs)
		}
	case gate.ToInDevelopment:
		if pathReadable {
			env.TicketContent, env.TicketReadOK = readTicket(abs)
		}
	}
	return gate.Decide(in, env)
}

// readTicket reads the on-disk ticket file (its pre-edit state, since PreToolUse
// fires before the edit applies). ok is false when the file cannot be read.
func readTicket(abs string) (content string, ok bool) {
	if data, err := os.ReadFile(abs); err == nil { // #nosec G304 -- ticket path from the hook payload
		return string(data), true
	}
	return "", false
}

// resolvePath makes a ticket path absolute, resolving a relative path (from a
// Bash command) against the hook payload's cwd.
func resolvePath(p, cwd string) string {
	if filepath.IsAbs(p) || cwd == "" {
		return p
	}
	return filepath.Join(cwd, p)
}

// repoRootFromTicket returns the repo root for a .tickets/<id>.md path: the
// segment before the LAST "/.tickets/" (the directory immediately holding the
// ticket, correct even when an ancestor path also contains ".tickets"). Returns
// "" when the path is not under .tickets/.
func repoRootFromTicket(absTicket string) string {
	i := strings.LastIndex(absTicket, "/.tickets/")
	if i < 0 {
		return ""
	}
	return absTicket[:i]
}

// readValidationDocs locates the validation doc(s) for the ticket at absTicket and
// returns their contents. The match is the tkid-glob herdle uses elsewhere:
// <repoRoot>/docs/superpowers/validation/*<tkid>*. found reports whether any path
// matched; allReadable is false when any matched path could not be read.
func readValidationDocs(absTicket string) (docs []string, found, allReadable bool) {
	root := repoRootFromTicket(absTicket)
	if root == "" {
		return nil, false, false
	}
	tkid := strings.TrimSuffix(filepath.Base(absTicket), ".md")
	pattern := filepath.Join(root, "docs", "superpowers", "validation", "*"+tkid+"*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil, false, false
	}
	allReadable = true
	for _, m := range matches {
		data, err := os.ReadFile(m) // #nosec G304 -- repo-local validation doc
		if err != nil {
			allReadable = false
			continue
		}
		docs = append(docs, string(data))
	}
	return docs, true, allReadable
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
