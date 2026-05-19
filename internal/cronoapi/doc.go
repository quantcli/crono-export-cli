// Package cronoapi is the MIT-licensed, clean-room Cronometer HTTP client
// used by crono-export-cli.
//
// Authoring inputs:
//
//  1. crono-export-cli's own MIT-licensed call sites — exported names
//     and Go type signatures already in use here.
//  2. The wire-shape capture at
//     internal/exporter/testdata/cronometer/WIRE_SHAPES.md, which
//     documents the observable HTTP behaviour of cronometer.com from
//     a real recording session.
//
// No source code from github.com/jrmycanady/gocronometer (GPL-2.0) was
// consulted during authoring; this package replaces that dependency end
// to end.
package cronoapi
