package cronoapi

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	exportGenServings     = "servings"
	exportGenExercises    = "exercises"
	exportGenBiometrics   = "biometrics"
	exportGenDailySummary = "dailySummary"
	exportGenNotes        = "notes"

	exportNonceTTL = 3600 // seconds, per WIRE_SHAPES.md §(4)

	csvDateLayout = "2006-01-02"
)

// exportRaw mints a one-shot nonce via GWT-RPC, then GETs the export
// CSV and returns the raw body as a string. WIRE_SHAPES.md §(4-5).
func (c *Client) exportRaw(ctx context.Context, generate string, start, end time.Time) (string, error) {
	if c.authToken == "" {
		return "", fmt.Errorf("%s export: not logged in (call Login first)", generate)
	}
	// Step 1: mint per-export nonce.
	nonceBody, err := c.gwtCall(ctx, generateAuthorizationTokenBody(c.permutation, c.authToken, c.userID, exportNonceTTL))
	if err != nil {
		return "", fmt.Errorf("%s nonce: %w", generate, err)
	}
	nonce, err := parseAuthorizationTokenResponse(nonceBody)
	if err != nil {
		return "", fmt.Errorf("%s nonce: %w", generate, err)
	}

	// Step 2: GET /export with the nonce.
	q := url.Values{}
	q.Set("start", start.Format(csvDateLayout))
	q.Set("end", end.Format(csvDateLayout))
	q.Set("generate", generate)
	q.Set("nonce", nonce)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/export?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET /export?generate=%s: %w", generate, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read %s export body: %w", generate, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET /export?generate=%s: HTTP %d: %s", generate, resp.StatusCode, truncate(string(body), 200))
	}
	return string(body), nil
}

// ExportServingsParsedWithLocation downloads the servings CSV and
// parses it into typed ServingRecords. RecordedTime values are parsed
// in loc; if loc is nil, time.UTC is used.
func (c *Client) ExportServingsParsedWithLocation(ctx context.Context, start, end time.Time, loc *time.Location) (ServingRecords, error) {
	raw, err := c.exportRaw(ctx, exportGenServings, start, end)
	if err != nil {
		return nil, err
	}
	return parseServingsCSV(raw, loc)
}

// ExportExercisesParsedWithLocation downloads the exercises CSV and
// parses it into typed ExerciseRecords.
func (c *Client) ExportExercisesParsedWithLocation(ctx context.Context, start, end time.Time, loc *time.Location) (ExerciseRecords, error) {
	raw, err := c.exportRaw(ctx, exportGenExercises, start, end)
	if err != nil {
		return nil, err
	}
	return parseExercisesCSV(raw, loc)
}

// ExportBiometricRecordsParsedWithLocation downloads the biometrics
// CSV and parses it into typed BiometricRecords.
func (c *Client) ExportBiometricRecordsParsedWithLocation(ctx context.Context, start, end time.Time, loc *time.Location) (BiometricRecords, error) {
	raw, err := c.exportRaw(ctx, exportGenBiometrics, start, end)
	if err != nil {
		return nil, err
	}
	return parseBiometricsCSV(raw, loc)
}

// ExportDailyNutrition downloads the daily-summary CSV and returns it
// verbatim. crono-export-cli parses the raw string with encoding/csv
// into []map[string]string.
func (c *Client) ExportDailyNutrition(ctx context.Context, start, end time.Time) (string, error) {
	return c.exportRaw(ctx, exportGenDailySummary, start, end)
}

// ExportNotes downloads the notes CSV and returns it verbatim.
func (c *Client) ExportNotes(ctx context.Context, start, end time.Time) (string, error) {
	return c.exportRaw(ctx, exportGenNotes, start, end)
}

// ---- CSV parsing ------------------------------------------------------

// readCSV trims a BOM if present, then parses raw with encoding/csv into
// (header, rows). Returns ("", nil) on empty input.
func readCSV(raw string) ([]string, [][]string, error) {
	if raw == "" {
		return nil, nil, nil
	}
	const utf8BOM = "\xef\xbb\xbf"
	if strings.HasPrefix(raw, utf8BOM) {
		raw = strings.TrimPrefix(raw, utf8BOM)
	}
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}
	return rows[0], rows[1:], nil
}

// parseFloat returns 0 for empty or unparseable input. Cronometer
// emits blank cells for missing values; we treat them as zero rather
// than as a parse error to match how crono-export-cli's renderer
// already filters zero-valued nutrients.
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	if err != nil {
		return 0
	}
	return f
}

// parseRecordedTime is tolerant of the timestamp shapes Cronometer
// emits on the various export endpoints (date-only, ISO date+time,
// space-separated date+time). Empty input returns the zero time.
// The parsed value is materialised in loc so subsequent
// .Format("2006-01-02") matches the user's local calendar.
func parseRecordedTime(s string, loc *time.Location) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if loc == nil {
		loc = time.UTC
	}
	layouts := []string{
		csvDateLayout,
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, loc); err == nil {
			return t
		}
	}
	return time.Time{}
}

