package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/catalogue"
	"github.com/jurrebuunk/buunk-hub/internal/favicon"
	"github.com/jurrebuunk/buunk-hub/internal/i18n"
)

var testApps = []catalogue.App{
	{ID: "files", Name: "Files", URL: "https://files.example.test", Icon: "cloud",
		Description: map[i18n.Lang]string{i18n.EN: "Files.", i18n.NL: "Bestanden."},
		Roles:       []string{"files/user"}},
	{ID: "lab", Name: "Lab", URL: "https://lab.example.test", Icon: "terminal",
		Description: map[i18n.Lang]string{i18n.EN: "Lab.", i18n.NL: "Lab."},
		Roles:       []string{"warp/warp"}, Internal: true},
}

var frontend = fstest.MapFS{
	"index.html":      {Data: []byte("<!doctype html>app")},
	"assets/app-1.js": {Data: []byte("js")},
	"favicon.svg":     {Data: []byte("<svg/>")},
}

// fakeIcons has an icon for files.example.test only.
type fakeIcons struct{}

func (fakeIcons) Get(_ context.Context, site string) (favicon.Icon, error) {
	if site == "https://files.example.test" {
		return favicon.Icon{Type: "image/png", Data: []byte("png")}, nil
	}
	return favicon.Icon{}, favicon.ErrNone
}

// fakeAuth lets everyone in as one person, or no one.
type fakeAuth struct{ signedIn bool }

func (fakeAuth) Routes(*http.ServeMux)                   {}
func (fakeAuth) CheckOrigin(h http.Handler) http.Handler { return h }
func (f fakeAuth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !f.signedIn {
			http.Error(w, "signed out", http.StatusUnauthorized)
			return
		}
		id := auth.Identity{User: auth.User{Sub: "u1", Name: "Ann", Locale: "nl"},
			Roles: auth.Roles{"files": {"user"}, "other": {"warp"}}}
		next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), id)))
	})
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	rec := get(t, New(fakeAuth{}, testApps, fakeIcons{}, frontend), "/healthz")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok\n" {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSignedOut(t *testing.T) {
	h := New(fakeAuth{}, testApps, fakeIcons{}, frontend)
	for _, p := range []string{"/", "/settings", "/assets/app-1.js", "/api/me", "/api/apps", "/api/nothing"} {
		if rec := get(t, h, p); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: code %d, want 401", p, rec.Code)
		}
	}
}

func TestFrontend(t *testing.T) {
	h := New(fakeAuth{signedIn: true}, testApps, fakeIcons{}, frontend)
	for _, tc := range []struct {
		path, body, cache string
		code              int
	}{
		{"/", "<!doctype html>app", "no-cache", 200},
		{"/settings", "<!doctype html>app", "no-cache", 200},
		{"/assets/app-1.js", "js", "public, max-age=31536000, immutable", 200},
		{"/favicon.svg", "<svg/>", "", 200},
		{"/assets/gone.js", "", "", 404},
		{"/api/nothing", "", "", 404},
	} {
		rec := get(t, h, tc.path)
		if rec.Code != tc.code {
			t.Errorf("%s: code %d, want %d", tc.path, rec.Code, tc.code)
			continue
		}
		if tc.body != "" && rec.Body.String() != tc.body {
			t.Errorf("%s: body %q", tc.path, rec.Body.String())
		}
		if got := rec.Header().Get("Cache-Control"); got != tc.cache {
			t.Errorf("%s: Cache-Control %q, want %q", tc.path, got, tc.cache)
		}
		if rec.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("%s: no X-Frame-Options", tc.path)
		}
	}
}

func TestMe(t *testing.T) {
	rec := get(t, New(fakeAuth{signedIn: true}, testApps, fakeIcons{}, frontend), "/api/me")
	var got map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["sub"] != "u1" || got["name"] != "Ann" || got["lang"] != "nl" {
		t.Errorf("got %v", got)
	}
}

func TestNotBuilt(t *testing.T) {
	if rec := get(t, New(fakeAuth{signedIn: true}, testApps, fakeIcons{}, fstest.MapFS{}), "/"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code %d, want 503", rec.Code)
	}
}

func TestOnlyGet(t *testing.T) {
	rec := httptest.NewRecorder()
	New(fakeAuth{signedIn: true}, testApps, fakeIcons{}, frontend).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /: code %d, want 405", rec.Code)
	}
}

// Only apps the person has a role for, described in their language.
func TestApps(t *testing.T) {
	rec := get(t, New(fakeAuth{signedIn: true}, testApps, fakeIcons{}, frontend), "/api/apps")
	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["id"] != "files" || got[0]["description"] != "Bestanden." || got[0]["internal"] != false {
		t.Errorf("got %v", got)
	}
}

func TestFavicon(t *testing.T) {
	h := New(fakeAuth{signedIn: true}, testApps, fakeIcons{}, frontend)
	rec := get(t, h, "/api/apps/files/favicon")
	if rec.Code != 200 || rec.Body.String() != "png" || rec.Header().Get("Content-Type") != "image/png" ||
		!strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("files: %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	// lab has no icon; nobody here has its role either way; unknown app.
	for _, p := range []string{"/api/apps/lab/favicon", "/api/apps/nope/favicon"} {
		if rec := get(t, h, p); rec.Code != 404 {
			t.Errorf("%s: %d, want 404", p, rec.Code)
		}
	}
}
