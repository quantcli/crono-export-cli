package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quantcli/crono-export-cli/internal/exporter"
)

func TestAuthStatusCacheState(t *testing.T) {
	const validCache = `{"version":1,"username":"alice@example.com","session":{"auth_token":"test-token"}}`
	cases := []struct {
		name      string
		cache     string
		absent    bool
		directory bool
		noCache   string
		want      string
	}{
		{name: "valid", cache: validCache, want: "valid cached session"},
		{name: "wrong username", cache: strings.ReplaceAll(validCache, "alice@example.com", "bob@example.com"), want: "no usable cached session"},
		{name: "wrong version", cache: strings.ReplaceAll(validCache, `"version":1`, `"version":0`), want: "no usable cached session"},
		{name: "empty auth token", cache: strings.ReplaceAll(validCache, "test-token", ""), want: "no usable cached session"},
		{name: "unparseable", cache: "not JSON", want: "no usable cached session"},
		{name: "truncated", cache: `{"version":1,`, want: "no usable cached session"},
		{name: "zero length", cache: "", want: "no usable cached session"},
		{name: "absent", absent: true, want: "no usable cached session"},
		{name: "read error", directory: true, want: "no usable cached session"},
		{name: "disabled", cache: validCache, noCache: "1", want: "cache disabled (CRONOMETER_NO_CACHE set)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", cacheDir)
			t.Setenv("HOME", cacheDir)
			t.Setenv("LOCALAPPDATA", cacheDir)
			t.Setenv("CRONOMETER_USERNAME", "alice@example.com")
			t.Setenv("CRONOMETER_PASSWORD", "test-password")
			t.Setenv("CRONOMETER_NO_CACHE", tc.noCache)

			p := exporter.SessionCachePath()
			if !tc.absent {
				if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
					t.Fatal(err)
				}
				if tc.directory {
					// Reading a directory fails even when the test runs as root.
					if err := os.Mkdir(p, 0o700); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(p, []byte(tc.cache), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			var out bytes.Buffer
			previousOut := authStatusCmd.OutOrStdout()
			authStatusCmd.SetOut(&out)
			t.Cleanup(func() { authStatusCmd.SetOut(previousOut) })
			if err := authStatusCmd.RunE(authStatusCmd, nil); err != nil {
				t.Fatalf("auth status: %v", err)
			}
			want := "credentials present for alice@example.com (" + tc.want + ")\n"
			if got := out.String(); got != want {
				t.Errorf("auth status output = %q, want %q", got, want)
			}
		})
	}
}
