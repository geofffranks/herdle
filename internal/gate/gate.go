// Package gate implements the code-review Finalize gate: given a PreToolUse hook
// payload and the session transcript, it decides whether a ticket may transition
// to lifecycle: pending-validation. The core is pure (no process/FS access) so it
// is unit-tested directly; the cmd layer adapts stdin and exit codes.
package gate

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

// HookInput is the normalized subset of the PreToolUse payload the gate needs.
type HookInput struct {
	ToolName       string
	FilePath       string // tool_input.file_path (Edit/Write)
	WrittenText    string // tool_input.new_string, falling back to tool_input.content
	Command        string // tool_input.command (Bash)
	TranscriptPath string // top-level transcript_path
}

// Decision is the gate's verdict. Missing names the absent passes; Reason is the
// stderr message shown to the agent on a block.
type Decision struct {
	Allow   bool
	Missing []string
	Reason  string
}

// ticketPathRE matches a .tickets/<id>.md path inside a shell command. The leading
// class stops at shell separators/quotes so the captured path is just the file.
var ticketPathRE = regexp.MustCompile(`[^\s'"]*\.tickets/[A-Za-z0-9._-]+\.md`)

// overrideRE matches the override marker followed by a non-empty reason; a bare
// marker (no reason) deliberately does not match.
var overrideRE = regexp.MustCompile(`\[skip-code-review-gate\]\s*\S+`)

// writeIndicators are tokens that signal a Bash command writes to a file. ">"
// also covers ">>"; " -i" catches sed/perl in-place edits (a sed/perl write
// always uses -i or a redirect, so the bare command name adds only false
// positives). This is a best-effort heuristic: a read-only command that both
// names a .tickets path and contains one of these tokens can still be gated
// (false positive), and an obfuscated write (base64, heredoc-to-var) can still
// slip (false negative). The override marker is the escape hatch for the former.
var writeIndicators = []string{">", "tee", "printf", " -i"}

const (
	ticketsMarker = "/.tickets/"
	pendingMarker = "lifecycle: pending-validation"
	indevMarker   = "lifecycle: in-development"
)

// ShouldEvaluate reports whether this tool call is a ticket pending-validation
// transition the gate must evaluate, and the ticket file path it targets.
func ShouldEvaluate(in HookInput) (ticketPath string, gate bool) {
	switch in.ToolName {
	case "Edit", "Write":
		if strings.Contains(in.FilePath, ticketsMarker) && strings.Contains(in.WrittenText, pendingMarker) {
			return in.FilePath, true
		}
	case "Bash":
		// Best-effort: a Bash write (sed/echo/printf) of pending-validation into a
		// ticket. An obfuscated write (base64, heredoc-to-var) can still slip — a
		// documented limitation. We require a tickets path AND a write indicator so
		// a read-only grep mentioning the marker is not gated.
		if strings.Contains(in.Command, "pending-validation") && hasWriteIndicator(in.Command) {
			if m := ticketPathRE.FindString(in.Command); m != "" {
				return m, true
			}
		}
	}
	return "", false
}

func hasWriteIndicator(cmd string) bool {
	for _, w := range writeIndicators {
		if strings.Contains(cmd, w) {
			return true
		}
	}
	return false
}

// HasOverride reports whether text carries the override marker with a reason.
func HasOverride(text string) bool { return overrideRE.MatchString(text) }

var (
	commandArgsRE   = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)
	codeReviewSlash = "<command-name>/code-review</command-name>"
)

