package server

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/catalogue"
	"github.com/jurrebuunk/buunk-hub/internal/favicon"
	"github.com/jurrebuunk/buunk-hub/internal/i18n"
	"github.com/jurrebuunk/buunk-hub/internal/links"
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

// fakePictures and fakeLinks record what the handlers asked for.
type fakePictures struct{ uploads []string }

func (f *fakePictures) Upload(_ context.Context, sub, old string, r io.Reader) (string, error) {
	b, _ := io.ReadAll(r)
	f.uploads = append(f.uploads, sub+":"+string(b))
	return "https://hub.example.test/avatars/" + sub + "/new.jpg", nil
}
func (f *fakePictures) Remove(context.Context, string, string) error { return nil }
func (f *fakePictures) Serve(w http.ResponseWriter, r *http.Request) { w.Write([]byte("jpg")) }

type fakeLinks struct{ subs []string }

func (f *fakeLinks) List(_ context.Context, sub string) ([]links.Link, error) {
	f.subs = append(f.subs, sub)
	return []links.Link{{ID: 1, Title: "NOS", URL: "https://nos.nl"}}, nil
}
func (f *fakeLinks) Add(_ context.Context, sub, title, url string) (links.Link, error) {
	f.subs = append(f.subs, sub)
	return links.Link{ID: 2, Title: title, URL: url}, nil
}
func (f *fakeLinks) Update(_ context.Context, sub string, id int64, _, _ string) error {
	f.subs = append(f.subs, sub)
	if id != 1 {
		return links.ErrNotFound
	}
	return nil
}
func (f *fakeLinks) Delete(_ context.Context, sub string, _ int64) error {
	f.subs = append(f.subs, sub)
	return nil
}
func (f *fakeLinks) Reorder(_ context.Context, sub string, _ []int64) error {
	f.subs = append(f.subs, sub)
	return nil
}

func newServer(a fakeAuth, fe fs.FS) http.Handler {
	return New(Deps{Auth: a, Apps: testApps, Icons: fakeIcons{}, Pictures: &fakePictures{}, Links: &fakeLinks{},
		Frontend: fe, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
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
	rec := get(t, newServer(fakeAuth{}, frontend), "/healthz")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok\n" {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSignedOut(t *testing.T) {
	h := newServer(fakeAuth{}, frontend)
	for _, p := range []string{"/", "/settings", "/assets/app-1.js", "/api/me", "/api/apps", "/api/nothing"} {
		if rec := get(t, h, p); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: code %d, want 401", p, rec.Code)
		}
	}
}

func TestFrontend(t *testing.T) {
	h := newServer(fakeAuth{signedIn: true}, frontend)
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
	rec := get(t, newServer(fakeAuth{signedIn: true}, frontend), "/api/me")
	var got map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["sub"] != "u1" || got["name"] != "Ann" || got["lang"] != "nl" {
		t.Errorf("got %v", got)
	}
}

func TestNotBuilt(t *testing.T) {
	if rec := get(t, newServer(fakeAuth{signedIn: true}, fstest.MapFS{}), "/"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code %d, want 503", rec.Code)
	}
}

func TestOnlyGet(t *testing.T) {
	rec := httptest.NewRecorder()
	newServer(fakeAuth{signedIn: true}, frontend).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /: code %d, want 405", rec.Code)
	}
}

// Only apps the person has a role for, described in their language.
func TestApps(t *testing.T) {
	rec := get(t, newServer(fakeAuth{signedIn: true}, frontend), "/api/apps")
	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["id"] != "files" || got[0]["description"] != "Bestanden." || got[0]["internal"] != false {
		t.Errorf("got %v", got)
	}
}

