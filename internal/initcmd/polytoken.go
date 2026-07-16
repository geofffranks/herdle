package initcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	polytokenHookName = "herdle-gatekeeper" // #nosec G101 -- hook name, not a credential
	contextBlock      = "<!-- herdle:begin -->\n@herdle.md\n<!-- herdle:end -->\n"
	contextBegin      = "<!-- herdle:begin -->"
	contextEnd        = "<!-- herdle:end -->"
)

type polytokenHook struct {
	Name    string `json:"name"`
	Event   string `json:"event"`
	Matcher string `json:"matcher"`
	Handler struct {
		Bash string `json:"bash"`
	} `json:"handler"`
}

// PolytokenHookInspection describes the managed gatekeeper hook, if present.
type PolytokenHookInspection struct {
	Count   int
	Event   string
	Matcher string
	Command string
}

// AgentContextInspection describes the managed AGENTS.md block, if present.
type AgentContextInspection struct {
	Count int
	Exact bool
}

type parsedHooks struct {
	entries []json.RawMessage
	index   int
	hook    polytokenHook
	exists  bool
}

type parsedContext struct {
	data   []byte
	count  int
	begin  int
	end    int
	exact  bool
	exists bool
}

func isPolytokenStandalone(path string) bool {
	return path == "herdle.md" ||
		path == "skills/herdle-tk-flow/SKILL.md" ||
		path == "skills/herdle-tk-artifacts/SKILL.md"
}

// InstallPolytoken installs standalone Polytoken assets and surgically merges
// Herdle's gatekeeper hook and AGENTS.md context block.
func InstallPolytoken(src fs.FS, dir, command string, force bool) ([]Result, error) {
	results, err := installSelected(src, dir, force, isPolytokenStandalone)
	if err != nil {
		return results, err
	}
	for _, merge := range []func() (Result, error){
		func() (Result, error) { return mergePolytokenHooks(filepath.Join(dir, "hooks.json"), command) },
		func() (Result, error) { return mergeAgentContext(filepath.Join(dir, "AGENTS.md")) },
	} {
		result, mergeErr := merge()
		if mergeErr != nil {
			return results, mergeErr
		}
		results = append(results, result)
	}
	return results, nil
}

// UninstallPolytoken removes standalone assets and only Herdle-owned shared content.
func UninstallPolytoken(src fs.FS, dir string) ([]Result, error) {
	var results []Result
	for _, unmerge := range []func() (Result, error){
		func() (Result, error) { return unmergePolytokenHooks(filepath.Join(dir, "hooks.json")) },
		func() (Result, error) { return unmergeAgentContext(filepath.Join(dir, "AGENTS.md")) },
	} {
		result, err := unmerge()
		if err != nil {
			return results, err
		}
		if result.Action != "" {
			results = append(results, result)
		}
	}
	standalone, err := uninstallSelected(src, dir, isPolytokenStandalone)
	return append(results, standalone...), err
}

// InspectPolytokenHooks reads and validates hooks.json without changing it.
func InspectPolytokenHooks(path string) (PolytokenHookInspection, error) {
	parsed, _, err := parsePolytokenHooks(path)
	if err != nil {
		return PolytokenHookInspection{}, err
	}
	if parsed.index < 0 {
		return PolytokenHookInspection{}, nil
	}
	return PolytokenHookInspection{Count: 1, Event: parsed.hook.Event, Matcher: parsed.hook.Matcher, Command: parsed.hook.Handler.Bash}, nil
}

// InspectAgentContext reads and validates AGENTS.md without changing it.
func InspectAgentContext(path string) (AgentContextInspection, error) {
	parsed, _, err := parseAgentContext(path)
	if err != nil {
		return AgentContextInspection{}, err
	}
	return AgentContextInspection{Count: parsed.count, Exact: parsed.exact}, nil
}

// PolytokenGatekeeperCommand builds the pre_tool_use hook command. It resolves
// herdle via $HOME/bin/herdle rather than baking in the absolute os.Executable
// path, so the shared config works across machines and containers. It fails
// open (allows) when herdle is absent.
func PolytokenGatekeeperCommand() string {
	const herdle = "$HOME/bin/herdle"
	return fmt.Sprintf("if [ -x %q ]; then exec %q hook gatekeeper --agent polytoken; else exit 0; fi", herdle, herdle)
}

func mergePolytokenHooks(path, command string) (Result, error) {
	parsed, mode, err := parsePolytokenHooks(path)
	if err != nil {
		return Result{}, err
	}
	hook := polytokenHook{Name: polytokenHookName, Event: "pre_tool_use", Matcher: "*"}
	hook.Handler.Bash = command
	raw, err := json.Marshal(hook)
	if err != nil {
		return Result{}, fmt.Errorf("%s: encode hook: %w", path, err)
	}
	// Serialize the current entries before mutation so a re-run that would not
	// change anything is a no-op (Skipped). MarshalIndent canonicalizes each
	// entry, so logical equality — even across formatting drift — is enough.
	before, err := json.MarshalIndent(parsed.entries, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("%s: encode hooks: %w", path, err)
	}
	if parsed.index >= 0 {
		parsed.entries[parsed.index] = raw
	} else {
		parsed.entries = append(parsed.entries, raw)
	}
	data, err := json.MarshalIndent(parsed.entries, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("%s: encode hooks: %w", path, err)
	}
	data = append(data, '\n')
	if parsed.exists && bytes.Equal(append(before, '\n'), data) {
		return Result{Path: path, Action: Skipped}, nil
	}
	action := Written
	switch {
	case parsed.index >= 0:
		action = Overwritten // refreshed a stale managed hook
	case parsed.exists:
		action = Merged // appended managed hook alongside existing content
	}
	if !parsed.exists {
		mode = 0o600
	}
	if err := writeAtomic(path, data, mode); err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	return Result{Path: path, Action: action}, nil
}

