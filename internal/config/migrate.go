package config

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WipProjectsPath returns the legacy wip config location:
// ${XDG_CONFIG_HOME:-$HOME/.config}/wip/projects.
func WipProjectsPath() (string, error) {
	base, err := baseDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "wip", "projects"), nil
}

// MigrateWipProjects parses the legacy wip projects file into sparse Projects.
// Each non-comment line is "path [gh=owner/repo] [slug=group/project]". Both keys
// map onto the single forge-agnostic Slug (herdle no longer has a GitHub-only
// gh=); if a line carries both, slug= wins. A missing file yields an empty slice
// and no error. The caller merges via Config.Add.
func MigrateWipProjects(wipPath string) ([]Project, error) {
	f, err := os.Open(wipPath) // #nosec G304 -- path is the legacy wip config location
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []Project
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		p := Project{Path: fields[0]}
		var gh, slug string
		for _, fld := range fields[1:] {
			switch {
			case strings.HasPrefix(fld, "gh="):
				gh = strings.TrimPrefix(fld, "gh=")
			case strings.HasPrefix(fld, "slug="):
				slug = strings.TrimPrefix(fld, "slug=")
			}
		}
		// Both legacy keys fold into Slug; an explicit slug= wins over a gh=.
		if slug != "" {
			p.Slug = slug
		} else {
			p.Slug = gh
		}
		out = append(out, p)
	}
	return out, sc.Err()
}
