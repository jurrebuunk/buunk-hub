package server

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/catalogue"
	"github.com/jurrebuunk/buunk-hub/internal/favicon"
	"github.com/jurrebuunk/buunk-hub/internal/i18n"
	"github.com/jurrebuunk/buunk-hub/internal/kcadmin"
	"github.com/jurrebuunk/buunk-hub/internal/links"
	"github.com/jurrebuunk/buunk-hub/internal/pages"
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
func (f *fakeLinks) Get(_ context.Context, sub string, id int64) (links.Link, error) {
	f.subs = append(f.subs, sub)
	if id != 1 {
		return links.Link{}, links.ErrNotFound
	}
	return links.Link{ID: 1, Title: "Files", URL: "https://files.example.test/some/page"}, nil
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

// fakePages has page 1, which belongs to u1; any other id is someone
// else's (or nobody's). It records whose pages were asked for.
type fakePages struct {
	subs  []string
	saved []pages.Page
}

func (f *fakePages) List(_ context.Context, sub string) ([]pages.Page, error) {
	f.subs = append(f.subs, sub)
	return []pages.Page{{ID: 1, Title: "Issues", URL: "https://issues.example.test"}}, nil
}
func (f *fakePages) Get(_ context.Context, sub string, id int64) (pages.Page, error) {
	f.subs = append(f.subs, sub)
	if id != 1 || sub != "u1" {
		return pages.Page{}, pages.ErrNotFound
	}
	return pages.Page{ID: 1, Title: "Issues", URL: "https://issues.example.test"}, nil
}
func (f *fakePages) Add(_ context.Context, sub string, p pages.Page) (pages.Page, error) {
	f.subs = append(f.subs, sub)
	p.ID = 2
	f.saved = append(f.saved, p)
	return p, nil
}
func (f *fakePages) Update(_ context.Context, sub string, p pages.Page) error {
	f.subs = append(f.subs, sub)
	f.saved = append(f.saved, p)
	return nil
}
func (f *fakePages) Delete(_ context.Context, sub string, id int64) error {
	f.subs = append(f.subs, sub)
	if id != 1 {
		return pages.ErrNotFound
	}
	return nil
}
func (f *fakePages) Reorder(_ context.Context, sub string, _ []int64) error {
	f.subs = append(f.subs, sub)
	return nil
}

// fakeFraming: deny.example.test refuses framing, the rest allows it.
type fakeFraming struct{ checked []string }

func (f *fakeFraming) Check(_ context.Context, address string) string {
	f.checked = append(f.checked, address)
	if strings.HasPrefix(address, "https://deny.example.test") {
		return "X-Frame-Options: deny"
	}
	return ""
}

var testHub, _ = url.Parse("https://hub.example.test")

// fakeAccount is u1's Keycloak account; taken@example.test is someone else's.
type fakeAccount struct {
	subs []string
	set  []kcadmin.Profile
}

func (f *fakeAccount) Profile(_ context.Context, sub string) (kcadmin.Profile, error) {
	f.subs = append(f.subs, sub)
	return kcadmin.Profile{FirstName: "Ann", LastName: "Smit", Email: "ann@example.test", EmailVerified: true}, nil
}
func (f *fakeAccount) SetProfile(_ context.Context, sub string, p kcadmin.Profile) (bool, error) {
	f.subs = append(f.subs, sub)
	if p.Email == "taken@example.test" {
		return false, kcadmin.ErrEmailTaken
	}
	f.set = append(f.set, p)
	return p.Email != "ann@example.test", nil
}
func (f *fakeAccount) Groups(_ context.Context, sub string) ([]string, error) {
	f.subs = append(f.subs, sub)
	return []string{"family"}, nil
}

// fakeUsers keeps one person's hub-side settings.
type fakeUsers struct {
	subs                   []string
	name, email, lang, css string
	welcomed               bool
}

func (f *fakeUsers) SetProfile(_ context.Context, sub, name, email string) error {
	f.subs, f.name, f.email = append(f.subs, sub), name, email
	return nil
}
func (f *fakeUsers) SetLang(_ context.Context, sub, lang string) error {
	f.subs, f.lang = append(f.subs, sub), lang
	return nil
}
func (f *fakeUsers) SetWelcomed(_ context.Context, sub string) error {
	f.subs, f.welcomed = append(f.subs, sub), true
	return nil
}
func (f *fakeUsers) StartCSS(_ context.Context, sub string) (string, error) {
	f.subs = append(f.subs, sub)
	return f.css, nil
}
func (f *fakeUsers) SetStartCSS(_ context.Context, sub, css string) error {
	f.subs, f.css = append(f.subs, sub), css
	return nil
}

func testDeps(a fakeAuth, fe fs.FS) Deps {
	return Deps{Auth: a, Apps: testApps, Icons: fakeIcons{}, Pictures: &fakePictures{}, Links: &fakeLinks{},
		Pages: &fakePages{}, Framing: &fakeFraming{}, Hub: testHub, Account: &fakeAccount{}, Users: &fakeUsers{},
		Frontend: fe, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func newServer(a fakeAuth, fe fs.FS) http.Handler {
	return New(testDeps(a, fe))
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
	var got map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["sub"] != "u1" || got["name"] != "Ann" || got["lang"] != "nl" || got["welcomed"] != false {
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
	d := testDeps(fakeAuth{signedIn: true}, frontend)
	d.Links = fl
	h := New(d)
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
		// The icon of the link's site (its origin), only for one's own link.
		{"GET", "/api/links/1/favicon", "", 200},
		{"GET", "/api/links/2/favicon", "", 404},
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
	d := testDeps(fakeAuth{signedIn: true}, frontend)
	d.Pictures = fp
	h := New(d)
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

// Every page call is for the signed-in person (u1); someone else's page is
// not found; the framing check runs on add and on a new address only.
func TestPagesAPI(t *testing.T) {
	fp, ff := &fakePages{}, &fakeFraming{}
	d := testDeps(fakeAuth{signedIn: true}, frontend)
	d.Pages, d.Framing = fp, ff
	h := New(d)
	json := map[string]string{"Content-Type": "application/json"}
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/api/pages", "", 200},
		{"GET", "/api/pages/1", "", 200},
		// Someone else's page, by id.
		{"GET", "/api/pages/7", "", 404},
		{"PUT", "/api/pages/7", `{"title":"Mine","url":"https://evil.example.test"}`, 404},
		{"DELETE", "/api/pages/7", "", 404},
		{"POST", "/api/pages", `{"title":"Board","url":"board.example.test"}`, 201},
		{"POST", "/api/pages", `{"title":"GitHub","url":"https://deny.example.test"}`, 201},
		{"POST", "/api/pages", `{"title":"x","url":"http://plain.example.test"}`, 422},
		{"POST", "/api/pages", `{"title":"x","url":"https://hub.example.test/"}`, 422},
		{"POST", "/api/pages", `not json`, 400},
		// Rename: same address, no new check.
		{"PUT", "/api/pages/1", `{"title":"Job issues","url":"https://issues.example.test"}`, 200},
		{"PUT", "/api/pages/1", `{"title":"Job issues","url":"https://deny.example.test/x"}`, 200},
		{"PUT", "/api/pages/order", `{"ids":[1]}`, 204},
		{"DELETE", "/api/pages/1", "", 204},
	} {
		if rec := do(h, tc.method, tc.path, tc.body, json); rec.Code != tc.want {
			t.Errorf("%s %s: %d, want %d (%s)", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
		}
	}
	for _, sub := range fp.subs {
		if sub != "u1" {
			t.Errorf("a call for %q", sub)
		}
	}
	want := []string{"https://board.example.test", "https://deny.example.test", "https://deny.example.test/x"}
	if strings.Join(ff.checked, " ") != strings.Join(want, " ") {
		t.Errorf("checked %v, want %v", ff.checked, want)
	}
	if len(fp.saved) != 4 || fp.saved[0].Refused != "" || fp.saved[1].Refused != "X-Frame-Options: deny" ||
		fp.saved[2].Title != "Job issues" || fp.saved[3].Refused != "X-Frame-Options: deny" {
		t.Errorf("saved %+v", fp.saved)
	}
	if rec := get(t, newServer(fakeAuth{}, frontend), "/api/pages"); rec.Code != 401 {
		t.Errorf("signed out: %d", rec.Code)
	}
}

// The profile is u1's own; a changed email is reported, a taken one refused.
func TestProfileAPI(t *testing.T) {
	fa, fu := &fakeAccount{}, &fakeUsers{}
	d := testDeps(fakeAuth{signedIn: true}, frontend)
	d.Account, d.Users = fa, fu
	h := New(d)
	json := map[string]string{"Content-Type": "application/json"}
	rec := do(h, "GET", "/api/me/profile", "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"groups":["family"]`) {
		t.Errorf("get: %d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"firstName":" Ann ","lastName":"de Vries","email":"ann@example.test","lang":"nl"}`, 200},
		{`{"firstName":"Ann","lastName":"de Vries","email":"new@example.test","lang":""}`, 200},
		{`{"firstName":"Ann","lastName":"de Vries","email":"taken@example.test"}`, 409},
		{`{"firstName":"","lastName":"de Vries","email":"ann@example.test"}`, 422},
		{`{"firstName":"Ann","lastName":"x","email":"not an email"}`, 422},
		{`{"firstName":"Ann","lastName":"x","email":"Ann <ann@example.test>"}`, 422},
		{`{"firstName":"Ann","lastName":"x","email":"ann@example.test","lang":"de"}`, 422},
	} {
		if rec := do(h, "PUT", "/api/me/profile", tc.body, json); rec.Code != tc.want {
			t.Errorf("PUT %s: %d, want %d (%s)", tc.body, rec.Code, tc.want, rec.Body.String())
		}
	}
	if len(fa.set) != 2 || fa.set[0].FirstName != "Ann" || fu.name != "Ann de Vries" || fu.email != "new@example.test" ||
		fu.lang != "" || !fu.welcomed {
		t.Errorf("saved %+v %+v", fa.set, fu)
	}
	if rec := do(h, "POST", "/api/me/welcomed", "", nil); rec.Code != 204 {
		t.Errorf("welcomed: %d", rec.Code)
	}
	for _, sub := range append(fa.subs, fu.subs...) {
		if sub != "u1" {
			t.Errorf("a call for %q", sub)
		}
	}
}

// The style is cleaned when saved and lands on the start page.
func TestStartStyle(t *testing.T) {
	fu := &fakeUsers{}
	d := testDeps(fakeAuth{signedIn: true}, frontend)
	d.Users = fu
	h := New(d)
	json := map[string]string{"Content-Type": "application/json"}
	rec := do(h, "PUT", "/api/start/style", `{"css":".start{--fw-bg:#121a17;background:url(https://evil.test/x)} </style>"}`, json)
	if rec.Code != 200 || strings.Contains(fu.css, "evil.test") || strings.Contains(fu.css, "<") || !strings.Contains(fu.css, "--fw-bg:#121a17") {
		t.Errorf("save: %d %s, stored %q", rec.Code, rec.Body.String(), fu.css)
	}
	if rec := do(h, "PUT", "/api/start/style", `{"css":"`+strings.Repeat("a", 11<<10)+`"}`, json); rec.Code != 422 {
		t.Errorf("too large: %d", rec.Code)
	}
	body := get(t, h, "/start").Body.String()
	if !strings.Contains(body, "--fw-bg:#121a17") || strings.Count(body, "</style>") != 2 {
		t.Errorf("start page style:\n%s", body)
	}
	if rec := get(t, h, "/api/start/style"); !strings.Contains(rec.Body.String(), "--fw-bg") {
		t.Errorf("get: %s", rec.Body.String())
	}
}

func TestPageFavicon(t *testing.T) {
	fp := &fakePages{}
	d := testDeps(fakeAuth{signedIn: true}, frontend)
	d.Pages = fp
	d.Icons = fakeIconsFor("https://issues.example.test")
	h := New(d)
	if rec := get(t, h, "/api/pages/1/favicon"); rec.Code != 200 {
		t.Errorf("own page: %d", rec.Code)
	}
	if rec := get(t, h, "/api/pages/7/favicon"); rec.Code != 404 {
		t.Errorf("someone else's page: %d", rec.Code)
	}
}

// fakeIconsFor has an icon for one site only.
type fakeIconsFor string

func (f fakeIconsFor) Get(_ context.Context, site string) (favicon.Icon, error) {
	if site == string(f) {
		return favicon.Icon{Type: "image/png", Data: []byte("png")}, nil
	}
	return favicon.Icon{}, favicon.ErrNone
}
