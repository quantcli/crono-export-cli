package cronoapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// fakeCronometer is a hand-rolled httptest stand-in for cronometer.com.
// It implements the 14-exchange storyboard from WIRE_SHAPES.md so the
// clean-room client can be exercised end-to-end without leaving the
// process. All response bodies here are synthesised — no real account
// data and no captured CSV bodies (those were redacted in Phase 3a for
// privacy posture).
type fakeCronometer struct {
	csrfToken      string
	authToken      string
	userID         int
	permutation    string
	postedLoginRaw string
	postedLogout   bool

	// Synthesised CSV bodies the test can override per-endpoint.
	servingsCSV     string
	exercisesCSV    string
	biometricsCSV   string
	dailySummaryCSV string
	notesCSV        string

	t      *testing.T
	server *httptest.Server
}

func newFakeCronometer(t *testing.T) *fakeCronometer {
	t.Helper()
	f := &fakeCronometer{
		csrfToken:   "abcdef0123456789abcdef0123456789",
		authToken:   "11112222333344445555666677778888",
		userID:      7654321,
		permutation: "TESTPERMUTATIONHASH00000000000000",
		t:           t,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/login/", f.handleLoginGet)
	mux.HandleFunc("/login", f.handleLoginPost)
	mux.HandleFunc("/cronometer/app", f.handleGWT)
	mux.HandleFunc("/export", f.handleExport)
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeCronometer) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	// Seed a session cookie so the cookie jar round-trips on subsequent
	// requests, mirroring WIRE_SHAPES.md §(1) "4 Set-Cookie entries".
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "s1", Path: "/"})
	w.Header().Set("Content-Type", "text/html;charset=UTF-8")
	fmt.Fprintf(w, `<html><body><form><input type="hidden" name="anticsrf" value="%s"/></form></body></html>`, f.csrfToken)
}

func (f *fakeCronometer) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	if c, _ := r.Cookie("session"); c == nil {
		http.Error(w, "missing session cookie", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.postedLoginRaw = r.PostForm.Encode()
	if r.PostFormValue("anticsrf") != f.csrfToken {
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = w.Write([]byte(`{"error":"AntiCSRF Token Invalid"}`))
		return
	}
	// Additional Set-Cookie marking the authenticated session.
	http.SetCookie(w, &http.Cookie{Name: "auth", Value: "ok", Path: "/"})
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	_, _ = w.Write([]byte(`{"ok":true,"user":"x"}`))
}

func (f *fakeCronometer) handleGWT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	if got := r.Header.Get("Content-Type"); got != gwtContentType {
		f.t.Errorf("/cronometer/app: Content-Type = %q, want %q", got, gwtContentType)
	}
	if got := r.Header.Get("X-Gwt-Module-Base"); got != gwtModuleBase {
		f.t.Errorf("/cronometer/app: X-Gwt-Module-Base = %q, want %q", got, gwtModuleBase)
	}
	if got := r.Header.Get("X-Gwt-Permutation"); got != f.permutation {
		f.t.Errorf("/cronometer/app: X-Gwt-Permutation = %q, want %q", got, f.permutation)
	}
	body, _ := io.ReadAll(r.Body)
	bodyStr := string(body)
	switch {
	case strings.Contains(bodyStr, "|authenticate|"):
		// Synthesise an authenticate response: leading userID then a
		// scatter of quoted hex strings ending in the session token.
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w,
			`//OK[%d,1,2,3,"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","%s","unused"]`,
			f.userID, f.authToken,
		)
	case strings.Contains(bodyStr, "|generateAuthorizationToken|"):
		if !strings.Contains(bodyStr, f.authToken) {
			f.t.Errorf("generateAuthorizationToken: request body missing session auth token")
		}
		// One-shot nonce per WIRE_SHAPES.md §(4) response shape.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`//OK[1,["cccccccccccccccccccccccccccccccc"],0,7]`))
	case strings.Contains(bodyStr, "|logout|"):
		f.postedLogout = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`//OK[1,0,7]`))
	default:
		f.t.Errorf("/cronometer/app: unknown method in body: %q", truncate(bodyStr, 120))
		http.Error(w, "unknown GWT method", http.StatusBadRequest)
	}
}

func (f *fakeCronometer) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	for _, want := range []string{"start", "end", "generate", "nonce"} {
		if q.Get(want) == "" {
			f.t.Errorf("/export: missing query param %q (url=%q)", want, r.URL.RawQuery)
		}
	}
	w.Header().Set("Content-Type", "text/csv")
	switch q.Get("generate") {
	case exportGenServings:
		_, _ = io.WriteString(w, f.servingsCSV)
	case exportGenExercises:
		_, _ = io.WriteString(w, f.exercisesCSV)
	case exportGenBiometrics:
		_, _ = io.WriteString(w, f.biometricsCSV)
	case exportGenDailySummary:
		_, _ = io.WriteString(w, f.dailySummaryCSV)
	case exportGenNotes:
		_, _ = io.WriteString(w, f.notesCSV)
	default:
		http.Error(w, "unknown generate type", http.StatusBadRequest)
	}
}

