package exporter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/quantcli/crono-export-cli/internal/cronoapi"
)

// cacheSchemaVersion is bumped whenever the on-disk session shape
// changes incompatibly.  An older cache is silently ignored (treated
// as a miss) so existing users transparently re-login on upgrade.
const cacheSchemaVersion = 1

// cachedSession is the on-disk representation of a Cronometer login.
// We key by username so flipping CRONOMETER_USERNAME invalidates the
// cache automatically instead of replaying another user's session.
type cachedSession struct {
	Version   int              `json:"version"`
	Username  string           `json:"username"`
	Session   cronoapi.Session `json:"session"`
	SavedAt   time.Time        `json:"saved_at"`
	UserAgent string           `json:"user_agent,omitempty"`
}

// sessionCachePath returns the path to the persisted session file,
// rooted at the OS user cache dir (respects XDG_CACHE_HOME on Linux,
// ~/Library/Caches on macOS, %LocalAppData% on Windows).
func sessionCachePath() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "crono-export", "session.json"), nil
}

// cacheEnabled reports whether we should read/write the session cache.
// Set CRONOMETER_NO_CACHE=1 (or any non-empty value) to force per-call
// login.
func cacheEnabled() bool {
	return os.Getenv("CRONOMETER_NO_CACHE") == ""
}

// loadCachedSession returns the persisted session for user, or
// (nil, nil) if no usable cache exists.  Errors are only returned for
// genuine I/O surprises — a missing file or a user mismatch is
// silently treated as "no cache" so callers fall through to a fresh
// login.
func loadCachedSession(user string) (*cachedSession, error) {
	p, err := sessionCachePath()
	if err != nil {
		return nil, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var s cachedSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, nil
	}
	if s.Version != cacheSchemaVersion {
		return nil, nil
	}
	if s.Username != user {
		return nil, nil
	}
	if s.Session.AuthToken == "" {
		return nil, nil
	}
	return &s, nil
}

// saveCachedSession writes snap to disk with mode 0600.  Best-effort —
// errors are returned for callers that care to log them but never block
// the export.
func saveCachedSession(user string, snap cronoapi.Session) error {
	p, err := sessionCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(cachedSession{
		Version:  cacheSchemaVersion,
		Username: user,
		Session:  snap,
		SavedAt:  time.Now(),
	})
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// DeleteCachedSession removes the on-disk session file.  Exported so
// the `auth logout` subcommand can invoke it.  Returns nil if the
// cache didn't exist.
func DeleteCachedSession() (string, error) {
	p, err := sessionCachePath()
	if err != nil {
		return "", err
	}
	if err := os.Remove(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return p, nil
		}
		return p, fmt.Errorf("delete session cache: %w", err)
	}
	return p, nil
}

// SessionCachePath returns the cache location for display purposes
// (e.g. by `auth status`).  Empty string if the OS cache dir is
// undiscoverable.
func SessionCachePath() string {
	p, err := sessionCachePath()
	if err != nil {
		return ""
	}
	return p
}
