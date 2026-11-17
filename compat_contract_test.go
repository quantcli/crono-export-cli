//go:build compat

// Compat-test entry point for crono-export-cli.
//
// This file is only compiled under the `compat` build tag, so it does
// not affect the default `go test ./...` run. CI invokes it as
// `go test -tags=compat ./...` after building the export binary and
// exposing its path through CRONO_EXPORT_BIN.
//
// The actual assertions live in github.com/quantcli/common/compat.
// Drift between this CLI and CONTRACT.md surfaces as a failure here.
package main_test

import (
	"os"
	"testing"

	"github.com/quantcli/common/compat"
	"github.com/quantcli/common/compat/dates"
	"github.com/quantcli/common/compat/formats"

	"github.com/quantcli/crono-export-cli/internal/cronotest"
)

// cronoSubcommands is the §3/§4 surface for crono — each subcommand
// owns its own --since/--until and --format flags. Shared between the
// dates and formats bundles so a single source-of-truth list keeps
// the two suites in sync.
var cronoSubcommands = []string{
	"biometrics",
	"exercises",
	"nutrition",
	"servings",
	"notes",
}

func TestContractDates(t *testing.T) {
	bin := os.Getenv("CRONO_EXPORT_BIN")
	if bin == "" {
		t.Skip("CRONO_EXPORT_BIN not set; skipping compat suite")
	}
	// crono is cobra-based: --since/--until live on each data-producing
	// subcommand, not the root binary. The compat suite dispatches per
	// subcommand under a `subcommand=NAME/...` subtree so any single
	// regression surfaces as a named subtest failure.
	dates.RunContract(t, compat.Runner{
		Binary:      bin,
		Subcommands: cronoSubcommands,
	})
}

func TestContractFormats(t *testing.T) {
	bin := os.Getenv("CRONO_EXPORT_BIN")
	if bin == "" {
		t.Skip("CRONO_EXPORT_BIN not set; skipping compat suite")
	}
	// crono implements all three §4 codecs, so every subtest of the
	// bundle runs: nothing is skipped via SupportedFormats, and nothing
	// via SkipDataPath.
	//
	// The data-path subtests (JSONIsArray, CSVHasHeader,
	// DefaultIsMarkdown) need a successful export, and CI has no
	// Cronometer credentials. They run against cronotest.Fake instead —
	// the same in-process stand-in the CLI's own E2E tests use — reached
	// through the CLI's CRONOMETER_BASE_URL override with placeholder
	// credentials. The fake answers each export the way Cronometer
	// answers a window with no records: a header row and nothing else.
	// So the bundle exercises the §4 empty-result rules: `[]` for json,
	// a header line for csv, and byte-identical (empty) stdout for the
	// default and --format markdown.
	fake := cronotest.New()
	t.Cleanup(fake.Close)
	fake.ServingsCSV = "Day,Group,Food Name,Amount,Energy (kcal)\n"
	fake.ExercisesCSV = "Day,Exercise,Minutes,Calories Burned,Group\n"
	fake.BiometricsCSV = "Day,Metric,Unit,Amount\n"
	fake.DailySummaryCSV = "Date,Energy (kcal)\n"
	fake.NotesCSV = "Day,Note\n"

	formats.RunContract(t, compat.Runner{
		Binary:           bin,
		Subcommands:      cronoSubcommands,
		SupportedFormats: []string{"markdown", "json", "csv"},
		Env: []string{
			"CRONOMETER_USERNAME=compat@example.com",
			"CRONOMETER_PASSWORD=compat-placeholder",
			"CRONOMETER_BASE_URL=" + fake.URL(),
			// No session cache: nothing is read from or written to
			// the machine running the suite.
			"CRONOMETER_NO_CACHE=1",
			"HOME=" + t.TempDir(),
			"TZ=UTC",
		},
	})
}

func TestContractResolvedWindow(t *testing.T) {
	bin := os.Getenv("CRONO_EXPORT_BIN")
	if bin == "" {
		t.Skip("CRONO_EXPORT_BIN not set")
	}
	dates.RunResolvedWindow(t, compat.Runner{Binary: bin, Subcommands: cronoSubcommands})
}
