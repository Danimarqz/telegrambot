package system

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBuildCommandString(t *testing.T) {
	got := buildCommandString("go", []string{"test", "./..."})
	if got != "go test ./..." {
		t.Fatalf("buildCommandString() = %q, want %q", got, "go test ./...")
	}
	if got := buildCommandString("go", nil); got != "go" {
		t.Fatalf("buildCommandString() with nil args = %q, want go", got)
	}
}

func TestWithTimeout(t *testing.T) {
	ctx := context.Background()
	derived, cancel := WithTimeout(ctx, 0)
	defer cancel()
	if _, ok := derived.Deadline(); ok {
		t.Fatalf("deadline should be absent when duration <= 0")
	}

	derived, cancel = WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	if _, ok := derived.Deadline(); !ok {
		t.Fatalf("expected deadline for positive duration")
	}
}

func TestCommandRunnerRunSuccess(t *testing.T) {
	runner := NewCommandRunner()
	stdout, stderr, err := runner.Run(context.Background(), "go", "version")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("Run() stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "go version") {
		t.Fatalf("stdout = %q, want contains go version", stdout)
	}
}

func TestCommandRunnerRunCancelled(t *testing.T) {
	runner := NewCommandRunner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := runner.Run(ctx, "go", "env", "GOVERSION")
	if err == nil {
		t.Fatalf("expected error when context cancelled")
	}
	if !strings.Contains(err.Error(), "command cancelled") {
		t.Fatalf("error = %v, want contains 'command cancelled'", err)
	}
}