// headerToFieldName converts a Cronometer CSV column name like
// "Energy (kcal)" or "Vitamin B12 (µg)" into the Go field name on
// ServingRecord that the renderer expects, e.g., "EnergyKcal" or
// "B12Ug". Returns "" when the header is not a nutrient column the
// caller should map onto a struct field.
//
// The mapping mirrors cmd/format.go's strippedSuffix in reverse: the
// renderer splits "EnergyKcal" → ("Energy", "kcal") using a fixed
// suffix table; this function recombines a CSV header with its unit
// into that same Go name.
func headerToNutrientField(header string) string {
	open := strings.LastIndex(header, "(")
	close := strings.LastIndex(header, ")")
	if open < 0 || close < open {
		return ""
	}
	name := strings.TrimSpace(header[:open])
	unit := strings.TrimSpace(header[open+1 : close])
	if name == "" || unit == "" {
		return ""
	}
	// Strip whitespace, hyphens, and other non-letter/digit chars from
	// the nutrient name so "Net Carbs" → "NetCarbs", "Trans-Fats" →
	// "TransFats", "Vitamin B12" → "B12" (drop the "Vitamin " prefix
	// per the captured Go field names — see ServingRecord.B12Ug).
	goName := compactName(name)
	if goName == "" {
		return ""
	}
	goUnit := unitToGoSuffix(unit)
	if goUnit == "" {
		return ""
	}
	return goName + goUnit
}

// compactName strips spaces and punctuation from a CSV column name and
// applies the small number of public Cronometer aliases observed in
// our spec doc (the B-vitamins drop the "Vitamin " prefix; "Net Carbs"
// concatenates, etc.).
func compactName(s string) string {
	// Drop a leading "Vitamin " for B-vitamins so "Vitamin B12" → "B12"
	// (the ServingRecord field is B12Ug, not VitaminB12Ug). Vitamin A,
	// C, D, E, K keep the "Vitamin" prefix per the captured field set.
	if strings.HasPrefix(s, "Vitamin B") {
		s = strings.TrimPrefix(s, "Vitamin ")
	}
	var b strings.Builder
	upperNext := true
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			if upperNext {
				b.WriteRune(r)
				upperNext = false
			} else {
				b.WriteRune(r)
			}
		case r >= 'a' && r <= 'z':
			if upperNext {
				b.WriteRune(r - 32)
				upperNext = false
			} else {
				b.WriteRune(r)
			}
		default:
			upperNext = true
		}
	}
	return b.String()
}

// unitToGoSuffix converts a CSV column's parenthesised unit to the
// Go-style suffix used by ServingRecord field names. Mirrors the
// fixed suffix table in cmd/format.go's strippedSuffix.
func unitToGoSuffix(u string) string {
	switch strings.ToLower(u) {
	case "kcal":
		return "Kcal"
	case "g":
		return "G"
	case "mg":
		return "Mg"
	case "µg", "ug", "mcg":
		return "Ug"
	case "iu", "ui":
		return "IU"
	}
	return ""
}

// servingFieldByName is built once at package load: a name→reflect.Value
// helper isn't safe across instances, so we just cache the list of
// nutrient field names declared on ServingRecord. The CSV parser uses
// reflect to set fields by name on a per-row instance.
var servingNutrientFields = buildServingNutrientFieldSet()

func buildServingNutrientFieldSet() map[string]bool {
	set := map[string]bool{}
	t := reflect.TypeOf(ServingRecord{})
	skip := map[string]bool{
		"RecordedTime": true, "Group": true, "FoodName": true,
		"QuantityValue": true, "QuantityUnits": true, "Category": true,
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if skip[f.Name] {
			continue
		}
		if f.Type.Kind() == reflect.Float64 {
			set[f.Name] = true
		}
	}
	return set
}

