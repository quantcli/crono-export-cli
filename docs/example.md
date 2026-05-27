# Worked example: crono-export

**Audience:** Cronometer users who want their food log, nutrition totals, or biometrics in a terminal or piped to an LLM agent.

**Prerequisites:** `crono-export` installed (see [GETTING_STARTED.md](https://github.com/quantcli/common/blob/main/GETTING_STARTED.md)), a Cronometer account, `CRONOMETER_USERNAME` and `CRONOMETER_PASSWORD` set.

---

## 1. Verify credentials

```sh
export CRONOMETER_USERNAME="you@example.com"
export CRONOMETER_PASSWORD="yourpassword"
crono-export auth status
```

**Expected output when credentials are set:**

```
error: missing CRONOMETER_USERNAME and CRONOMETER_PASSWORD
```

Wait — that message appears when the variables are *not* set. When they are set, `auth status` exits 0 with a message like:

```
logged in as you@example.com
```

(The exact wording depends on whether a cached session exists. Exit 0 = ready to export.)

**If credentials are missing:**

```sh
$ crono-export auth status
error: missing CRONOMETER_USERNAME and CRONOMETER_PASSWORD
$ echo $?
1
```

Exit 1 with a stderr message naming the missing variable. Set both and re-run.

---

## 2. Orient with `prime`

```sh
crono-export prime
```

**Output** (reproduced at HEAD, 2026-05-19):

```
crono-export — primer for LLM agents
=====================================

WHAT IT IS
  CLI for personal Cronometer data: per-food log, daily nutrition totals,
  biometrics (weight/fat/BP/...), exercises, and notes.

I/O
  stdout: data in --format markdown (default) or json.
  stderr: errors. Exit 0 on success including empty results.

AUTH
  Env-var credentials (always required):
    CRONOMETER_USERNAME   your Cronometer email
    CRONOMETER_PASSWORD   your Cronometer password

  Session is cached at $XDG_CACHE_HOME/crono-export/session.json (mode 0600)
  so consecutive calls reuse one login.  Set CRONOMETER_NO_CACHE=1 to
  disable.  'crono-export auth logout' clears the cache.

  crono-export auth status   Exit 0 if both vars set, 1 with "missing X".

DATE FLAGS  (every subcommand)
  --since VALUE / --until VALUE
  VALUE: today | yesterday | YYYY-MM-DD | Nd/Nw/Nm/Ny
  Default when neither given: last 7 days ending today.
  See https://github.com/quantcli/common/blob/main/CONTRACT.md#3-date-flags

SUBCOMMANDS
  servings    per-food log; one row per food eaten, full nutrient breakdown
  nutrition   daily totals across all foods (one row per day, all macros + micros)
  biometrics  weight, body fat, blood pressure, custom metrics
  exercises   logged cardio / strength / custom activities
  notes       user-entered notes per day

  Inspect any subcommand's row schema with: <subcommand> --since today --format json

EXAMPLES
  crono-export nutrition --since today
  crono-export servings --since 7d --format json | jq '[.[] | .ProteinG] | add'
  crono-export biometrics --since 30d --format json |
    jq 'map(select(.Metric == "Weight")) | sort_by(.RecordedTime) | last'

GOTCHAS
  - 'today' is your LOCAL calendar day, not UTC.
  - 'RecordedTime' is date-only (midnight in your local zone); Cronometer's
    CSV exports don't carry meal-time, so all times sort as 00:00.
  - Markdown drops zero-valued nutrients; use --format json for every column.
  - 'servings' rows have a 'Day' field that is always null — use 'RecordedTime'.
```

---

## 3. Export last week's nutrition totals

```sh
crono-export nutrition --since 7d
```

**Expected output** (markdown, one row per day):

```
| Day        | Energy (kcal) | Protein (g) | Carbs (g) | Fat (g) |
|------------|---------------|-------------|-----------|---------|
| 2026-05-12 | 2150          | 142         | 210       | 68      |
| 2026-05-13 | 1980          | 130         | 195       | 62      |
...
```

*Note: actual column set is wider (full micro/macronutrient breakdown). Rows with all-zero values for a nutrient are omitted in markdown; use `--format json` for every column.*

To get JSON for scripting:

```sh
crono-export nutrition --since 7d --format json | jq '.[0]'
```

---

## 4. Sum protein from the food log

```sh
crono-export servings --since 7d --format json | jq '[.[] | .ProteinG] | add'
```

This pipes the structured food-log (one object per food serving) to `jq` and sums the `ProteinG` field.

---

## 5. Get most recent weight measurement

```sh
crono-export biometrics --since 30d --format json \
  | jq 'map(select(.Metric == "Weight")) | sort_by(.RecordedTime) | last'
```

---

## 6. Check flag validation (contract §3, §7)

These commands verify that the CLI conforms to [CONTRACT.md §3](https://github.com/quantcli/common/blob/main/CONTRACT.md#3-date-flags) and [§7](https://github.com/quantcli/common/blob/main/CONTRACT.md#7-hermeticity) without live credentials:

```sh
crono-export nutrition --help    # exits 0; no network call
```

```sh
crono-export nutrition --since lol 2>&1; echo "exit: $?"
```

**Expected:**

```
error: bad --since: invalid date "lol" (use YYYY-MM-DD, today, yesterday, or Nd/Nw/Nm/Ny)
exit: 1
```

Error on stderr, nothing on stdout, exit 1. Matches CONTRACT.md §3.

---

## 7. Run the contract conformance suite

```sh
git clone https://github.com/quantcli/crono-export-cli
cd crono-export-cli
go build -o /tmp/crono-export .
EXPORT_CLI_BIN=/tmp/crono-export go test -tags=compat ./...
```

**Expected:**

```
ok      github.com/quantcli/crono-export-cli    0.010s
ok      github.com/quantcli/crono-export-cli/cmd    0.005s
...
```

All green. The `compat` suite covers CONTRACT.md §3 (date flags, hermetic `--help`) and §7 (hermeticity). Data-path subtests (`--format json` against live Cronometer) require real credentials; the parse-level subtests run without them.

---

## What to look at next

- `crono-export servings --help` — full flag list and column schema
- `crono-export prime` — jq recipes and gotchas for LLM agent use
- [CONTRACT.md §3](https://github.com/quantcli/common/blob/main/CONTRACT.md#3-date-flags) — date flag semantics
- [CONTRACT.md §4](https://github.com/quantcli/common/blob/main/CONTRACT.md#4-output-format) — output format contract
- [liftoff-export example](https://github.com/quantcli/liftoff-export-cli/blob/main/docs/example.md) — if you also track gym workouts
- [withings-export example](https://github.com/quantcli/withings-export-cli/blob/main/docs/example.md) — if you have Withings devices
