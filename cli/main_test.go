package main_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
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

// startMockGitHubAPI points NEWBREW_GITHUB_API_BASE at a local stub GitHub API so
// behavioural tests do not depend on live Homebrew merge activity.
func startMockGitHubAPI(t *testing.T, searchBody string) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			_, _ = io.WriteString(w, searchBody)
		case "/repos/Homebrew/homebrew-core/pulls/101/files":
			rawURL := "http://" + r.Host + "/raw/fixture.rb"
			_, _ = io.WriteString(w, `[{"filename":"Formula/fixture.rb","status":"added","raw_url":"`+rawURL+`"}]`)
		case "/raw/fixture.rb":
			_, _ = io.WriteString(w, "class Fixture < Formula\n  desc \"Fixture tool for tests\"\n  homepage \"https://fixture.example.com\"\nend\n")
		default:
			t.Errorf("unexpected GitHub API path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("NEWBREW_GITHUB_API_BASE", server.URL)
}

func TestHelpOutputContainsFlags(t *testing.T) {
	stdout, stderr, code := run("--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	combined := stdout + stderr
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
	if !strings.Contains(combined, "-plain") {
		t.Error("--help output missing -plain flag")
	}
	if !strings.Contains(combined, "-json") {
		t.Error("--help output missing -json flag")
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

func TestPlainAndJsonMutuallyExclusive(t *testing.T) {
	stdout, stderr, code := run("--plain", "--json")
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
}

func TestPlainFlagRecognized(t *testing.T) {
	// --plain is a valid flag — verify the binary doesn't reject it as unknown.
	stdout, stderr, code := run("--plain", "--bogus")
	if code == 0 {
		t.Fatal("expected non-zero exit for --bogus")
	}
	// The error should be about --bogus, not --plain.
	if strings.Contains(stderr, "plain") && !strings.Contains(stderr, "bogus") {
		t.Errorf("expected error about --bogus, not --plain, got %q", stderr)
	}
	_ = stdout
}

func TestJsonFlagRecognized(t *testing.T) {
	// --json is a valid flag — verify the binary doesn't reject it as unknown.
	stdout, stderr, code := run("--json", "--bogus")
	if code == 0 {
		t.Fatal("expected non-zero exit for --bogus")
	}
	// The error should be about --bogus, not --json.
	if strings.Contains(stderr, "json") && !strings.Contains(stderr, "bogus") {
		t.Errorf("expected error about --bogus, not --json, got %q", stderr)
	}
	_ = stdout
}

// TestNonTTYWithoutFlagExitsTwo verifies that running without a TTY and
// without --plain or --json produces exit code 2 (usage error) *before*
// any network call is attempted.
func TestNonTTYWithoutFlagExitsTwo(t *testing.T) {
	stdout, stderr, code := run()
	if code != 2 {
		t.Fatalf("expected exit code 2 for non-TTY without --plain/--json, got %d", code)
	}
	if !strings.Contains(stderr, "needs a terminal") {
		t.Errorf("expected 'needs a terminal' in stderr, got %q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
}

// TestPlainOutputFormat runs --plain against a stub GitHub API and verifies
// tab-separated output with the expected fields.
func TestPlainOutputFormat(t *testing.T) {
	startMockGitHubAPI(t, `{"items":[{"number":101,"title":"fixture 1.0.0 (new formula)","closed_at":"2026-06-14T10:00:00Z"}]}`)

	stdout, stderr, code := run("--plain", "--days=1", "--limit=1", "--no-cache")
	if code == 2 {
		t.Fatalf("--plain should not cause exit 2 (usage error), got stderr: %s", stderr)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code %d, stderr: %s", code, stderr)
	}

	want := "fixture 1.0.0 (new formula)\tFixture tool for tests\thttps://fixture.example.com\t2026-06-14\n"
	if stdout != want {
		t.Errorf("unexpected --plain output:\ngot:  %q\nwant: %q", stdout, want)
	}
}

// TestPlainOutputFormatEmptyResults verifies that a successful fetch with no
// formulae produces no stdout lines (not a single empty line).
func TestPlainOutputFormatEmptyResults(t *testing.T) {
	startMockGitHubAPI(t, `{"items":[]}`)

	stdout, stderr, code := run("--plain", "--days=1", "--limit=1", "--no-cache")
	if code != 0 {
		t.Fatalf("unexpected exit code %d, stderr: %s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout for zero formulae, got %q", stdout)
	}
}

// TestJsonOutputFormat runs --json against a stub GitHub API and verifies the
// encoded array shape and field values.
func TestJsonOutputFormat(t *testing.T) {
	startMockGitHubAPI(t, `{"items":[{"number":101,"title":"fixture 1.0.0 (new formula)","closed_at":"2026-06-14T10:00:00Z"}]}`)

	stdout, stderr, code := run("--json", "--days=1", "--limit=1", "--no-cache")
	if code == 2 {
		t.Fatalf("--json should not cause exit 2 (usage error), got stderr: %s", stderr)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code %d, stderr: %s", code, stderr)
	}

	want := `[{"PRTitle":"fixture 1.0.0 (new formula)","Description":"Fixture tool for tests","Homepage":"https://fixture.example.com","MergedAt":"2026-06-14T10:00:00Z"}]` + "\n"
	if stdout != want {
		t.Errorf("unexpected --json output:\ngot:  %q\nwant: %q", stdout, want)
	}
}

// TestJsonOutputFormatEmptyResults verifies that zero formulae still emit [].
func TestJsonOutputFormatEmptyResults(t *testing.T) {
	startMockGitHubAPI(t, `{"items":[]}`)

	stdout, stderr, code := run("--json", "--days=1", "--limit=1", "--no-cache")
	if code != 0 {
		t.Fatalf("unexpected exit code %d, stderr: %s", code, stderr)
	}
	if stdout != "[]\n" {
		t.Errorf("expected empty JSON array, got %q", stdout)
	}
}

// TestPlainAndJsonDontEmitExitOneForFlagHandling confirms that --plain and
// --json are valid flags — the binary should never exit with code 1 due to
// flag parsing. Exit 2 is for usage errors; exit 1 is only for operational
// errors like network failures.
func TestPlainAndJsonDontEmitExitOneForFlagHandling(t *testing.T) {
	// --plain alone should be accepted syntactically.
	_, _, code := run("--plain", "--days=1", "--limit=1", "--no-cache")
	if code == 2 {
		t.Error("--plain flag should not trigger a usage error (exit 2)")
	}

	// --json alone should be accepted syntactically.
	_, _, code = run("--json", "--days=1", "--limit=1", "--no-cache")
	if code == 2 {
		t.Error("--json flag should not trigger a usage error (exit 2)")
	}
}

// --- pflag migration tests ---

func TestShortVersionFlagPrintsVersionAndExitsZero(t *testing.T) {
	stdout, stderr, code := run("-v")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "dev") {
		t.Errorf("version output should contain 'dev', got: %s", stdout)
	}
	if stderr != "" {
		t.Errorf("expected no stderr output, got: %s", stderr)
	}
}

func TestShortVersionUpperFlagPrintsVersionAndExitsZero(t *testing.T) {
	stdout, stderr, code := run("-V")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "dev") {
		t.Errorf("version output should contain 'dev', got: %s", stdout)
	}
	if stderr != "" {
		t.Errorf("expected no stderr output, got: %s", stderr)
	}
}

func TestShortDaysFlagParsesCorrectly(t *testing.T) {
	_, _, code := run("-d", "7", "--version")
	if code != 0 {
		t.Fatalf("expected exit 0 for -d 7 --version, got %d", code)
	}
}

func TestShortLimitFlagParsesCorrectly(t *testing.T) {
	_, _, code := run("-l", "30", "--version")
	if code != 0 {
		t.Fatalf("expected exit 0 for -l 30 --version, got %d", code)
	}
}

func TestShortNoCacheFlagParsesCorrectly(t *testing.T) {
	_, _, code := run("-n", "--version")
	if code != 0 {
		t.Fatalf("expected exit 0 for -n --version, got %d", code)
	}
}

func TestAllShortFlagsTogether(t *testing.T) {
	_, _, code := run("-d", "7", "-l", "50", "-n", "--version")
	if code != 0 {
		t.Fatalf("expected exit 0 for -d 7 -l 50 -n --version, got %d", code)
	}
}

func TestSingleDashVersionRejected(t *testing.T) {
	_, stderr, code := run("-version")
	if code != 2 {
		t.Fatalf("expected exit code 2 for -version, got %d", code)
	}
	if !strings.Contains(stderr, "unknown") {
		t.Errorf("expected 'unknown' in error message, got: %s", stderr)
	}
}

func TestHelpShowsEnvVars(t *testing.T) {
	stdout, stderr, code := run("--help")
	if code != 0 {
		t.Fatalf("expected exit 0 for --help, got %d", code)
	}
	helpText := stdout + stderr
	if !strings.Contains(helpText, "GITHUB_TOKEN") {
		t.Error("--help should mention GITHUB_TOKEN")
	}
	if !strings.Contains(helpText, "XDG_CACHE_HOME") {
		t.Error("--help should mention XDG_CACHE_HOME")
	}
	if !strings.Contains(helpText, "Examples:") {
		t.Error("--help should include example invocations")
	}
	if !strings.Contains(helpText, "-d") || !strings.Contains(helpText, "-l") {
		t.Error("--help should show short flag forms")
	}
}
