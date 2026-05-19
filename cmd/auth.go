package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/quantcli/crono-export-cli/internal/cronoclient"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print one-line auth readiness state and exit 0 if usable",
	Long: `Print a one-line summary of whether the CLI has the credentials it needs
to talk to Cronometer. Exit code 0 if usable, 1 if missing.

This is a local check — no network call. It only verifies the
CRONOMETER_USERNAME and CRONOMETER_PASSWORD environment variables are set.
The CLI logs in fresh on every invocation, so a successful status here is
necessary but not sufficient: a wrong password will still fail at run time.

Per the quantcli shared contract:
https://github.com/quantcli/common/blob/main/CONTRACT.md#5-auth`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		user := os.Getenv("CRONOMETER_USERNAME")
		pass := os.Getenv("CRONOMETER_PASSWORD")
		switch {
		case user == "" && pass == "":
			return fmt.Errorf("missing CRONOMETER_USERNAME and CRONOMETER_PASSWORD")
		case user == "":
			return fmt.Errorf("missing CRONOMETER_USERNAME")
		case pass == "":
			return fmt.Errorf("missing CRONOMETER_PASSWORD")
		}
		cacheState := "no cache"
		if os.Getenv("CRONOMETER_NO_CACHE") == "" {
			if p := cronoclient.SessionCachePath(); p != "" {
				if _, err := os.Stat(p); err == nil {
					cacheState = "session cached"
				} else {
					cacheState = "no session cached"
				}
			}
		} else {
			cacheState = "cache disabled (CRONOMETER_NO_CACHE set)"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "credentials present for %s (%s)\n", user, cacheState)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Delete the cached Cronometer session, forcing a fresh login next call",
	Long: `Remove the on-disk session cache at $XDG_CACHE_HOME/crono-export/session.json.
Useful after rotating your password or when you suspect a stale session
is causing failures.

This is a local-only operation: it does NOT call Cronometer's logout
endpoint (that would invalidate the cached cookies for a session we've
already deleted). The next export call will perform a fresh login.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		p, err := cronoclient.DeleteCachedSession()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "session cache cleared (%s)\n", p)
		return nil
	},
}

func init() {
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}
