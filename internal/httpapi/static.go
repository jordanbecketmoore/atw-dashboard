package httpapi

import (
	"net/http"

	webfs "github.com/jordanm/atw-dashboard"
)

// StaticHandler returns a handler that serves the embedded frontend at /.
func StaticHandler() http.Handler {
	return http.FileServerFS(webfs.FS())
}
