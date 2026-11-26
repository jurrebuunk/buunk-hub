// Package server routes HTTP requests: health, sign-in, the API and the
// built frontend.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/catalogue"
	"github.com/jurrebuunk/buunk-hub/internal/favicon"
	"github.com/jurrebuunk/buunk-hub/internal/i18n"
	"github.com/jurrebuunk/buunk-hub/internal/links"
	"github.com/jurrebuunk/buunk-hub/internal/startpage"
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

// Deps is what the hub is made of; see main.go.
type Deps struct {
	Auth     Auth
	Apps     []catalogue.App
	Icons    Favicons
	Pictures Pictures
	Links    Links
	Pages    Pages
	Framing  Framing
	Hub      *url.URL // the hub's public address: a page may not embed it
	Frontend fs.FS    // the built web app (web.Dist())
	Log      *slog.Logger
}

// New returns the hub's handler. Everything is behind sign-in except
// /healthz, /auth/ and the profile pictures at /avatars/.
func New(d Deps) http.Handler {
	a := d.Auth
	mux := http.NewServeMux()
	// No auth: Kubernetes probes this.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	a.Routes(mux)
	// Public: other apps show these (the picture claim points here).
	mux.HandleFunc("GET /avatars/{sub}/{file}", d.Pictures.Serve)

	mux.Handle("GET /start", a.Require(startpage.Handler(d.Apps, func(r *http.Request, sub string) ([]links.Link, error) {
		return d.Links.List(r.Context(), sub)
	}, d.Log)))

	mux.Handle("GET /api/me", a.Require(http.HandlerFunc(me)))
	mux.Handle("GET /api/apps", a.Require(appsHandler(d.Apps)))
	mux.Handle("GET /api/apps/{id}/favicon", a.Require(faviconHandler(d.Apps, d.Icons)))
	api := &api{pictures: d.Pictures, links: d.Links, icons: d.Icons, log: d.Log}
	mux.Handle("POST /api/me/picture", a.Require(http.HandlerFunc(api.uploadPicture)))
	mux.Handle("DELETE /api/me/picture", a.Require(http.HandlerFunc(api.removePicture)))
	mux.Handle("GET /api/links", a.Require(http.HandlerFunc(api.listLinks)))
	mux.Handle("POST /api/links", a.Require(http.HandlerFunc(api.addLink)))
	mux.Handle("PUT /api/links/order", a.Require(http.HandlerFunc(api.reorderLinks)))
	mux.Handle("PUT /api/links/{id}", a.Require(http.HandlerFunc(api.updateLink)))
	mux.Handle("DELETE /api/links/{id}", a.Require(http.HandlerFunc(api.deleteLink)))
	mux.Handle("GET /api/links/{id}/favicon", a.Require(http.HandlerFunc(api.linkFavicon)))
	pg := &pagesAPI{pages: d.Pages, framing: d.Framing, hub: d.Hub, fail: api.fail}
	mux.Handle("GET /api/pages", a.Require(http.HandlerFunc(pg.list)))
	mux.Handle("POST /api/pages", a.Require(http.HandlerFunc(pg.add)))
	mux.Handle("PUT /api/pages/order", a.Require(http.HandlerFunc(pg.reorder)))
	mux.Handle("GET /api/pages/{id}", a.Require(http.HandlerFunc(pg.get)))
	mux.Handle("PUT /api/pages/{id}", a.Require(http.HandlerFunc(pg.update)))
	mux.Handle("DELETE /api/pages/{id}", a.Require(http.HandlerFunc(pg.delete)))
	mux.Handle("GET /api/", a.Require(http.NotFoundHandler()))
	mux.Handle("GET /", a.Require(spa(d.Frontend)))
	return securityHeaders(a.CheckOrigin(mux))
}

// me tells the app who is signed in and which language to show.
func me(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	writeJSON(w, map[string]any{
		"sub":      id.User.Sub,
		"name":     id.User.Name,
		"email":    id.User.Email,
		"username": id.User.Username,
		"picture":  id.User.Picture,
		"lang":     i18n.Pick(id.User.Locale, r.Header.Get("Accept-Language")),
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
			serveIcon(w, r, icons, a.URL)
			return
		}
		http.NotFound(w, r)
	})
}

// serveIcon sends a site's icon, or 404 when it has none (the page then
// shows a letter or the catalogue's icon).
func serveIcon(w http.ResponseWriter, r *http.Request, icons Favicons, site string) {
	icon, err := icons.Get(r.Context(), site)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", icon.Type)
	h.Set("Cache-Control", "private, max-age=86400")
	// Another site's file on our origin: an SVG opened directly must not
	// run scripts here.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Write(icon.Data)
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
