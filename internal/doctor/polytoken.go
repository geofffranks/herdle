package doctor

import (
	"path/filepath"

	"github.com/geofffranks/herdle/internal/initcmd"
)

const (
	polytokenInit      = "herdle init --agent polytoken"         // #nosec G101 -- command string, not a credential
	polytokenForceInit = "herdle init --agent polytoken --force" // #nosec G101 -- command string, not a credential
)

func checkPolytoken(env Env) []Result {
	return []Result{
		checkIntegrity(env.PolytokenAssets, env.PolytokenDir, "polytoken: skills + context", polytokenInit, polytokenForceInit),
		checkPolytokenContext(env),
		checkPolytokenGate(env),
	}
}

func checkPolytokenContext(env Env) Result {
	const name = "polytoken: AGENTS.md link"
	path := filepath.Join(env.PolytokenDir, "AGENTS.md")
	inspection, err := initcmd.InspectAgentContext(path)
	if err != nil {
		return Result{Name: name, Status: Fail, Detail: "cannot inspect " + path + ": " + err.Error(), Remediation: "repair " + path + ", then run: " + polytokenInit}
	}
	if inspection.Count == 0 {
		return Result{Name: name, Status: Fail, Detail: "managed context block not found in " + path, Remediation: polytokenInit}
	}
	if inspection.Count != 1 || !inspection.Exact {
		return Result{Name: name, Status: Fail, Detail: "managed context block is malformed or duplicated in " + path, Remediation: "repair " + path + ", then run: " + polytokenInit}
	}
	return Result{Name: name, Status: OK, Detail: "managed context block present and current"}
}

func checkPolytokenGate(env Env) Result {
	const name = "polytoken: lifecycle gatekeeper"
	path := env.PolytokenHooksPath
	inspection, err := initcmd.InspectPolytokenHooks(path)
	if err != nil {
		return Result{Name: name, Status: Fail, Detail: "cannot inspect " + path + ": " + err.Error(), Remediation: "repair " + path + ", then run: " + polytokenInit}
	}
	if inspection.Count == 0 {
		return Result{Name: name, Status: Fail, Detail: "managed hook not found in " + path, Remediation: polytokenInit}
	}
	if inspection.Count != 1 {
		return Result{Name: name, Status: Fail, Detail: "managed hook is duplicated in " + path, Remediation: "repair " + path + ", then run: " + polytokenInit}
	}
	if inspection.Event != "pre_tool_use" || inspection.Matcher != "*" || inspection.Command != env.PolytokenCommand {
		return Result{Name: name, Status: Fail, Detail: "managed hook is stale in " + path, Remediation: polytokenInit}
	}
	return Result{Name: name, Status: OK, Detail: "managed hook present and current"}
}
