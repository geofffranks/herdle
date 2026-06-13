package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	// ErrDuplicate is returned by Add when a project with the same Path exists.
	ErrDuplicate = errors.New("project already configured")
	// ErrNotFound is returned by Find when no project matches.
	ErrNotFound = errors.New("no such project")
)

// AmbiguousError is returned by Find when a basename matches more than one
// project. The caller must disambiguate with an exact path.
type AmbiguousError struct {
	Name  string
	Paths []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q matches multiple projects: %s", e.Name, strings.Join(e.Paths, ", "))
}

// Add appends a project, rejecting a duplicate Path. Pure: no filesystem access.
func (c *Config) Add(p Project) error {
	for _, e := range c.Projects {
		if e.Path == p.Path {
			return ErrDuplicate
		}
	}
	c.Projects = append(c.Projects, p)
	return nil
}

// Find resolves key to a project index: first by exact Path, then by basename.
// A basename matching multiple projects returns *AmbiguousError.
func (c *Config) Find(key string) (int, error) {
	for i, e := range c.Projects {
		if e.Path == key {
			return i, nil
		}
	}
	base := filepath.Base(key)
	var idxs []int
	for i, e := range c.Projects {
		if filepath.Base(e.Path) == base {
			idxs = append(idxs, i)
		}
	}
	switch len(idxs) {
	case 0:
		return -1, ErrNotFound
	case 1:
		return idxs[0], nil
	default:
		paths := make([]string, len(idxs))
		for n, i := range idxs {
			paths[n] = c.Projects[i].Path
		}
		return -1, &AmbiguousError{Name: key, Paths: paths}
	}
}

// Remove deletes the project at idx (caller obtains idx from Find).
func (c *Config) Remove(idx int) {
	c.Projects = append(c.Projects[:idx], c.Projects[idx+1:]...)
}
