package main

import (
	"errors"
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
	default: // routeDrilldownName, routeDrilldownHere
		return notImplemented("", "S5 — dashboard drilldown")(c)
	}
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
