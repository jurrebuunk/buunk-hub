package pages

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jurrebuunk/buunk-hub/internal/db"
)

var hub, _ = url.Parse("https://hub.buunk.org")

func TestClean(t *testing.T) {
	for _, tc := range []struct{ title, url, wantURL string }{
		{" Job issues ", "issues.example.com", "https://issues.example.com"},
		{"Board", "https://board.example.com/x?y=1", "https://board.example.com/x?y=1"},
	} {
		title, u, err := Clean(tc.title, tc.url, hub)
		if err != nil || u != tc.wantURL || strings.TrimSpace(tc.title) != title {
			t.Errorf("Clean(%q, %q) = %q, %q, %v", tc.title, tc.url, title, u, err)
		}
	}
	for _, bad := range [][2]string{
		{"", "https://example.com"},
		{"x", "http://example.com"},
		{"x", "javascript:alert(1)"},
		{"x", "javascript://%0aalert(1)"},
		{"x", "data:text/html,hi"},
		{"x", "https://"},
		{"x", "https://user:pw@example.com"},
		// The hub itself: framed with allow-same-origin it could unsandbox.
		{"x", "https://hub.buunk.org/settings"},
		{"x", "HUB.buunk.org"},
		{string(make([]rune, maxTitle+1)), "example.com"},
	} {
		if _, _, err := Clean(bad[0], bad[1], hub); err == nil {
			t.Errorf("Clean(%q, %q) accepted", bad[0], bad[1])
		}
	}
}

func TestRefusal(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers map[string][]string
		refused bool
	}{
		{"nothing", nil, false},
		{"xfo deny", map[string][]string{"X-Frame-Options": {"DENY"}}, true},
		{"xfo sameorigin", map[string][]string{"X-Frame-Options": {"sameorigin"}}, true},
		{"xfo invalid is ignored", map[string][]string{"X-Frame-Options": {"ALLOW-FROM https://hub.buunk.org"}}, false},
		{"none", map[string][]string{"Content-Security-Policy": {"default-src 'self'; frame-ancestors 'none'"}}, true},
		{"self", map[string][]string{"Content-Security-Policy": {"frame-ancestors 'self'"}}, true},
		{"star", map[string][]string{"Content-Security-Policy": {"frame-ancestors *"}}, false},
		{"https scheme", map[string][]string{"Content-Security-Policy": {"frame-ancestors https:"}}, false},
		{"our host", map[string][]string{"Content-Security-Policy": {"frame-ancestors 'self' https://hub.buunk.org"}}, false},
		{"wildcard", map[string][]string{"Content-Security-Policy": {"frame-ancestors *.buunk.org"}}, false},
		{"bare domain is no wildcard", map[string][]string{"Content-Security-Policy": {"frame-ancestors *.hub.buunk.org"}}, true},
		{"other host", map[string][]string{"Content-Security-Policy": {"frame-ancestors https://example.com"}}, true},
		{"other port", map[string][]string{"Content-Security-Policy": {"frame-ancestors hub.buunk.org:8443"}}, true},
		{"port star", map[string][]string{"Content-Security-Policy": {"frame-ancestors hub.buunk.org:*"}}, false},
		// frame-ancestors wins over X-Frame-Options, as in browsers.
		{"csp allows over xfo", map[string][]string{"Content-Security-Policy": {"frame-ancestors *"}, "X-Frame-Options": {"DENY"}}, false},
		// Every policy is enforced: one refusing is enough.
		{"two policies", map[string][]string{"Content-Security-Policy": {"frame-ancestors *, frame-ancestors 'none'"}}, true},
		{"two headers", map[string][]string{"Content-Security-Policy": {"frame-ancestors *", "frame-ancestors https://example.com"}}, true},
		{"csp without frame-ancestors", map[string][]string{"Content-Security-Policy": {"default-src 'none'"}, "X-Frame-Options": {"deny"}}, true},
	} {
		h := http.Header{}
		for k, vs := range tc.headers {
			for _, v := range vs {
				h.Add(k, v)
			}
		}
		if got := Refusal(h, hub); (got != "") != tc.refused {
			t.Errorf("%s: Refusal = %q, want refused %v", tc.name, got, tc.refused)
		}
	}
}

