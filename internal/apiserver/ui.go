package apiserver

import (
	"embed"
	"net/http"
)

//go:embed ui/*
var uiFS embed.FS

// serveUIFile writes one embedded UI asset with its content type.
func serveUIFile(w http.ResponseWriter, name string, contentType string) {
	data, err := uiFS.ReadFile("ui/" + name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(data)
}

// handleIndex serves GET / with the single-page UI.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	serveUIFile(w, "index.html", "text/html; charset=utf-8")
}

// handleCSS serves GET /app.css.
func (s *Server) handleCSS(w http.ResponseWriter, r *http.Request) {
	serveUIFile(w, "app.css", "text/css; charset=utf-8")
}

// handleJS serves GET /app.js.
func (s *Server) handleJS(w http.ResponseWriter, r *http.Request) {
	serveUIFile(w, "app.js", "text/javascript; charset=utf-8")
}
