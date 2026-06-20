package main

import (
	"encoding/json"
	"io"
	"os"

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
				Name:  "code-review-gate",
				Usage: "PreToolUse gate enforcing the mandatory /code-review Finalize passes",
				Action: func(c *cli.Context) error {
					d := runCodeReviewGate(os.Stdin)
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
}

// runCodeReviewGate parses the PreToolUse payload from r, opens the transcript,
// and returns the gate decision. A malformed envelope fails OPEN (allow): we
// cannot tell it is a gating edit, and blocking every tool on a parse glitch is
// worse than missing one. A missing/unreadable transcript on a confirmed gating
// edit fails CLOSED inside gate.Decide (nil reader).
func runCodeReviewGate(r io.Reader) gate.Decision {
	var raw rawHookInput
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return gate.Decision{Allow: true}
	}
	in := gate.HookInput{
		ToolName:       raw.ToolName,
		FilePath:       raw.ToolInput.FilePath,
		WrittenText:    firstNonEmpty(raw.ToolInput.NewString, raw.ToolInput.Content),
		Command:        raw.ToolInput.Command,
		TranscriptPath: raw.TranscriptPath,
	}
	var tr io.Reader
	if in.TranscriptPath != "" {
		if f, err := os.Open(in.TranscriptPath); err == nil { // #nosec G304 -- path is supplied by Claude Code in the hook payload
			defer func() { _ = f.Close() }()
			tr = f
		}
	}
	return gate.Decide(in, tr)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
