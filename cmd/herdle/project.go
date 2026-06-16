package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/vcs"
)

// projectCommand builds the `herdle project` command and its subcommands.
func projectCommand() *cli.Command {
	flags := []cli.Flag{
		&cli.StringFlag{Name: "gh", Usage: "legacy GitHub owner/repo override (use --slug)"},
		&cli.StringFlag{Name: "slug", Usage: "forge-agnostic [group/]owner/repo override (GitHub or GitLab, by remote host)"},
		&cli.StringFlag{Name: "remote", Usage: "git remote name (autodetect if unset)"},
		&cli.StringFlag{Name: "base", Usage: "trunk branch (autodetect if unset)"},
		&cli.StringFlag{Name: "integration", Usage: "personal integration branch"},
	}
	return &cli.Command{
		Name:  "project",
		Usage: "manage configured projects",
		Subcommands: []*cli.Command{
			{
				Name:            "add",
				Usage:           "add a project",
				ArgsUsage:       "<path>",
				Flags:           flags,
				SkipFlagParsing: true,
				Action:          projectAdd,
			},
			{
				Name:            "set",
				Usage:           "update a project",
				ArgsUsage:       "<name|path>",
				Flags:           flags,
				SkipFlagParsing: true,
				Action:          projectSet,
			},
			{Name: "rm", Usage: "remove a project", ArgsUsage: "<name|path>", Action: projectRm},
			{Name: "list", Usage: "list configured projects", Action: projectList},
		},
	}
}

// parseManualArgs scans args (from c.Args().Slice() when SkipFlagParsing is set)
// and returns: all non-flag positional arguments, a map of flag values, and
// a set of which flags were explicitly provided (for IsSet semantics).
// Supports --flag value and --flag=value forms.
// In the --flag value form, if the next token starts with "--" it is NOT consumed
// as the value; instead the current flag is recorded as empty and the next token
// is processed as its own flag.
func parseManualArgs(args []string) (positionals []string, flags map[string]string, set map[string]bool) {
	flags = make(map[string]string)
	set = make(map[string]bool)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" { // POSIX end-of-options: rest are positionals
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			name := strings.TrimPrefix(arg, "--")
			if idx := strings.IndexByte(name, '='); idx >= 0 {
				// --flag=value form
				flags[name[:idx]] = name[idx+1:]
				set[name[:idx]] = true
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				// --flag value form (next token is not itself a flag)
				flags[name] = args[i+1]
				set[name] = true
				i++
			} else {
				// boolean flag with no value — treat as empty string
				flags[name] = ""
				set[name] = true
			}
		} else {
			positionals = append(positionals, arg)
		}
	}
	return positionals, flags, set
}

// firstNonEmpty returns the first non-empty argument, or "" if all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// normalizePath expands a leading ~, makes the path absolute, and cleans it.
func normalizePath(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// rejectUnknownFlags returns an error if any key in set is not a declared flag
// on c's current subcommand. The flags slice on the subcommand is the single
// source of truth, so adding a flag there is sufficient.
func rejectUnknownFlags(c *cli.Context, set map[string]bool) error {
	known := make(map[string]bool)
	for _, f := range c.Command.Flags {
		for _, n := range f.Names() {
			known[n] = true
		}
	}
	cmd := c.Command.FullName()
	for name := range set {
		if !known[name] {
			return fmt.Errorf("%s: unknown flag --%s", cmd, name)
		}
	}
	return nil
}

// wantsHelp reports whether any arg is -h or --help.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func projectAdd(c *cli.Context) error {
	if wantsHelp(c.Args().Slice()) {
		return cli.ShowSubcommandHelp(c)
	}
	positionals, flagVals, flagSet := parseManualArgs(c.Args().Slice())
	if err := rejectUnknownFlags(c, flagSet); err != nil {
		return err
	}
	if len(positionals) != 1 {
		return fmt.Errorf("project add: exactly one <path> argument is required")
	}
	path, err := normalizePath(positionals[0])
	if err != nil {
		return err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("project add: %s: %w", path, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("project add: %s is not a directory", path)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := cfg.Add(config.Project{
		Path:        path,
		GH:          flagVals["gh"],
		Slug:        flagVals["slug"],
		Remote:      flagVals["remote"],
		Base:        flagVals["base"],
		Integration: flagVals["integration"],
	}); err != nil {
		return fmt.Errorf("project add: %w", err)
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	if _, err := vcs.NewGitRunner().RepoRoot(path); err != nil {
		fmt.Fprintf(c.App.ErrWriter, "warning: %s is not a git repository yet\n", path)
	}
	fmt.Fprintf(c.App.Writer, "added %s\n", path)
	return nil
}

func projectSet(c *cli.Context) error {
	if wantsHelp(c.Args().Slice()) {
		return cli.ShowSubcommandHelp(c)
	}
	positionals, flagVals, flagSet := parseManualArgs(c.Args().Slice())
	if err := rejectUnknownFlags(c, flagSet); err != nil {
		return err
	}
	if len(positionals) != 1 {
		return fmt.Errorf("project set: exactly one <name|path> argument is required")
	}
	if len(flagSet) == 0 {
		return fmt.Errorf("project set: no fields to update (pass at least one of --gh/--slug/--remote/--base/--integration)")
	}
	key, err := normalizePath(positionals[0])
	if err != nil {
		// normalizePath can fail on bad input; fall back to the raw name so
		// bare names like "herdle" (no path separators) still work.
		key = positionals[0]
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	idx, err := cfg.Find(key)
	if err != nil {
		return fmt.Errorf("project set: %w", err)
	}
	// Only flags the user actually passed are touched; an empty value clears.
	if flagSet["gh"] {
		cfg.Projects[idx].GH = flagVals["gh"]
	}
	if flagSet["slug"] {
		cfg.Projects[idx].Slug = flagVals["slug"]
	}
	if flagSet["remote"] {
		cfg.Projects[idx].Remote = flagVals["remote"]
	}
	if flagSet["base"] {
		cfg.Projects[idx].Base = flagVals["base"]
	}
	if flagSet["integration"] {
		cfg.Projects[idx].Integration = flagVals["integration"]
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "updated %s\n", cfg.Projects[idx].Path)
	return nil
}

func projectRm(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("project rm: exactly one <name|path> argument is required")
	}
	key := c.Args().First()
	if norm, err := normalizePath(key); err == nil {
		key = norm
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	idx, err := cfg.Find(key)
	if err != nil {
		return fmt.Errorf("project rm: %w", err)
	}
	removed := cfg.Projects[idx].Path
	cfg.Remove(idx)
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "removed %s\n", removed)
	return nil
}

func projectList(c *cli.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Projects) == 0 {
		fmt.Fprintln(c.App.Writer, "no projects configured")
		return nil
	}
	git := vcs.NewGitRunner()
	// mark renders a resolved value, flagging autodetected/defaulted values (raw
	// field empty) with a trailing "*"; an empty resolved value renders "-".
	mark := func(raw, resolved string) string {
		if resolved == "" {
			return "-"
		}
		if raw == "" {
			return resolved + "*"
		}
		return resolved
	}

	tw := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPATH\tREMOTE\tBASE\tINTEGRATION\tSLUG")
	for _, p := range cfg.Projects {
		r, _ := cfg.Resolve(p, git)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Name, r.Path,
			mark(p.Remote, r.Remote),
			mark(p.Base, r.Base),
			mark(p.Integration, r.Integration),
			mark(firstNonEmpty(p.GH, p.Slug), r.Slug),
		)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(c.App.Writer, "\n* = autodetected or default")
	return nil
}
