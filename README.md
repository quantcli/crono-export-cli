# crono-export-cli

Export your personal nutrition, biometric, and food-log data from [Cronometer](https://cronometer.com) as narrow markdown (the default) or JSON. Built for personal LLM agents and scripts that want structured nutrition data — for example, an LLM-driven bariatric or fitness coach that needs to know how much protein, B12, iron, or calcium you actually got today.

[![Latest Release](https://img.shields.io/github/v/release/quantcli/crono-export-cli)](https://github.com/quantcli/crono-export-cli/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/quantcli/crono-export-cli)](go.mod)
![Platforms](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)

## Features

- **Five export endpoints** — servings (per-food log with full nutrient breakdown), nutrition (daily totals), biometrics (weight, body fat, custom metrics), exercises, and notes
- **Markdown by default, JSON on demand** — narrow fitdown-style markdown reads well in chat and terminals; pass `--format json` for the full structured row to pipe through `jq`
- **Date selection** — `--since` / `--until` accepting `today`, `yesterday`, `YYYY-MM-DD`, or `Nd`/`Nw`/`Nm`/`Ny` on every subcommand
- **Single static binary** — no Python or Node runtime; drop it in `~/bin/` and go
- **Credentials via env** — `CRONOMETER_USERNAME` / `CRONOMETER_PASSWORD`, no config file needed
- **Built for agents** — designed to be called as a terminal tool by LLMs (Claude, hermes-agent, etc.); run `crono-export prime` for a one-screen orientation (I/O contract, subcommands, jq recipes)

## Quick Start

```sh
# Install with Homebrew
brew tap quantcli/tap
brew install crono-export

# Set credentials and try a query
export CRONOMETER_USERNAME="you@example.com"
export CRONOMETER_PASSWORD="…"
crono-export servings --since today
```

## Install

**Homebrew (macOS / Linux):**
```sh
brew tap quantcli/tap
brew install crono-export
```

Or download a pre-built binary from the [releases page](https://github.com/quantcli/crono-export-cli/releases/latest):

**macOS (Apple Silicon):**
```sh
curl -Lo /tmp/crono-export.zip https://github.com/quantcli/crono-export-cli/releases/latest/download/crono-export_darwin_arm64.zip
unzip -jo /tmp/crono-export.zip -d ~/bin && rm /tmp/crono-export.zip
chmod +x ~/bin/crono-export
```

**macOS (Intel):**
```sh
curl -Lo /tmp/crono-export.zip https://github.com/quantcli/crono-export-cli/releases/latest/download/crono-export_darwin_amd64.zip
unzip -jo /tmp/crono-export.zip -d ~/bin && rm /tmp/crono-export.zip
chmod +x ~/bin/crono-export
```

**Linux (amd64):**
```sh
curl -Lo /tmp/crono-export.zip https://github.com/quantcli/crono-export-cli/releases/latest/download/crono-export_linux_amd64.zip
unzip -jo /tmp/crono-export.zip -d ~/bin && rm /tmp/crono-export.zip
chmod +x ~/bin/crono-export
```

**Windows (amd64):**

Download `crono-export_windows_amd64.zip` from the [releases page](https://github.com/quantcli/crono-export-cli/releases/latest), extract it, and add the directory to your PATH.

Make sure `~/bin` is in your `PATH`. If not, add this to your `~/.zshrc` or `~/.bashrc`:
```sh
export PATH="$HOME/bin:$PATH"
```

## Credentials

Set these in your shell, in a `.env` file your runner sources, or in your LLM agent's environment:

```sh
export CRONOMETER_USERNAME="you@example.com"
export CRONOMETER_PASSWORD="your-cronometer-password"
```

The CLI logs in on every invocation; there's no token cache. Cronometer doesn't (yet) offer SSO or API tokens for individuals, so a real password is the only auth option.

## Usage

Every subcommand accepts the same date flags, per the [shared quantcli contract](https://github.com/quantcli/common/blob/main/CONTRACT.md#3-date-flags):

| Flag | Meaning |
|---|---|
| `--since VALUE` | Inclusive lower bound |
| `--until VALUE` | Inclusive upper bound (omit for "today") |
| *(none)* | Last 7 days, ending today |

`VALUE` is one of: `today`, `yesterday`, `YYYY-MM-DD`, or a relative duration like `7d`, `4w`, `6m`, `1y`.

### Servings — per-food log

One row per food item logged, with full macro and micronutrient breakdown.

```sh
crono-export servings --since today
crono-export servings --since 7d
crono-export servings --since 2026-04-01 --until 2026-04-15
```

Default markdown output (per food, zero-valued nutrients suppressed):

```markdown
## 2026-04-11

### Breakfast · Cheese Cracker (20 square)
- Energy: 97.8 kcal
- Protein: 1.95 g
- Carbs: 11.88 g
- Fiber: 0.46 g
- Fat: 4.94 g
- B12: 0.07 mg
- Calcium: 27.2 mg
- Iron: 0.69 mg
```

### Nutrition — daily totals

One row per day, totals across every food logged that day.

```sh
crono-export nutrition --since 30d
```

### Biometrics — weight, body fat, custom metrics

```sh
crono-export biometrics --since 30d
```

```markdown
## 2026-04-10
- Weight: 237 lbs
```

### Exercises

```sh
crono-export exercises --since 7d
```

### Notes

```sh
crono-export notes --since 30d
```

## Output Format

Default output is narrow, [Fitdown](https://github.com/datavis-tech/fitdown)-style markdown — date-grouped headings, one bullet per non-zero field, no wide tables. Markdown reads well in chat and on a terminal and is easy for an LLM to consume inline.

For programmatic use, pass `--format json` to get the full structured row as a JSON array on stdout — nothing suppressed, easy to pipe through `jq`. Errors always go to stderr, so JSON output stays clean for piping.

```sh
crono-export servings --since today                 # markdown, default
crono-export servings --since today --format json | jq '[.[] | {food: .FoodName, protein: .ProteinG}]'
```

LLM agents: run `crono-export prime` for a one-screen orientation describing both formats, all subcommands, the date flags, and `jq` recipes.

## About Cronometer

[Cronometer](https://cronometer.com) is a nutrition tracking app with one of the best micronutrient databases of any consumer tool — a major reason it's commonly recommended for bariatric patients, anyone tracking specific vitamin/mineral targets, or athletes managing recovery nutrition.

This CLI is an unofficial tool for exporting your own data. It speaks directly to the same web export endpoints the Cronometer SPA uses, via an MIT-licensed in-tree HTTP client (`internal/cronoapi`). It is intended for personal single-user use only.

## License

MIT — see [LICENSE](LICENSE).

The CLI is MIT-clean: it has no transitive GPL dependencies. See [LICENSING.md](LICENSING.md) for the history.
