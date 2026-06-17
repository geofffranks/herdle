package vcs

import "strings"

// yamlBareKeys extracts the "bare map key" lines (a "key:" with no inline value —
// the shape both gh's hosts.yml and glab's config.yml use for host entries) from
// a YAML config, returning them lowercased for case-insensitive host matching.
//
// When parent is "", keys are collected at the top level (gh's hosts.yml lists
// hosts as top-level keys). When parent is non-empty (e.g. "hosts"), keys are
// collected at the first indent level directly under that top-level parent and the
// block ends at the next top-level key (glab's config.yml nests hosts under a
// top-level `hosts:` map).
//
// This is a deliberately minimal, dependency-free parser: it understands only the
// bare-key shape it needs, not general YAML. It normalizes leading tabs to a space
// before measuring indent, so a tab-indented child is not mistaken for a top-level
// key (YAML forbids tab indentation, but real configs occasionally contain it).
func yamlBareKeys(cfg, parent string) []string {
	var keys []string
	inParent := parent == "" // top-level mode is "always inside"
	childIndent := -1
	for raw := range strings.SplitSeq(cfg, "\n") {
		line := strings.TrimRight(raw, "\r")
		// Normalize leading tabs to spaces so indent is measured consistently and a
		// tab-indented line is never read as top-level.
		line = strings.ReplaceAll(line, "\t", " ")
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(trimmed)

		if parent != "" {
			// Nested mode: find the parent block, collect its first-level children,
			// stop at the next top-level key.
			if !inParent {
				if indent == 0 && trimmed == parent+":" {
					inParent = true
				}
				continue
			}
			if indent == 0 {
				break // a new top-level key ends the parent block
			}
			if childIndent == -1 {
				childIndent = indent
			}
			if indent != childIndent {
				continue // deeper (a child's own settings) — skip
			}
		} else if indent != 0 {
			continue // top-level mode: only indent-0 keys are host keys
		}

		if key, ok := bareKey(trimmed); ok {
			keys = append(keys, key)
		}
	}
	return keys
}

// bareKey reports whether a trimmed line is a bare map key ("key:" with no inline
// value) and returns the key lowercased. A line like "host: gitlab.com" carries an
// inline value and is not a bare key; "gitlab.com:" is.
func bareKey(trimmed string) (string, bool) {
	i := strings.IndexByte(trimmed, ':')
	if i <= 0 || strings.TrimSpace(trimmed[i+1:]) != "" {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(trimmed[:i])), true
}
