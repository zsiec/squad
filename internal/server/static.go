package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web/*
var webFS embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Allow", "GET")
			writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
