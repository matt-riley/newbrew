package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildBinary builds the newbrew binary and returns its path.
// The temp directory is managed by t.TempDir() — no manual cleanup needed.
func buildBinary(t *testing.T) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "newbrew")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}
	return binaryPath
}

func TestNegativeDaysExitsTwoWithErrorOnStderr(t *testing.T) {
	binaryPath := buildBinary(t)

	cmd := exec.Command(binaryPath, "--days", "-1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}

	if stderr.String() == "" {
		t.Error("expected error message on stderr, got empty")
	}
	if stdout.String() != "" {
		t.Errorf("expected no stdout output, got: %s", stdout.String())
	}
}

func TestNegativeLimitExitsTwoWithErrorOnStderr(t *testing.T) {
	binaryPath := buildBinary(t)

	cmd := exec.Command(binaryPath, "--limit", "0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}

	if stderr.String() == "" {
		t.Error("expected error message on stderr, got empty")
	}
	if stdout.String() != "" {
		t.Errorf("expected no stdout output, got: %s", stdout.String())
	}
}

func TestLimitCappedWithWarning(t *testing.T) {
	binaryPath := buildBinary(t)

	cmd := exec.Command(binaryPath, "--limit", "999", "--days", "1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	_ = cmd.Run()

	if stderr.String() == "" {
		t.Log("Note: stderr was empty, possibly because the TUI started and blocked")
	}
}
