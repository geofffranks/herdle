package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/assets"
	"github.com/geofffranks/herdle/internal/agent"
	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/doctor"
	"github.com/geofffranks/herdle/internal/initcmd"
	"github.com/geofffranks/herdle/internal/render"
	"github.com/geofffranks/herdle/internal/vcs"
)

// doctorCommand builds the `herdle doctor` command.
func doctorCommand() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "diagnose the herdle setup",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "agent", Usage: "agent harness to configure: claude or polytoken (repeatable)"},
		},
		Action: doctorAction,
	}
}

func doctorAction(c *cli.Context) error {
	selected, err := agent.Parse(c.StringSlice("agent"))
	if err != nil {
		return err
	}
	env, err := buildDoctorEnv(selected)
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
func buildDoctorEnv(selected []agent.Name) (doctor.Env, error) {
	claudeDir, err := config.ClaudeDir()
	if err != nil {
		return doctor.Env{}, err
	}
	polytokenDir, err := config.PolytokenDir()
	if err != nil {
		return doctor.Env{}, err
	}
	configPath, err := config.Path()
	if err != nil {
		return doctor.Env{}, err
	}
	settingsPath, err := config.SettingsPath()
	if err != nil {
		return doctor.Env{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return doctor.Env{}, err
	}
	herdleOnPath, _ := exec.LookPath("herdle") // "" when not found — not an error here
	env := doctor.Env{
		Git:                vcs.NewGitRunner(),
		GH:                 vcs.NewGHRunner(),
		GL:                 vcs.NewGLRunner(),
		TK:                 vcs.NewTKRunner(),
		Agents:             selected,
		ClaudeAssets:       assets.ClaudeFS,
		PolytokenAssets:    assets.PolytokenFS,
		ClaudeDir:          claudeDir,
		PolytokenDir:       polytokenDir,
		PolytokenHooksPath: filepath.Join(polytokenDir, "hooks.json"),
		PolytokenCommand:   initcmd.PolytokenGatekeeperCommand(),
		ConfigPath:         configPath,
		SettingsPath:       settingsPath,
		ExecPath:           exe,
		HerdleOnPath:       herdleOnPath,
		PathDirs:           filepath.SplitList(os.Getenv("PATH")),
	}
	// CWD is only needed for Polytoken's current-project drift check; resolve it
	// best-effort so an unresolvable cwd never aborts all doctor checks.
	for _, s := range selected {
		if s == agent.Polytoken {
			if raw, wdErr := os.Getwd(); wdErr == nil {
				if canonical, cerr := config.CanonicalProjectPath(raw); cerr == nil {
					env.CWD = canonical
				} else {
					env.CWD = raw // doctor reports the identity issue as a diagnostic
				}
			}
			break
		}
	}
	return env, nil
}
