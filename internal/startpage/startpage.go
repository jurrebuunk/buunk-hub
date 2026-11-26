// Package startpage renders /start: the light page a person sets as their
// browser's home or new-tab page. Server-rendered HTML with its CSS inline,
// no web fonts and no framework, so it shows at once; the clock is the only
// script and the page works without it.
package startpage

import (
	"bytes"
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/catalogue"
	"github.com/jurrebuunk/buunk-hub/internal/i18n"
	"github.com/jurrebuunk/buunk-hub/internal/links"
)

//go:embed start.html
var page string

var tmpl = template.Must(template.New("start").Parse(page))

type texts struct {
	Title, Search, SearchLabel, Apps, QuickLinks, Settings, NeedsWarp, NoLinks, AddLinks string
}

var allTexts = map[i18n.Lang]texts{
	i18n.EN: {
		Title: "Start", Search: "Search", SearchLabel: "Search with DuckDuckGo", Apps: "Apps",
		QuickLinks: "Quick links", Settings: "Start page settings, signed in as %s", NeedsWarp: "needs WARP",
		NoLinks: "No quick links yet.", AddLinks: "Add some",
	},
	i18n.NL: {
		Title: "Start", Search: "Zoeken", SearchLabel: "Zoeken met DuckDuckGo", Apps: "Apps",
		QuickLinks: "Snelle links", Settings: "Instellingen van de startpagina, ingelogd als %s", NeedsWarp: "WARP nodig",
		NoLinks: "Nog geen snelle links.", AddLinks: "Voeg er een paar toe",
	},
}

type app struct {
	ID, Name, URL, Letter string
	Internal              bool
}

type link struct {
	ID                       int64
	Title, URL, Host, Letter string
}

// Handler serves /start; it must run behind auth.Require.
// style returns the person's own CSS, already cleaned (internal/startcss).
func Handler(apps []catalogue.App, list func(r *http.Request, sub string) ([]links.Link, error),
	style func(r *http.Request, sub string) (string, error), log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := auth.FromContext(r.Context())
		lang := i18n.Pick(id.User.Language(), r.Header.Get("Accept-Language"))
		t := allTexts[lang]

		var mine []app
		for _, a := range apps {
			if a.VisibleTo(id.Roles) {
				mine = append(mine, app{a.ID, a.Name, a.URL, firstLetter(a.Name), a.Internal})
			}
		}
		stored, err := list(r, id.User.Sub)
		if err != nil {
			log.Error("start page links", "err", err)
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		var ls []link
		for _, l := range stored {
			host := l.URL
			if u, err := url.Parse(l.URL); err == nil {
				host = strings.TrimPrefix(u.Host, "www.")
			}
			ls = append(ls, link{l.ID, l.Title, l.URL, host, firstLetter(l.Title)})
		}
		css, err := style(r, id.User.Sub)
		if err != nil {
			// The page works without it.
			log.Warn("start page style", "err", err)
		}
		name := id.User.Name
		if name == "" {
			name = id.User.Email
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, map[string]any{
			"Lang": lang, "T": t, "Apps": mine, "Links": ls,
			"SettingsLabel": strings.Replace(t.Settings, "%s", name, 1),
			"Picture":       id.User.Picture, "Initials": initials(name),
			// Cleaned when saved: no "<", so it can't end the element, and the
			// CSP below loads nothing it names from elsewhere.
			"Style": template.CSS(css),
		}); err != nil {
			log.Error("start page", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "text/html; charset=utf-8")
		// Personal: never in a shared cache, always fresh.
		h.Set("Cache-Control", "private, no-cache")
		// Pictures come from this site; the form goes to DuckDuckGo only. The
		// Cloudflare logo in start.html is simple-icons' (CC0).
		h.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; "+
			"script-src 'unsafe-inline'; form-action https://duckduckgo.com; frame-ancestors 'none'; base-uri 'none'")
		w.Write(buf.Bytes())
	})
}

func firstLetter(s string) string {
	r, _ := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r))
}

func initials(name string) string {
	var out []rune
	for _, w := range strings.FieldsFunc(name, func(r rune) bool { return unicode.IsSpace(r) || r == '@' || r == '.' }) {
		r, _ := utf8.DecodeRuneInString(w)
		out = append(out, unicode.ToUpper(r))
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}
