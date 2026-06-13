package vcs

import (
	"bytes"
	"errors"
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
