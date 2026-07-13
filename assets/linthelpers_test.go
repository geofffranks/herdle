package assets_test

import (
	"io/fs"
	"path"
	"strings"
)

// parseFrontmatter extracts top-level "key: value" pairs from a leading
// ---\n...\n--- block. ok is false when no well-formed block is present.
func parseFrontmatter(s string) (map[string]string, bool) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return nil, false
	}
	rest := s[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, false
	}
	m := map[string]string{}
	for _, line := range strings.Split(rest[:end], "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, ":"); i >= 0 {
			m[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
	}
	return m, true
}

// lintSkills structurally validates an embedded skills tree and its standing
// context file. It returns human-readable problems; an empty slice means clean.
// It is content-agnostic (no personal denylist) so it is safe to commit.
func lintSkills(fsys fs.FS, contextPath string) []string {
	var problems []string

	entries, err := fs.ReadDir(fsys, "skills")
	if err != nil {
		problems = append(problems, "skills/: "+err.Error())
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		p := path.Join("skills", name, "SKILL.md")
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			problems = append(problems, p+": missing SKILL.md")
			continue
		}
		fm, ok := parseFrontmatter(string(data))
		if !ok {
			problems = append(problems, p+": missing or malformed frontmatter")
			continue
		}
		if fm["name"] != name {
			problems = append(problems, p+": name "+fm["name"]+" does not match directory "+name)
		}
		if fm["description"] == "" {
			problems = append(problems, p+": empty or missing description")
		}
	}

	context, err := fs.ReadFile(fsys, contextPath)
	switch {
	case err != nil:
		problems = append(problems, contextPath+": missing")
	case strings.TrimSpace(string(context)) == "":
		problems = append(problems, contextPath+": empty")
	case contextPath == "rules/herdle.md":
		if fm, ok := parseFrontmatter(string(context)); ok {
			if _, has := fm["paths"]; has {
				problems = append(problems, contextPath+": has paths: key (must load every session)")
			}
		}
	}

	return problems
}
