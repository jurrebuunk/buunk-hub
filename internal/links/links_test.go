package links

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jurrebuunk/buunk-hub/internal/db"
)

func TestClean(t *testing.T) {
	for _, tc := range []struct{ title, url, wantURL string }{
		{" NOS ", "nos.nl", "https://nos.nl"},
		{"Lab", "http://lab.example.test/x?y=1", "http://lab.example.test/x?y=1"},
	} {
		title, u, err := Clean(tc.title, tc.url)
		if err != nil || u != tc.wantURL || title != "NOS" && title != "Lab" {
			t.Errorf("Clean(%q, %q) = %q, %q, %v", tc.title, tc.url, title, u, err)
		}
	}
	for _, bad := range [][2]string{
		{"", "https://nos.nl"},
		{"x", "javascript:alert(1)"},
		{"x", "javascript://%0aalert(1)"},
		{"x", "data:text/html,hi"},
		{"x", "ftp://files.example.test"},
		{"x", "https://"},
		{string(make([]rune, maxTitle+1)), "nos.nl"},
	} {
		if _, _, err := Clean(bad[0], bad[1]); err == nil {
			t.Errorf("Clean(%q, %q) accepted", bad[0], bad[1])
		}
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
	a1, err := s.Add(ctx, "ann", "NOS", "https://nos.nl")
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := s.Add(ctx, "ann", "Buienradar", "https://buienradar.nl")
	b1, _ := s.Add(ctx, "bob", "Bob's", "https://bob.example.test")

	if _, err := s.Get(ctx, "bob", a1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob got ann's link: %v", err)
	}
	if l, err := s.Get(ctx, "ann", a1.ID); err != nil || l.URL != "https://nos.nl" {
		t.Errorf("ann's own link: %v %v", l, err)
	}
	if got, _ := s.List(ctx, "bob"); len(got) != 1 || got[0].ID != b1.ID {
		t.Errorf("bob sees %v", got)
	}
	// Bob can't touch Ann's links, even knowing their ids.
	if err := s.Update(ctx, "bob", a1.ID, "mine", "https://evil.test"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob updated ann's link: %v", err)
	}
	if err := s.Delete(ctx, "bob", a1.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("bob deleted ann's link: %v", err)
	}
	if err := s.Reorder(ctx, "bob", []int64{a1.ID, b1.ID}); !errors.Is(err, ErrOrder) {
		t.Errorf("bob reordered with ann's id: %v", err)
	}
	if got, _ := s.List(ctx, "ann"); len(got) != 2 || got[0].Title != "NOS" {
		t.Errorf("ann's links changed: %v", got)
	}

	// Ann reorders, edits and removes her own.
	if err := s.Reorder(ctx, "ann", []int64{a2.ID, a1.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.Reorder(ctx, "ann", []int64{a2.ID, a2.ID}); !errors.Is(err, ErrOrder) {
		t.Errorf("duplicate ids: %v", err)
	}
	if err := s.Update(ctx, "ann", a1.ID, "NOS nieuws", "https://nos.nl/nieuws"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.List(ctx, "ann")
	if len(got) != 2 || got[0].ID != a2.ID || got[1].Title != "NOS nieuws" {
		t.Errorf("after reorder and edit: %v", got)
	}
	if err := s.Delete(ctx, "ann", a2.ID); err != nil {
		t.Fatal(err)
	}
	// A new link goes at the end.
	a3, _ := s.Add(ctx, "ann", "Last", "https://last.example.test")
	if got, _ := s.List(ctx, "ann"); len(got) != 2 || got[1].ID != a3.ID {
		t.Errorf("new link not last: %v", got)
	}
}

func TestStoreMax(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	for i := 0; i < MaxLinks; i++ {
		if _, err := s.Add(ctx, "ann", fmt.Sprint(i), "https://example.test"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Add(ctx, "ann", "one too many", "https://example.test"); !errors.Is(err, ErrTooMany) {
		t.Errorf("link %d: %v", MaxLinks+1, err)
	}
}
