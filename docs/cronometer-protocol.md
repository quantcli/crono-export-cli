# Cronometer Protocol Spec — for clean-room client (Phase 1)

Status: **draft, Phase 1 of [QUA-12](https://github.com/quantcli/common) plan v4**

This document specifies the surface that an MIT-licensed internal Cronometer
client must reimplement so that `crono-export-cli` can drop its dependency
on the GPL-2.0 `github.com/jrmycanady/gocronometer` package.

## Clean-room ground rules

The implementer **must not consult `gocronometer`'s source code** while
working from this spec. The only permitted inputs to the reimplementation
are:

1. This spec document.
2. `gocronometer`'s **public API surface** — exported names and Go type
   signatures, as already named and called from `crono-export-cli`'s own
   MIT-licensed code (`internal/cronoclient/client.go`, `cmd/format.go`).
3. Cronometer's **observable HTTP behaviour** — what the Cronometer web
   app sends and receives in a browser session against
   `https://cronometer.com`, captured as HAR / proxy logs against a real
   test account.

`gocronometer`'s source files, internal helpers, comments, request
construction, and parsing logic are **out of scope** and must not be
read or copied. If a question can only be answered by reading
`gocronometer` source, capture the answer empirically from a HAR
instead.

This document was authored without consulting `gocronometer` source.

## Where the spec comes from

Everything in the "Public surface we consume" section below is derived
from `crono-export-cli`'s own code — that is, the call sites and field
references already present in our MIT-licensed source. Nothing in this
document was copied from `gocronometer`.

Wire-level details (URL paths, request bodies, exact response CSV
headers, cookie names) are marked **TBD** wherever they cannot be
established from public sources, and must be filled in empirically
during Phase 3 by HAR capture against a real Cronometer account.

## Public surface we consume

`crono-export-cli` currently uses the following exported names from
`github.com/jrmycanady/gocronometer`. The clean-room replacement must
provide equivalents (under different package and type names — see
"Naming" below) sufficient to delete every reference to the upstream
package from our codebase.

### Constructor and session

| Upstream call                          | Used in                                    | Purpose                                  |
| -------------------------------------- | ------------------------------------------ | ---------------------------------------- |
| `gocronometer.NewClient(httpClient)`   | `internal/cronoclient/client.go` (Login)   | Construct a client. We pass `nil` today. |
| `(*Client).Login(ctx, user, pass)`     | `internal/cronoclient/client.go` (Login)   | Username/password session login.         |
| `(*Client).Logout(ctx)`                | `internal/cronoclient/client.go` (Logout)  | Tear down the session, best-effort.      |

Auth model the replacement must support:

- Credentials are read from environment variables `CRONOMETER_USERNAME`
  and `CRONOMETER_PASSWORD`. The CLI logs in fresh on every invocation
  and does not cache tokens.
- There is no SSO / API-token / OAuth pathway exposed to individual
  Cronometer users today (per the project README, last verified
  2026-05). Password POST is the only available auth mode.
- Session lifetime, cookie names, anti-forgery tokens, redirect chain:
  **TBD via HAR**.

### Export operations

| Upstream call                                                       | Returns                                  | Used by                          |
| ------------------------------------------------------------------- | ---------------------------------------- | -------------------------------- |
| `ExportServingsParsedWithLocation(ctx, start, end, loc)`            | `gocronometer.ServingRecords`            | `cronoclient.Client.Servings`    |
| `ExportExercisesParsedWithLocation(ctx, start, end, loc)`           | `gocronometer.ExerciseRecords`           | `cronoclient.Client.Exercises`   |
| `ExportBiometricRecordsParsedWithLocation(ctx, start, end, loc)`    | `gocronometer.BiometricRecords`          | `cronoclient.Client.Biometrics`  |
| `ExportDailyNutrition(ctx, start, end)`                             | raw CSV `string`                         | `cronoclient.Client.Nutrition`   |
| `ExportNotes(ctx, start, end)`                                      | raw CSV `string`                         | `cronoclient.Client.Notes`       |

Parameter conventions (the only ones we depend on):

- `ctx context.Context` — cancellation / timeout. Replacement must
  honour `ctx.Done()` on the underlying HTTP request.
- `start, end time.Time` — inclusive `[start, end]` window. Only the
  calendar date (`YYYY-MM-DD`) of each endpoint is significant on the
  wire. Time of day and zone on these values do **not** round-trip.
  Date strings on the wire use the user's local calendar.
- `loc *time.Location` — the parsed `RecordedTime` on rows must be
  produced in this zone. We always pass `time.Local`.
- CSV-only methods (`ExportDailyNutrition`, `ExportNotes`) do not
  accept a `*time.Location`. The CSV `Date` / `Day` columns are
  rendered verbatim as Cronometer emits them.

### Record types we read

These are the only fields `crono-export-cli` reads off
`gocronometer`'s typed record types (proof: `cmd/format.go` and
`cmd/auth.go` are the only consumers). The replacement must expose
identically-named fields with these Go types on its own record types
so the `cmd/format.go` reflection-driven renderer continues to work
unchanged (or so that a thin compatibility shim does).

