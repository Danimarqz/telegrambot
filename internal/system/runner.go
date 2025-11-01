package system

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner executes system commands with context support and output capture.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, stderr string, err error)
}

// CommandRunner is the default Runner implementation backed by os/exec.
type CommandRunner struct {
}

// NewCommandRunner allocates a ready-to-use CommandRunner.
func NewCommandRunner() *CommandRunner {
	return &CommandRunner{}
}

// Run executes the command, returning stdout, stderr and a wrapped error when it fails.
func (r *CommandRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := stdout.String()
	errStr := stderr.String()

	if ctxErr := ctx.Err(); ctxErr != nil {
		return outStr, errStr, fmt.Errorf("command cancelled: %w", ctxErr)
	}
	if err != nil {
		return outStr, errStr, fmt.Errorf("command %q failed: %w", buildCommandString(name, args), err)
	}

	return outStr, errStr, nil
}

func buildCommandString(name string, args []string) string {
	var b strings.Builder
	b.WriteString(name)
	if len(args) == 0 {
		return b.String()
	}

	for _, arg := range args {
		b.WriteByte(' ')
		b.WriteString(arg)
	}
	return b.String()
}

// WithTimeout creates a cancellable context with the provided timeout.
func WithTimeout(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	if duration <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, duration)
}
