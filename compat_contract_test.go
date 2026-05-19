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
	// crono implements --format markdown (default) and --format json
	// today; CSV is not yet wired (see cmd/format.go chosenFormat).
	// SupportedFormats: ["markdown","json"] skips CSVHasHeader with a
	// named reason rather than failing it.
	//
	// SkipDataPath: true opts out of JSONIsArray / CSVHasHeader /
	// DefaultIsMarkdown — crono's data path requires
	// CRONOMETER_USERNAME/PASSWORD which the compat CI job does not
	// provide, so the data-path subtests would fail at "not logged in"
	// before the codec assertions could run. The parse-level subtests
	// (HelpDocumentsFormatFlag, UnknownFormatFails,
	// FlagValidationIsHermetic) still attest the §4 surface.
	formats.RunContract(t, compat.Runner{
		Binary:           bin,
		Subcommands:      cronoSubcommands,
		SupportedFormats: []string{"markdown", "json"},
		SkipDataPath:     true,
	})
}
