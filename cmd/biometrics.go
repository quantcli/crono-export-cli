package cmd

import (
	"github.com/spf13/cobra"

	"github.com/quantcli/crono-export-cli/internal/cronoclient"
)

var biometricsCmd = &cobra.Command{
	Use:     "biometrics",
	Short:   "Export biometric records (weight, body fat, blood pressure, custom metrics)",
	Args:    cobra.NoArgs,
	PreRunE: ValidateExportFlags,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rng, err := cronoclient.ParseDateRangeFromFlags(cmd)
		if err != nil {
			return err
		}
		if rng.IsEmpty() {
			return emit(cmd, kindBiometrics, emptyValueFor(kindBiometrics))
		}
		ctx := cmd.Context()
		c, err := cronoclient.NewLoggedIn(ctx)
		if err != nil {
			return err
		}
		defer c.Logout()
		recs, err := c.Biometrics(ctx, rng)
		if err != nil {
			return err
		}
		return emit(cmd, kindBiometrics, recs)
	},
}

func init() {
	cronoclient.AddDateRangeFlags(biometricsCmd)
	AddFormatFlags(biometricsCmd)
	rootCmd.AddCommand(biometricsCmd)
}
