// Package server routes HTTP requests: health, sign-in, the API and the
// built frontend.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/catalogue"
	"github.com/jurrebuunk/buunk-hub/internal/favicon"
	"github.com/jurrebuunk/buunk-hub/internal/i18n"
)

// Auth is what the server needs from internal/auth.
type Auth interface {
	Routes(mux *http.ServeMux)
	Require(next http.Handler) http.Handler
	CheckOrigin(next http.Handler) http.Handler
}

// Favicons finds the icon of a site (internal/favicon).
type Favicons interface {
	Get(ctx context.Context, siteURL string) (favicon.Icon, error)
}

// New returns the hub's handler. apps is the catalogue, frontend the built
// web app (web.Dist()). Everything is behind sign-in except /healthz and
// /auth/.
func New(a Auth, apps []catalogue.App, icons Favicons, frontend fs.FS) http.Handler {
	mux := http.NewServeMux()
	// No auth: Kubernetes probes this.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	a.Routes(mux)
	mux.Handle("GET /api/me", a.Require(http.HandlerFunc(me)))
	mux.Handle("GET /api/apps", a.Require(appsHandler(apps)))
	mux.Handle("GET /api/apps/{id}/favicon", a.Require(faviconHandler(apps, icons)))
	mux.Handle("GET /api/", a.Require(http.NotFoundHandler()))
	mux.Handle("GET /", a.Require(spa(frontend)))
	return securityHeaders(a.CheckOrigin(mux))
}

// me tells the app who is signed in and which language to show.
func me(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	writeJSON(w, map[string]any{
		"sub":     id.User.Sub,
		"name":    id.User.Name,
		"email":   id.User.Email,
		"picture": id.User.Picture,
		"lang":    i18n.Pick(id.User.Locale, r.Header.Get("Accept-Language")),
	})
}

// appsHandler lists the catalogue apps the person has a role for, in their
// language. Nothing personal: the same app looks the same for everyone.
func appsHandler(apps []catalogue.App) http.Handler {
	type app struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
		Internal    bool   `json:"internal"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := auth.FromContext(r.Context())
		lang := i18n.Pick(id.User.Locale, r.Header.Get("Accept-Language"))
		out := []app{}
		for _, a := range apps {
			if a.VisibleTo(id.Roles) {
				out = append(out, app{a.ID, a.Name, a.URL, a.Icon, a.Description[lang], a.Internal})
			}
		}
		writeJSON(w, out)
	})
}

// faviconHandler serves an app's own icon, for apps the person may see.
// 404 when the site has none; Home then shows the catalogue's icon.
func faviconHandler(apps []catalogue.App, icons Favicons) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := auth.FromContext(r.Context())
		for _, a := range apps {
			if a.ID != r.PathValue("id") || !a.VisibleTo(id.Roles) {
				continue
			}
			icon, err := icons.Get(r.Context(), a.URL)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			h := w.Header()
			h.Set("Content-Type", icon.Type)
			h.Set("Cache-Control", "private, max-age=86400")
			// Another site's file on our origin: an SVG opened directly
			// must not run scripts here.
			h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
			w.Write(icon.Data)
			return
		}
		http.NotFound(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
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
