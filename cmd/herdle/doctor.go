package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/assets"
	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/doctor"
	"github.com/geofffranks/herdle/internal/render"
	"github.com/geofffranks/herdle/internal/vcs"
)

// doctorCommand builds the `herdle doctor` command.
func doctorCommand() *cli.Command {
	return &cli.Command{
		Name:   "doctor",
		Usage:  "diagnose the herdle setup",
		Action: doctorAction,
	}
}

func doctorAction(c *cli.Context) error {
	env, err := buildDoctorEnv()
	if err != nil {
		return err
	}
	w := c.App.Writer
	fmt.Fprintf(w, "herdle %s\n\n", Version)
	results := doctor.Run(env)
	doctor.Render(w, results, render.DetectColor(w))
	if n := doctor.FailCount(results); n > 0 {
		return fmt.Errorf("herdle doctor: %d check(s) need attention", n)
	}
	return nil
}

// buildDoctorEnv assembles the doctor.Env from the real environment.
func buildDoctorEnv() (doctor.Env, error) {
	claudeDir, err := config.ClaudeDir()
	if err != nil {
		return doctor.Env{}, err
	}
	configPath, err := config.Path()
	if err != nil {
		return doctor.Env{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return doctor.Env{}, err
	}
	herdleOnPath, _ := exec.LookPath("herdle") // "" when not found — not an error here
	return doctor.Env{
		Git:          vcs.NewGitRunner(),
		GH:           vcs.NewGHRunner(),
		TK:           vcs.NewTKRunner(),
		Assets:       assets.FS,
		ClaudeDir:    claudeDir,
		ConfigPath:   configPath,
		ExecPath:     exe,
		HerdleOnPath: herdleOnPath,
		PathDirs:     filepath.SplitList(os.Getenv("PATH")),
	}, nil
}
