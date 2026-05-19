// Package cronotest provides a permissive httptest stand-in for
// cronometer.com used by exporter and CLI-binary E2E tests.
//
// It implements the four endpoints the clean-room client touches:
//
//   - GET  /login/           anti-CSRF bootstrap
//   - POST /login            credential submit
//   - POST /cronometer/app   GWT-RPC (authenticate, generateAuthorizationToken, logout)
//   - GET  /export           CSV export, keyed by ?generate=
//
// The cronoapi package's own tests pin the strict wire shape
// (GWT headers, framing strings, CSV parsing). This fake is deliberately
// looser — it accepts any permutation/auth token, returns canned CSV
// bodies, and exists so the exporter wrapper and the CLI binary can
// be exercised end-to-end without hitting the real Cronometer host.
package cronotest

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// Fake is an httptest.Server playing the role of cronometer.com.
type Fake struct {
	Server *httptest.Server

	// Canned CSV bodies served by /export per `generate=` query.
	ServingsCSV     string
	ExercisesCSV    string
	BiometricsCSV   string
	DailySummaryCSV string
	NotesCSV        string

	// Test-observable state.
	LoginPosts     int
	GWTPosts       int
	ExportRequests []string // captured /export ?generate= values

	csrfToken string
	authToken string
	userID    int
}

// New starts an httptest server with sensible defaults and returns the
// Fake. The caller is responsible for closing it (typically via t.Cleanup).
func New() *Fake {
	f := &Fake{
		csrfToken: "abcdef0123456789abcdef0123456789",
		authToken: "11112222333344445555666677778888",
		userID:    7654321,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/login/", f.handleLoginGet)
	mux.HandleFunc("/login", f.handleLoginPost)
	mux.HandleFunc("/cronometer/app", f.handleGWT)
	mux.HandleFunc("/export", f.handleExport)
	f.Server = httptest.NewServer(mux)
	return f
}

// URL returns the base URL of the underlying httptest server.
func (f *Fake) URL() string { return f.Server.URL }

// Close shuts down the underlying httptest server.
func (f *Fake) Close() { f.Server.Close() }

func (f *Fake) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "s1", Path: "/"})
	w.Header().Set("Content-Type", "text/html;charset=UTF-8")
	fmt.Fprintf(w, `<html><body><form><input type="hidden" name="anticsrf" value="%s"/></form></body></html>`, f.csrfToken)
}

func (f *Fake) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	f.LoginPosts++
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if r.PostFormValue("anticsrf") != f.csrfToken {
		w.Header().Set("Content-Type", "application/json;charset=UTF-8")
		_, _ = w.Write([]byte(`{"error":"AntiCSRF Token Invalid"}`))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "auth", Value: "ok", Path: "/"})
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	_, _ = w.Write([]byte(`{"ok":true,"user":"x"}`))
}

func (f *Fake) handleGWT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	f.GWTPosts++
	body, _ := io.ReadAll(r.Body)
	bodyStr := string(body)
	switch {
	case strings.Contains(bodyStr, "|authenticate|"):
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w,
			`//OK[%d,1,2,3,"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","%s","unused"]`,
			f.userID, f.authToken,
		)
	case strings.Contains(bodyStr, "|generateAuthorizationToken|"):
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`//OK[1,["cccccccccccccccccccccccccccccccc"],0,7]`))
	case strings.Contains(bodyStr, "|logout|"):
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`//OK[1,0,7]`))
	default:
		http.Error(w, "unknown GWT method", http.StatusBadRequest)
	}
}

func (f *Fake) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	gen := r.URL.Query().Get("generate")
	f.ExportRequests = append(f.ExportRequests, gen)
	w.Header().Set("Content-Type", "text/csv")
	switch gen {
	case "servings":
		_, _ = io.WriteString(w, f.ServingsCSV)
	case "exercises":
		_, _ = io.WriteString(w, f.ExercisesCSV)
	case "biometrics":
		_, _ = io.WriteString(w, f.BiometricsCSV)
	case "dailySummary":
		_, _ = io.WriteString(w, f.DailySummaryCSV)
	case "notes":
		_, _ = io.WriteString(w, f.NotesCSV)
	default:
		http.Error(w, "unknown generate type", http.StatusBadRequest)
	}
}