`ServingRecord` (one row per food logged, full nutrient breakdown):

| Field             | Go type     | Notes                                                  |
| ----------------- | ----------- | ------------------------------------------------------ |
| `RecordedTime`    | `time.Time` | Parsed in caller-supplied `*time.Location`.            |
| `Group`           | `string`    | Meal group / category bucket (e.g., "Breakfast").       |
| `FoodName`        | `string`    | Free-form.                                             |
| `QuantityValue`   | `float64`   | Serving size numeric.                                  |
| `QuantityUnits`   | `string`    | Serving size unit ("g", "ml", "cup", …).               |
| `Category`        | `string`    | (Currently skipped from markdown rendering.)           |
| many `float64`    | `float64`   | Nutrient columns; field names follow `<Name><Unit>`.   |

Nutrient field naming convention (used by `cmd/format.go`'s
`strippedSuffix`): the upstream type's Go field names embed the unit
as a suffix. The renderer recognises these suffixes (longest-first):

| Field suffix | Display unit |
| ------------ | ------------ |
| `Kcal`       | `kcal`       |
| `Ug`         | `µg`         |
| `Mg`         | `mg`         |
| `UI`         | `IU`         |
| `G`          | `g`          |

So `EnergyKcal` → "Energy: X kcal", `ProteinG` → "Protein: X g",
`B12Ug` → "B12: X µg", `VitaminAUI` → "Vitamin A: X IU". The
replacement record type's nutrient fields must follow the same
suffix convention so the existing renderer keeps working without
changes. The exact list of nutrient columns must be captured from
the live CSV header during Phase 3 (Cronometer adds new nutrient
columns periodically).

`BiometricRecord` (weight, body fat, blood pressure, custom metrics):

| Field          | Go type     | Notes                                            |
| -------------- | ----------- | ------------------------------------------------ |
| `RecordedTime` | `time.Time` | Parsed in caller-supplied `*time.Location`.      |
| `Metric`       | `string`    | Metric name (e.g., "Weight").                    |
| `Amount`       | `float64`   | Numeric value.                                   |
| `Unit`         | `string`    | Unit string emitted by Cronometer (e.g., "lbs"). |

`ExerciseRecord` (logged exercises):

| Field            | Go type     | Notes                                          |
| ---------------- | ----------- | ---------------------------------------------- |
| `RecordedTime`   | `time.Time` | Parsed in caller-supplied `*time.Location`.    |
| `Exercise`       | `string`    | Exercise name.                                 |
| `Minutes`        | `float64`   | Duration. Zero when unset by the user.         |
| `CaloriesBurned` | `float64`   | Reported energy burn. Zero when unset.         |
| `Group`          | `string`    | Optional tag/category.                         |

Aliases used in callers:

- `gocronometer.ServingRecords` is the slice form
  `[]gocronometer.ServingRecord`. Same for `BiometricRecords` and
  `ExerciseRecords`.

### CSV-only endpoints we use raw

For `Nutrition` and `Notes`, our existing `internal/cronoclient/client.go`
treats the upstream return as an opaque CSV string and parses it with
`encoding/csv` into `[]map[string]string`. We do not use any upstream
typed parser for these. The replacement must either:

- return CSV verbatim from those endpoints (same `string` shape), or
- decode to `[]map[string]string` directly with the same column
  semantics described below.

Notes column tolerance: `cmd/format.go`'s `renderNotes` picks the
date column from `Day` or `Date` (in that order) and the note column
from `Note`, `Notes`, or `Comment`, with a fallback to dumping all
non-empty fields. The replacement must preserve at least one of each
group's column names so this fallback is rarely triggered.

Known nutrition gotcha: per the prime text, the `Day` field on
`ServingRecord` is always null and the renderer uses `RecordedTime`
instead; the replacement must preserve that behaviour (i.e., do not
silently start populating `Day` with a stale value, since downstream
LLM agents have been instructed it is null).

