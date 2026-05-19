package cmd

import (
	"github.com/spf13/cobra"

	"github.com/quantcli/crono-export-cli/internal/cronoclient"
)

var servingsCmd = &cobra.Command{
	Use:     "servings",
	Short:   "Export logged food servings (one row per food eaten, full nutrient breakdown)",
	Args:    cobra.NoArgs,
	PreRunE: ValidateExportFlags,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rng, err := cronoclient.ParseDateRangeFromFlags(cmd)
		if err != nil {
			return err
		}
		if rng.IsEmpty() {
			return emit(cmd, kindServings, emptyValueFor(kindServings))
		}
		ctx := cmd.Context()
		c, err := cronoclient.NewLoggedIn(ctx)
		if err != nil {
			return err
		}
		defer c.Logout()
		recs, err := c.Servings(ctx, rng)
		if err != nil {
			return err
		}
		return emit(cmd, kindServings, recs)
	},
}

func init() {
	cronoclient.AddDateRangeFlags(servingsCmd)
	AddFormatFlags(servingsCmd)
	rootCmd.AddCommand(servingsCmd)
}
