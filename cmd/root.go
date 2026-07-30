// Package cmd wires the Cobra command tree for crono-export.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overwritten at release time via
// -ldflags "-X github.com/quantcli/crono-export-cli/cmd.version=v1.1.0".
// Setting rootCmd.Version below makes cobra register --version for free.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "crono-export",
	Short:   "Export Cronometer nutrition, biometrics, and food log data",
	Version: version,
	Long: `crono-export reads your personal Cronometer data via the same export
endpoints the web app uses and prints it on stdout.  Default output is
narrow, fitdown-style markdown; pass --format json for the full
structured row.

Credentials must be supplied via environment variables:
  CRONOMETER_USERNAME  your Cronometer email
  CRONOMETER_PASSWORD  your Cronometer password

Designed for use by personal LLM agents and scripts — markdown reads
well in chat, JSON pipes well to jq.

LLM agents: run 'crono-export prime' for a one-screen orientation
(I/O contract, subcommands, date flags, jq recipes).`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.  Called by main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
