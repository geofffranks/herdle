package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/geofffranks/herdle/internal/config"
	"github.com/geofffranks/herdle/internal/initcmd"
)

const (
	polytokenInit      = "herdle init --agent polytoken"         // #nosec G101 -- command string, not a credential
	polytokenForceInit = "herdle init --agent polytoken --force" // #nosec G101 -- command string, not a credential
)

type polytokenCandidate struct {
	scope            string
	root             string
	identity         string
	identityErr      error
	layout           initcmd.PolytokenLayout
	initCommand      string
	forceInitCommand string
	uninstallCommand string
}

func checkPolytoken(env Env) []Result {
	candidates, loadResult := polytokenCandidates(env)
	results := make([]Result, 0, len(candidates)*3+2)
	if loadResult != nil {
		results = append(results, *loadResult)
	}
	for _, candidate := range candidates {
		if candidate.scope == "global" && !recognizablePolytoken(candidate.layout) {
			results = append(results, absentGlobalPolytokenResults()...)
			continue
		}
		if candidate.identityErr != nil {
			results = append(results, checkPolytokenIdentity(candidate))
		}
		results = append(results, checkPolytokenCandidate(env, candidate)...)
	}
	if drift := checkUnregisteredCurrentProject(env, candidates); drift != nil {
		results = append(results, *drift)
	}
	results = append(results, checkPolytokenScopes(candidates))
	return results
}

func polytokenCandidates(env Env) ([]polytokenCandidate, *Result) {
	global := polytokenCandidate{
		scope:    "global",
		root:     env.PolytokenDir,
		identity: env.PolytokenDir,
		layout: initcmd.PolytokenLayout{
			StandaloneDir:  env.PolytokenDir,
			HooksPath:      env.PolytokenHooksPath,
			ContextPath:    filepath.Join(env.PolytokenDir, "AGENTS.md"),
			ContextInclude: "@herdle.md",
		},
		initCommand:      polytokenInit,
		forceInitCommand: polytokenForceInit,
		uninstallCommand: polytokenInit + " --scope global --uninstall",
	}
	candidates := []polytokenCandidate{global}
	cfg, err := config.LoadFrom(env.ConfigPath)
	if err != nil {
		result := Result{Name: "polytoken: candidates", Status: Fail, Detail: "cannot load " + env.ConfigPath + ": " + err.Error(), Remediation: "fix " + env.ConfigPath}
		return candidates, &result
	}
	for _, project := range cfg.Projects {
		if !project.Polytoken {
			continue
		}
		root, identityErr := diagnosticPathIdentity(project.Path)
		candidates = append(candidates, projectPolytokenCandidate(root, identityErr))
	}
	return candidates, nil
}

func absentGlobalPolytokenResults() []Result {
	return []Result{
		{Name: "polytoken: skills + context", Status: OK, Detail: "global installation not present"},
		{Name: "polytoken: AGENTS.md link", Status: OK, Detail: "global installation not present"},
		{Name: "polytoken: lifecycle gatekeeper", Status: OK, Detail: "global installation not present"},
	}
}

func projectPolytokenCandidate(root string, identityErr error) polytokenCandidate {
	base := "cd " + shellQuote(root) + " && herdle init --agent polytoken --scope project"
	return polytokenCandidate{
		scope:       "project",
		root:        root,
		identity:    root,
		identityErr: identityErr,
		layout: initcmd.PolytokenLayout{
			StandaloneDir:  filepath.Join(root, ".polytoken"),
			HooksPath:      filepath.Join(root, ".polytoken", "hooks.json"),
			ContextPath:    filepath.Join(root, "AGENTS.md"),
			ContextInclude: "@.polytoken/herdle.md",
		},
		initCommand:      base,
		forceInitCommand: base + " --force",
		uninstallCommand: base + " --uninstall",
	}
}

func checkPolytokenCandidate(env Env, candidate polytokenCandidate) []Result {
	prefix := "polytoken: "
	if candidate.scope == "project" {
		prefix += "project " + candidate.root + ": "
	}
	return []Result{
		checkIntegrity(env.PolytokenAssets, candidate.layout.StandaloneDir, prefix+"skills + context", candidate.initCommand, candidate.forceInitCommand),
		checkPolytokenContext(candidate, prefix+"AGENTS.md link"),
		checkPolytokenGate(env, candidate, prefix+"lifecycle gatekeeper"),
	}
}

func recognizablePolytoken(layout initcmd.PolytokenLayout) bool {
	if _, err := os.Stat(filepath.Join(layout.StandaloneDir, "herdle.md")); err == nil {
		return true
	}
	if initcmd.HasAgentContextSignature(layout.ContextPath) {
		return true
	}
	return initcmd.HasPolytokenHookSignature(layout.HooksPath)
}