func TestFavicon(t *testing.T) {
	h := newServer(fakeAuth{signedIn: true}, frontend)
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

func do(h http.Handler, method, target, body string, header map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	for k, v := range header {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// Every link call is for the signed-in person (u1), whatever the request says.
func TestLinksAPI(t *testing.T) {
	fl := &fakeLinks{}
	h := New(Deps{Auth: fakeAuth{signedIn: true}, Apps: testApps, Icons: fakeIcons{}, Pictures: &fakePictures{},
		Links: fl, Frontend: frontend, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	json := map[string]string{"Content-Type": "application/json"}
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/links", "", 200},
		{"POST", "/api/links", `{"title":"Weer","url":"buienradar.nl"}`, 201},
		{"POST", "/api/links", `{"title":"x","url":"javascript:alert(1)"}`, 422},
		{"POST", "/api/links", `not json`, 400},
		{"PUT", "/api/links/1", `{"title":"NOS","url":"https://nos.nl"}`, 200},
		{"PUT", "/api/links/9", `{"title":"NOS","url":"https://nos.nl"}`, 404},
		{"PUT", "/api/links/abc", `{"title":"NOS","url":"https://nos.nl"}`, 400},
		{"PUT", "/api/links/order", `{"ids":[1]}`, 204},
		{"DELETE", "/api/links/1", "", 204},
	} {
		if rec := do(h, tc.method, tc.path, tc.body, json); rec.Code != tc.want {
			t.Errorf("%s %s: %d, want %d (%s)", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
		}
	}
	for _, sub := range fl.subs {
		if sub != "u1" {
			t.Errorf("a call for %q", sub)
		}
	}
}

func TestPictureAPI(t *testing.T) {
	fp := &fakePictures{}
	h := New(Deps{Auth: fakeAuth{signedIn: true}, Apps: testApps, Icons: fakeIcons{}, Pictures: fp,
		Links: &fakeLinks{}, Frontend: frontend, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	body := "--x\r\nContent-Disposition: form-data; name=\"picture\"; filename=\"a.png\"\r\n\r\nPNGDATA\r\n--x--\r\n"
	rec := do(h, "POST", "/api/me/picture", body, map[string]string{"Content-Type": "multipart/form-data; boundary=x"})
	if rec.Code != 200 || len(fp.uploads) != 1 || fp.uploads[0] != "u1:PNGDATA" {
		t.Errorf("upload: %d %s %v", rec.Code, rec.Body.String(), fp.uploads)
	}
	if rec := do(h, "POST", "/api/me/picture", "", nil); rec.Code != 400 {
		t.Errorf("no file: %d", rec.Code)
	}
	if rec := do(h, "DELETE", "/api/me/picture", "", nil); rec.Code != 204 {
		t.Errorf("remove: %d", rec.Code)
	}
	// Pictures are public: no sign-in.
	if rec := do(newServer(fakeAuth{}, frontend), "GET", "/avatars/u1/0123456789abcdef0123456789abcdef.jpg", "", nil); rec.Code != 200 {
		t.Errorf("avatar signed out: %d", rec.Code)
	}
}

func TestStartPage(t *testing.T) {
	rec := get(t, newServer(fakeAuth{signedIn: true}, frontend), "/start")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `action="https://duckduckgo.com/"`) ||
		!strings.Contains(body, `href="https://nos.nl"`) || !strings.Contains(body, ">Files<") && !strings.Contains(body, "Files\n") {
		t.Errorf("start page: %d\n%s", rec.Code, body)
	}
	// Dutch (the person's locale), the person's apps only, and small.
	if !strings.Contains(body, `lang="nl"`) || strings.Contains(body, "Lab") || len(body) > 50<<10 {
		t.Errorf("lang, apps or size (%d bytes) wrong", len(body))
	}
	if !strings.Contains(rec.Header().Get("Content-Security-Policy"), "form-action https://duckduckgo.com") {
		t.Errorf("CSP %q", rec.Header().Get("Content-Security-Policy"))
	}
	if rec := get(t, newServer(fakeAuth{}, frontend), "/start"); rec.Code != 401 {
		t.Errorf("signed out: %d", rec.Code)
	}
}
