package cronoclient

import (
	"net/http"
	"os"
	"testing"

	"github.com/quantcli/crono-export-cli/internal/cronoapi"
)

func TestSessionCacheRoundTrip(t *testing.T) {
	// Redirect the cache to a temp dir so we don't clobber the real one.
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	// macOS UserCacheDir ignores XDG_CACHE_HOME; on platforms where it
	// reads HOME instead, also point HOME at the temp dir.
	t.Setenv("HOME", tmp)

	user := "alice@example.com"
	snap := cronoapi.Session{
		UserID:    42,
		AuthToken: "0123456789abcdef0123456789abcdef",
		Cookies: []*http.Cookie{
			{Name: "JSESSIONID", Value: "abc", Path: "/"},
			{Name: "PHPSESSID", Value: "def", Path: "/"},
		},
	}

	if err := saveCachedSession(user, snap); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := loadCachedSession(user)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("load returned nil cache after save")
	}
	if got.Session.UserID != snap.UserID || got.Session.AuthToken != snap.AuthToken {
		t.Errorf("session mismatch: got %+v, want %+v", got.Session, snap)
	}
	if len(got.Session.Cookies) != 2 {
		t.Errorf("cookies: got %d, want 2", len(got.Session.Cookies))
	}

	// Different user → cache miss, not error.
	missed, err := loadCachedSession("someone-else@example.com")
	if err != nil {
		t.Fatalf("load other user: %v", err)
	}
	if missed != nil {
		t.Error("expected nil cache for different user")
	}

	// Delete clears it.
	if _, err := DeleteCachedSession(); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(SessionCachePath()); !os.IsNotExist(err) {
		t.Errorf("cache file still exists after delete: %v", err)
	}
	// Delete on missing cache is a no-op.
	if _, err := DeleteCachedSession(); err != nil {
		t.Errorf("delete-when-missing should be no-op, got %v", err)
	}
}
