// Package cronoapi is the MIT-licensed, clean-room Cronometer HTTP client
// used by crono-export-cli.
//
// This package was authored from two inputs only:
//
//  1. The Phase 1 protocol spec at docs/cronometer-protocol.md, which
//     itself was derived from crono-export-cli's own MIT-licensed call
//     sites.
//  2. The Phase 3a wire-shape capture at
//     internal/cronoclient/testdata/cronometer/WIRE_SHAPES.md, which
//     documents the observable HTTP behaviour of cronometer.com from
//     a real recording session.
//
// No source code from github.com/jrmycanady/gocronometer (GPL-2.0) was
// consulted during authoring. The package replaces that dependency end
// to end; see QUA-37 for the clean-room rationale.
package cronoapi
