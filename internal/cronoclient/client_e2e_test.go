package cronoclient_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/quantcli/crono-export-cli/internal/cronoapi"
	"github.com/quantcli/crono-export-cli/internal/cronoclient"
	"github.com/quantcli/crono-export-cli/internal/cronotest"
)

// withFake spins up a cronotest.Fake and wires the env/base-URL hook
// that cronoclient.NewLoggedIn honors.
func withFake(t *testing.T) *cronotest.Fake {
	t.Helper()
	f := cronotest.New()
	t.Cleanup(f.Close)
	t.Setenv("CRONOMETER_USERNAME", "alice@example.com")
	t.Setenv("CRONOMETER_PASSWORD", "p@ssw0rd")
	t.Setenv("CRONOMETER_BASE_URL", f.URL())
	t.Setenv("CRONOMETER_NO_CACHE", "1")
	return f
}

// fixedRange is a stable test window; cronoclient just forwards Start/End
// to cronoapi, so the dates don't need to match anything in the canned CSV.
func fixedRange() cronoclient.DateRange {
	return cronoclient.DateRange{
		Start: time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local),
		End:   time.Date(2026, 5, 11, 0, 0, 0, 0, time.Local),
	}
}

func TestNewLoggedIn_RequiresCredentials(t *testing.T) {
	t.Setenv("CRONOMETER_USERNAME", "")
	t.Setenv("CRONOMETER_PASSWORD", "")
	if _, err := cronoclient.NewLoggedIn(context.Background()); err == nil {
		t.Fatal("expected error when env credentials missing")
	}
}

func TestNewLoggedIn_HonorsBaseURLEnv(t *testing.T) {
	f := withFake(t)
	c, err := cronoclient.NewLoggedIn(context.Background())
	if err != nil {
		t.Fatalf("NewLoggedIn: %v", err)
	}
	defer c.Logout()
	if f.LoginPosts != 1 {
		t.Errorf("LoginPosts = %d, want 1 (BASE_URL hook may not be wired)", f.LoginPosts)
	}
}

func TestClient_Servings_E2E(t *testing.T) {
	f := withFake(t)
	f.ServingsCSV = strings.Join([]string{
		`Day,Group,Food Name,Amount,Category,Energy (kcal),Protein (g)`,
		`2026-05-04,Breakfast,"Oats, rolled",50.00 g,Cereal,194,6.5`,
		`2026-05-04,Breakfast,"Milk, whole",250 ml,Dairy,150,8`,
	}, "\n")

	c, err := cronoclient.NewLoggedIn(context.Background())
	if err != nil {
		t.Fatalf("NewLoggedIn: %v", err)
	}
	defer c.Logout()

	got, err := c.Servings(context.Background(), fixedRange())
	if err != nil {
		t.Fatalf("Servings: %v", err)
	}
	recs, ok := got.(cronoapi.ServingRecords)
	if !ok {
		t.Fatalf("Servings returned %T, want cronoapi.ServingRecords", got)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0].FoodName != "Oats, rolled" || recs[0].EnergyKcal != 194 || recs[0].ProteinG != 6.5 {
		t.Errorf("recs[0] = %+v", recs[0])
	}
	if recs[1].FoodName != "Milk, whole" || recs[1].QuantityValue != 250 {
		t.Errorf("recs[1] = %+v", recs[1])
	}
}

func TestClient_Exercises_E2E(t *testing.T) {
	f := withFake(t)
	f.ExercisesCSV = strings.Join([]string{
		`Day,Exercise,Minutes,Calories Burned,Category`,
		`2026-05-05,Running,30,310,Cardio`,
		`2026-05-06,Yoga,45,150,Flexibility`,
	}, "\n")

	c, err := cronoclient.NewLoggedIn(context.Background())
	if err != nil {
		t.Fatalf("NewLoggedIn: %v", err)
	}
	defer c.Logout()

	got, err := c.Exercises(context.Background(), fixedRange())
	if err != nil {
		t.Fatalf("Exercises: %v", err)
	}
	recs, ok := got.(cronoapi.ExerciseRecords)
	if !ok {
		t.Fatalf("Exercises returned %T, want cronoapi.ExerciseRecords", got)
	}
	if len(recs) != 2 || recs[0].Exercise != "Running" || recs[0].Minutes != 30 {
		t.Fatalf("unexpected exercises: %+v", recs)
	}
}