func unmergePolytokenHooks(path string) (Result, error) {
	parsed, mode, err := parsePolytokenHooks(path)
	if err != nil {
		return Result{}, err
	}
	if parsed.index < 0 {
		return Result{}, nil
	}
	parsed.entries = append(parsed.entries[:parsed.index], parsed.entries[parsed.index+1:]...)
	data, err := json.MarshalIndent(parsed.entries, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("%s: encode hooks: %w", path, err)
	}
	if err := writeAtomic(path, append(data, '\n'), mode); err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	return Result{Path: path, Action: Removed}, nil
}

func parsePolytokenHooks(path string) (parsedHooks, os.FileMode, error) {
	parsed := parsedHooks{index: -1}
	data, mode, exists, err := readShared(path)
	if err != nil {
		return parsed, 0, err
	}
	parsed.exists = exists
	if !exists || len(bytes.TrimSpace(data)) == 0 {
		return parsed, mode, nil
	}
	if err := json.Unmarshal(data, &parsed.entries); err != nil {
		return parsed, mode, fmt.Errorf("%s: invalid hooks array: %w", path, err)
	}
	if parsed.entries == nil {
		return parsed, mode, fmt.Errorf("%s: hooks must be a JSON array", path)
	}
	for i, raw := range parsed.entries {
		var named struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &named); err != nil {
			continue
		}
		if named.Name != polytokenHookName {
			continue
		}
		if parsed.index >= 0 {
			return parsed, mode, fmt.Errorf("%s: duplicate %q hooks", path, polytokenHookName)
		}
		parsed.index = i
		if err := json.Unmarshal(raw, &parsed.hook); err != nil {
			return parsed, mode, fmt.Errorf("%s: invalid managed hook: %w", path, err)
		}
	}
	return parsed, mode, nil
}

func mergeAgentContext(path string) (Result, error) {
	parsed, mode, err := parseAgentContext(path)
	if err != nil {
		return Result{}, err
	}
	// Idempotent: the managed block is already present and exact, so there is
	// nothing to write.
	if parsed.count == 1 && parsed.exact {
		return Result{Path: path, Action: Skipped}, nil
	}
	action := Written
	switch {
	case parsed.count == 1:
		action = Overwritten // refreshed an existing managed block
	case parsed.exists:
		action = Merged // appended managed block to an existing file
	}
	var data []byte
	if parsed.count == 1 {
		data = append(append(append([]byte{}, parsed.data[:parsed.begin]...), contextBlock...), parsed.data[parsed.end:]...)
	} else if len(parsed.data) == 0 {
		data = []byte(contextBlock)
	} else {
		// A new managed block is always preceded by one installation-owned newline.
		// Existing bytes, including any trailing newline, remain untouched.
		data = append(append(append([]byte{}, parsed.data...), '\n'), contextBlock...)
	}
	if !parsed.exists {
		mode = 0o644
	}
	if err := writeAtomic(path, data, mode); err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	return Result{Path: path, Action: action}, nil
}

func unmergeAgentContext(path string) (Result, error) {
	parsed, mode, err := parseAgentContext(path)
	if err != nil {
		return Result{}, err
	}
	if parsed.count == 0 {
		return Result{}, nil
	}
	data := append(append([]byte{}, parsed.data[:parsed.begin]...), parsed.data[parsed.end:]...)
	if err := writeAtomic(path, data, mode); err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	return Result{Path: path, Action: Removed}, nil
}

func parseAgentContext(path string) (parsedContext, os.FileMode, error) {
	var parsed parsedContext
	data, mode, exists, err := readShared(path)
	if err != nil {
		return parsed, 0, err
	}
	parsed.exists = exists
	if !exists {
		return parsed, 0, nil
	}
	parsed.data = data
	text := string(data)
	begins := strings.Count(text, contextBegin)
	ends := strings.Count(text, contextEnd)
	if begins == 0 && ends == 0 {
		return parsed, mode, nil
	}
	if begins != 1 || ends != 1 {
		return parsed, mode, fmt.Errorf("%s: ambiguous herdle context markers", path)
	}
	begin := strings.Index(text, contextBegin)
	endMarker := strings.Index(text, contextEnd)
	if endMarker < begin {
		return parsed, mode, fmt.Errorf("%s: reversed herdle context markers", path)
	}
	end := endMarker + len(contextEnd)
	// Consume the newline that always trails the managed block (it is part of
	// contextBlock). Handle both LF and CRLF so a Windows-encoded AGENTS.md
	// captures the same logical bytes as the LF-only contextBlock constant.
	if end+1 < len(data) && data[end] == '\r' && data[end+1] == '\n' {
		end += 2
	} else if end < len(data) && data[end] == '\n' {
		end++
	}
	parsed.count = 1
	parsed.begin = begin
	parsed.end = end
	// contextBlock uses LF; a CRLF-encoded AGENTS.md (common on Windows or after
	// editor re-saving) captures \r\n sequences, which would fail the exact byte
	// comparison and make doctor false-positive "malformed". Normalize CR away
	// before comparing so only the logical block text is checked.
	parsed.exact = strings.ReplaceAll(string(data[begin:end]), "\r", "") == contextBlock
	return parsed, mode, nil
}

func readShared(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, false, nil
		}
		return nil, 0, false, fmt.Errorf("%s: %w", path, err)
	}
	data, err := os.ReadFile(path) // #nosec G304 -- caller explicitly supplies config path
	if err != nil {
		return nil, 0, false, fmt.Errorf("%s: %w", path, err)
	}
	return data, info.Mode().Perm(), true, nil
}
