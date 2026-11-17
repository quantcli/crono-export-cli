package cmd

import (
	"encoding/csv"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/quantcli/crono-export-cli/internal/cronoapi"
	"github.com/quantcli/crono-export-cli/internal/exporter"
)

// The csv codec, per the quantcli shared contract §4.
// https://github.com/quantcli/common/blob/main/CONTRACT.md#4-output-format
//
// §4 fixes very little about csv: it is "spreadsheet-friendly", an empty
// result is "no rows", and (§2) timestamps render in the user's local
// zone. The compat/formats bundle adds that the header row is present
// even when there are no rows. Everything else below follows
// withings-export-cli, the family's existing csv implementation, or —
// where that has no precedent either — this CLI's own json codec:
//
//   - Framing is Go's encoding/csv defaults, as in withings: RFC 4180
//     quoting (a field holding a comma, quote, CR or LF is wrapped in
//     double quotes, embedded quotes doubled), "\n" record terminator,
//     UTF-8, no byte-order mark.
//   - One row per JSON array element, in the same order, and one column
//     per JSON key, named exactly as the JSON key. `--format csv` and
//     `--format json` are two views of the same rows, so a column name
//     learned from one works in the other. Unlike markdown, nothing is
//     dropped: zero values are printed.
//   - Column order is the order of the source record: struct field order
//     for servings/biometrics/exercises (which is also the json key
//     order), and Cronometer's own header order for nutrition/notes
//     (whose json objects come from a Go map and so print their keys
//     alphabetically — an order no spreadsheet wants, with Date in the
//     middle).
//   - A cell is the text of the JSON scalar: strings verbatim, numbers
//     in shortest round-trip decimal form (no exponent, as withings'
//     strconv.FormatFloat(v, 'f', -1, 64)), booleans true/false, and
//     JSON null as the empty cell. A cell is never the literal "null".
//   - Timestamps are RFC 3339 with the local offset (§2; same layout
//     withings uses).
//
// Every crono row is flat, so there is no nested-value flattening rule
// here and §4 does not define one.

// renderCSV writes v as csv: a header row, then one row per record.
func renderCSV(w io.Writer, kind recordKind, v any) error {
	cw := csv.NewWriter(w)
	var err error
	switch kind {
	case kindServings:
		recs, _ := v.(cronoapi.ServingRecords)
		err = writeStructCSV(cw, recs)
	case kindBiometrics:
		recs, _ := v.(cronoapi.BiometricRecords)
		err = writeStructCSV(cw, recs)
	case kindExercises:
		recs, _ := v.(cronoapi.ExerciseRecords)
		err = writeStructCSV(cw, recs)
	case kindNutrition, kindNotes:
		t, _ := v.(exporter.Table)
		err = writeTableCSV(cw, kind, t)
	default:
		return fmt.Errorf("renderCSV: unknown kind %d", kind)
	}
	if err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// writeStructCSV writes a slice of flat record structs. Columns are the
// struct's json keys in declaration order — the same names and order
// encoding/json emits — so the header is known from the type alone and
// is written even when recs is empty.
func writeStructCSV[T any](cw *csv.Writer, recs []T) error {
	typ := reflect.TypeFor[T]()
	var header []string
	var fields []int
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		header = append(header, name)
		fields = append(fields, i)
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	row := make([]string, len(fields))
	for _, rec := range recs {
		rv := reflect.ValueOf(rec)
		for j, i := range fields {
			row[j] = csvCell(rv.Field(i).Interface())
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// fallbackTableColumns is the header written for a header-keyed export
// when Cronometer's own header is not known: the window was inverted so
// no request was made, or the response body was empty. The compat
// bundle requires a header row on every csv result, so these name the
// identifying columns this CLI already looks for in each export (see
// renderNutrition and renderNotes). A non-empty result always carries
// Cronometer's full header instead, which is wider.
var fallbackTableColumns = map[recordKind][]string{
	kindNutrition: {"Date"},
	kindNotes:     {"Day", "Note"},
}

// writeTableCSV writes a header-keyed export. Columns come from
// Cronometer's header, in Cronometer's order.
func writeTableCSV(cw *csv.Writer, kind recordKind, t exporter.Table) error {
	cols := t.Columns
	if len(cols) == 0 {
		cols = fallbackTableColumns[kind]
	}
	if err := cw.Write(cols); err != nil {
		return err
	}
	row := make([]string, len(cols))
	for _, r := range t.Rows {
		for i, c := range cols {
			row[i] = csvCell(r[c])
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// csvCell renders one value as the text of its JSON scalar; nil (JSON
// null) is the empty cell.
func csvCell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case time.Time:
		if x.IsZero() {
			// An unparseable upstream date. Converting the zero time
			// to a local zone yields a meaningless pre-standard-time
			// offset, so print it as json does.
			return x.Format(time.RFC3339)
		}
		return x.Local().Format(time.RFC3339)
	default:
		return fmt.Sprint(x)
	}
}
