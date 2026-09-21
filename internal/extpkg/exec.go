package extpkg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// Runner runs one manager command. It is an interface so tests can drive the
// service without any manager installed.
type Runner interface {
	Run(ctx context.Context, bin string, args []string, timeout time.Duration) (CommandResult, error)
}

// ExecRunner runs manager commands as child processes.
//
// It deliberately inherits the parent environment and does not inject eget's
// own proxy settings: each manager has its own registry/proxy configuration.
type ExecRunner struct{}

func NewExecRunner() Runner { return ExecRunner{} }

func (ExecRunner) Run(ctx context.Context, bin string, args []string, timeout time.Duration) (CommandResult, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, bin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := CommandResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if err == nil {
		return result, nil
	}

	// A non-zero exit is a normal outcome, not a run failure: npm exits 1 when
	// it finds outdated packages. The parser decides what it means.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return result, fmt.Errorf("%s: command timed out after %s", bin, timeout)
	}
	return result, fmt.Errorf("%s: %w", bin, err)
}