func checkUnregisteredCurrentProject(env Env, candidates []polytokenCandidate) *Result {
	if env.CWD == "" {
		return nil
	}
	identity, err := diagnosticPathIdentity(env.CWD)
	if err != nil {
		return &Result{
			Name:        "polytoken: current project path identity",
			Status:      Fail,
			Detail:      "cannot fully resolve current directory; using " + identity + ": " + err.Error(),
			Remediation: "change to an accessible project directory and rerun herdle doctor",
		}
	}
	for _, candidate := range candidates {
		if candidate.scope == "project" && candidate.identity == identity {
			return nil
		}
	}
	candidate := projectPolytokenCandidate(identity, nil)
	if !recognizablePolytoken(candidate.layout) {
		return nil
	}
	return &Result{
		Name:        "polytoken: unregistered project " + identity,
		Status:      Fail,
		Detail:      "recognizable unregistered project installation at " + identity,
		Remediation: "register or refresh: " + candidate.initCommand + "\nor remove: " + candidate.uninstallCommand,
	}
}

func checkPolytokenIdentity(candidate polytokenCandidate) Result {
	return Result{
		Name:        "polytoken: project " + candidate.root + ": path identity",
		Status:      Fail,
		Detail:      "cannot fully resolve project path; using " + candidate.identity + ": " + candidate.identityErr.Error(),
		Remediation: candidate.initCommand,
	}
}

func checkPolytokenScopes(candidates []polytokenCandidate) Result {
	globalPresent := len(candidates) > 0 && candidates[0].scope == "global" && recognizablePolytoken(candidates[0].layout)
	conflicting := make(map[int]bool)
	if globalPresent {
		for i := 1; i < len(candidates); i++ {
			conflicting[0] = true
			conflicting[i] = true
		}
	}
	for i := 1; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if pathsOverlap(candidates[i].identity, candidates[j].identity) {
				conflicting[i] = true
				conflicting[j] = true
			}
		}
	}
	if len(conflicting) == 0 {
		return Result{Name: "polytoken: installation scopes", Status: OK, Detail: "installation scopes are disjoint"}
	}
	var paths, commands []string
	for i, candidate := range candidates {
		if !conflicting[i] {
			continue
		}
		paths = append(paths, candidate.identity)
		commands = append(commands, candidate.uninstallCommand)
	}
	return Result{
		Name:        "polytoken: installation scopes",
		Status:      Fail,
		Detail:      "conflicting installation scopes: " + strings.Join(paths, ", "),
		Remediation: strings.Join(commands, "\n"),
	}
}

func pathsOverlap(first, second string) bool {
	first = filepath.Clean(first)
	second = filepath.Clean(second)
	if first == second {
		return true
	}
	rel, err := filepath.Rel(first, second)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	rel, err = filepath.Rel(second, first)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func diagnosticPathIdentity(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path), err
	}
	absolute = filepath.Clean(absolute)
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	originalErr := err
	ancestor := absolute
	var suffix []string
	for {
		resolved, resolveErr := filepath.EvalSymlinks(ancestor)
		if resolveErr == nil {
			parts := append([]string{resolved}, suffix...)
			return filepath.Clean(filepath.Join(parts...)), originalErr
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return absolute, originalErr
		}
		suffix = append([]string{filepath.Base(ancestor)}, suffix...)
		ancestor = parent
	}
}

func checkPolytokenContext(candidate polytokenCandidate, name string) Result {
	path := candidate.layout.ContextPath
	inspection, err := initcmd.InspectAgentContextInclude(path, candidate.layout.ContextInclude)
	if err != nil {
		return Result{Name: name, Status: Fail, Detail: "cannot inspect " + path + ": " + err.Error(), Remediation: "repair " + path + ", then run: " + candidate.initCommand}
	}
	if inspection.Count == 0 {
		return Result{Name: name, Status: Fail, Detail: "managed context block not found in " + path, Remediation: candidate.initCommand}
	}
	if inspection.Count != 1 || !inspection.Exact {
		return Result{Name: name, Status: Fail, Detail: "managed context block is malformed or duplicated in " + path, Remediation: "repair " + path + ", then run: " + candidate.initCommand}
	}
	return Result{Name: name, Status: OK, Detail: "managed context block present and current in " + path}
}

func checkPolytokenGate(env Env, candidate polytokenCandidate, name string) Result {
	path := candidate.layout.HooksPath
	inspection, err := initcmd.InspectPolytokenHooks(path)
	if err != nil {
		return Result{Name: name, Status: Fail, Detail: "cannot inspect " + path + ": " + err.Error(), Remediation: "repair " + path + ", then run: " + candidate.initCommand}
	}
	if inspection.Count == 0 {
		return Result{Name: name, Status: Fail, Detail: "managed hook not found in " + path, Remediation: candidate.initCommand}
	}
	if inspection.Count != 1 {
		return Result{Name: name, Status: Fail, Detail: "managed hook is duplicated in " + path, Remediation: "repair " + path + ", then run: " + candidate.initCommand}
	}
	if inspection.Event != "pre_tool_use" || inspection.Matcher != "*" || inspection.Command != env.PolytokenCommand {
		return Result{Name: name, Status: Fail, Detail: "managed hook is stale in " + path, Remediation: candidate.initCommand}
	}
	return Result{Name: name, Status: OK, Detail: "managed hook present and current in " + path}
}

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
}
