// Package web exposes the built dashboard assets in web/dist.
//
// The dashboard is a Vite/React app; `make web` builds it into web/dist, which
// this package embeds into the notebotd binary. There is no copy step — dist
// is the single source of truth for the frontend assets and is committed so
// `go build ./...` works without a Node toolchain.
package web

import "embed"

// Dist holds the contents of web/dist (index.html and assets/).
//go:embed all:dist
var Dist embed.FS
