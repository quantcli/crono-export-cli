package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/quantcli/crono-export-cli/internal/cronoapi"
	"github.com/spf13/cobra"
)

type recordKind int

const (
	kindServings recordKind = iota
	kindNutrition
	kindBiometrics
	kindExercises
	kindNotes
)

// AddFormatFlags registers --format on every export subcommand, per the
// quantcli shared contract §4.
// https://github.com/quantcli/common/blob/main/CONTRACT.md#4-output-format
func AddFormatFlags(cmd *cobra.Command) {
	cmd.Flags().String("format", "markdown",
		"Output format: markdown (default, fitdown-style) or json")
}

func chosenFormat(cmd *cobra.Command) (string, error) {
	f, _ := cmd.Flags().GetString("format")
	switch f {
	case "", "markdown", "md":
		return "markdown", nil
	case "json":
		return "json", nil
	default:
		return "", fmt.Errorf("unknown --format %q (use markdown or json)", f)
	}
}

// emit writes v in the format chosen on cmd.  kind tells the markdown
// renderer which layout to use.
func emit(cmd *cobra.Command, kind recordKind, v any) error {
	f, err := chosenFormat(cmd)
	if err != nil {
		return err
	}
	if f == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	return renderMarkdown(os.Stdout, kind, v)
}

func renderMarkdown(w io.Writer, kind recordKind, v any) error {
	switch kind {
	case kindServings:
		recs, _ := v.(cronoapi.ServingRecords)
		return renderServings(w, recs)
	case kindBiometrics:
		recs, _ := v.(cronoapi.BiometricRecords)
		return renderBiometrics(w, recs)
	case kindExercises:
		recs, _ := v.(cronoapi.ExerciseRecords)
		return renderExercises(w, recs)
	case kindNutrition:
		rows, _ := v.([]map[string]string)
		return renderNutrition(w, rows)
	case kindNotes:
		rows, _ := v.([]map[string]string)
		return renderNotes(w, rows)
	}
	return fmt.Errorf("renderMarkdown: unknown kind %d", kind)
}

// ---- shared helpers ---------------------------------------------------

func emptyMsg(w io.Writer) error {
	_, err := fmt.Fprintln(w, "_(no records in window)_")
	return err
}

// fmtFloat trims trailing zeros so 1.95 → "1.95" and 100.000 → "100".
func fmtFloat(f float64) string {
	s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", f), "0"), ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

// strippedSuffix splits a CamelCase Go field name like "EnergyKcal" or
// "B12Mg" into ("Energy","kcal") or ("B12","mg").  Order matters: longer
// suffixes first so we don't strip "g" out of "Mg".
func strippedSuffix(field string) (name, unit string) {
	for _, suf := range []struct{ go_, display string }{
		{"Kcal", "kcal"},
		{"Mg", "mg"},
		{"Ug", "µg"},
		{"UI", "IU"},
		{"G", "g"},
	} {
		if strings.HasSuffix(field, suf.go_) && len(field) > len(suf.go_) {
			return field[:len(field)-len(suf.go_)], suf.display
		}
	}
	return field, ""
}

// ---- servings ---------------------------------------------------------

func renderServings(w io.Writer, recs cronoapi.ServingRecords) error {
	if len(recs) == 0 {
		return emptyMsg(w)
	}
	// Group by local calendar date.
	byDate := map[string][]cronoapi.ServingRecord{}
	for _, r := range recs {
		d := r.RecordedTime.Format("2006-01-02")
		byDate[d] = append(byDate[d], r)
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	for di, d := range dates {
		if di > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "## %s\n\n", d)
		for _, r := range byDate[d] {
			renderServingRecord(w, r)
		}
	}
	fmt.Fprintln(w, "_zero-valued nutrients omitted; use --format json for the full row_")
	return nil
}

func renderServingRecord(w io.Writer, r cronoapi.ServingRecord) {
	header := fmt.Sprintf("### %s · %s", strDefault(r.Group, "—"), r.FoodName)
	if r.QuantityValue != 0 || r.QuantityUnits != "" {
		header += fmt.Sprintf(" (%s %s)", fmtFloat(r.QuantityValue), r.QuantityUnits)
	}
	fmt.Fprintln(w, header)

	v := reflect.ValueOf(r)
	t := v.Type()
	skip := map[string]bool{
		"RecordedTime":  true,
		"Group":         true,
		"FoodName":      true,
		"QuantityValue": true,
		"QuantityUnits": true,
		"Category":      true,
	}
	for i := 0; i < t.NumField(); i++ {
		fname := t.Field(i).Name
		if skip[fname] {
			continue
		}
		if v.Field(i).Kind() != reflect.Float64 {
			continue
		}
		val := v.Field(i).Float()
		if val == 0 {
			continue
		}
		name, unit := strippedSuffix(fname)
		if unit != "" {
			fmt.Fprintf(w, "- %s: %s %s\n", name, fmtFloat(val), unit)
		} else {
			fmt.Fprintf(w, "- %s: %s\n", name, fmtFloat(val))
		}
	}
	fmt.Fprintln(w)
}

func strDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// ---- biometrics -------------------------------------------------------

func renderBiometrics(w io.Writer, recs cronoapi.BiometricRecords) error {
	if len(recs) == 0 {
		return emptyMsg(w)
	}
	byDate := map[string][]cronoapi.BiometricRecord{}
	for _, r := range recs {
		d := r.RecordedTime.Format("2006-01-02")
		byDate[d] = append(byDate[d], r)
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	for di, d := range dates {
		if di > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "## %s\n", d)
		for _, r := range byDate[d] {
			unit := r.Unit
			if unit != "" {
				unit = " " + unit
			}
			fmt.Fprintf(w, "- %s: %s%s\n", r.Metric, fmtFloat(r.Amount), unit)
		}
	}
	return nil
}

// ---- exercises --------------------------------------------------------

func renderExercises(w io.Writer, recs cronoapi.ExerciseRecords) error {
	if len(recs) == 0 {
		return emptyMsg(w)
	}
	byDate := map[string][]cronoapi.ExerciseRecord{}
	for _, r := range recs {
		d := r.RecordedTime.Format("2006-01-02")
		byDate[d] = append(byDate[d], r)
	}
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	for di, d := range dates {
		if di > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "## %s\n", d)
		for _, r := range byDate[d] {
			parts := []string{r.Exercise}
			if r.Minutes != 0 {
				parts = append(parts, fmtFloat(r.Minutes)+" min")
			}
			if r.CaloriesBurned != 0 {
				parts = append(parts, fmtFloat(r.CaloriesBurned)+" kcal")
			}
			line := strings.Join(parts, ", ")
			if r.Group != "" {
				line += fmt.Sprintf(" (%s)", r.Group)
			}
			fmt.Fprintf(w, "- %s\n", line)
		}
	}
	return nil
}

