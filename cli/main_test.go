package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all behavioural tests.
	tmpDir, err := os.MkdirTemp("", "newbrew-main-test-*")
	if err != nil {
		os.Exit(1)
	}
	binaryPath = filepath.Join(tmpDir, "newbrew")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		_, _ = os.Stderr.Write(out)
		_ = os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}

func run(args ...string) (stdout, stderr string, exitCode int) {
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return stdout, stderr, exitCode
}

func TestHelpOutputContainsFlags(t *testing.T) {
	stdout, stderr, code := run("--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Go's flag package writes usage to stderr by default.
	combined := stdout + stderr
	// Flag package auto-generates help listing defined flags.
	if !strings.Contains(combined, "-days") {
		t.Error("--help output missing -days flag")
	}
	if !strings.Contains(combined, "-limit") {
		t.Error("--help output missing -limit flag")
	}
	if !strings.Contains(combined, "-no-cache") {
		t.Error("--help output missing -no-cache flag")
	}
	if !strings.Contains(combined, "-version") {
		t.Error("--help output missing -version flag")
	}
}

func TestBogusFlagExitsTwo(t *testing.T) {
	stdout, stderr, code := run("--bogus")
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	// Error text must mention the unknown flag.
	if !strings.Contains(stderr, "bogus") {
		t.Errorf("expected stderr to mention bogus, got %q", stderr)
	}
	// stdout should be empty.
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
}

func TestVersionExitsZero(t *testing.T) {
	stdout, stderr, code := run("--version")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr for --version, got %q", stderr)
	}
	if !strings.Contains(stdout, "dev") {
		t.Error("--version output missing version string")
	}
}

func TestFatalErrorsWrittenToStderrNotStdout(t *testing.T) {
	stdout, stderr, code := run("--bogus")
	if code == 0 {
		t.Fatal("expected non-zero exit for invalid flag")
	}
	if stderr == "" {
		t.Fatal("expected error output on stderr")
	}
	if stdout != "" {
		t.Errorf("expected empty stdout on error, got %q", stdout)
	}
}

func TestPlainFlag(t *testing.T) {
	// Plain mode is not yet implemented; skip until it lands.
	// The test is here as a placeholder so the acceptance criteria are visible.
	t.Skip("plain mode flag not yet implemented — see plain-mode task")
}
