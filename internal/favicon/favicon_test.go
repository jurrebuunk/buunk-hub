package favicon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

var png = []byte("\x89PNG\r\n\x1a\nrest")

func TestIconLinks(t *testing.T) {
	page := []byte(`<html><head>
		<link rel="stylesheet" href="/s.css">
		<link rel="shortcut icon" href="/static/img/favicon.ico" sizes="64x64">
		<link rel="icon" href="http://insecure.test/i.png">
		<link rel="apple-touch-icon" href="/apple.png">
		<link rel="icon" href="/small.png" sizes="16x16">
		</head><body><link rel="icon" href="/in-body.png"></body></html>`)
	base, _ := url.Parse("https://site.test/app/")
	got := IconLinks(page, base)
	want := []string{"https://site.test/apple.png", "https://site.test/static/img/favicon.ico", "https://site.test/small.png"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestImageType(t *testing.T) {
	for in, want := range map[string]string{
		string(png):                   "image/png",
		"\x00\x00\x01\x00rest":        "image/x-icon",
		" <svg xmlns='...'></svg>":    "image/svg+xml",
		"<!doctype html><html>":       "",
		"Hostname: whoami\nIP: 1.2.3": "",
	} {
		got, _ := imageType([]byte(in))
		if got != want {
			t.Errorf("imageType(%q) = %q, want %q", in, got, want)
		}
	}
}

func newCache(t *testing.T, mux *http.ServeMux) (*Cache, *httptest.Server, *int) {
	hits := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	c := NewCache(slog.New(slog.NewTextHandler(io.Discard, nil)))
	c.Client.Transport = srv.Client().Transport
	return c, srv, &hits
}

// Like Cloudreve: /favicon.ico is the HTML page, the icon is declared.
func TestDeclaredIcon(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><link rel="shortcut icon" href="/static/i.png"></head></html>`))
	})
	mux.HandleFunc("/static/i.png", func(w http.ResponseWriter, r *http.Request) { w.Write(png) })
	c, srv, hits := newCache(t, mux)
	icon, err := c.Get(context.Background(), srv.URL)
	if err != nil || icon.Type != "image/png" {
		t.Fatalf("got %v %v", icon.Type, err)
	}
	before := *hits
	c.Get(context.Background(), srv.URL)
	if *hits != before {
		t.Error("not cached")
	}
}

// Like whoami: text everywhere, so no icon; tried again after keepMiss.
func TestNoIcon(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("Hostname: whoami")) })
	c, srv, hits := newCache(t, mux)
	now := time.Now()
	c.now = func() time.Time { return now }
	if _, err := c.Get(context.Background(), srv.URL); !errors.Is(err, ErrNone) {
		t.Fatalf("err %v, want ErrNone", err)
	}
	before := *hits
	c.Get(context.Background(), srv.URL)
	if *hits != before {
		t.Error("miss not cached")
	}
	now = now.Add(keepMiss + time.Second)
	c.Get(context.Background(), srv.URL)
	if *hits == before {
		t.Error("miss not retried")
	}
}

func TestOnlyHTTPS(t *testing.T) {
	c := NewCache(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := c.Get(context.Background(), "http://site.test"); !errors.Is(err, ErrNone) {
		t.Errorf("http site: %v", err)
	}
}

// The real client only connects to public addresses.
func TestPublicOnly(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(png) }))
	defer srv.Close()
	c := NewCache(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := c.Get(context.Background(), srv.URL); !errors.Is(err, ErrNone) {
		t.Errorf("fetched from %s: %v", srv.URL, err)
	}
}
