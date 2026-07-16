package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/assets"
	"github.com/geofffranks/herdle/internal/agent"
	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/initcmd"
	"github.com/geofffranks/herdle/internal/vcs"
)

// initCommand builds the `herdle init` command.
func initCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "write embedded skills and rules, and seed config",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "agent", Usage: "agent harness to configure: claude or polytoken (repeatable)"},
			&cli.BoolFlag{Name: "force", Usage: "overwrite existing skills/rules (use after an upgrade)"},
			&cli.BoolFlag{Name: "uninstall", Usage: "remove the skills/rules herdle installed"},
		},
		Action: initAction,
	}
}

func initAction(c *cli.Context) error {
	selected, err := agent.Parse(c.StringSlice("agent"))
	if err != nil {
		return err
	}
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