func TestClient_Biometrics_E2E(t *testing.T) {
	f := withFake(t)
	f.BiometricsCSV = strings.Join([]string{
		`Day,Metric,Amount,Unit`,
		`2026-05-04,Weight,180.5,lbs`,
		`2026-05-04,Body Fat,18.2,%`,
	}, "\n")

	c, err := cronoclient.NewLoggedIn(context.Background())
	if err != nil {
		t.Fatalf("NewLoggedIn: %v", err)
	}
	defer c.Logout()

	got, err := c.Biometrics(context.Background(), fixedRange())
	if err != nil {
		t.Fatalf("Biometrics: %v", err)
	}
	recs, ok := got.(cronoapi.BiometricRecords)
	if !ok {
		t.Fatalf("Biometrics returned %T, want cronoapi.BiometricRecords", got)
	}
	if len(recs) != 2 || recs[0].Metric != "Weight" || recs[1].Unit != "%" {
		t.Fatalf("unexpected biometrics: %+v", recs)
	}
}

func TestClient_Nutrition_CSVToObjects_E2E(t *testing.T) {
	f := withFake(t)
	f.DailySummaryCSV = "Date,Calories,Protein\n2026-05-04,1800,90\n2026-05-05,2000,110\n"

	c, err := cronoclient.NewLoggedIn(context.Background())
	if err != nil {
		t.Fatalf("NewLoggedIn: %v", err)
	}
	defer c.Logout()

	got, err := c.Nutrition(context.Background(), fixedRange())
	if err != nil {
		t.Fatalf("Nutrition: %v", err)
	}
	rows, ok := got.([]map[string]any)
	if !ok {
		t.Fatalf("Nutrition returned %T, want []map[string]any", got)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0]["Calories"] != 1800.0 || rows[0]["Protein"] != 90.0 {
		t.Errorf("rows[0] = %+v", rows[0])
	}
	if rows[1]["Date"] != "2026-05-05" {
		t.Errorf("rows[1] = %+v", rows[1])
	}
}

func TestClient_Notes_CSVToObjects_E2E(t *testing.T) {
	f := withFake(t)
	f.NotesCSV = "Day,Note\n2026-05-04,Slept poorly\n"

	c, err := cronoclient.NewLoggedIn(context.Background())
	if err != nil {
		t.Fatalf("NewLoggedIn: %v", err)
	}
	defer c.Logout()

	got, err := c.Notes(context.Background(), fixedRange())
	if err != nil {
		t.Fatalf("Notes: %v", err)
	}
	rows, ok := got.([]map[string]any)
	if !ok {
		t.Fatalf("Notes returned %T, want []map[string]any", got)
	}
	if len(rows) != 1 || rows[0]["Note"] != "Slept poorly" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestClient_AllExportsInOneSession(t *testing.T) {
	f := withFake(t)
	f.ServingsCSV = "Day,Food Name\n2026-05-04,Apple\n"
	f.ExercisesCSV = "Day,Exercise,Minutes,Calories Burned,Category\n"
	f.BiometricsCSV = "Day,Metric,Amount,Unit\n"
	f.DailySummaryCSV = "Date,Calories\n2026-05-04,1800\n"
	f.NotesCSV = "Day,Note\n"

	c, err := cronoclient.NewLoggedIn(context.Background())
	if err != nil {
		t.Fatalf("NewLoggedIn: %v", err)
	}
	defer c.Logout()
	rng := fixedRange()
	if _, err := c.Servings(context.Background(), rng); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exercises(context.Background(), rng); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Biometrics(context.Background(), rng); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Nutrition(context.Background(), rng); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Notes(context.Background(), rng); err != nil {
		t.Fatal(err)
	}
	wantSeen := []string{"servings", "exercises", "biometrics", "dailySummary", "notes"}
	if len(f.ExportRequests) != len(wantSeen) {
		t.Fatalf("ExportRequests = %v, want %v", f.ExportRequests, wantSeen)
	}
	for i, w := range wantSeen {
		if f.ExportRequests[i] != w {
			t.Errorf("ExportRequests[%d] = %q, want %q", i, f.ExportRequests[i], w)
		}
	}
}
