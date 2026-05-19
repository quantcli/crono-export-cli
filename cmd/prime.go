package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const primeText = `crono-export — primer for LLM agents
=====================================

WHAT IT IS
  CLI for personal Cronometer data: per-food log, daily nutrition totals,
  biometrics (weight/fat/BP/...), exercises, and notes.

I/O
  stdout: data in --format markdown (default) or json.
  stderr: errors. Exit 0 on success including empty results.

AUTH
  Env-var credentials (always required):
    CRONOMETER_USERNAME   your Cronometer email
    CRONOMETER_PASSWORD   your Cronometer password

  Session is cached at $XDG_CACHE_HOME/crono-export/session.json (mode 0600)
  so consecutive calls reuse one login.  Set CRONOMETER_NO_CACHE=1 to
  disable.  'crono-export auth logout' clears the cache.

  crono-export auth status   Exit 0 if both vars set, 1 with "missing X".

DATE FLAGS  (every subcommand)
  --since VALUE / --until VALUE
  VALUE: today | yesterday | YYYY-MM-DD | Nd/Nw/Nm/Ny
  Default when neither given: last 7 days ending today.
  See https://github.com/quantcli/common/blob/main/CONTRACT.md#3-date-flags

SUBCOMMANDS
  servings    per-food log; one row per food eaten, full nutrient breakdown
  nutrition   daily totals across all foods (one row per day, all macros + micros)
  biometrics  weight, body fat, blood pressure, custom metrics
  exercises   logged cardio / strength / custom activities
  notes       user-entered notes per day

  Inspect any subcommand's row schema with: <subcommand> --since today --format json

EXAMPLES
  crono-export nutrition --since today
  crono-export servings --since 7d --format json | jq '[.[] | .ProteinG] | add'
  crono-export biometrics --since 30d --format json |
    jq 'map(select(.Metric == "Weight")) | sort_by(.RecordedTime) | last'

GOTCHAS
  - 'today' is your LOCAL calendar day, not UTC.
  - 'RecordedTime' is date-only (midnight in your local zone); Cronometer's
    CSV exports don't carry meal-time, so all times sort as 00:00.
  - Markdown drops zero-valued nutrients; use --format json for every column.
  - 'servings' rows have a 'Day' field that is always null — use 'RecordedTime'.
`

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Print an LLM-targeted primer (one screen)",
	Long: `Print a one-screen primer aimed at LLM agents calling this CLI as a tool.
Covers I/O, auth, the shared date flags, the subcommand menu, and a few jq
recipes. Per the quantcli contract, prime is short — anything that wants
to grow into a man page belongs in --help on the relevant subcommand or
in https://github.com/quantcli/common/blob/main/CONTRACT.md.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprint(cmd.OutOrStdout(), primeText)
		return err
	},
}

func init() {
	rootCmd.AddCommand(primeCmd)
}
