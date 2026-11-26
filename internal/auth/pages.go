package auth

import (
	"html/template"
	"net/http"

	"github.com/jurrebuunk/buunk-hub/internal/i18n"
)

// The few pages the server renders itself, around signing in. Plain HTML
// with the flatwork-saas colours (its tokens.json), so they work without
// the React app.

type page int

const (
	pageSignedOut page = iota
	pageFailed
)

type pageText struct {
	Title, Text, Action string
}

var pageTexts = map[i18n.Lang]map[page]pageText{
	i18n.EN: {
		pageSignedOut: {"You're signed out", "You have been signed out of the hub and your buunk account.", "Sign in again"},
		pageFailed:    {"Signing in didn't work", "Something went wrong while signing in, or it took too long. Please try again.", "Try again"},
	},
	i18n.NL: {
		pageSignedOut: {"Je bent uitgelogd", "Je bent uitgelogd bij de hub en je buunk-account.", "Opnieuw inloggen"},
		pageFailed:    {"Inloggen is niet gelukt", "Er ging iets mis bij het inloggen, of het duurde te lang. Probeer het opnieuw.", "Opnieuw proberen"},
	},
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="dark light">
<title>{{.Title}} · buunk hub</title>
<style>
:root{--bg:#1b1818;--surface:#221e1e;--border:#3a3434;--heading:#ece9e9;--text:#c4c1c1;--muted:#9a9595;--accent:#8aa1a8;--on-accent:#141212;--focus:#a5bbc2}
@media (prefers-color-scheme:light){:root{--bg:#f6f5f5;--surface:#fff;--border:#e4e0e0;--heading:#1b1818;--text:#3a3535;--muted:#625c5c;--accent:#4b6670;--on-accent:#fff;--focus:#4b6670}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:grid;place-items:center;padding:16px;background:var(--bg);color:var(--text);font:15px/1.5 ui-sans-serif,system-ui,sans-serif}
main{max-width:400px;width:100%;padding:32px;background:var(--surface);border:1px solid var(--border);border-radius:12px}
h1{margin:0 0 8px;font-size:20px;font-weight:600;color:var(--heading)}
p{margin:0 0 24px;color:var(--muted)}
a{display:inline-block;padding:8px 16px;border-radius:6px;background:var(--accent);color:var(--on-accent);text-decoration:none;font-weight:500}
a:focus-visible{outline:2px solid var(--focus);outline-offset:2px}
</style>
</head>
<body>
<main>
<h1>{{.Title}}</h1>
<p>{{.Text}}</p>
<a href="/auth/login">{{.Action}}</a>
</main>
</body>
</html>
`))

func renderPage(w http.ResponseWriter, r *http.Request, status int, p page) {
	lang := i18n.Pick("", r.Header.Get("Accept-Language"))
	t := pageTexts[lang][p]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	pageTemplate.Execute(w, struct {
		Lang i18n.Lang
		pageText
	}{lang, t})
}
