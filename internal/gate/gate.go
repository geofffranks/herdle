// Package gate implements the herdle lifecycle gatekeeper: given a PreToolUse hook
// payload and pre-gathered evidence, it decides whether a ticket may transition
// to lifecycle: pending-validation, validated, or in-development. The core is pure
// (no process/FS access) so it is unit-tested directly; the cmd layer adapts stdin,
// reads evidence from disk, and maps exit codes.
package gate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// HookInput is the normalized subset of the PreToolUse payload the gate needs.
type HookInput struct {
	ToolName    string
	FilePath    string // tool_input.file_path (Edit/Write)
	WrittenText string // tool_input.new_string, falling back to tool_input.content
	Command     string // tool_input.command (Bash)
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

var (
	// Each override matches "[<token>] <reason>"; a bare token (no reason)
	// deliberately does not match.
	overrideCodeReview = regexp.MustCompile(`\[skip-code-review-gate\]\s*\S+`)
	overrideValidation = regexp.MustCompile(`\[skip-validation-gate\]\s*\S+`)
	overrideBranch     = regexp.MustCompile(`\[skip-branch-linkage\]\s*\S+`)

	// lifecycleRE captures a ticket's lifecycle frontmatter value.
	lifecycleRE = regexp.MustCompile(`(?m)^lifecycle:\s*(\S+)`)
	// lifecycleLineRE matches a real "lifecycle: <state>" frontmatter LINE in
	// Edit/Write text: the value must be the whole rest of the line (anchored at
	// end), so a value merely mentioned in prose (e.g. "lifecycle: validated is
	// the goal") or a bogus suffixed value ("validated-ish") does not match, and
	// a parenthetical note ("(lifecycle: validated)") is skipped (line not started
	// by "lifecycle:"). Anchored at column 0 (no leading whitespace), matching the
	// on-disk frontmatter reader lifecycleRE. The first match is the frontmatter line.
	lifecycleLineRE = regexp.MustCompile(`(?m)^lifecycle:\s*(in-development|pending-validation|validated)\s*$`)
	// linkRE matches a non-empty branch:/external-ref: frontmatter field — the
	// tk⇄branch correlation the dashboard needs.
	linkRE = regexp.MustCompile(`(?m)^(branch|external-ref):\s*\S+`)
)

// HasOverride reports whether text carries the code-review override with a reason.
func HasOverride(text string) bool { return overrideCodeReview.MatchString(text) }

// editedText is the text a transition wrote: the Bash command, else the
// Edit/Write new_string/content. Used for override and link scanning.
func editedText(in HookInput) string {
	if in.ToolName == "Bash" {
		return in.Command
	}
	return in.WrittenText
}

// currentLifecycle returns the lifecycle value from a ticket's YAML frontmatter
// (the block before the first closing "---"), or "" when absent. Bounding the
// scan to the frontmatter keeps a stray body line such as "lifecycle: validated"
// (a quote, a changelog note) from being read as the ticket's real state — which
// would let a forward bump masquerade as a rollback and skip the code-review gate.
func currentLifecycle(ticket string) string {
	if m := lifecycleRE.FindStringSubmatch(frontmatter(ticket)); m != nil {
		return m[1]
	}
	return ""
}

// frontmatter returns a ticket's YAML frontmatter — the text between the opening
// "---" line and the next "---" line. Content that does not open with "---" (the
// bare "lifecycle: x" form used in unit fixtures) is treated as all-frontmatter,
// as is an unterminated block.
func frontmatter(ticket string) string {
	if !strings.HasPrefix(ticket, "---\n") {
		return ticket
	}
	rest := ticket[len("---\n"):]
	if i := strings.Index(rest, "\n---"); i >= 0 {
		return rest[:i]
	}
	return rest
}

// hasLink reports whether text carries a non-empty branch:/external-ref: field.
func hasLink(text string) bool { return linkRE.MatchString(text) }

// writeIndicators are tokens that signal a Bash command writes to a file. ">"
// also covers ">>"; " -i" catches sed/perl in-place edits (a sed/perl write
// always uses -i or a redirect, so the bare command name adds only false
// positives). This is a best-effort heuristic: a read-only command that both
// names a .tickets path and contains one of these tokens can still be gated
// (false positive), and an obfuscated write (base64, heredoc-to-var) can still
// slip (false negative). The override marker is the escape hatch for the former.
var writeIndicators = []string{">", "tee", "printf", " -i"}

const (
	ticketsMarker   = "/.tickets/"
	indevMarker     = "lifecycle: in-development"
	pendingMarker   = "lifecycle: pending-validation"
	validatedMarker = "lifecycle: validated"
)

// Transition is the lifecycle bump a gating tool call performs; None means the
// call is not a gated transition and must be allowed.
type Transition int

const (
	None Transition = iota
	ToInDevelopment
	ToPendingValidation
	ToValidated
)

// ShouldEvaluate classifies a tool call as a gated lifecycle transition and
// returns the ticket file it targets. A None transition means the call is not a
// gated ticket edit and must be allowed. Edit/Write classify by the actual
// "lifecycle: <state>" frontmatter LINE in the written text (line-anchored, so a
// lifecycle value mentioned in prose/notes does not misclassify); Bash matches
// the same value as a substring in the command plus a write indicator and a
// .tickets path. Bash detection is best-effort — an obfuscated write (base64,
// split key/value) or a command naming multiple lifecycle values (e.g. a sed
// old→new) can misclassify or slip; the override markers are the escape hatch.
func ShouldEvaluate(in HookInput) (ticketPath string, t Transition) {
	switch in.ToolName {
	case "Edit", "Write":
		if !strings.Contains(in.FilePath, ticketsMarker) {
			return "", None
		}
		return in.FilePath, transitionFromWritten(in.WrittenText)
	case "Bash":
		bt := transitionFromCommand(in.Command)
		if bt == None || !hasWriteIndicator(in.Command) {
			return "", None
		}
		if m := ticketPathRE.FindString(in.Command); m != "" {
			return m, bt
		}
	}
	return "", None
}

// transitionFromWritten classifies an Edit/Write by the first real
// "lifecycle: <state>" frontmatter line in its text. The first match is the
// frontmatter field (it precedes any body prose); a value mentioned in a note or
// prose line (which does not begin a line with "lifecycle:" followed only by the
// value) is ignored.
func transitionFromWritten(text string) Transition {
	if m := lifecycleLineRE.FindStringSubmatch(text); m != nil {
		return transitionForValue(m[1])
	}
	return None
}

// transitionFromCommand classifies a Bash command by substring. validated and
// pending-validation are distinct markers (they diverge at "validat-ed" vs
// "validat-ion"); a command naming more than one lifecycle value picks the
// highest-precedence match here, a documented best-effort limitation.
func transitionFromCommand(cmd string) Transition {
	switch {
	case strings.Contains(cmd, validatedMarker):
		return ToValidated
	case strings.Contains(cmd, pendingMarker):
		return ToPendingValidation
	case strings.Contains(cmd, indevMarker):
		return ToInDevelopment
	default:
		return None
	}
}

// transitionForValue maps a lifecycle state value to its transition.
func transitionForValue(v string) Transition {
	switch v {
	case "validated":
		return ToValidated
	case "pending-validation":
		return ToPendingValidation
	case "in-development":
		return ToInDevelopment
	default:
		return None
	}
}

func hasWriteIndicator(cmd string) bool {
	for _, w := range writeIndicators {
		if strings.Contains(cmd, w) {
			return true
		}
	}
	return false
}

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
				// Line-anchored (not a bare substring) so a ticket NOTE that merely
				// mentions "lifecycle: in-development" does not advance the bound and
				// wrongly exclude this session's earlier /code-review passes.
				if sameTicket(ei.FilePath, ticketPath) && transitionFromWritten(text) == ToInDevelopment && pos > indevPos {
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

// ReviewEvidence is the harness-neutral proof required before a forward bump to
// pending-validation. Required order is also the stable order used in denials.
type ReviewEvidence struct {
	ReadOK        bool
	Required      []string
	Present       map[string]bool
	Unreadable    string
	BlockedIntro  string
	BlockedSuffix string
}

// ClaudeReviewEvidence adapts the existing session transcript evidence without
// changing its matching or user-facing denial text.
func ClaudeReviewEvidence(r io.Reader, ticketPath string) ReviewEvidence {
	ev := ReviewEvidence{
		ReadOK:        r != nil,
		Required:      []string{"medium", "high"},
		Present:       map[string]bool{},
		Unreadable:    failClosedReason,
		BlockedIntro:  claudeBlockedIntro,
		BlockedSuffix: claudeBlockedSuffix,
	}
	if r != nil {
		ev.Present = EffortsFromTranscript(r, ticketPath)
	}
	return ev
}

var polytokenReviewMarkers = []struct {
	key  string
	line string
}{
	{"standard-completed", "- [x] Standard review completed"},
	{"standard-addressed", "- [x] Standard review findings addressed"},
	{"deep-completed", "- [x] Deep review completed"},
	{"deep-addressed", "- [x] Deep review findings addressed"},
}

func markdownFence(line string) (char byte, run int, trailingWhitespace bool) {
	leadingSpaces := 0
	for leadingSpaces < len(line) && line[leadingSpaces] == ' ' {
		leadingSpaces++
	}
	if leadingSpaces > 3 {
		return 0, 0, false
	}
	trimmed := line[leadingSpaces:]
	if len(trimmed) == 0 || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, 0, false
	}
	char = trimmed[0]
	for run < len(trimmed) && trimmed[run] == char {
		run++
	}
	return char, run, strings.TrimSpace(trimmed[run:]) == ""
}

// PolytokenReviewEvidence recognizes each durable validation-doc marker only
// when its complete line occurs exactly once outside fenced code blocks.
func PolytokenReviewEvidence(docs []string, found bool) ReviewEvidence {
	ev := ReviewEvidence{
		ReadOK:       found,
		Required:     make([]string, 0, len(polytokenReviewMarkers)),
		Present:      map[string]bool{},
		Unreadable:   polytokenUnreadableReason,
		BlockedIntro: polytokenBlockedIntro,
	}
	counts := map[string]int{}
	for _, marker := range polytokenReviewMarkers {
		ev.Required = append(ev.Required, marker.key)
	}
	for _, doc := range docs {
		var fenceChar byte
		fenceLen := 0
		sc := bufio.NewScanner(strings.NewReader(doc))
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			char, run, trailingWhitespace := markdownFence(line)
			if fenceLen == 0 {
				if run >= 3 {
					fenceChar, fenceLen = char, run
					continue
				}
			} else {
				if char == fenceChar && run >= fenceLen && trailingWhitespace {
					fenceChar, fenceLen = 0, 0
				}
				continue
			}
			for _, marker := range polytokenReviewMarkers {
				if line == marker.line {
					counts[marker.key]++
				}
			}
		}
		if sc.Err() != nil {
			ev.ReadOK = false
		}
	}
	for _, marker := range polytokenReviewMarkers {
		ev.Present[marker.key] = counts[marker.key] == 1
	}
	return ev
}