func TestCheck(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/deny":
			w.Header().Set("X-Frame-Options", "DENY")
		case "/moved":
			http.Redirect(w, r, "/deny", http.StatusFound)
			return
		}
		w.Write([]byte("hi"))
	}))
	defer srv.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	f := NewFraming(hub, log)
	f.Client.Transport = srv.Client().Transport
	ctx := context.Background()
	if got := f.Check(ctx, srv.URL+"/"); got != "" {
		t.Errorf("open site refused: %q", got)
	}
	if got := f.Check(ctx, srv.URL+"/deny"); got != "X-Frame-Options: deny" {
		t.Errorf("deny: %q", got)
	}
	// After redirects, as the frame would see it.
	if got := f.Check(ctx, srv.URL+"/moved"); got != "X-Frame-Options: deny" {
		t.Errorf("redirect to deny: %q", got)
	}
	// The real client doesn't reach a local address: not checked, embedded.
	if got := NewFraming(hub, log).Check(ctx, srv.URL+"/deny"); got != "" {
		t.Errorf("local address: %q", got)
	}
}

// Against a real Postgres (HUB_TEST_DATABASE_URL), in a schema of its own.
func testStore(t *testing.T) Store {
	url := os.Getenv("HUB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("HUB_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	b := make([]byte, 6)
	rand.Read(b)
	schema := fmt.Sprintf("test_%x", b)
	setup, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	setup.Close()
	pool, err := db.Open(ctx, url+"?search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		pool.Close()
	})
	for _, sub := range []string{"ann", "bob"} {
		if _, err := pool.Exec(ctx, "INSERT INTO users (sub) VALUES ($1)", sub); err != nil {
			t.Fatal(err)
		}
	}
	return Store{DB: pool}
}

func TestStoreOwnership(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a1, err := s.Add(ctx, "ann", Page{Title: "Job issues", URL: "https://issues.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := s.Add(ctx, "ann", Page{Title: "GitHub", URL: "https://github.com", Refused: "X-Frame-Options: deny"})
	b1, _ := s.Add(ctx, "bob", Page{Title: "Bob's", URL: "https://bob.example.com"})

	// Bob can't open, change, delete or reorder Ann's pages, even knowing their ids.
	if _, err := s.Get(ctx, "bob", a1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob opened ann's page: %v", err)
	}
	if got, _ := s.List(ctx, "bob"); len(got) != 1 || got[0].ID != b1.ID {
		t.Errorf("bob sees %v", got)
	}
	if err := s.Update(ctx, "bob", Page{ID: a1.ID, Title: "mine", URL: "https://evil.example"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob updated ann's page: %v", err)
	}
	if err := s.Delete(ctx, "bob", a1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob deleted ann's page: %v", err)
	}
	if err := s.Reorder(ctx, "bob", []int64{a1.ID, b1.ID}); !errors.Is(err, ErrOrder) {
		t.Errorf("bob reordered with ann's id: %v", err)
	}
	if p, err := s.Get(ctx, "ann", a2.ID); err != nil || p.Refused != "X-Frame-Options: deny" {
		t.Errorf("ann's own page: %v %v", p, err)
	}

	// Ann reorders, renames and deletes her own.
	if err := s.Reorder(ctx, "ann", []int64{a2.ID, a1.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(ctx, "ann", Page{ID: a1.ID, Title: "Issues", URL: a1.URL}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.List(ctx, "ann")
	if len(got) != 2 || got[0].ID != a2.ID || got[1].Title != "Issues" {
		t.Errorf("after reorder and rename: %v", got)
	}
	if err := s.Delete(ctx, "ann", a2.ID); err != nil {
		t.Fatal(err)
	}
	a3, _ := s.Add(ctx, "ann", Page{Title: "Last", URL: "https://last.example.com"})
	if got, _ := s.List(ctx, "ann"); len(got) != 2 || got[1].ID != a3.ID {
		t.Errorf("new page not last: %v", got)
	}
}

func TestStoreMax(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for i := 0; i < MaxPages; i++ {
		if _, err := s.Add(ctx, "ann", Page{Title: fmt.Sprint(i), URL: "https://example.com"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Add(ctx, "ann", Page{Title: "one too many", URL: "https://example.com"}); !errors.Is(err, ErrTooMany) {
		t.Errorf("page %d: %v", MaxPages+1, err)
	}
}
