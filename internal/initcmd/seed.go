package initcmd

import (
	"errors"
	"io/fs"
	"os"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
)

// SeedConfig performs first-run config seeding, gated on configPath being absent.
// When configPath exists it is a no-op (ran=false). When absent it merges the
// legacy wip projects (migrate) then the discovered Claude projects, via
// Config.Add (dedupe by path, keep first), and writes configPath — always, even
// with zero projects, so the gate closes and seeding never repeats. Migrate runs
// first so a wip entry's gh= slug survives when the same path is also discovered.
func SeedConfig(configPath, wipPath, claudeProjectsDir string, git vcs.GitRunner) (n int, ran bool, err error) {
	if _, statErr := os.Stat(configPath); statErr == nil {
		return 0, false, nil // config present -> gate closed
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return 0, false, statErr
	}

	cfg := &config.Config{}

	migrated, err := config.MigrateWipProjects(wipPath)
	if err != nil {
		return 0, false, err
	}
	for _, p := range migrated {
		_ = cfg.Add(p) // dedupe handled by Add
	}

	roots, err := config.DiscoverClaudeProjects(claudeProjectsDir, git)
	if err != nil {
		return 0, false, err
	}
	for _, r := range roots {
		_ = cfg.Add(config.Project{Path: r})
	}

	if err := cfg.SaveTo(configPath); err != nil {
		return 0, false, err
	}
	return len(cfg.Projects), true, nil
}
