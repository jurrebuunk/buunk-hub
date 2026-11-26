// Package web holds the built frontend. Build it (npm run build) before the
// Go binary: go:embed needs dist/ to exist.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist is the contents of dist/, with index.html at its root.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // only if the embed directive above is wrong
	}
	return sub
}
