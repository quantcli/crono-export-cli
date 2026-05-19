package cmd

import (
	"github.com/spf13/cobra"

	"github.com/quantcli/crono-export-cli/internal/exporter"
)

var notesCmd = &cobra.Command{
	Use:     "notes",
	Short:   "Export user-entered notes",
	Args:    cobra.NoArgs,
	PreRunE: ValidateExportFlags,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rng, err := exporter.ParseDateRangeFromFlags(cmd)
		if err != nil {
			return err
		}
		if rng.IsEmpty() {
			return emit(cmd, kindNotes, emptyValueFor(kindNotes))
		}
		ctx := cmd.Context()
		c, err := exporter.NewLoggedIn(ctx)
		if err != nil {
			return err
		}
		defer c.Logout()
		rows, err := c.Notes(ctx, rng)
		if err != nil {
			return err
		}
		return emit(cmd, kindNotes, rows)
	},
}

func init() {
	exporter.AddDateRangeFlags(notesCmd)
	AddFormatFlags(notesCmd)
	rootCmd.AddCommand(notesCmd)
}