// Env carries everything Decide needs, pre-read by the cmd adapter so the core
// stays free of process and filesystem access.
type Env struct {
	Transition       Transition
	TicketPath       string         // gated ticket file, for evidence correlation
	ReviewEvidence   ReviewEvidence // ToPendingValidation
	TicketContent    string         // on-disk ticket (pre-edit); all transitions
	TicketReadOK     bool           // the on-disk ticket was readable
	ValidationDocs   []string       // ToValidated: contents of matched validation docs
	ValidationFound  bool           // ToValidated: at least one validation doc matched
	ValidationReadOK bool           // ToValidated: every matched validation doc was readable
}

// Decide is the gatekeeper verdict for one tool call, routed by transition. The
// adapter has already classified the transition and gathered Env.
func Decide(in HookInput, env Env) Decision {
	switch env.Transition {
	case ToPendingValidation:
		return decidePending(in, env)
	case ToValidated:
		return decideValidated(in, env)
	case ToInDevelopment:
		return decideInDevelopment(in, env)
	default: // None or unknown
		return Decision{Allow: true}
	}
}

func decidePending(in HookInput, env Env) Decision {
	if HasOverride(editedText(in)) {
		return Decision{Allow: true}
	}
	// A backward rollback (validated → pending-validation, e.g. a reviewer
	// reopens an approved ticket) or an idempotent re-write changes no code, so
	// the /code-review passes that gated the original forward bump need not be
	// re-run. Confirm the prior state from the on-disk ticket; an unreadable
	// ticket falls through to the fail-closed forward check below.
	if env.TicketReadOK {
		switch currentLifecycle(env.TicketContent) {
		case "validated", "pending-validation":
			return Decision{Allow: true}
		}
	}
	ev := env.ReviewEvidence
	if !ev.ReadOK {
		return Decision{Allow: false, Missing: append([]string(nil), ev.Required...), Reason: ev.Unreadable}
	}
	var missing []string
	for _, key := range ev.Required {
		if !ev.Present[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) == 0 {
		return Decision{Allow: true}
	}
	reason := ev.BlockedIntro + strings.Join(missing, ", ") + "." + ev.BlockedSuffix
	return Decision{Allow: false, Missing: missing, Reason: reason}
}

func decideValidated(in HookInput, env Env) Decision {
	if overrideValidation.MatchString(editedText(in)) {
		return Decision{Allow: true}
	}
	// Monotonic: confirm the prior on-disk state is pending-validation. An
	// unreadable ticket fails closed — we cannot confirm the bump is legal.
	if !env.TicketReadOK {
		return Decision{Allow: false, Reason: validatedUnreadableReason}
	}
	switch currentLifecycle(env.TicketContent) {
	case "validated":
		return Decision{Allow: true} // idempotent re-write of an already-validated ticket
	case "pending-validation":
		// fall through to the open-items check
	default:
		return Decision{Allow: false, Reason: monotonicReason}
	}
	// Open-items: a validation doc must exist, every match must be readable, and
	// the readable contents must have zero unchecked boxes.
	if !env.ValidationFound || !env.ValidationReadOK {
		return Decision{Allow: false, Reason: missingDocReason}
	}
	open := 0
	for _, d := range env.ValidationDocs {
		open += OpenItemCount(d)
	}
	if open > 0 {
		return Decision{Allow: false, Reason: openItemsReason(open)}
	}
	return Decision{Allow: true}
}

func decideInDevelopment(in HookInput, env Env) Decision {
	if overrideBranch.MatchString(editedText(in)) {
		return Decision{Allow: true}
	}
	// A link in the edit itself (Setup may add branch: and the bump together) or
	// already on the on-disk ticket both satisfy the gate.
	if hasLink(editedText(in)) || (env.TicketReadOK && hasLink(env.TicketContent)) {
		return Decision{Allow: true}
	}
	return Decision{Allow: false, Reason: branchLinkageReason}
}

const failClosedReason = "Gatekeeper: cannot read the session transcript to verify the /code-review passes. " +
	"Re-run after invoking the code-review Skill for medium and high, or add [skip-code-review-gate] <reason>."

const claudeBlockedIntro = "Gatekeeper: lifecycle:pending-validation requires both /code-review passes this session. " +
	"Missing: "

const claudeBlockedSuffix = " Invoke the code-review Skill directly (not a hand-rolled sweep or a subagent), " +
	"or add [skip-code-review-gate] <reason>."

const polytokenUnreadableReason = "Gatekeeper: cannot read the ticket-correlated validation doc to verify the Polytoken review markers. " +
	"Record the standard and deep review completion and findings-addressed markers, or add [skip-code-review-gate] <reason>."

const polytokenBlockedIntro = "Gatekeeper: lifecycle:pending-validation requires durable Polytoken review evidence. Missing: "

const monotonicReason = "Gatekeeper: lifecycle:validated requires the ticket to be at " +
	"pending-validation first (no skipping). Bump to pending-validation (which runs the code-review " +
	"check), validate, then set validated — or add [skip-validation-gate] <reason>."

const missingDocReason = "Gatekeeper: lifecycle:validated requires a validation doc " +
	"(docs/superpowers/validation/*<tkid>*). Write one with concrete acceptance steps, check them " +
	"off, then set validated — or add [skip-validation-gate] <reason>."

const validatedUnreadableReason = "Gatekeeper: cannot read the ticket to confirm it is at " +
	"pending-validation before validated. Re-run, or add [skip-validation-gate] <reason>."

const branchLinkageReason = "Gatekeeper: lifecycle:in-development requires the ticket to carry a " +
	"branch: or external-ref so herdle can correlate it. Add one (Setup records branch:), or add " +
	"[skip-branch-linkage] <reason>."

func openItemsReason(n int) string {
	return fmt.Sprintf("Gatekeeper: lifecycle:validated is blocked — the validation doc still has "+
		"%d unchecked item(s) (\"- [ ]\"). Human validation steps must be checked before validated. "+
		"Check them off, or add [skip-validation-gate] <reason>.", n)
}

// taskItemRE matches a Markdown task-list item; the capture is the box content
// (" " unchecked, "x"/"X" checked).
var taskItemRE = regexp.MustCompile(`^\s*[-*+]\s+\[([ xX])\]`)

// OpenItemCount returns the number of unchecked task items ("- [ ]") in a
// validation document. Lines inside fenced code blocks are skipped so an
// example checkbox in prose does not count; a checked box never counts.
//
// Fence detection uses the same markdownFence function as
// PolytokenReviewEvidence so the two scanners agree on what counts as fenced:
// a stray single ``` (or ~~~) before unchecked items does not open a fence on
// its own and the items below are counted normally. Both detectors track the
// opening delimiter character and run length, requiring a matching close.
func OpenItemCount(doc string) int {
	n := 0
	var fenceChar byte
	fenceLen := 0
	sc := bufio.NewScanner(strings.NewReader(doc))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		char, run, trailingWhitespace := markdownFence(line)
		if fenceLen == 0 {
			if run >= 3 {
				fenceChar, fenceLen = char, run
				continue
			}
		} else {
			if char == fenceChar && run >= fenceLen && trailingWhitespace {
				fenceChar, fenceLen = 0, 0
			}
			continue
		}
		if m := taskItemRE.FindStringSubmatch(line); m != nil && m[1] == " " {
			n++
		}
	}
	return n
}
