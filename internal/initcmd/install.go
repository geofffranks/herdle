// Package initcmd implements `herdle init`: it lays the embedded convention
// artifacts on disk and performs first-run config seeding. Install/Uninstall take
// an injected fs.FS source and a destination base dir so the logic is unit-tested
// with fstest.MapFS, independent of the real embedded bundle.
package initcmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Action describes what Install or Uninstall did to a single destination file.
type Action string

const (
	Written     Action = "written"
	Overwritten Action = "overwritten"
	Skipped     Action = "skipped" // already present; Install without force
	Removed     Action = "removed"
)

// Result reports the Action taken on one destination file.
type Result struct {
	Path   string
	Action Action
}

// Install mirrors every file in src into claudeDir, preserving the relative tree
// (src "skills/x/SKILL.md" -> claudeDir/skills/x/SKILL.md). Parent dirs are created
// 0o750; files are written atomically. Without force an existing destination file
// is left untouched (Skipped); with force it is Overwritten.
func Install(src fs.FS, claudeDir string, force bool) ([]Result, error) {
	var results []Result
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		dest := filepath.Join(claudeDir, filepath.FromSlash(p))
		action := Written
		if _, statErr := os.Stat(dest); statErr == nil {
			if !force {
				results = append(results, Result{Path: dest, Action: Skipped})
				return nil
			}
			action = Overwritten
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return statErr // a real stat failure (e.g. permissions) must not pass as a fresh write
		}
		data, readErr := fs.ReadFile(src, p)
		if readErr != nil {
			return readErr
		}
		if writeErr := writeAtomic(dest, data, 0o644); writeErr != nil {
			return writeErr
		}
		results = append(results, Result{Path: dest, Action: action})
		return nil
	})
	return results, err
}

// writeAtomic writes data to path via a temp file + rename, creating parent dirs.
// Mirrors config.SaveTo's house style. os.CreateTemp makes the temp file 0o600,
// so it is explicitly chmod'd to mode before the rename — setting the final mode
// on the temp file (not after the rename) means the destination is never briefly
// visible at the wrong permissions. Skills/rules pass 0o644 (world-readable);
// settings.json passes its preserved/owner-only mode.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".init-*.tmp")
	if err != nil {
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// Uninstall removes the destination files Install(src, claudeDir, ...) would write
// and prunes any now-empty directories it owns. It never removes files herdle did
// not ship (os.Remove only deletes empty dirs, so a foreign file keeps its dir
// alive), and never touches config or CLAUDE.md. A destination already gone is not
// an error.
func Uninstall(src fs.FS, claudeDir string) ([]Result, error) {
	var results []Result
	var dirs []string // collected parent-first; pruned child-first (reverse)
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if p != "." {
				dirs = append(dirs, filepath.Join(claudeDir, filepath.FromSlash(p)))
			}
			return nil
		}
		dest := filepath.Join(claudeDir, filepath.FromSlash(p))
		if _, statErr := os.Stat(dest); statErr != nil {
			if errors.Is(statErr, fs.ErrNotExist) {
				return nil // already gone
			}
			return statErr // a real stat failure (e.g. permissions) must not pass as removed
		}
		if rmErr := os.Remove(dest); rmErr != nil {
			return rmErr
		}
		results = append(results, Result{Path: dest, Action: Removed})
		return nil
	})
	if err != nil {
		return results, err
	}
	// WalkDir lists parents before children, so reverse order removes the deepest
	// dirs first. os.Remove silently fails on a non-empty (foreign-occupied) dir.
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i])
	}
	return results, nil
}
