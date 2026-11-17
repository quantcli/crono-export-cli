package cmd

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/quantcli/crono-export-cli/internal/cronoapi"
	"github.com/quantcli/crono-export-cli/internal/exporter"
)

func renderCSVString(t *testing.T, kind recordKind, v any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := renderCSV(&buf, kind, v); err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	return buf.String()
}

func parseCSV(t *testing.T, s string) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(s))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("output is not parseable csv: %v\n%s", err, s)
	}
	return rows
}

// jsonKeys returns the keys of v's JSON object in the order
// encoding/json emits them.
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil { // opening {
		t.Fatal(err)
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

// The header of every typed export is exactly the JSON keys of one row,
// in JSON order — so csv and json can never drift apart on naming.
func TestCSV_StructHeaderMatchesJSONKeys(t *testing.T) {
	cases := []struct {
		name string
		kind recordKind
		recs any
		one  any
	}{
		{"servings", kindServings, cronoapi.ServingRecords{}, cronoapi.ServingRecord{}},
		{"biometrics", kindBiometrics, cronoapi.BiometricRecords{}, cronoapi.BiometricRecord{}},
		{"exercises", kindExercises, cronoapi.ExerciseRecords{}, cronoapi.ExerciseRecord{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := parseCSV(t, renderCSVString(t, c.kind, c.recs))
			if len(rows) != 1 {
				t.Fatalf("empty result: got %d lines, want exactly the header", len(rows))
			}
			if want := jsonKeys(t, c.one); !reflect.DeepEqual(rows[0], want) {
				t.Errorf("header = %v\nwant JSON keys %v", rows[0], want)
			}
		})
	}
}

// A nil slice (as opposed to an empty one) must still produce a header.
func TestCSV_NilRecordsStillHaveHeader(t *testing.T) {
	got := renderCSVString(t, kindBiometrics, cronoapi.BiometricRecords(nil))
	if got != "RecordedTime,Metric,Amount,Unit\n" {
		t.Errorf("got %q", got)
	}
}

// Exact bytes: pins the RFC 4180 quoting rules and the "\n" terminator.
func TestCSV_QuotingGolden(t *testing.T) {
	day := time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local)
	recs := cronoapi.BiometricRecords{
		{RecordedTime: day, Metric: "Weight", Amount: 81.5, Unit: "kg"},
		{RecordedTime: day, Metric: "Blood Pressure, systolic", Amount: 120, Unit: "mmHg"},
		{RecordedTime: day, Metric: `Waist "relaxed"`, Amount: 0, Unit: ""},
		{RecordedTime: day, Metric: "line one\nline two", Amount: -0.25, Unit: "µg/dL"},
		{RecordedTime: day, Metric: "体重 🏋️ café", Amount: 1e21, Unit: " padded "},
	}
	ts := day.Format(time.RFC3339)
	want := "RecordedTime,Metric,Amount,Unit\n" +
		ts + ",Weight,81.5,kg\n" +
		ts + `,"Blood Pressure, systolic",120,mmHg` + "\n" +
		ts + `,"Waist ""relaxed""",0,` + "\n" +
		ts + `,"line one` + "\n" + `line two",-0.25,µg/dL` + "\n" +
		ts + ",体重 🏋️ café,1000000000000000000000,\" padded \"\n"
	if got := renderCSVString(t, kindBiometrics, recs); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Awkward values survive a write/read round trip unchanged.
func TestCSV_AwkwardValuesRoundTrip(t *testing.T) {
	awkward := []string{
		"plain",
		"comma, inside",
		`quote " inside`,
		`""`,
		"newline\ninside",
		"crlf\r\ninside",
		"tab\tinside",
		" leading and trailing ",
		"=SUM(A1:A2)",
		"naïve café 日本語 🍎",
		"",
	}
	var recs cronoapi.ExerciseRecords
	for _, s := range awkward {
		recs = append(recs, cronoapi.ExerciseRecord{Exercise: s, Group: s, Minutes: 30})
	}
	rows := parseCSV(t, renderCSVString(t, kindExercises, recs))
	if len(rows) != len(awkward)+1 {
		t.Fatalf("got %d rows, want %d", len(rows), len(awkward)+1)
	}
	for i, s := range awkward {
		// encoding/csv's reader folds \r\n inside a quoted field to \n;
		// that is the reader's doing, the writer emitted the bytes as-is.
		want := strings.ReplaceAll(s, "\r\n", "\n")
		row := rows[i+1]
		if row[1] != want || row[4] != want {
			t.Errorf("row %d: Exercise=%q Group=%q, want %q", i, row[1], row[4], want)
		}
	}
	if !strings.Contains(renderCSVString(t, kindExercises, recs), "crlf\r\ninside") {
		t.Error("writer should emit an embedded CRLF verbatim inside the quoted field")
	}
}

func TestCSV_Table(t *testing.T) {
	tbl := exporter.Table{
		Columns: []string{"Date", "Energy (kcal)", "Protein (g)", "Completed", "Comment"},
		Rows: []map[string]any{
			{"Date": "2026-05-04", "Energy (kcal)": 1800.0, "Protein (g)": 90.25, "Completed": true, "Comment": "ate out, \"big\" lunch"},
			{"Date": "2026-05-05", "Energy (kcal)": nil, "Protein (g)": 0.0, "Completed": false, "Comment": nil},
			// A short upstream row: keys present but null, or absent entirely.
			{"Date": "2026-05-06"},
		},
	}
	want := "Date,Energy (kcal),Protein (g),Completed,Comment\n" +
		`2026-05-04,1800,90.25,true,"ate out, ""big"" lunch"` + "\n" +
		"2026-05-05,,0,false,\n" +
		"2026-05-06,,,,\n"
	if got := renderCSVString(t, kindNutrition, tbl); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Null is an empty cell, never the word "null", and is distinguishable
// from a real zero.
func TestCSV_NullIsEmptyCellNotZero(t *testing.T) {
	tbl := exporter.Table{
		Columns: []string{"Date", "Weight"},
		Rows:    []map[string]any{{"Date": "d1", "Weight": nil}, {"Date": "d2", "Weight": 0.0}},
	}
	rows := parseCSV(t, renderCSVString(t, kindNutrition, tbl))
	if rows[1][1] != "" || rows[2][1] != "0" {
		t.Errorf("null cell = %q (want empty), zero cell = %q (want 0)", rows[1][1], rows[2][1])
	}
}

func TestCSV_EmptyTable(t *testing.T) {
	// Cronometer answered with a header and no rows: keep its header.
	withHeader := exporter.Table{Columns: []string{"Day", "Group", "Note"}, Rows: []map[string]any{}}
	if got := renderCSVString(t, kindNotes, withHeader); got != "Day,Group,Note\n" {
		t.Errorf("header-only table: got %q", got)
	}
	// No header known (inverted window, no request made): fallback header.
	for kind, want := range map[recordKind]string{
		kindNutrition: "Date\n",
		kindNotes:     "Day,Note\n",
	} {
		if got := renderCSVString(t, kind, emptyValueFor(kind)); got != want {
			t.Errorf("kind %d with unknown header: got %q, want %q", kind, got, want)
		}
	}
}

func TestCSVCell(t *testing.T) {
	est := time.FixedZone("EST", -5*3600)
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"", ""},
		{"text", "text"},
		{true, "true"},
		{false, "false"},
		{0.0, "0"},
		{100.0, "100"},
		{1.5, "1.5"},
		{-3.0, "-3"},
		{0.30000000000000004, "0.30000000000000004"}, // shortest round-trip, like json
		{1e-7, "0.0000001"},                          // never exponent notation
		{123456789012.0, "123456789012"},
		{time.Time{}, "0001-01-01T00:00:00Z"},
		{time.Date(2026, 5, 4, 12, 0, 0, 0, est), time.Date(2026, 5, 4, 12, 0, 0, 0, est).Local().Format(time.RFC3339)},
	}
	for _, c := range cases {
		if got := csvCell(c.in); got != c.want {
			t.Errorf("csvCell(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}
