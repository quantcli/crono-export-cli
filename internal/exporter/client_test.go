package exporter

import (
	"reflect"
	"testing"
)

func TestCoerceCSVValue(t *testing.T) {
	cases := []struct {
		in   string
		want any
	}{
		{"", nil},
		{"   ", nil},
		{"1.5", 1.5},
		{"100", 100.0},
		{"-3", -3.0},
		{"true", true},
		{"FALSE", false},
		{"Slept well", "Slept well"},
		{"2026-05-04", "2026-05-04"},
	}
	for _, c := range cases {
		got := coerceCSVValue(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("coerceCSVValue(%q) = %v (%T), want %v (%T)",
				c.in, got, got, c.want, c.want)
		}
	}
}

func TestCsvToJSON(t *testing.T) {
	raw := "Date,Calories,Protein,Completed\n2026-05-04,1800,90,true\n2026-05-05,,75.5,false\n"
	got, err := csvToJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
	row0 := got[0]
	if row0["Date"] != "2026-05-04" || row0["Calories"] != 1800.0 || row0["Protein"] != 90.0 || row0["Completed"] != true {
		t.Errorf("row0 = %+v", row0)
	}
	row1 := got[1]
	if row1["Calories"] != nil || row1["Protein"] != 75.5 || row1["Completed"] != false {
		t.Errorf("row1 = %+v", row1)
	}
}
