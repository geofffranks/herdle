package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/assets"
	"github.com/geofffranks/herdle/internal/agent"
	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/initcmd"
	"github.com/geofffranks/herdle/internal/vcs"
)

type initDependencies struct {
	getwd                func() (string, error)
	canonicalProjectPath func(string) (string, error)
	loadConfig           func() (*config.Config, error)
	saveConfig           func(*config.Config) error
}

type projectFileSnapshot struct {
	path   string
	data   []byte
	mode   os.FileMode
	exists bool
}

func snapshotProjectFiles(paths ...string) ([]projectFileSnapshot, error) {
	snapshots := make([]projectFileSnapshot, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				snapshots = append(snapshots, projectFileSnapshot{path: path})
				continue
			}
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: expected a regular file", path)
		}
		data, err := os.ReadFile(path) // #nosec G304 -- paths are derived from the canonical project layout
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, projectFileSnapshot{path: path, data: data, mode: info.Mode().Perm(), exists: true})
	}
	return snapshots, nil
}

func restoreProjectFiles(snapshots []projectFileSnapshot) error {
	for _, snapshot := range snapshots {
		if !snapshot.exists {
			if err := os.Remove(snapshot.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		if err := os.WriteFile(snapshot.path, snapshot.data, snapshot.mode); err != nil {
			return err
		}
		if err := os.Chmod(snapshot.path, snapshot.mode); err != nil {
			return err
		}
	}
	return nil
}

func defaultInitDependencies() initDependencies {
	return initDependencies{
		getwd:                os.Getwd,
		canonicalProjectPath: config.CanonicalProjectPath,
		loadConfig:           config.Load,
		saveConfig:           func(cfg *config.Config) error { return cfg.Save() },
	}
}

// initCommand builds the `herdle init` command.
func initCommand() *cli.Command {
	return initCommandWithDependencies(defaultInitDependencies())
}

func initCommandWithDependencies(deps initDependencies) *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "write embedded skills and rules, and seed config",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "agent", Usage: "agent harness to configure: claude or polytoken (repeatable)"},
			&cli.StringFlag{Name: "scope", Value: "global", Usage: "installation scope: global or project"},
			&cli.BoolFlag{Name: "force", Usage: "overwrite existing skills/rules (use after an upgrade)"},
			&cli.BoolFlag{Name: "uninstall", Usage: "remove the skills/rules herdle installed"},
		},
		Action: func(c *cli.Context) error { return initAction(c, deps) },
	}
}

func initAction(c *cli.Context, deps initDependencies) error {
	selected, err := agent.Parse(c.StringSlice("agent"))
	if err != nil {
		return err
	}

	scope := c.String("scope")
	if scope != "global" && scope != "project" {
		return fmt.Errorf("unknown scope %q (expected global or project)", scope)
	}
	if scope == "project" {
		explicitAgents := c.StringSlice("agent")
		if len(explicitAgents) == 0 || len(selected) != 1 || selected[0] != agent.Polytoken {
			return errors.New("project scope requires explicit exclusive --agent polytoken")
		}
		cwd, err := deps.getwd()
		if err != nil {
			return err
		}
		project, err := deps.canonicalProjectPath(cwd)
		if err != nil {
			return err
		}
		return initProjectAction(c, project, deps)
	}

	return initGlobalAction(c, selected)
}

