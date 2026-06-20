package initcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

const gateMarker = "code-review-gate" // identifies our PreToolUse entry across runs

// MergeSettings ensures settings.json carries exactly one PreToolUse entry that
// runs the code-review gate via command. It is idempotent and self-healing: an
// existing gate entry with a stale command is updated. All other keys and hooks
// are preserved. Top-level key order follows Go's json marshaller (sorted).
func MergeSettings(path, command string) (Result, error) {
	m, err := loadSettings(path)
	if err != nil {
		return Result{}, err
	}
	before, _ := json.Marshal(m)

	hooks, err := settingsHooks(m)
	if err != nil {
		return Result{}, err
	}
	pre, err := preToolUse(hooks)
	if err != nil {
		return Result{}, err
	}
	pre = dropGateEntries(pre)
	pre = append(pre, gateEntry(command))
	hooks["PreToolUse"] = pre
	m["hooks"] = hooks

	after, _ := json.Marshal(m)
	if bytes.Equal(before, after) {
		return Result{Path: path, Action: Skipped}, nil
	}
	if err := writeSettings(path, m); err != nil {
		return Result{}, err
	}
	// Written vs Overwritten: Overwritten when a gate entry already existed.
	action := Written
	if bytes.Contains(before, []byte(gateMarker)) {
		action = Overwritten
	}
	return Result{Path: path, Action: action}, nil
}

// UnmergeSettings removes the gate's PreToolUse entry. Absent file or no entry is
// a no-op (Skipped).
func UnmergeSettings(path string) (Result, error) {
	m, err := loadSettings(path)
	if err != nil {
		return Result{}, err
	}
	hooks, err := settingsHooks(m)
	if err != nil {
		return Result{}, err
	}
	pre, err := preToolUse(hooks)
	if err != nil {
		return Result{}, err
	}
	filtered := dropGateEntries(pre)
	if len(filtered) == len(pre) {
		return Result{Path: path, Action: Skipped}, nil
	}
	hooks["PreToolUse"] = filtered
	m["hooks"] = hooks
	if err := writeSettings(path, m); err != nil {
		return Result{}, err
	}
	return Result{Path: path, Action: Removed}, nil
}

func gateEntry(command string) map[string]interface{} {
	return map[string]interface{}{
		"matcher": "Edit|Write|Bash",
		"hooks": []interface{}{
			map[string]interface{}{"type": "command", "command": command},
		},
	}
}

// dropGateEntries returns pre without any entry whose nested command contains the
// gate marker.
func dropGateEntries(pre []interface{}) []interface{} {
	out := make([]interface{}, 0, len(pre))
	for _, e := range pre {
		if entryIsGate(e) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func entryIsGate(e interface{}) bool {
	em, ok := e.(map[string]interface{})
	if !ok {
		return false
	}
	hs, ok := em["hooks"].([]interface{})
	if !ok {
		return false
	}
	for _, h := range hs {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && strings.Contains(cmd, gateMarker) {
			return true
		}
	}
	return false
}

// settingsHooks returns m's "hooks" object, or a fresh empty one when absent. It
// errors on a present-but-non-object value rather than silently discarding it, so
// a merge never clobbers an unexpected user configuration.
func settingsHooks(m map[string]interface{}) (map[string]interface{}, error) {
	raw, ok := m["hooks"]
	if !ok || raw == nil {
		return map[string]interface{}{}, nil
	}
	hooks, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("settings.json: \"hooks\" is %T, expected an object; refusing to modify", raw)
	}
	return hooks, nil
}

// preToolUse returns hooks' "PreToolUse" array, or nil when absent. It errors on
// a present-but-non-array value rather than discarding it.
func preToolUse(hooks map[string]interface{}) ([]interface{}, error) {
	raw, ok := hooks["PreToolUse"]
	if !ok || raw == nil {
		return nil, nil
	}
	pre, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("settings.json: \"hooks.PreToolUse\" is %T, expected an array; refusing to modify", raw)
	}
	return pre, nil
}

// loadSettings reads settings.json into a generic map; an absent file yields an
// empty map and no error.
func loadSettings(path string) (map[string]interface{}, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path is the user's Claude settings.json under ClaudeDir
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]interface{}{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return map[string]interface{}{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, nil
}

// writeSettings marshals m as 2-space-indented JSON and writes it atomically,
// preserving the existing file's permissions. A brand-new settings.json defaults
// to 0o600 since it can hold secrets (env, tokens) — writeAtomic alone would
// force 0o644 and silently widen a user's deliberately-restricted config.
func writeSettings(path string, m map[string]interface{}) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	mode := os.FileMode(0o600)
	if fi, statErr := os.Stat(path); statErr == nil {
		mode = fi.Mode().Perm()
	}
	return writeAtomic(path, data, mode)
}
