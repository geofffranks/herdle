package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

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
				Action: func(c *cli.Context) error {
					d := runGatekeeper(os.Stdin)
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

type rawHookInput struct {
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

// runGatekeeper parses the PreToolUse payload, classifies the lifecycle
// transition, gathers only the evidence that transition needs, and returns the
// gate decision. A malformed envelope fails OPEN (we cannot tell it is a gating
// edit). A confirmed gating edit whose evidence is unreadable fails CLOSED inside
// gate.Decide.
func runGatekeeper(r io.Reader) gate.Decision {
	var raw rawHookInput
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return gate.Decision{Allow: true}
	}
	in := gate.HookInput{
		ToolName:    raw.ToolName,
		FilePath:    raw.ToolInput.FilePath,
		WrittenText: firstNonEmpty(raw.ToolInput.NewString, raw.ToolInput.Content),
		Command:     raw.ToolInput.Command,
	}
	ticketPath, t := gate.ShouldEvaluate(in)
	if t == gate.None {
		return gate.Decision{Allow: true}
	}
	env := gate.Env{Transition: t, TicketPath: ticketPath}
	abs := resolvePath(ticketPath, raw.Cwd)

	switch t {
	case gate.ToPendingValidation:
		env.TicketContent, env.TicketReadOK = readTicket(abs)
		if raw.TranscriptPath != "" {
			if f, err := os.Open(raw.TranscriptPath); err == nil { // #nosec G304 -- path is supplied by Claude Code
				defer func() { _ = f.Close() }()
				env.Transcript = f
			}
		}
		return gate.Decide(in, env) // nil Transcript → fail closed (unless rollback)
	case gate.ToValidated:
		env.TicketContent, env.TicketReadOK = readTicket(abs)
		env.ValidationDocs, env.ValidationFound = readValidationDocs(abs)
		return gate.Decide(in, env)
	case gate.ToInDevelopment:
		env.TicketContent, env.TicketReadOK = readTicket(abs)
		return gate.Decide(in, env)
	}
	// Unreachable: ShouldEvaluate returns only None (handled above) or one of the
	// three transitions handled in the switch. Kept to satisfy the compiler.
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
// <repoRoot>/docs/superpowers/validation/*<tkid>*. found is false when none match
// or none are readable (the validated gate then fails closed).
func readValidationDocs(absTicket string) (docs []string, found bool) {
	root := repoRootFromTicket(absTicket)
	if root == "" {
		return nil, false
	}
	tkid := strings.TrimSuffix(filepath.Base(absTicket), ".md")
	pattern := filepath.Join(root, "docs", "superpowers", "validation", "*"+tkid+"*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil, false
	}
	for _, m := range matches {
		if data, err := os.ReadFile(m); err == nil { // #nosec G304 -- repo-local validation doc
			docs = append(docs, string(data))
		}
	}
	return docs, len(docs) > 0
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
