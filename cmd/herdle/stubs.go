package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// notImplemented returns an action that reports the command is not yet built,
// pointing at the story that will implement it. It returns a plain error (not
// cli.Exit) so App.Run returns instead of calling os.Exit — keeping it testable
// in-process. main() turns the returned error into a non-zero exit.
func notImplemented(name, story string) cli.ActionFunc {
	return func(*cli.Context) error {
		label := name
		if label != "" {
			label = " " + label
		}
		return fmt.Errorf("herdle%s: not implemented yet (%s)", label, story)
	}
}
