package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// Version is stamped at build time via -ldflags "-X main.Version=<v>".
var Version = "dev"

func newApp() *cli.App {
	return &cli.App{
		Name:    "herdle",
		Usage:   "Wrangle the herd, spot the hurdles.",
		Version: Version,
		Action:  notImplemented("", "S4/S5 — dashboard"),
		Commands: []*cli.Command{
			{
				Name:  "version",
				Usage: "print the herdle version",
				Action: func(c *cli.Context) error {
					_, _ = fmt.Fprintf(c.App.Writer, "herdle %s\n", Version)
					return nil
				},
			},
			{
				Name:  "project",
				Usage: "manage configured projects",
				Subcommands: []*cli.Command{
					{Name: "add", Usage: "add a project", Action: notImplemented("project add", "S3")},
					{Name: "set", Usage: "update a project", Action: notImplemented("project set", "S3")},
					{Name: "rm", Usage: "remove a project", Action: notImplemented("project rm", "S3")},
					{Name: "list", Usage: "list configured projects", Action: notImplemented("project list", "S3")},
				},
			},
			{
				Name:   "init",
				Usage:  "write embedded skills, rules, and config",
				Action: notImplemented("init", "S7"),
			},
			{
				Name:   "doctor",
				Usage:  "diagnose the herdle setup",
				Action: notImplemented("doctor", "S8"),
			},
		},
	}
}

func main() {
	if err := newApp().Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