// parseServingsCSV maps each CSV header onto either a documented named
// field or a nutrient field on ServingRecord. Columns whose header
// can't be mapped (e.g., a new Cronometer-added nutrient column) are
// silently dropped — a future WIRE_SHAPES.md recapture should extend
// the ServingRecord type to include them.
func parseServingsCSV(raw string, loc *time.Location) (ServingRecords, error) {
	header, rows, err := readCSV(raw)
	if err != nil {
		return nil, fmt.Errorf("servings: %w", err)
	}
	if header == nil {
		return ServingRecords{}, nil
	}

	type binding struct {
		col    int
		setter func(*ServingRecord, string)
	}
	var bindings []binding
	for i, col := range header {
		i := i
		c := strings.TrimSpace(col)
		switch c {
		case "Day", "Date", "Time", "RecordedTime", "Logged At":
			bindings = append(bindings, binding{i, func(r *ServingRecord, v string) {
				r.RecordedTime = parseRecordedTime(v, loc)
			}})
		case "Group", "Meal":
			bindings = append(bindings, binding{i, func(r *ServingRecord, v string) {
				r.Group = strings.TrimSpace(v)
			}})
		case "Food Name", "Food", "FoodName":
			bindings = append(bindings, binding{i, func(r *ServingRecord, v string) {
				r.FoodName = strings.TrimSpace(v)
			}})
		case "Amount", "Quantity", "Serving Amount":
			bindings = append(bindings, binding{i, func(r *ServingRecord, v string) {
				r.QuantityValue, r.QuantityUnits = splitQuantity(v)
			}})
		case "Units", "Serving Units":
			bindings = append(bindings, binding{i, func(r *ServingRecord, v string) {
				r.QuantityUnits = strings.TrimSpace(v)
			}})
		case "Category":
			bindings = append(bindings, binding{i, func(r *ServingRecord, v string) {
				r.Category = strings.TrimSpace(v)
			}})
		default:
			if f := headerToNutrientField(c); f != "" && servingNutrientFields[f] {
				field := f
				bindings = append(bindings, binding{i, func(r *ServingRecord, v string) {
					setFloatField(r, field, parseFloat(v))
				}})
			}
		}
	}

	out := make(ServingRecords, 0, len(rows))
	for _, row := range rows {
		var rec ServingRecord
		for _, b := range bindings {
			if b.col < len(row) {
				b.setter(&rec, row[b.col])
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

// splitQuantity handles CSV cells like "1.00 g" or "1.5 cup". When
// the cell is purely numeric the unit is left empty (the caller's
// separate "Units" column, if any, fills it in).
func splitQuantity(s string) (float64, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ""
	}
	if sp := strings.IndexAny(s, " \t"); sp > 0 {
		return parseFloat(s[:sp]), strings.TrimSpace(s[sp+1:])
	}
	return parseFloat(s), ""
}

// setFloatField sets a float64 field on a ServingRecord by name. The
// field name must already be present in servingNutrientFields.
func setFloatField(r *ServingRecord, name string, v float64) {
	rv := reflect.ValueOf(r).Elem()
	f := rv.FieldByName(name)
	if f.IsValid() && f.CanSet() && f.Kind() == reflect.Float64 {
		f.SetFloat(v)
	}
}

// parseExercisesCSV parses the exercises CSV into ExerciseRecords.
func parseExercisesCSV(raw string, loc *time.Location) (ExerciseRecords, error) {
	header, rows, err := readCSV(raw)
	if err != nil {
		return nil, fmt.Errorf("exercises: %w", err)
	}
	if header == nil {
		return ExerciseRecords{}, nil
	}

	idx := func(names ...string) int {
		for _, want := range names {
			for i, col := range header {
				if strings.EqualFold(strings.TrimSpace(col), want) {
					return i
				}
			}
		}
		return -1
	}
	iDate := idx("Day", "Date", "RecordedTime", "Logged At")
	iName := idx("Exercise", "Activity")
	iMins := idx("Minutes", "Duration (min)", "Duration")
	iKcal := idx("Calories Burned", "Calories Burned (kcal)", "Energy (kcal)")
	iGroup := idx("Category", "Group")

	get := func(row []string, i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return row[i]
	}

	out := make(ExerciseRecords, 0, len(rows))
	for _, row := range rows {
		out = append(out, ExerciseRecord{
			RecordedTime:   parseRecordedTime(get(row, iDate), loc),
			Exercise:       strings.TrimSpace(get(row, iName)),
			Minutes:        parseFloat(get(row, iMins)),
			CaloriesBurned: parseFloat(get(row, iKcal)),
			Group:          strings.TrimSpace(get(row, iGroup)),
		})
	}
	return out, nil
}

// parseBiometricsCSV parses the biometrics CSV into BiometricRecords.
func parseBiometricsCSV(raw string, loc *time.Location) (BiometricRecords, error) {
	header, rows, err := readCSV(raw)
	if err != nil {
		return nil, fmt.Errorf("biometrics: %w", err)
	}
	if header == nil {
		return BiometricRecords{}, nil
	}

	idx := func(names ...string) int {
		for _, want := range names {
			for i, col := range header {
				if strings.EqualFold(strings.TrimSpace(col), want) {
					return i
				}
			}
		}
		return -1
	}
	iDate := idx("Day", "Date", "RecordedTime", "Logged At")
	iMetric := idx("Metric", "Name")
	iAmount := idx("Amount", "Value")
	iUnit := idx("Unit", "Units")

	get := func(row []string, i int) string {
		if i < 0 || i >= len(row) {
			return ""
		}
		return row[i]
	}

	out := make(BiometricRecords, 0, len(rows))
	for _, row := range rows {
		out = append(out, BiometricRecord{
			RecordedTime: parseRecordedTime(get(row, iDate), loc),
			Metric:       strings.TrimSpace(get(row, iMetric)),
			Amount:       parseFloat(get(row, iAmount)),
			Unit:         strings.TrimSpace(get(row, iUnit)),
		})
	}
	return out, nil
}
