// Package server routes HTTP requests: health, and the built frontend.
package server

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// New returns the hub's handler. frontend is the built web app (web.Dist()).
func New(frontend fs.FS) http.Handler {
	mux := http.NewServeMux()
	// No auth: Kubernetes probes this.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	mux.Handle("GET /", spa(frontend))
	return securityHeaders(mux)
}

// spa serves files from the built frontend. Paths that aren't a file get
// index.html, so the React router handles them. Vite puts content-hashed
// files in assets/, so those can be cached forever; index.html never is.
func spa(frontend fs.FS) http.Handler {
	files := http.FileServerFS(frontend)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(frontend, name); err == nil && !st.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			// A missing asset is a real 404, not the app.
			if strings.HasPrefix(name, "assets/") {
				http.NotFound(w, r)
				return
			}
		}
		index, err := fs.ReadFile(frontend, "index.html")
		if err != nil {
			http.Error(w, "frontend not built", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}

// securityHeaders sets headers every response gets. The hub embeds other
// sites, but is never embedded itself.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}
