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
			projectCommand(),
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
