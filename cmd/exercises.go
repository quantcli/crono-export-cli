package cmd

import (
	"github.com/spf13/cobra"

	"github.com/quantcli/crono-export-cli/internal/exporter"
)

var exercisesCmd = &cobra.Command{
	Use:     "exercises",
	Short:   "Export logged exercises (cardio, strength, custom activities)",
	Args:    cobra.NoArgs,
	PreRunE: ValidateExportFlags,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rng, err := exporter.ParseDateRangeFromFlags(cmd)
		if err != nil {
			return err
		}
		if rng.IsEmpty() {
			return emit(cmd, kindExercises, emptyValueFor(kindExercises))
		}
		ctx := cmd.Context()
		c, err := exporter.NewLoggedIn(ctx)
		if err != nil {
			return err
		}
		defer c.Logout()
		recs, err := c.Exercises(ctx, rng)
		if err != nil {
			return err
		}
		return emit(cmd, kindExercises, recs)
	},
}

func init() {
	exporter.AddDateRangeFlags(exercisesCmd)
	AddFormatFlags(exercisesCmd)
	rootCmd.AddCommand(exercisesCmd)
}
