// Package webfs embeds the frontend static assets so the server can ship them
// in a single binary.
package webfs

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var embedded embed.FS

// FS returns the web/ subtree as a filesystem rooted at the frontend directory.
func FS() fs.FS {
	sub, err := fs.Sub(embedded, "web")
	if err != nil {
		panic(err)
	}
	return sub
}
