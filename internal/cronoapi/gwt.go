package cronoapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// DefaultGWTPermutation is the GWT permutation hash captured against
// cronometer.com on 2026-05-12. Cronometer rotates this value when they
// redeploy their frontend; callers can override via Client.SetPermutation.
//
// Provenance: WIRE_SHAPES.md §"X-Gwt-Permutation".
const DefaultGWTPermutation = "7B121DC5483BF272B1BC1916DA9FA963"

const (
	gwtModuleBase = "https://cronometer.com/cronometer/"
	gwtServiceIfc = "com.cronometer.shared.rpc.CronometerService"
	gwtContentType = "text/x-gwt-rpc; charset=UTF-8"
)

// gwtCall posts a raw GWT-RPC body to /cronometer/app and returns the
// response body as text. It enforces the cronometer.com-required headers
// (Content-Type, X-Gwt-Module-Base, X-Gwt-Permutation) and lets the
// http.Client's cookie jar manage session cookies.
func (c *Client) gwtCall(ctx context.Context, body string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/cronometer/app", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", gwtContentType)
	req.Header.Set("X-Gwt-Module-Base", gwtModuleBase)
	req.Header.Set("X-Gwt-Permutation", c.permutation)
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gwt-rpc: HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	return string(raw), nil
}

// authenticateBody returns the GWT-RPC payload for
// CronometerService.authenticate(int utcOffsetMinutes). The framing
// follows the literal call template captured in WIRE_SHAPES.md §(3).
func authenticateBody(permutation string, utcOffsetMinutes int) string {
	return fmt.Sprintf(
		"7|0|5|%s|%s|%s|authenticate|java.lang.Integer/3438268394|1|2|3|4|1|5|5|%d|",
		gwtModuleBase, permutation, gwtServiceIfc, utcOffsetMinutes,
	)
}

// generateAuthorizationTokenBody returns the GWT-RPC payload for
// CronometerService.generateAuthorizationToken(String, int, AuthScope).
// Captured framing per WIRE_SHAPES.md §(4). The trailing
// "7|2|" tail is the string-table back-reference set observed across
// every export type in the capture; it is re-emitted verbatim until a
// future capture proves it should vary.
func generateAuthorizationTokenBody(permutation, authToken string, userID int, ttlSeconds int) string {
	return fmt.Sprintf(
		"7|0|8|%s|%s|%s|generateAuthorizationToken|java.lang.String/2004016611|I|com.cronometer.shared.user.AuthScope/2065601159|%s|1|2|3|4|4|5|6|6|7|8|%d|%d|7|2|",
		gwtModuleBase, permutation, gwtServiceIfc, authToken, userID, ttlSeconds,
	)
}

// logoutBody returns the GWT-RPC payload for
// CronometerService.logout(String authToken). Captured framing per
// WIRE_SHAPES.md §(14).
func logoutBody(permutation, authToken string) string {
	return fmt.Sprintf(
		"7|0|6|%s|%s|%s|logout|java.lang.String/2004016611|%s|1|2|3|4|1|5|6|",
		gwtModuleBase, permutation, gwtServiceIfc, authToken,
	)
}

// GWT-RPC responses are prefixed `//OK[` on success and `//EX[` on
// exception. parseGwtOK strips the //OK prefix and returns the inner
// payload (everything between the [ and the trailing ]).
var (
	gwtOKRe       = regexp.MustCompile(`^//OK\[(.*)\]\s*$`)
	gwtUserIDRe   = regexp.MustCompile(`^//OK\[(\d+),`)
	gwtAuthTokRe  = regexp.MustCompile(`"([0-9a-fA-F]{32})"`)
	gwtNonceRe    = regexp.MustCompile(`\["([0-9a-fA-F]{32})"\]`)
)

// parseAuthenticateResponse pulls (userID, sessionAuthToken) out of the
// //OK[...] body of an authenticate response. The body is a flat array
// of GWT-interned values; we use targeted regexes instead of decoding
// the full string table (WIRE_SHAPES.md §(3) "Robust decoder approach").
//
// The session auth token is taken as the last 32-hex string in the
// response — empirically the position the subsequent
// generateAuthorizationToken calls echo back as their first argument.
func parseAuthenticateResponse(body string) (userID int, authToken string, err error) {
	if !strings.HasPrefix(body, "//OK[") {
		return 0, "", fmt.Errorf("authenticate: unexpected response prefix: %q", truncate(body, 80))
	}
	idMatch := gwtUserIDRe.FindStringSubmatch(body)
	if idMatch == nil {
		return 0, "", fmt.Errorf("authenticate: could not extract userId from response")
	}
	if _, err := fmt.Sscanf(idMatch[1], "%d", &userID); err != nil {
		return 0, "", fmt.Errorf("authenticate: parse userId %q: %w", idMatch[1], err)
	}
	tokens := gwtAuthTokRe.FindAllStringSubmatch(body, -1)
	if len(tokens) == 0 {
		return 0, "", fmt.Errorf("authenticate: no 32-hex session token found in response")
	}
	return userID, tokens[len(tokens)-1][1], nil
}

// parseAuthorizationTokenResponse pulls the export nonce out of a
// generateAuthorizationToken response. The body shape is
// `//OK[1,["<nonce>"],0,7]` (WIRE_SHAPES.md §(4) response shape).
func parseAuthorizationTokenResponse(body string) (string, error) {
	if !strings.HasPrefix(body, "//OK[") {
		return "", fmt.Errorf("auth token: unexpected response prefix: %q", truncate(body, 80))
	}
	m := gwtNonceRe.FindStringSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("auth token: no 32-hex nonce found in response: %q", truncate(body, 80))
	}
	return m[1], nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
