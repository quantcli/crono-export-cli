# Licensing history

`crono-export-cli` is released under the MIT licence (see [LICENSE](LICENSE)).
This file records the licensing history of the project so that downstream
consumers can reason about which releases are MIT-clean.

## Summary

- **v1.1.0 and later — MIT-clean.** The CLI talks to Cronometer through an
  in-tree, fresh-authored HTTP client under [`internal/cronoapi/`](internal/cronoapi).
  Every transitive dependency reported by `go list -m all` is MIT, BSD, or
  Apache-2.0. No GPL code is linked into the binary.
- **v0.1.0 through v1.0.1 — linked GPL-2.0 code.** These releases imported
  [`github.com/jrmycanady/gocronometer`](https://github.com/jrmycanady/gocronometer)
  (GPL-2.0) as the Cronometer client and were therefore subject to GPL-2.0
  obligations at distribution time. Users who built from those tags should
  treat the resulting binaries as GPL-2.0.

The earlier release notes have been amended to point at v1.1.0 as the first
MIT-clean cut.

## Clean-room replacement (QUA-12 / QUA-37)

The replacement Cronometer client in `internal/cronoapi/` was written from a
specification, not from the GPL source. The ground rules followed during the
rewrite:

- **Inputs allowed.** `gocronometer`'s *public* API surface (exported names
  and signatures, visible via `go doc`) and Cronometer's own observable HTTP
  behaviour, captured against a real account into redacted fixtures.
- **Inputs not allowed.** `gocronometer`'s source code, internal helpers, or
  unexported identifiers.
- **Spec doc.** [`docs/cronometer-protocol.md`](docs/cronometer-protocol.md)
  describes the Cronometer endpoints, GWT request/response shapes, and the
  authentication handshake that the new client implements.
- **Fixtures.** Recorded HTTP traces live under
  [`internal/cronoapi/testdata/`](internal/cronoapi/testdata/) and were
  redacted before commit (`tools/wirecap/`).
- **Tests.** Unit tests for the new client exercise the fixtures only; they
  do not import `gocronometer` and never have.

The implementation PR ([#37](https://github.com/quantcli/crono-export-cli/pull/37))
states explicitly that no `gocronometer` source was consulted during the
rewrite.

## Verifying MIT-cleanliness

```sh
# Confirms gocronometer is not in the build graph.
go mod why github.com/jrmycanady/gocronometer
# Expected: "(main module does not need package github.com/jrmycanady/gocronometer)"

# Lists every module in the build graph; review for licences.
go list -m all
```

If you discover a transitive GPL (or other copyleft) dependency in a
v1.1.0+ release, please open an issue against `quantcli/crono-export-cli`
so we can either drop or replace it.
