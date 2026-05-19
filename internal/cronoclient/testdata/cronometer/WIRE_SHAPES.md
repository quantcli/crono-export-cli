# Cronometer wire shapes — synthesized from a real capture session

Status: **draft, Phase 3a of [QUA-12](https://github.com/quantcli/common) plan v4 / [QUA-37](https://github.com/quantcli/common)**.
Captured: 2026-05-12 against `cronometer.com` with a real test account.

This document records the **observable HTTP behaviour** of `cronometer.com`
during a full crono-export-cli session, captured by routing the existing
(gocronometer-backed) CLI through the recording transport in
`tools/wirecapture/`. Per the [Phase 1 spec](../../../../docs/cronometer-protocol.md),
captured HTTP behaviour is an explicitly allowed input for the
clean-room replacement — `gocronometer`'s source remains out of scope.

The companion `*.json` files in this directory are redacted captures of
each exchange: only metadata (URL, headers, status) survives, and all
cookie values, anti-CSRF tokens, per-session auth tokens, and real
account data are stripped. The synthesized shapes below stand in for
the bodies, with placeholder values clearly marked.

## Session storyboard

A complete export session is exactly 14 HTTP exchanges in this order:

| #  | Method | URL                                         | Purpose                                                |
| -- | ------ | ------------------------------------------- | ------------------------------------------------------ |
|  1 | GET    | `https://cronometer.com/login/`             | Anonymous page load. Sets initial cookies. Returns HTML containing the anti-CSRF token. |
|  2 | POST   | `https://cronometer.com/login`              | Submit credentials. Returns JSON; sets the authenticated session cookies. |
|  3 | POST   | `https://cronometer.com/cronometer/app`     | GWT-RPC `authenticate`. Returns the user payload (incl. session-scoped auth token). |
|  4 | POST   | `https://cronometer.com/cronometer/app`     | GWT-RPC `generateAuthorizationToken`. Mints the per-export `nonce` for `servings`. |
|  5 | GET    | `https://cronometer.com/export?...&generate=servings&nonce=…`        | Servings CSV download. |
|  6 | POST   | `https://cronometer.com/cronometer/app`     | `generateAuthorizationToken` for `exercises`.          |
|  7 | GET    | `https://cronometer.com/export?...&generate=exercises&nonce=…`       | Exercises CSV download. |
|  8 | POST   | `https://cronometer.com/cronometer/app`     | `generateAuthorizationToken` for `biometrics`.         |
|  9 | GET    | `https://cronometer.com/export?...&generate=biometrics&nonce=…`      | Biometrics CSV download. |
| 10 | POST   | `https://cronometer.com/cronometer/app`     | `generateAuthorizationToken` for `dailySummary`.       |
| 11 | GET    | `https://cronometer.com/export?...&generate=dailySummary&nonce=…`    | Daily nutrition CSV download. |
| 12 | POST   | `https://cronometer.com/cronometer/app`     | `generateAuthorizationToken` for `notes`.              |
| 13 | GET    | `https://cronometer.com/export?...&generate=notes&nonce=…`           | Notes CSV download. |
| 14 | POST   | `https://cronometer.com/cronometer/app`     | GWT-RPC `logout`.                                       |

A fresh per-export `nonce` is required for each `/export` GET; reusing
one is not exercised here and should be avoided in the clean-room
implementation.

## (1) GET `/login/`

Anonymous request, no required headers beyond a permissive `User-Agent`.

The response is the login page HTML (~680KB). The clean-room client only
needs two things out of it:

- **The anti-CSRF token**, embedded as a hidden form input. Shape (extracted
  literally from the captured HTML, with the value redacted):

  ```html
  <input type="hidden" name="anticsrf" value="<32-char-hex-token>"/>
  ```

  Suggested extraction: regex `name="anticsrf"\s+value="([^"]+)"` against
  the response body. Robust to surrounding markup churn.

- **Session cookies** set via `Set-Cookie` (4 entries observed). Names and
  values rotate per-session; the clean-room client just needs to use a
  `*http.cookiejar.Jar` so they round-trip on subsequent requests. Do not
  hardcode cookie names.

## (2) POST `/login`

Form-encoded credential submission.

Request:

```
POST /login HTTP/1.1
Content-Type: application/x-www-form-urlencoded
Cookie: <session cookies from step 1>

anticsrf=<token-from-step-1>&password=<url-encoded-password>&username=<url-encoded-username>
```

Body key order observed: `anticsrf`, `password`, `username`. Cronometer
likely accepts any order, but matching observed order is safer.

Response on success: HTTP 200, `Content-Type: application/json;charset=UTF-8`,
body length ~39 chars. Sets two additional `Set-Cookie` entries (the
authenticated session). The exact JSON success body was not captured in
detail (small enough that future captures should preserve it verbatim
into a redacted fixture). Future capture TODO: capture the success body
shape and add it here.

Response on bad CSRF: HTTP 200 with body `{"error":"AntiCSRF Token Invalid"}`
(literal, observed during a debugging run where cookies were not being
round-tripped). Other failure shapes are TBD — capture the
"bad-credentials" and "rate-limited" cases in a future run.

## (3) POST `/cronometer/app` — GWT-RPC `authenticate`

All `/cronometer/app` calls are GWT-RPC. The request and response wire
format is GWT's text-pipe-delimited stream, not JSON. Required headers:

```
Content-Type: text/x-gwt-rpc; charset=UTF-8
X-Gwt-Module-Base: https://cronometer.com/cronometer/
X-Gwt-Permutation: <32-char-uppercase-hex hash>
Cookie: <session cookies>
```

The `X-Gwt-Permutation` header value is the **deployed GWT permutation
hash** — the same value any anonymous browser visiting `cronometer.com`
would send for the same browser/locale. It is public, but it does change
when Cronometer redeploys their frontend. The clean-room client should
either (a) hardcode the current value as a default and surface a clear
error when Cronometer rotates it, or (b) discover it dynamically by
parsing the GWT module nocache JS served at `cronometer.com/cronometer/cronometer.nocache.js`
or similar. The captured value as of 2026-05-12 was
`7B121DC5483BF272B1BC1916DA9FA963` — record-only; assume it will rotate.

### Request body shape (GWT-RPC encoding)

GWT-RPC frames a method call as `|`-delimited fields:

```
7|0|<arg-count>|<base-url>|<module-permutation>|<service-iface>|<method>|<arg-types-and-string-table-indices>...
```

For `authenticate`, the captured payload was 179 bytes shaped like:

```
7|0|5|https://cronometer.com/cronometer/|<32-char-hex permutation>|com.cronometer.shared.rpc.CronometerService|authenticate|java.lang.Integer/3438268394|1|2|3|4|1|5|5|<INT>|
```

Trailing `|<INT>|` was `-300` in the capture — almost certainly the
caller's UTC offset in minutes (-300 ≈ America/New_York DST). The
clean-room client should send the local UTC offset of the host running
the CLI.

The leading `7|0|N|...` framing is a GWT version + flags + arg-count
prefix. The trailing `1|2|3|4|1|5|5|<INT>|` chunk is GWT's interned
string-table back-references; treat it as part of the literal call
template for `authenticate(int)` and re-emit it verbatim until we have a
reason to vary it.

### Response body shape

Response was 8294 bytes. Wire format:

```
//OK[<comma-separated values, GWT string-table interned>]
```

The body is a flat array of integers, floats, quoted strings, and
back-references that decode into a `User` object plus its preferences,
custom-charts JSON, gold status, etc. Concrete fields the
clean-room client must extract from this response:

- `userId` (integer at the start of the array; e.g., the first 7-9 digit
  decimal value).
- `authToken` (32-char hex string near the end of the array, also passed
  back as the first arg of `generateAuthorizationToken` in the
  subsequent calls — see step 4).
- Everything else (custom charts, macro targets, body-fat history) can
  be ignored; `crono-export-cli` does not consume it.

**Robust decoder approach:** rather than hand-decoding the GWT string
table, the clean-room client can scrape the response with two regexes:

```go
userIDRe   = regexp.MustCompile(`^//OK\[(\d+),`)
authTokenRe = regexp.MustCompile(`"([0-9a-f]{32})"`)
```

`authTokenRe` will match multiple 32-hex strings; experimentally the
session auth token appears as the **last** such string in the
response that is followed immediately by other quoted entries (it is
the token that subsequent `generateAuthorizationToken` calls echo back
as their first argument). Future captures should pin this down by
diffing against a recapture; for Phase 3b, reflecting back the same
token that step 14 (`logout`) sends in its body is a sufficient
correctness check.

## (4 / 6 / 8 / 10 / 12) POST `/cronometer/app` — GWT-RPC `generateAuthorizationToken`

Same headers as (3). Five identical-shape calls, one per export type.

### Request body shape

Captured payload was 232 bytes; framing:

```
7|0|8|https://cronometer.com/cronometer/|<32-char-hex permutation>|com.cronometer.shared.rpc.CronometerService|generateAuthorizationToken|java.lang.String/2004016611|I|com.cronometer.shared.user.AuthScope/2065601159|<32-char session-auth-token>|1|2|3|4|4|5|6|6|7|8|<userId>|3600|7|2|
```

Field semantics:

- `<32-char session-auth-token>` — the auth token from the
  `authenticate` response (step 3).
- `<userId>` — the user ID from the `authenticate` response.
- The literal `3600` is almost certainly a TTL in seconds (1 hour) — the
  newly-minted nonce's lifetime.
- Trailing `7|2|` may correspond to an `AuthScope` enum value. The five
  capture flows all sent the same `7|2|` suffix, so a single value
  appears to cover all five export types. Vary only if a future capture
  proves otherwise.

### Response body shape

Response was 48 bytes:

```
//OK[1,["<32-char-hex export-nonce>"],0,7]
```

Extract the 32-hex nonce; pass it as `nonce=<value>` on the next
`/export` GET.

## (5 / 7 / 9 / 11 / 13) GET `/export`

CSV download. Required headers: just the session cookies.

```
GET /export?start=YYYY-MM-DD&end=YYYY-MM-DD&generate=<type>&nonce=<32-char-hex>
```

Query-string keys observed (alphabetically here, but query-string order
is not significant on the wire):

| Key      | Value                                                    |
| -------- | -------------------------------------------------------- |
| start    | `YYYY-MM-DD`, the local-calendar start of the inclusive window |
| end      | `YYYY-MM-DD`, the local-calendar end of the inclusive window   |
| generate | `servings` \| `exercises` \| `biometrics` \| `dailySummary` \| `notes` |
| nonce    | the 32-char hex token from the immediately-preceding `generateAuthorizationToken` |

Response: HTTP 200, `Content-Type: text/csv`. Body is RFC 4180-shaped
CSV with a header row.

CSV body sizes captured for an 8-day window with low-traffic data:

- `servings` → 6500 bytes (real food log present)
- `exercises` → 43 bytes (header only — no exercises in window)
- `biometrics` → 29 bytes (header only — no biometric entries in window)
- `dailySummary` → 2239 bytes (8 days of summary rows)
- `notes` → 15 bytes (header only — no notes in window)

The exact CSV column headers were not preserved in the redacted
fixtures — future capture TODO: extract just the header row from each
CSV and commit it as `headers/<type>.csv` (header-only CSVs contain no
account data and are safe to commit). The Phase 1 spec already
documents the typed-record field names; the Phase 3b implementer can
confirm the column-name mapping by capturing a fresh header row and
diffing against the spec's `<Name><Unit>` convention.

## (14) POST `/cronometer/app` — GWT-RPC `logout`

Same headers as (3).

### Request body shape

Captured payload was 162 bytes:

```
7|0|6|https://cronometer.com/cronometer/|<32-char-hex permutation>|com.cronometer.shared.rpc.CronometerService|logout|java.lang.String/2004016611|<32-char session-auth-token>|1|2|3|4|1|5|6|
```

Pass the same session auth token from step 3.

### Response body shape

Response was 12 bytes; success body shape: `//OK[1,0,7]`.

Logout is best-effort; `crono-export-cli` already calls it via `defer`
and ignores errors, and the clean-room replacement should preserve
that.

## Things explicitly NOT captured in this run

These should be filled in by future captures, ideally before Phase 3b
locks the API:

- **Long-window export pagination** — captured window was 8 days with
  low-volume data. Cronometer's behaviour for very long windows
  (multi-year, large data sets) is unknown. If `crono-export-cli` users
  hit a row cap, we need to know what shape it takes (HTTP error vs
  truncated CSV vs continuation cursor).
- **Auth failure response shapes** — bad password, expired session,
  rate-limit. We saw `{"error":"AntiCSRF Token Invalid"}` only as a
  side-effect of a recorder bug; the production failure modes are TBD.
- **Two-factor auth (2FA)** — the captured login HTML included a
  `name="userCode"` 6-digit input, suggesting Cronometer supports TOTP.
  Account used for capture had 2FA off; the 2FA-on flow is TBD.
- **Logout without a prior login** — defensive behaviour TBD.
- **Concurrent sessions** — does a fresh login invalidate existing
  cookies? Out of scope for the export use-case.
- **Exact JSON success body shape for POST `/login`** — content was 39
  bytes but not preserved; recapture and document.
- **Exact CSV header row contents** — recapture header-only and commit
  to `headers/<type>.csv`.

## Reproducing this capture

See [`CAPTURE.md`](CAPTURE.md) in this directory.
