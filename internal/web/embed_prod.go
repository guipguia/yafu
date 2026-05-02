//go:build embed

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var dist embed.FS

func handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// fs.Sub on a known path of a successfully embedded FS cannot fail.
		panic(err)
	}
	return http.FileServerFS(sub)
}
