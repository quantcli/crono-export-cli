package cmd

import (
	"github.com/spf13/cobra"

	"github.com/quantcli/crono-export-cli/internal/exporter"
)

var nutritionCmd = &cobra.Command{
	Use:     "nutrition",
	Short:   "Export daily total nutrition (one row per day, all macros + micros)",
	Args:    cobra.NoArgs,
	PreRunE: ValidateExportFlags,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rng, err := exporter.ParseDateRangeFromFlags(cmd)
		if err != nil {
			return err
		}
		if rng.IsEmpty() {
			return emit(cmd, kindNutrition, emptyValueFor(kindNutrition))
		}
		ctx := cmd.Context()
		c, err := exporter.NewLoggedIn(ctx)
		if err != nil {
			return err
		}
		defer c.Logout()
		rows, err := c.Nutrition(ctx, rng)
		if err != nil {
			return err
		}
		return emit(cmd, kindNutrition, rows)
	},
}

func init() {
	exporter.AddDateRangeFlags(nutritionCmd)
	AddFormatFlags(nutritionCmd)
	rootCmd.AddCommand(nutritionCmd)
}
