package cronoclient

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/quantcli/crono-export-cli/internal/cronoapi"
)

// Client wraps a logged-in cronoapi.Client and exposes export methods
// that return JSON-ready Go values.
type Client struct {
	inner *cronoapi.Client
	user  string
	pass  string
	// fresh is true if this client just performed a Login (i.e. the
	// session is known-good).  If false, the session came from the
	// on-disk cache and may be stale; a single auto-retry on the first
	// export call will fall back to a fresh login.
	fresh bool
}

// NewLoggedIn creates a client ready to call the export methods.  It
// prefers a cached session under $XDG_CACHE_HOME/crono-export/session.json
// (mode 0600) and falls back to a fresh Cronometer login when no
// cache exists or the cache is invalid for the current user.  Set
// CRONOMETER_NO_CACHE=1 to force a per-invocation login.
//
// CRONOMETER_USERNAME and CRONOMETER_PASSWORD must be set in either
// case — even with a cache hit we hold the password so the wrapper
// can transparently re-login if the cached session turns out to be
// stale.
func NewLoggedIn(ctx context.Context) (*Client, error) {
	user := os.Getenv("CRONOMETER_USERNAME")
	pass := os.Getenv("CRONOMETER_PASSWORD")
	if user == "" || pass == "" {
		return nil, fmt.Errorf("CRONOMETER_USERNAME and CRONOMETER_PASSWORD must be set")
	}
	inner := cronoapi.NewClient(nil)
	c := &Client{inner: inner, user: user, pass: pass}

	if cacheEnabled() {
		if cached, err := loadCachedSession(user); err == nil && cached != nil {
			inner.Restore(cached.Session)
			return c, nil
		}
	}
	if err := c.login(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// login performs a fresh Cronometer login and, on success, persists
// the session to the on-disk cache (unless caching is disabled).
func (c *Client) login(ctx context.Context) error {
	if err := c.inner.Login(ctx, c.user, c.pass); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	c.fresh = true
	if cacheEnabled() {
		_ = saveCachedSession(c.user, c.inner.Snapshot())
	}
	return nil
}

// callWithRetry runs fn; if fn fails AND the session was loaded from
// cache (could be stale), it invalidates the cache, performs a fresh
// login, and retries once.  Returns the result of the (possibly
// retried) call.
func callWithRetry[T any](c *Client, ctx context.Context, fn func() (T, error)) (T, error) {
	out, err := fn()
	if err == nil || c.fresh {
		return out, err
	}
	_, _ = DeleteCachedSession()
	if rerr := c.login(ctx); rerr != nil {
		return out, err // surface the original error, not the re-login one
	}
	return fn()
}

// Logout best-effort, suitable for `defer`.  When the cache is in
// play we intentionally do NOT call the network logout — that would
// invalidate the cached session for the next invocation.  We only
// tear down the server-side session when caching is disabled.
func (c *Client) Logout() {
	if cacheEnabled() {
		return
	}
	_ = c.inner.Logout(context.Background())
}

// Servings returns parsed serving records (one row per food item logged).
func (c *Client) Servings(ctx context.Context, rng DateRange) (any, error) {
	recs, err := callWithRetry(c, ctx, func() (cronoapi.ServingRecords, error) {
		return c.inner.ExportServingsParsedWithLocation(ctx, rng.Start, rng.End, time.Local)
	})
	if err != nil {
		return nil, fmt.Errorf("export servings: %w", err)
	}
	return recs, nil
}

// Exercises returns parsed exercise records.
func (c *Client) Exercises(ctx context.Context, rng DateRange) (any, error) {
	recs, err := callWithRetry(c, ctx, func() (cronoapi.ExerciseRecords, error) {
		return c.inner.ExportExercisesParsedWithLocation(ctx, rng.Start, rng.End, time.Local)
	})
	if err != nil {
		return nil, fmt.Errorf("export exercises: %w", err)
	}
	return recs, nil
}

// Biometrics returns parsed biometric records (weight, body fat, etc.).
func (c *Client) Biometrics(ctx context.Context, rng DateRange) (any, error) {
	recs, err := callWithRetry(c, ctx, func() (cronoapi.BiometricRecords, error) {
		return c.inner.ExportBiometricRecordsParsedWithLocation(ctx, rng.Start, rng.End, time.Local)
	})
	if err != nil {
		return nil, fmt.Errorf("export biometrics: %w", err)
	}
	return recs, nil
}

// Nutrition returns daily-totals nutrition rows.  cronoapi does not
// expose a typed parser for this endpoint, so we hand back a list of
// string-keyed objects derived from the raw CSV header.  Values are
// best-effort coerced (numeric → float64, true/false → bool, empty → null).
func (c *Client) Nutrition(ctx context.Context, rng DateRange) (any, error) {
	raw, err := callWithRetry(c, ctx, func() (string, error) {
		return c.inner.ExportDailyNutrition(ctx, rng.Start, rng.End)
	})
	if err != nil {
		return nil, fmt.Errorf("export nutrition: %w", err)
	}
	rows, err := csvToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse nutrition csv: %w", err)
	}
	return rows, nil
}

// Notes returns user-entered notes.  Same shape as Nutrition.
func (c *Client) Notes(ctx context.Context, rng DateRange) (any, error) {
	raw, err := callWithRetry(c, ctx, func() (string, error) {
		return c.inner.ExportNotes(ctx, rng.Start, rng.End)
	})
	if err != nil {
		return nil, fmt.Errorf("export notes: %w", err)
	}
	rows, err := csvToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse notes csv: %w", err)
	}
	return rows, nil
}

func csvToJSON(raw string) ([]map[string]any, error) {
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []map[string]any{}, nil
	}
	header := rows[0]
	out := make([]map[string]any, 0, len(rows)-1)
	for _, row := range rows[1:] {
		obj := make(map[string]any, len(header))
		for i, col := range header {
			var raw string
			if i < len(row) {
				raw = row[i]
			}
			obj[col] = coerceCSVValue(raw)
		}
		out = append(out, obj)
	}
	return out, nil
}

// coerceCSVValue best-effort-parses a CSV cell into a typed value: empty
// → nil, numeric → float64, true/false → bool, otherwise the original
// string.  Keeps the JSON output of /nutrition and /notes consumable
// with jq without forcing a `tonumber` cast on every column.
func coerceCSVValue(s string) any {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f
	}
	switch strings.ToLower(trimmed) {
	case "true":
		return true
	case "false":
		return false
	}
	return s
}
