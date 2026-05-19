package cmd

import (
	"github.com/spf13/cobra"

	"github.com/quantcli/crono-export-cli/internal/exporter"
)

var biometricsCmd = &cobra.Command{
	Use:     "biometrics",
	Short:   "Export biometric records (weight, body fat, blood pressure, custom metrics)",
	Args:    cobra.NoArgs,
	PreRunE: ValidateExportFlags,
	RunE: func(cmd *cobra.Command, _ []string) error {
		rng, err := exporter.ParseDateRangeFromFlags(cmd)
		if err != nil {
			return err
		}
		if rng.IsEmpty() {
			return emit(cmd, kindBiometrics, emptyValueFor(kindBiometrics))
		}
		ctx := cmd.Context()
		c, err := exporter.NewLoggedIn(ctx)
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
	exporter.AddDateRangeFlags(biometricsCmd)
	AddFormatFlags(biometricsCmd)
	rootCmd.AddCommand(biometricsCmd)
}
