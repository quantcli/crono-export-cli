package cronoapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultBaseURL is the production Cronometer host.
const DefaultBaseURL = "https://cronometer.com"

const (
	defaultTimeout = 60 * time.Second
	userAgentHdr   = "crono-export-cli (clean-room; https://github.com/quantcli/crono-export-cli)"
)

// Client is an authenticated Cronometer session. Construct it with
// NewClient, then call Login. After Login the client holds the GWT
// session auth token and user ID and can call any of the Export*
// methods. Logout tears down the session, best-effort.
//
// Client is not safe for concurrent use by multiple goroutines.
type Client struct {
	HTTPClient *http.Client

	baseURL     string
	permutation string
	userAgent   string

	userID    int
	authToken string
}

// NewClient returns a fresh Client. If httpClient is nil a default
// client with a 60-second timeout and a cookie jar is constructed.
// If httpClient is non-nil but has no Jar set, a cookie jar is
// installed so session cookies round-trip across requests.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		jar, _ := cookiejar.New(nil)
		httpClient = &http.Client{
			Jar:     jar,
			Timeout: defaultTimeout,
		}
	} else if httpClient.Jar == nil {
		jar, _ := cookiejar.New(nil)
		httpClient.Jar = jar
	}
	return &Client{
		HTTPClient:  httpClient,
		baseURL:     DefaultBaseURL,
		permutation: DefaultGWTPermutation,
		userAgent:   userAgentHdr,
	}
}

// SetBaseURL overrides the Cronometer host. Intended for tests.
func (c *Client) SetBaseURL(u string) { c.baseURL = strings.TrimRight(u, "/") }

// SetPermutation overrides the GWT permutation hash sent on
// /cronometer/app calls. Useful when Cronometer rotates their deploy
// hash and we need to point at the new value without a code release.
func (c *Client) SetPermutation(p string) { c.permutation = p }

// AuthToken returns the GWT session auth token from the most recent
// successful Login. Empty before Login or after Logout. Exposed for
// debugging; not normally needed by callers.
func (c *Client) AuthToken() string { return c.authToken }

// UserID returns the Cronometer user ID from the most recent
// successful Login. Zero before Login or after Logout.
func (c *Client) UserID() int { return c.userID }

// anticsrfRe extracts the hidden anti-CSRF token from the login HTML.
// WIRE_SHAPES.md §(1).
var anticsrfRe = regexp.MustCompile(`name="anticsrf"\s+value="([^"]+)"`)

// fetchAntiCSRF performs the anonymous GET /login/ that bootstraps the
// session cookie jar and returns the anti-CSRF form token embedded in
// the response HTML.
func (c *Client) fetchAntiCSRF(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/login/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET /login/: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET /login/: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read /login/ body: %w", err)
	}
	m := anticsrfRe.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("anti-CSRF token not found in /login/ response")
	}
	return string(m[1]), nil
}

// submitLogin posts the credential form to /login. Cronometer responds
// with a small JSON body and additional Set-Cookie entries on success.
// The cookie jar on c.HTTPClient picks those up automatically. We do
// not parse the JSON success body — its only documented failure mode
// is `{"error":"AntiCSRF Token Invalid"}`, which we surface explicitly.
func (c *Client) submitLogin(ctx context.Context, username, password, csrfToken string) error {
	form := url.Values{}
	form.Set("anticsrf", csrfToken)
	form.Set("password", password)
	form.Set("username", username)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST /login: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read /login body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST /login: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	bodyStr := string(body)
	if strings.Contains(bodyStr, `"error"`) {
		return fmt.Errorf("login failed: %s", truncate(bodyStr, 200))
	}
	return nil
}

// Login performs the three-step Cronometer authentication handshake:
//
//  1. GET /login/      — bootstrap cookies + read anti-CSRF token.
//  2. POST /login      — credential submission.
//  3. POST /cronometer/app (GWT-RPC authenticate) — fetch userID +
//     session auth token used to mint export nonces.
//
// Steps map 1:1 to WIRE_SHAPES.md §(1), §(2), and §(3).
func (c *Client) Login(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("login: username and password required")
	}
	csrf, err := c.fetchAntiCSRF(ctx)
	if err != nil {
		return err
	}
	if err := c.submitLogin(ctx, username, password, csrf); err != nil {
		return err
	}

	// Step 3: GWT-RPC authenticate. UTC offset in minutes for the host
	// timezone — captured payload sent -300 (NYC DST). We send the
	// current local zone's offset so the response reflects the user's
	// local calendar.
	_, offsetSec := time.Now().Zone()
	body, err := c.gwtCall(ctx, authenticateBody(c.permutation, offsetSec/60))
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	uid, tok, err := parseAuthenticateResponse(body)
	if err != nil {
		return err
	}
	c.userID = uid
	c.authToken = tok
	return nil
}

// Logout calls the GWT-RPC logout method. WIRE_SHAPES.md §(14) notes
// this is best-effort: crono-export-cli already calls it via defer and
// ignores the returned error.
func (c *Client) Logout(ctx context.Context) error {
	if c.authToken == "" {
		return nil
	}
	body, err := c.gwtCall(ctx, logoutBody(c.permutation, c.authToken))
	c.authToken = ""
	c.userID = 0
	if err != nil {
		return err
	}
	if !strings.HasPrefix(body, "//OK") {
		return fmt.Errorf("logout: unexpected response: %s", truncate(body, 80))
	}
	return nil
}