type tEvent struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}
type tMessage struct {
	Content json.RawMessage `json:"content"`
}
type tBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// EffortsFromTranscript scans the JSONL transcript and returns the set of
// /code-review effort levels ({"medium","high"} subset) invoked after the last
// Edit/Write that set this ticket to lifecycle: in-development. If no such marker
// is found (e.g. a cross-session finalize), all review events are counted. Both
// invocation shapes are recognized: agent Skill tool_use, and a user-typed
// /code-review slash command. Malformed lines are skipped.
func EffortsFromTranscript(r io.Reader, ticketPath string) map[string]bool {
	type review struct {
		pos  int
		args string
	}
	var reviews []review
	indevPos := -1
	pos := 0

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 64*1024*1024) // transcripts can hold large lines
	for sc.Scan() {
		pos++
		var ev tEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		var msg tMessage
		if len(ev.Message) > 0 {
			_ = json.Unmarshal(ev.Message, &msg)
		}
		// Shape B: user slash command — content is a JSON string.
		var s string
		if json.Unmarshal(msg.Content, &s) == nil {
			if strings.Contains(s, codeReviewSlash) {
				reviews = append(reviews, review{pos, extractCommandArgs(s)})
			}
			continue
		}
		// Shape A: content is an array of blocks.
		var blocks []tBlock
		if json.Unmarshal(msg.Content, &blocks) != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type != "tool_use" {
				continue
			}
			switch b.Name {
			case "Skill":
				var si struct {
					Skill string `json:"skill"`
					Args  string `json:"args"`
				}
				_ = json.Unmarshal(b.Input, &si)
				if si.Skill == "code-review" {
					reviews = append(reviews, review{pos, si.Args})
				}
			case "Edit", "Write":
				var ei struct {
					FilePath  string `json:"file_path"`
					NewString string `json:"new_string"`
					Content   string `json:"content"`
				}
				_ = json.Unmarshal(b.Input, &ei)
				text := ei.NewString
				if text == "" {
					text = ei.Content
				}
				if sameTicket(ei.FilePath, ticketPath) && strings.Contains(text, indevMarker) && pos > indevPos {
					indevPos = pos
				}
			}
		}
	}

	efforts := map[string]bool{}
	for _, rv := range reviews {
		if rv.pos <= indevPos { // indevPos == -1 (not found) keeps every review
			continue
		}
		// The effort is a whitespace-delimited token in the args (e.g.
		// "feat/x medium --fix"); match it as a field so a branch name that
		// itself contains "medium"/"high" (e.g. feat/high-priority) is not
		// miscounted as a pass.
		for _, tok := range strings.Fields(strings.ToLower(rv.args)) {
			switch tok {
			case "medium":
				efforts["medium"] = true
			case "high":
				efforts["high"] = true
			}
		}
	}
	return efforts
}

func extractCommandArgs(s string) string {
	if m := commandArgsRE.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// sameTicket reports whether two ticket paths refer to the same ticket file. It
// matches when one path is a path-boundary suffix of the other, so a relative
// path extracted from a Bash command (e.g. ".tickets/her-5s12.md") still matches
// the absolute file_path the transcript records for the in-development Edit —
// without the false match a base-name comparison would make between two tickets
// that share a file name in different projects (".../projA/.tickets/t.md" vs
// ".../projB/.tickets/t.md").
func sameTicket(a, b string) bool {
	return a == b || strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a)
}

// Decide is the gate verdict for one tool call. A non-gating call is allowed
// immediately. A gating call is allowed if it carries the override marker, else
// the transcript must show both medium and high passes after the in-development
// bound. A nil transcript (open failed / absent) fails closed: a gating edit we
// cannot verify is treated as not verified.
func Decide(in HookInput, transcript io.Reader) Decision {
	ticketPath, gating := ShouldEvaluate(in)
	if !gating {
		return Decision{Allow: true}
	}
	overrideText := in.WrittenText
	if in.ToolName == "Bash" {
		overrideText = in.Command
	}
	if HasOverride(overrideText) {
		return Decision{Allow: true}
	}
	if transcript == nil {
		return Decision{Allow: false, Missing: []string{"medium", "high"},
			Reason: failClosedReason}
	}
	efforts := EffortsFromTranscript(transcript, ticketPath)
	var missing []string
	if !efforts["medium"] {
		missing = append(missing, "medium")
	}
	if !efforts["high"] {
		missing = append(missing, "high")
	}
	if len(missing) == 0 {
		return Decision{Allow: true}
	}
	return Decision{Allow: false, Missing: missing, Reason: blockReason(missing)}
}

const failClosedReason = "Finalize gate: cannot read the session transcript to verify the /code-review passes. " +
	"Re-run after invoking the code-review Skill for medium and high, or add [skip-code-review-gate] <reason>."

func blockReason(missing []string) string {
	return "Finalize gate: lifecycle:pending-validation requires both /code-review passes this session. " +
		"Missing: " + strings.Join(missing, ", ") + ". Invoke the code-review Skill directly (not a hand-rolled " +
		"sweep or a subagent), or add [skip-code-review-gate] <reason>."
}