// ---- nutrition (daily totals, string-keyed CSV) ----------------------

func renderNutrition(w io.Writer, rows []map[string]string) error {
	if len(rows) == 0 {
		return emptyMsg(w)
	}
	// Sort by Date asc.
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i]["Date"] < rows[j]["Date"]
	})
	for di, row := range rows {
		if di > 0 {
			fmt.Fprintln(w)
		}
		date := row["Date"]
		if date == "" {
			date = "(unknown date)"
		}
		fmt.Fprintf(w, "## %s\n", date)

		keys := make([]string, 0, len(row))
		for k := range row {
			if k == "Date" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := row[k]
			if isZeroish(v) {
				continue
			}
			fmt.Fprintf(w, "- %s: %s\n", k, v)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "_zero-valued nutrients omitted; use --format json for the full row_")
	return nil
}

// isZeroish reports whether a CSV value should be treated as "no data" and
// hidden from the markdown output.  Empty strings, "0", "0.0", "0.00", etc.
// are zeroish; everything else (including "false", "true", arbitrary text)
// is rendered.
func isZeroish(s string) bool {
	if s == "" {
		return true
	}
	// Try numeric: if it parses to 0, it's zeroish.
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil && f == 0 {
		// But only if the entire string was numeric.
		t := strings.TrimSpace(s)
		for _, r := range t {
			if !(r >= '0' && r <= '9') && r != '.' && r != '-' && r != '+' {
				return false
			}
		}
		return true
	}
	return false
}

// ---- notes ------------------------------------------------------------

func renderNotes(w io.Writer, rows []map[string]string) error {
	if len(rows) == 0 {
		return emptyMsg(w)
	}
	dateKey := pickKey(rows[0], "Day", "Date")
	noteKey := pickKey(rows[0], "Note", "Notes", "Comment")
	timeKey := pickKey(rows[0], "Time")

	for di, row := range rows {
		if di > 0 {
			fmt.Fprintln(w)
		}
		date := row[dateKey]
		if date == "" {
			date = "(unknown date)"
		}
		header := "## " + date
		if t := row[timeKey]; t != "" {
			header += " " + t
		}
		fmt.Fprintln(w, header)
		if note := strings.TrimSpace(row[noteKey]); note != "" {
			fmt.Fprintln(w, note)
		} else {
			// Fall back to dumping all non-empty fields if we can't find a Note column.
			for k, v := range row {
				if k == dateKey || k == timeKey || v == "" {
					continue
				}
				fmt.Fprintf(w, "- %s: %s\n", k, v)
			}
		}
	}
	return nil
}

func pickKey(row map[string]string, candidates ...string) string {
	for _, c := range candidates {
		if _, ok := row[c]; ok {
			return c
		}
	}
	return ""
}

