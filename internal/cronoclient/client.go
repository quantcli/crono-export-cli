package cronoclient

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/quantcli/crono-export-cli/internal/cronoapi"
)

// Client wraps a logged-in cronoapi.Client and exposes export methods
// that return JSON-ready Go values.
type Client struct {
	inner *cronoapi.Client
}

// NewLoggedIn creates a client and logs in using CRONOMETER_USERNAME and
// CRONOMETER_PASSWORD from the environment.
func NewLoggedIn(ctx context.Context) (*Client, error) {
	user := os.Getenv("CRONOMETER_USERNAME")
	pass := os.Getenv("CRONOMETER_PASSWORD")
	if user == "" || pass == "" {
		return nil, fmt.Errorf("CRONOMETER_USERNAME and CRONOMETER_PASSWORD must be set")
	}
	inner := cronoapi.NewClient(nil)
	if err := inner.Login(ctx, user, pass); err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	return &Client{inner: inner}, nil
}

// Logout best-effort, suitable for `defer`.
func (c *Client) Logout() {
	_ = c.inner.Logout(context.Background())
}

// Servings returns parsed serving records (one row per food item logged).
func (c *Client) Servings(ctx context.Context, rng DateRange) (any, error) {
	recs, err := c.inner.ExportServingsParsedWithLocation(ctx, rng.Start, rng.End, time.Local)
	if err != nil {
		return nil, fmt.Errorf("export servings: %w", err)
	}
	return recs, nil
}

// Exercises returns parsed exercise records.
func (c *Client) Exercises(ctx context.Context, rng DateRange) (any, error) {
	recs, err := c.inner.ExportExercisesParsedWithLocation(ctx, rng.Start, rng.End, time.Local)
	if err != nil {
		return nil, fmt.Errorf("export exercises: %w", err)
	}
	return recs, nil
}

// Biometrics returns parsed biometric records (weight, body fat, etc.).
func (c *Client) Biometrics(ctx context.Context, rng DateRange) (any, error) {
	recs, err := c.inner.ExportBiometricRecordsParsedWithLocation(ctx, rng.Start, rng.End, time.Local)
	if err != nil {
		return nil, fmt.Errorf("export biometrics: %w", err)
	}
	return recs, nil
}

// Nutrition returns daily-totals nutrition rows.  gocronometer does not
// expose a typed parser for this endpoint, so we hand back a list of
// string-keyed objects derived from the raw CSV header.
func (c *Client) Nutrition(ctx context.Context, rng DateRange) (any, error) {
	raw, err := c.inner.ExportDailyNutrition(ctx, rng.Start, rng.End)
	if err != nil {
		return nil, fmt.Errorf("export nutrition: %w", err)
	}
	rows, err := csvToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse nutrition csv: %w", err)
	}
	return rows, nil
}

// Notes returns user-entered notes.  Same string-keyed shape as Nutrition.
func (c *Client) Notes(ctx context.Context, rng DateRange) (any, error) {
	raw, err := c.inner.ExportNotes(ctx, rng.Start, rng.End)
	if err != nil {
		return nil, fmt.Errorf("export notes: %w", err)
	}
	rows, err := csvToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse notes csv: %w", err)
	}
	return rows, nil
}

func csvToJSON(raw string) ([]map[string]string, error) {
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []map[string]string{}, nil
	}
	header := rows[0]
	out := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		obj := make(map[string]string, len(header))
		for i, col := range header {
			if i < len(row) {
				obj[col] = row[i]
			} else {
				obj[col] = ""
			}
		}
		out = append(out, obj)
	}
	return out, nil
}
