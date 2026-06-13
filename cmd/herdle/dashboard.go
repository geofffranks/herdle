package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/dashboard"
	"github.com/geofffranks/herdle/internal/render"
	"github.com/geofffranks/herdle/internal/vcs"
)

// routeKind is the view the root command selects.
type routeKind int

const (
	routeSummary routeKind = iota
	routeDrilldownName
	routeDrilldownHere
)

// route mirrors wip's arg dispatch: a named project always drills down; otherwise
// --all or being outside a repo shows the summary; otherwise drill down the
// current repo. Pure so it can be table-tested.
func route(all bool, name string, inRepo bool) routeKind {
	switch {
	case name != "":
		return routeDrilldownName
	case all || !inRepo:
		return routeSummary
	default:
		return routeDrilldownHere
	}
}

// rootAction is the `herdle` (no subcommand) entry point.
func rootAction(c *cli.Context) error {
	git := vcs.NewGitRunner()

	name := c.Args().First()
	inRepo := false
	if name == "" && !c.Bool("all") {
		// Only the no-name, no--all case needs repo detection to decide.
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if _, err := git.RepoRoot(cwd); err == nil {
			inRepo = true
		} else if !errors.Is(err, vcs.ErrNotARepo) {
			return err
		}
	}

	switch route(c.Bool("all"), name, inRepo) {
	case routeSummary:
		return runSummary(c, git)
	case routeDrilldownName:
		return runDrilldownName(c, git, name)
	default: // routeDrilldownHere
		return runDrilldownHere(c, git)
	}
}

// runDrilldownHere drills down the current repo, using its configured project if
// present, else a synthesized one (wip drills the current repo even when it is
// not in the config).
func runDrilldownHere(c *cli.Context, git vcs.GitRunner) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := git.RepoRoot(cwd)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	proj := config.Project{Path: root}
	if idx, err := cfg.Find(root); err == nil {
		proj = cfg.Projects[idx]
	}
	r, _ := cfg.Resolve(proj, git)
	return runDrilldown(c, git, r)
}

// runDrilldownName drills down a configured project selected by name (or path).
func runDrilldownName(c *cli.Context, git vcs.GitRunner, name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	idx, err := cfg.Find(name)
	if err != nil {
		// Surface an ambiguous-basename error with its disambiguation detail;
		// a plain not-found stays a clear "no project named" message.
		var amb *config.AmbiguousError
		if errors.As(err, &amb) {
			return fmt.Errorf("herdle: %w", amb)
		}
		return fmt.Errorf("herdle: no project named %q", name)
	}
	r, _ := cfg.Resolve(cfg.Projects[idx], git)
	return runDrilldown(c, git, r)
}

func runDrilldown(c *cli.Context, git vcs.GitRunner, r config.Resolved) error {
	eng := dashboard.Engine{Git: git, GH: vcs.NewGHRunner(), TK: vcs.NewTKRunner()}
	d, err := eng.Drilldown(r, c.Bool("fetch"))
	if err != nil {
		return err
	}
	w := c.App.Writer
	return render.Drilldown(w, d, render.DetectColor(w))
}

// runSummary loads config, gathers rows, and renders the cross-project summary.
func runSummary(c *cli.Context, git vcs.GitRunner) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	eng := dashboard.Engine{Git: git, GH: vcs.NewGHRunner(), TK: vcs.NewTKRunner()}
	rows, err := eng.Summary(cfg, c.Bool("fetch"))
	if err != nil {
		return err
	}
	return render.Summary(c.App.Writer, rows, c.Bool("fetch"))
}
