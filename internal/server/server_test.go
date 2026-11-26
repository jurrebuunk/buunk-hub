package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

var frontend = fstest.MapFS{
	"index.html":      {Data: []byte("<!doctype html>app")},
	"assets/app-1.js": {Data: []byte("js")},
	"favicon.svg":     {Data: []byte("<svg/>")},
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	rec := get(t, New(frontend), "/healthz")
	if rec.Code != http.StatusOK || rec.Body.String() != "ok\n" {
		t.Errorf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestFrontend(t *testing.T) {
	h := New(frontend)
	for _, tc := range []struct {
		path, body, cache string
		code              int
	}{
		{"/", "<!doctype html>app", "no-cache", 200},
		{"/settings", "<!doctype html>app", "no-cache", 200},
		{"/assets/app-1.js", "js", "public, max-age=31536000, immutable", 200},
		{"/favicon.svg", "<svg/>", "", 200},
		{"/assets/gone.js", "", "", 404},
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

func TestNotBuilt(t *testing.T) {
	if rec := get(t, New(fstest.MapFS{}), "/"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code %d, want 503", rec.Code)
	}
}

func TestOnlyGet(t *testing.T) {
	rec := httptest.NewRecorder()
	New(frontend).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /: code %d, want 405", rec.Code)
	}
}
