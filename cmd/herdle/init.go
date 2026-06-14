package main

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/assets"
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
			&cli.BoolFlag{Name: "force", Usage: "overwrite existing skills/rules (use after an upgrade)"},
			&cli.BoolFlag{Name: "uninstall", Usage: "remove the skills/rules herdle installed"},
		},
		Action: initAction,
	}
}

func initAction(c *cli.Context) error {
	claudeDir, err := config.ClaudeDir()
	if err != nil {
		return err
	}
	w := c.App.Writer

	if c.Bool("uninstall") { // --uninstall takes precedence over --force
		results, err := initcmd.Uninstall(assets.FS, claudeDir)
		if err != nil {
			return err
		}
		for _, r := range results {
			fmt.Fprintf(w, "%s %s\n", r.Action, r.Path)
		}
		fmt.Fprintf(w, "uninstalled %d file(s); config and CLAUDE.md left untouched\n", len(results))
		return nil
	}

	results, err := initcmd.Install(assets.FS, claudeDir, c.Bool("force"))
	if err != nil {
		return err
	}
	for _, r := range results {
		fmt.Fprintf(w, "%s %s\n", r.Action, r.Path)
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
