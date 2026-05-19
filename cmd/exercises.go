package cmd

import (
	"github.com/spf13/cobra"

	"github.com/quantcli/crono-export-cli/internal/cronoclient"
)

var exercisesCmd = &cobra.Command{
	Use:     "exercises",
	Short:   "Export logged exercises (cardio, strength, custom activities)",
	Args:    cobra.NoArgs,
	PreRunE: ValidateExportFlags,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rng, err := cronoclient.ParseDateRangeFromFlags(cmd)
		if err != nil {
			return err
		}
		if rng.IsEmpty() {
			return emit(cmd, kindExercises, emptyValueFor(kindExercises))
		}
		ctx := cmd.Context()
		c, err := cronoclient.NewLoggedIn(ctx)
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
	cronoclient.AddDateRangeFlags(exercisesCmd)
	AddFormatFlags(exercisesCmd)
	rootCmd.AddCommand(exercisesCmd)
}
