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
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "all", Aliases: []string{"a"}, Usage: "force the cross-project summary even inside a repo"},
			&cli.BoolFlag{Name: "fetch", Aliases: []string{"f"}, Usage: "git fetch each repo first (network; default is offline)"},
		},
		Action: rootAction,
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
			initCommand(),
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