func initProjectAction(c *cli.Context, project string, deps initDependencies) error {
	cfg, err := deps.loadConfig()
	if err != nil {
		return err
	}
	standalone := filepath.Join(project, ".polytoken")
	layout := initcmd.PolytokenLayout{
		StandaloneDir:  standalone,
		HooksPath:      filepath.Join(standalone, "hooks.json"),
		ContextPath:    filepath.Join(project, "AGENTS.md"),
		ContextInclude: "@.polytoken/herdle.md",
	}
	var snapshots []projectFileSnapshot
	if !c.Bool("uninstall") {
		snapshots, err = snapshotProjectFiles(
			filepath.Join(standalone, "herdle.md"),
			filepath.Join(standalone, "skills", "herdle-tk-flow", "SKILL.md"),
			filepath.Join(standalone, "skills", "herdle-tk-artifacts", "SKILL.md"),
			layout.HooksPath,
			layout.ContextPath,
		)
		if err != nil {
			return err
		}
	}
	var results []initcmd.Result
	if c.Bool("uninstall") {
		results, err = initcmd.UninstallPolytokenLayout(assets.PolytokenFS, layout)
	} else {
		results, err = initcmd.InstallPolytokenLayout(assets.PolytokenFS, layout, initcmd.PolytokenGatekeeperCommand(), c.Bool("force"))
	}
	for _, result := range results {
		fmt.Fprintf(c.App.Writer, "%s: %s %s\n", agent.Polytoken, result.Action, result.Path)
	}
	if err != nil {
		return err
	}
	if err := saveProjectConfig(cfg, project, c.Bool("uninstall"), deps); err != nil {
		if rollbackErr := restoreProjectFiles(snapshots); rollbackErr != nil {
			return fmt.Errorf("%w (project artifact rollback failed: %v)", err, rollbackErr)
		}
		return err
	}
	return nil
}

func updateProjectConfig(path string, uninstall bool, deps initDependencies) error {
	cfg, err := deps.loadConfig()
	if err != nil {
		return err
	}
	return saveProjectConfig(cfg, path, uninstall, deps)
}

func saveProjectConfig(cfg *config.Config, path string, uninstall bool, deps initDependencies) error {
	var changed bool
	if uninstall {
		changed = cfg.ClearProjectPolytoken(path)
	} else {
		changed = cfg.UpsertProjectPolytoken(path)
	}
	if !changed {
		return nil
	}
	return deps.saveConfig(cfg)
}

func initGlobalAction(c *cli.Context, selected []agent.Name) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	w := c.App.Writer
	uninstall := c.Bool("uninstall") // --uninstall takes precedence over --force

	for _, name := range selected {
		var results []initcmd.Result
		switch name {
		case agent.Claude:
			claudeDir, pathErr := config.ClaudeDir()
			if pathErr != nil {
				return pathErr
			}
			settingsPath, pathErr := config.SettingsPath()
			if pathErr != nil {
				return pathErr
			}
			if uninstall {
				results, err = initcmd.Uninstall(assets.ClaudeFS, claudeDir)
			} else {
				results, err = initcmd.Install(assets.ClaudeFS, claudeDir, c.Bool("force"))
			}
			if err == nil {
				var settingsResult initcmd.Result
				if uninstall {
					settingsResult, err = initcmd.UnmergeSettings(settingsPath)
				} else {
					settingsResult, err = initcmd.MergeSettings(settingsPath, exe+" hook gatekeeper")
				}
				if err == nil {
					results = append(results, settingsResult)
				}
			}
		case agent.Polytoken:
			polytokenDir, pathErr := config.PolytokenDir()
			if pathErr != nil {
				return pathErr
			}
			if uninstall {
				results, err = initcmd.UninstallPolytoken(assets.PolytokenFS, polytokenDir)
			} else {
				results, err = initcmd.InstallPolytoken(assets.PolytokenFS, polytokenDir, initcmd.PolytokenGatekeeperCommand(), c.Bool("force"))
			}
		}
		for _, result := range results {
			fmt.Fprintf(w, "%s: %s %s\n", name, result.Action, result.Path)
		}
		if err != nil {
			return err
		}
	}

	if uninstall {
		fmt.Fprintln(w, "uninstalled managed files; config and user-owned context left untouched")
		return nil
	}

	configPath, err := config.Path()
	if err != nil {
		return err
	}
	wipPath, err := config.WipProjectsPath()
	if err != nil {
		return err
	}
	cpd, err := config.ClaudeProjectsDir()
	if err != nil {
		return err
	}
	n, ran, err := initcmd.SeedConfig(configPath, wipPath, cpd, vcs.NewGitRunner())
	if err != nil {
		return err
	}
	if ran {
		fmt.Fprintf(w, "seeded %d project(s) into %s\n", n, configPath)
	} else {
		fmt.Fprintf(w, "config present at %s; skipped seeding\n", configPath)
	}

	fmt.Fprintln(w, "done — run `herdle doctor` to verify your setup")
	return nil
}
