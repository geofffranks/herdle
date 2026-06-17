package vcs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// result captures one command invocation.
type result struct {
	stdout string
	stderr string
	code   int
}

func (r result) trimmed() string { return strings.TrimSpace(r.stdout) }

// resolveBinary returns the executable to run for a tool. If envVar is set, its
// value is used directly (an absolute path supports custom installs); otherwise
// defaultName is looked up on PATH.
func resolveBinary(envVar, defaultName string) (string, error) {
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	return exec.LookPath(defaultName)
}

// binaryAvailable reports whether a tool can be located. With envVar set it must
// point at an existing non-directory file (a custom-install override); otherwise
// defaultName must be on PATH. Shared by Git/TK/GH Available().
func binaryAvailable(envVar, defaultName string) bool {
	if v := os.Getenv(envVar); v != "" {
		info, err := os.Stat(v) // #nosec G304,G703 -- deliberate user-supplied override (see resolveBinary)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(defaultName)
	return err == nil
}

// retryJSONFetch runs a forge CLI invocation (fn) up to twice — the gh/glab
// binaries are occasionally flaky — and decodes the first successful run's stdout
// as a JSON array of T. label is the command prefix used in error messages (e.g.
// "gh pr list -R owner/repo"). It only trusts output that begins with "[": a
// transient failure must never be mistaken for an empty result. On total failure
// the last observed error is returned. Shared by GHRunner.PRList and
// GLRunner.PRList (which decodes into its raw glMR shape, then maps to PR).
func retryJSONFetch[T any](label string, fn func() (result, error)) ([]T, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		res, err := fn()
		if err != nil {
			lastErr = err
			continue
		}
		if res.code != 0 {
			lastErr = fmt.Errorf("%s: %s", label, strings.TrimSpace(res.stderr))
			continue
		}
		out := strings.TrimSpace(res.stdout)
		if !strings.HasPrefix(out, "[") {
			lastErr = fmt.Errorf("%s: unexpected output %q", label, out)
			continue
		}
		var items []T
		if err := json.Unmarshal([]byte(out), &items); err != nil {
			lastErr = fmt.Errorf("%s: parse json: %w", label, err)
			continue
		}
		return items, nil
	}
	return nil, lastErr
}

// run executes bin with args in working directory dir, capturing stdout, stderr,
// and the exit code. A non-zero exit is NOT an error here: err is non-nil only
// when the process could not be started (or failed for a non-exit reason).
// Callers classify exit codes per their own semantics.
func run(dir, bin string, args ...string) (result, error) {
	cmd := exec.Command(bin, args...) // #nosec G204 -- this package's purpose is to shell out to git/gh/tk with caller-controlled args
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	res := result{stdout: out.String(), stderr: errb.String()}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.code = ee.ExitCode()
			return res, nil // ran, exited non-zero — caller decides meaning
		}
		return res, err // could not start
	}
	return res, nil
}