// ----- tests --------------------------------------------------------

func newTestClient(t *testing.T, f *fakeCronometer) *Client {
	t.Helper()
	c := NewClient(nil)
	c.SetBaseURL(f.server.URL)
	c.SetPermutation(f.permutation)
	return c
}

func TestLoginAndLogout(t *testing.T) {
	f := newFakeCronometer(t)
	c := newTestClient(t, f)

	if err := c.Login(context.Background(), "alice", "p@ssw0rd"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if c.UserID() != f.userID {
		t.Errorf("UserID = %d, want %d", c.UserID(), f.userID)
	}
	if c.AuthToken() != f.authToken {
		t.Errorf("AuthToken = %q, want %q", c.AuthToken(), f.authToken)
	}

	// /login body must contain the round-tripped CSRF token and
	// url-encoded credentials (WIRE_SHAPES.md §(2)).
	posted, err := url.ParseQuery(f.postedLoginRaw)
	if err != nil {
		t.Fatalf("parse posted login form: %v", err)
	}
	if got := posted.Get("anticsrf"); got != f.csrfToken {
		t.Errorf("posted anticsrf = %q, want %q", got, f.csrfToken)
	}
	if got := posted.Get("username"); got != "alice" {
		t.Errorf("posted username = %q, want %q", got, "alice")
	}
	if got := posted.Get("password"); got != "p@ssw0rd" {
		t.Errorf("posted password = %q, want %q", got, "p@ssw0rd")
	}

	if err := c.Logout(context.Background()); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if !f.postedLogout {
		t.Error("Logout did not call the GWT logout RPC")
	}
	if c.AuthToken() != "" || c.UserID() != 0 {
		t.Errorf("after Logout: AuthToken=%q UserID=%d, want empty/zero", c.AuthToken(), c.UserID())
	}
	// Logout with no auth token must be a no-op.
	if err := c.Logout(context.Background()); err != nil {
		t.Errorf("Logout (no-op): %v", err)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	f := newFakeCronometer(t)
	c := newTestClient(t, f)
	// Force the CSRF mismatch path on the fake.
	f.csrfToken = "expected"
	c2 := newTestClient(t, f)
	// Hand-roll a login that posts the wrong CSRF — easiest is to
	// reset the in-memory expected value after the GET fires; here we
	// just call submitLogin directly with a wrong token to cover the
	// `"error"` branch.
	_ = c // unused for this branch
	if err := c2.submitLogin(context.Background(), "alice", "x", "wrong-csrf"); err == nil {
		t.Fatal("submitLogin with wrong CSRF: expected error, got nil")
	}
}

func TestExportServingsParsedWithLocation(t *testing.T) {
	f := newFakeCronometer(t)
	// Synthesised CSV covering the documented field set + a couple of
	// nutrient columns. Real Cronometer CSV column headers were not
	// preserved in fixtures (privacy posture); this is a hand-written
	// sample shaped per docs/cronometer-protocol.md "Record types we
	// read".
	f.servingsCSV = strings.Join([]string{
		`Day,Group,Food Name,Amount,Category,Energy (kcal),Protein (g),Vitamin B12 (µg),Vitamin D (IU)`,
		`2026-05-04,Breakfast,"Oats, rolled",50.00 g,Cereal,194,6.5,0,0`,
		`2026-05-04,Breakfast,"Milk, whole",250 ml,Dairy,150,8,1.2,120`,
	}, "\n")

	c := newTestClient(t, f)
	if err := c.Login(context.Background(), "alice", "x"); err != nil {
		t.Fatal(err)
	}
	defer c.Logout(context.Background())

	start := time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local)
	end := time.Date(2026, 5, 11, 0, 0, 0, 0, time.Local)
	recs, err := c.ExportServingsParsedWithLocation(context.Background(), start, end, time.Local)
	if err != nil {
		t.Fatalf("ExportServings: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}

	want := recs[0]
	if want.FoodName != "Oats, rolled" {
		t.Errorf("recs[0].FoodName = %q, want %q", want.FoodName, "Oats, rolled")
	}
	if want.Group != "Breakfast" {
		t.Errorf("recs[0].Group = %q, want %q", want.Group, "Breakfast")
	}
	if want.QuantityValue != 50.0 || want.QuantityUnits != "g" {
		t.Errorf("recs[0] quantity = (%v, %q), want (50, \"g\")", want.QuantityValue, want.QuantityUnits)
	}
	if want.Category != "Cereal" {
		t.Errorf("recs[0].Category = %q, want %q", want.Category, "Cereal")
	}
	if want.EnergyKcal != 194 || want.ProteinG != 6.5 {
		t.Errorf("recs[0] nutrients: EnergyKcal=%v ProteinG=%v", want.EnergyKcal, want.ProteinG)
	}
	if recs[1].B12Ug != 1.2 {
		t.Errorf("recs[1].B12Ug = %v, want 1.2", recs[1].B12Ug)
	}
	if recs[1].VitaminDIU != 120 {
		t.Errorf("recs[1].VitaminDIU = %v, want 120", recs[1].VitaminDIU)
	}
	if got := recs[0].RecordedTime.Format("2006-01-02"); got != "2026-05-04" {
		t.Errorf("recs[0].RecordedTime date = %q, want %q", got, "2026-05-04")
	}
	if recs[0].RecordedTime.Location() != time.Local {
		t.Errorf("recs[0].RecordedTime zone = %v, want time.Local", recs[0].RecordedTime.Location())
	}
}

func TestExportExercisesParsedWithLocation(t *testing.T) {
	f := newFakeCronometer(t)
	f.exercisesCSV = strings.Join([]string{
		`Day,Exercise,Minutes,Calories Burned,Category`,
		`2026-05-05,Running,30,310,Cardio`,
		`2026-05-06,Yoga,45,150,Flexibility`,
	}, "\n")

	c := newTestClient(t, f)
	if err := c.Login(context.Background(), "u", "p"); err != nil {
		t.Fatal(err)
	}
	defer c.Logout(context.Background())

	recs, err := c.ExportExercisesParsedWithLocation(context.Background(),
		time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local),
		time.Date(2026, 5, 11, 0, 0, 0, 0, time.Local),
		time.Local,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0].Exercise != "Running" || recs[0].Minutes != 30 || recs[0].CaloriesBurned != 310 || recs[0].Group != "Cardio" {
		t.Errorf("recs[0] = %+v", recs[0])
	}
}

func TestExportBiometricsParsedWithLocation(t *testing.T) {
	f := newFakeCronometer(t)
	f.biometricsCSV = strings.Join([]string{
		`Day,Metric,Amount,Unit`,
		`2026-05-04,Weight,180.5,lbs`,
		`2026-05-04,Body Fat,18.2,%`,
	}, "\n")

	c := newTestClient(t, f)
	if err := c.Login(context.Background(), "u", "p"); err != nil {
		t.Fatal(err)
	}
	defer c.Logout(context.Background())

	recs, err := c.ExportBiometricRecordsParsedWithLocation(context.Background(),
		time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local),
		time.Date(2026, 5, 11, 0, 0, 0, 0, time.Local),
		time.Local,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0].Metric != "Weight" || recs[0].Amount != 180.5 || recs[0].Unit != "lbs" {
		t.Errorf("recs[0] = %+v", recs[0])
	}
	if recs[1].Metric != "Body Fat" || recs[1].Unit != "%" {
		t.Errorf("recs[1] = %+v", recs[1])
	}
}

func TestExportDailyNutritionAndNotesAreRawCSV(t *testing.T) {
	f := newFakeCronometer(t)
	f.dailySummaryCSV = "Date,Calories,Protein\n2026-05-04,1800,90\n"
	f.notesCSV = "Day,Note\n2026-05-04,Slept poorly\n"

	c := newTestClient(t, f)
	if err := c.Login(context.Background(), "u", "p"); err != nil {
		t.Fatal(err)
	}
	defer c.Logout(context.Background())

	gotNut, err := c.ExportDailyNutrition(context.Background(),
		time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local),
		time.Date(2026, 5, 11, 0, 0, 0, 0, time.Local),
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotNut != f.dailySummaryCSV {
		t.Errorf("ExportDailyNutrition body mismatch:\n got %q\nwant %q", gotNut, f.dailySummaryCSV)
	}

	gotNotes, err := c.ExportNotes(context.Background(),
		time.Date(2026, 5, 4, 0, 0, 0, 0, time.Local),
		time.Date(2026, 5, 11, 0, 0, 0, 0, time.Local),
	)
	if err != nil {
		t.Fatal(err)
	}
	if gotNotes != f.notesCSV {
		t.Errorf("ExportNotes body mismatch:\n got %q\nwant %q", gotNotes, f.notesCSV)
	}
}

func TestExportRequiresLogin(t *testing.T) {
	f := newFakeCronometer(t)
	c := newTestClient(t, f)
	_, err := c.ExportServingsParsedWithLocation(context.Background(),
		time.Now(), time.Now(), time.Local)
	if err == nil {
		t.Fatal("expected error when calling export without Login, got nil")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("error = %v, want %q", err, "not logged in")
	}
}

func TestHeaderToNutrientField(t *testing.T) {
	cases := []struct {
		header, want string
	}{
		{"Energy (kcal)", "EnergyKcal"},
		{"Protein (g)", "ProteinG"},
		{"Vitamin B12 (µg)", "B12Ug"},
		{"Vitamin B12 (ug)", "B12Ug"},
		{"Vitamin D (IU)", "VitaminDIU"},
		{"Net Carbs (g)", "NetCarbsG"},
		{"Trans-Fats (g)", "TransFatsG"},
		{"Cholesterol (mg)", "CholesterolMg"},
		{"NoUnitColumn", ""},
		{"Food Name", ""},
		{"Energy (joules)", ""}, // unrecognised unit
	}
	for _, tc := range cases {
		got := headerToNutrientField(tc.header)
		if got != tc.want {
			t.Errorf("headerToNutrientField(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestParseAuthenticateResponse(t *testing.T) {
	body := `//OK[1234567,1,2,3,"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","SESSION_TOKEN_HERE_NOT_HEX","deadbeefdeadbeefdeadbeefdeadbeef","tail"]`
	uid, tok, err := parseAuthenticateResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if uid != 1234567 {
		t.Errorf("uid = %d, want 1234567", uid)
	}
	if tok != "deadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("token = %q, want last 32-hex string", tok)
	}
}

func TestParseAuthorizationTokenResponse(t *testing.T) {
	body := `//OK[1,["cafebabecafebabecafebabecafebabe"],0,7]`
	nonce, err := parseAuthorizationTokenResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if nonce != "cafebabecafebabecafebabecafebabe" {
		t.Errorf("nonce = %q", nonce)
	}
}

func TestServingsCSVWithBOM(t *testing.T) {
	bom := "\xef\xbb\xbf"
	raw := bom + "Day,Food Name,Energy (kcal)\n2026-05-04,Apple,52\n"
	recs, err := parseServingsCSV(raw, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].FoodName != "Apple" || recs[0].EnergyKcal != 52 {
		t.Errorf("BOM-handling failed: %+v", recs)
	}
}

func TestEmptyCSVIsHeaderOnly(t *testing.T) {
	// Mimics WIRE_SHAPES.md §(5/7/9) "header-only" responses for
	// exercises/biometrics/notes when the window holds no data.
	recs, err := parseServingsCSV("Day,Food Name\n", time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 records from header-only CSV, got %d", len(recs))
	}
	if _, err := parseServingsCSV("", time.UTC); err != nil {
		t.Errorf("parseServingsCSV(\"\"): %v", err)
	}
}

func TestGWTBodyShapesAreLiteralFromSpec(t *testing.T) {
	// Pin the exact GWT-RPC framing strings so a future drift gets
	// caught here rather than at runtime.
	perm := "ABCDEF0123456789ABCDEF0123456789"
	got := authenticateBody(perm, -300)
	want := "7|0|5|https://cronometer.com/cronometer/|ABCDEF0123456789ABCDEF0123456789|com.cronometer.shared.rpc.CronometerService|authenticate|java.lang.Integer/3438268394|1|2|3|4|1|5|5|-300|"
	if got != want {
		t.Errorf("authenticateBody:\n got %q\nwant %q", got, want)
	}

	got = generateAuthorizationTokenBody(perm, "11112222333344445555666677778888", 1234567, 3600)
	want = "7|0|8|https://cronometer.com/cronometer/|ABCDEF0123456789ABCDEF0123456789|com.cronometer.shared.rpc.CronometerService|generateAuthorizationToken|java.lang.String/2004016611|I|com.cronometer.shared.user.AuthScope/2065601159|11112222333344445555666677778888|1|2|3|4|4|5|6|6|7|8|1234567|3600|7|2|"
	if got != want {
		t.Errorf("generateAuthorizationTokenBody:\n got %q\nwant %q", got, want)
	}

	got = logoutBody(perm, "11112222333344445555666677778888")
	want = "7|0|6|https://cronometer.com/cronometer/|ABCDEF0123456789ABCDEF0123456789|com.cronometer.shared.rpc.CronometerService|logout|java.lang.String/2004016611|11112222333344445555666677778888|1|2|3|4|1|5|6|"
	if got != want {
		t.Errorf("logoutBody:\n got %q\nwant %q", got, want)
	}
}
