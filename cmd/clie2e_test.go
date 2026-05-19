package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/quantcli/crono-export-cli/internal/cronotest"
)

// binPath is the compiled crono-export binary; built once in TestMain
// so individual test cases can exec it as a subprocess.
var binPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "crono-export-e2e")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	exe := "crono-export"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	binPath = filepath.Join(tmpDir, exe)

	// Build from the repo root (parent of cmd/).
	build := exec.Command("go", "build", "-o", binPath, "..")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func runCLI(t *testing.T, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	// Redirect every cache-dir env var the binary might consult to a
	// per-test temp dir, mirroring session_test.go's in-process
	// redirect. Without this the subprocess inherits the developer's
	// real HOME/XDG_CACHE_HOME/LOCALAPPDATA and can write a fake
	// session into ~/.cache/crono-export/, clobbering the real cache
	// and letting a pre-existing real session mask the fake-login path.
	cacheDir := t.TempDir()
	isolation := []string{
		"HOME=" + cacheDir,
		"XDG_CACHE_HOME=" + cacheDir,
		"LOCALAPPDATA=" + cacheDir,
	}
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(append(os.Environ(), isolation...), env...)
	var sout, serr bytes.Buffer
	cmd.Stdout = &sout
	cmd.Stderr = &serr
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return sout.String(), serr.String(), ee.ExitCode()
		}
		t.Fatalf("exec %s: %v", binPath, err)
	}
	return sout.String(), serr.String(), 0
}

// fakeEnv returns the env-var overrides that point the CLI at a
// cronotest.Fake — credentials are placeholders since the fake accepts
// anything that round-trips the CSRF token.
func fakeEnv(f *cronotest.Fake) []string {
	return []string{
		"CRONOMETER_USERNAME=alice@example.com",
		"CRONOMETER_PASSWORD=p@ssw0rd",
		"CRONOMETER_BASE_URL=" + f.URL(),
	}
}

func TestCLI_AuthStatus_NoCreds_Fails(t *testing.T) {
	// auth status is a local check; no network. With empty env it must
	// exit non-zero.
	_, stderr, code := runCLI(t,
		[]string{"CRONOMETER_USERNAME=", "CRONOMETER_PASSWORD="},
		"auth", "status",
	)
	if code == 0 {
		t.Fatalf("auth status with no creds should exit non-zero (stderr=%q)", stderr)
	}
	if !strings.Contains(stderr, "CRONOMETER_USERNAME") {
		t.Errorf("stderr should mention missing env var; got %q", stderr)
	}
}

func TestCLI_Nutrition_JSON_E2E(t *testing.T) {
	f := cronotest.New()
	defer f.Close()
	f.DailySummaryCSV = "Date,Calories,Protein\n2026-05-04,1800,90\n2026-05-05,2000,110\n"

	stdout, stderr, code := runCLI(t, fakeEnv(f),
		"nutrition",
		"--since", "2026-05-04",
		"--until", "2026-05-11",
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n--- stdout ---\n%s", err, stdout)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(rows), rows)
	}
	if rows[0]["Calories"] != 1800.0 || rows[1]["Protein"] != 110.0 {
		t.Errorf("rows = %+v", rows)
	}
}

func TestCLI_Nutrition_Markdown_E2E(t *testing.T) {
	f := cronotest.New()
	defer f.Close()
	f.DailySummaryCSV = "Date,Calories,Protein\n2026-05-04,1800,90\n"

	stdout, stderr, code := runCLI(t, fakeEnv(f),
		"nutrition",
		"--since", "2026-05-04",
		"--until", "2026-05-04",
		"--format", "markdown",
	)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "## 2026-05-04") {
		t.Errorf("expected date header in markdown; got %q", stdout)
	}
	if !strings.Contains(stdout, "Calories: 1800") || !strings.Contains(stdout, "Protein: 90") {
		t.Errorf("expected nutrient bullets in markdown; got %q", stdout)
	}
}

func TestCLI_Servings_JSON_E2E(t *testing.T) {
	f := cronotest.New()
	defer f.Close()
	f.ServingsCSV = strings.Join([]string{
		`Day,Group,Food Name,Amount,Energy (kcal),Protein (g)`,
		`2026-05-04,Breakfast,Apple,150 g,78,0.4`,
	}, "\n")

	stdout, stderr, code := runCLI(t, fakeEnv(f),
		"servings",
		"--since", "2026-05-04",
		"--until", "2026-05-04",
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	var recs []map[string]any
	if err := json.Unmarshal([]byte(stdout), &recs); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n--- stdout ---\n%s", err, stdout)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["FoodName"] != "Apple" {
		t.Errorf("recs[0].FoodName = %v, want Apple", recs[0]["FoodName"])
	}
}

func TestCLI_BadFormat_Exits1(t *testing.T) {
	f := cronotest.New()
	defer f.Close()

	_, stderr, code := runCLI(t, fakeEnv(f),
		"nutrition",
		"--since", "2026-05-04",
		"--until", "2026-05-04",
		"--format", "xml",
	)
	if code == 0 {
		t.Fatalf("--format xml should fail; stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "unknown --format") {
		t.Errorf("stderr should mention --format; got %q", stderr)
	}
}

func TestCLI_Prime_NoNetwork(t *testing.T) {
	// `prime` is a local orientation dump per CONTRACT §6; it must not
	// require credentials or hit the network.
	stdout, stderr, code := runCLI(t,
		[]string{"CRONOMETER_USERNAME=", "CRONOMETER_PASSWORD="},
		"prime",
	)
	if code != 0 {
		t.Fatalf("prime should succeed without creds; exit=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "crono-export") {
		t.Errorf("prime output should mention the binary name; got %q", stdout)
	}
}
