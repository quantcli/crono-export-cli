package cmd

import "testing"

func TestStrippedSuffix(t *testing.T) {
	cases := []struct {
		field, wantName, wantUnit string
	}{
		{"EnergyKcal", "Energy", "kcal"},
		{"CarbsG", "Carbs", "g"},
		{"CalciumMg", "Calcium", "mg"},
		{"VitaminAUg", "VitaminA", "µg"},
		{"VitaminDIU", "VitaminD", "IU"},
		{"PolyunsaturatedG", "Polyunsaturated", "g"},
		{"Group", "Group", ""},
		{"G", "G", ""},
	}
	for _, c := range cases {
		gotName, gotUnit := strippedSuffix(c.field)
		if gotName != c.wantName || gotUnit != c.wantUnit {
			t.Errorf("strippedSuffix(%q) = (%q, %q), want (%q, %q)",
				c.field, gotName, gotUnit, c.wantName, c.wantUnit)
		}
	}
}
