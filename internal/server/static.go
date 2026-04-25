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
			// Mux didn't match a registered route: the path doesn't exist.
			// 404 (route absent) is the right signal here, not 405 (route
			// present but wrong method).
			writeErr(w, http.StatusNotFound, "no such api route: "+r.URL.Path)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
