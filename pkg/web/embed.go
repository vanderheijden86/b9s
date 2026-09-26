package web

import (
	"embed"
	"io/fs"
)

// dist is the SPA that `make web` builds from web/. It is committed, so a
// plain `go build` needs no Node toolchain; TestEmbeddedBundleIsCurrent
// fails when web/src changed without a rebuild.
//
//go:embed dist
var dist embed.FS

// EmbeddedAssets returns the built SPA rooted at its index.html.
func EmbeddedAssets() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