## Datetime semantics

- All `RecordedTime` values are parsed using the caller-supplied
  `*time.Location` (we always pass `time.Local`). They must reflect
  the calendar moment the Cronometer UI shows for that row, not UTC.
- The `--since` / `--until` flags parse to local-midnight `time.Time`
  values (see `internal/cronoclient/daterange.go`); only the calendar
  date (`YYYY-MM-DD`) of each endpoint of the inclusive window matters
  on the wire.
- CSV `Date` / `Day` columns are passed through as Cronometer emits
  them — no re-parsing in our client today.

These semantics are codified in [`quantcli/common`'s `CONTRACT.md`
§3 Date flags](https://github.com/quantcli/common/blob/main/CONTRACT.md#3-date-flags)
and must continue to hold post-rewrite.

## Wire layer — TBD during Phase 3

The items below cannot be specified without hitting the real
`cronometer.com` endpoints. They will be captured empirically in
Phase 3 against a real Cronometer test account (board-provided HAR
or DT-supplied credentials). They are listed here so the Phase 3
implementer has a checklist; nothing here may be answered by
reading `gocronometer` source.

| Concern                                                | Resolution route                       |
| ------------------------------------------------------ | -------------------------------------- |
| Login URL, method, request body shape                  | HAR: cronometer.com login form         |
| Anti-forgery / CSRF token bootstrap                    | HAR: first page load                   |
| Session cookie name(s) and `Set-Cookie` flags          | HAR: post-login response               |
| Logout URL, request shape, expected response           | HAR: logout click                      |
| Servings export URL + query params + date format       | HAR: "Export" servings flow            |
| Exercises export URL + params                          | HAR: "Export" exercises flow           |
| Biometrics export URL + params                         | HAR: "Export" biometrics flow          |
| Daily-nutrition export URL + params                    | HAR: "Export" daily-nutrition flow     |
| Notes export URL + params                              | HAR: "Export" notes flow               |
| Response content types (CSV vs JSON; charset)          | HAR: response headers                  |
| Exact CSV header column names per endpoint             | HAR: response bodies                   |
| Time-of-day format inside `RecordedTime` parsing       | HAR: response body shape               |
| Pagination / cursor behaviour, if any                  | HAR: long-window export                |
| Rate-limit and retry-after semantics                   | HAR: forced rapid replays              |
| Error response shapes (auth fail, range too long, …)   | HAR: deliberate-failure runs           |

The Phase 3 implementer must record these HAR samples under
`internal/cronoclient/testdata/fixtures/` (golden files) so the
clean-room client's tests run hermetically with no live network.

## Naming for the replacement

The replacement package must not be named `gocronometer` and must
not re-export gocronometer-style identifiers verbatim. Suggested
shape (final naming decided in Phase 3 PR review):

- Package path: `github.com/quantcli/crono-export-cli/internal/cronoapi`
  (or a similar name under `internal/`). Keep the existing
  `internal/cronoclient` wrapper layer intact so call sites in
  `cmd/*.go` don't churn.
- Client type: `cronoapi.Client` (or similar) with `NewClient`,
  `Login`, `Logout`.
- Record types: `cronoapi.ServingRecord`, `cronoapi.BiometricRecord`,
  `cronoapi.ExerciseRecord`, with collection aliases as needed.
- Field names on record types: **identical** to those listed in
  "Record types we read" above, so `cmd/format.go`'s reflection
  walker keeps working without modification.

## Testing strategy

- All client-package tests must run against **recorded fixtures**
  (HAR or hand-written CSV golden files) under
  `internal/cronoclient/testdata/` and friends.
- No CI job may hit `cronometer.com`. Live calls happen only on
  developer workstations, only when capturing fresh fixtures.
- The existing `compat_contract_test.go` against
  `github.com/quantcli/common/compat` continues to be the
  cross-CLI compliance gate. The replacement client must not
  break its expectations (date-flag parsing, format flag,
  prime subcommand, env-var auth, exit codes).

## Out of scope for this spec

- Anything beyond the five export operations actually used today
  (servings, exercises, biometrics, daily nutrition, notes).
- Cronometer endpoints that are not exposed through `gocronometer`'s
  public surface (we don't depend on them, so they are not in our
  surface).
- Cronometer Pro / Gold / team-account features.
- The Cronometer ToS §10(b) AI/automation question — that is a
  board-accepted record-only risk per
  [QUA-12 plan v4](https://github.com/quantcli/common) and is
  resolved at the policy layer, not at the protocol layer.
