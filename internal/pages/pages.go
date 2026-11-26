// Package pages keeps a person's own pages ("My pages"): each embeds one
// site inside the hub. Every query is by the person's sub, so nobody reads
// or changes another's.
package pages

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MaxPages = 50
	maxTitle = 80
	maxURL   = 2048
)

var (
	ErrNotFound = errors.New("no such page")
	ErrTooMany  = fmt.Errorf("at most %d pages", MaxPages)
	ErrOrder    = errors.New("the order must name each of your pages once")
)

type Page struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	// Refused is the header that stops the site from being framed, as seen
	// when the page was saved ("X-Frame-Options: deny"); empty when the page
	// embeds it.
	Refused string `json:"refused"`
}

// Clean checks a title and address and returns them tidied. An address
// without a scheme gets https://. Only https: the hub is https, and a
// browser won't show an http frame in it. Not the hub itself: framed with
// allow-scripts and allow-same-origin, our own page could lift its sandbox.
func Clean(title, address string, hub *url.URL) (string, string, error) {
	title = strings.TrimSpace(title)
	address = strings.TrimSpace(address)
	if title == "" || utf8.RuneCountInString(title) > maxTitle {
		return "", "", fmt.Errorf("a title of 1 to %d characters", maxTitle)
	}
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || len(address) > maxURL {
		return "", "", errors.New("an https:// address")
	}
	if strings.EqualFold(u.Hostname(), hub.Hostname()) {
		return "", "", errors.New("not the hub itself")
	}
	return title, u.String(), nil
}

type Store struct{ DB *pgxpool.Pool }

func (s Store) List(ctx context.Context, sub string) ([]Page, error) {
	rows, err := s.DB.Query(ctx, "SELECT id, title, url, refused FROM pages WHERE sub = $1 ORDER BY position, id", sub)
	if err != nil {
		return nil, err
	}
	out := []Page{}
	for rows.Next() {
		var p Page
		if err := rows.Scan(&p.ID, &p.Title, &p.URL, &p.Refused); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get returns one of the person's pages, or ErrNotFound.
func (s Store) Get(ctx context.Context, sub string, id int64) (Page, error) {
	p := Page{ID: id}
	err := s.DB.QueryRow(ctx, "SELECT title, url, refused FROM pages WHERE sub = $1 AND id = $2", sub, id).
		Scan(&p.Title, &p.URL, &p.Refused)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// Add puts a new page at the end of the person's pages.
func (s Store) Add(ctx context.Context, sub string, p Page) (Page, error) {
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		// One add at a time per person, so the count and position hold.
		if _, err := tx.Exec(ctx, "SELECT 1 FROM users WHERE sub = $1 FOR UPDATE", sub); err != nil {
			return err
		}
		var n, next int
		if err := tx.QueryRow(ctx, "SELECT count(*), coalesce(max(position), 0) + 1 FROM pages WHERE sub = $1", sub).Scan(&n, &next); err != nil {
			return err
		}
		if n >= MaxPages {
			return ErrTooMany
		}
		return tx.QueryRow(ctx, "INSERT INTO pages (sub, position, title, url, refused) VALUES ($1, $2, $3, $4, $5) RETURNING id",
			sub, next, p.Title, p.URL, p.Refused).Scan(&p.ID)
	})
	return p, err
}

// Update sets a page's title, address and framing result.
func (s Store) Update(ctx context.Context, sub string, p Page) error {
	tag, err := s.DB.Exec(ctx, "UPDATE pages SET title = $3, url = $4, refused = $5 WHERE sub = $1 AND id = $2",
		sub, p.ID, p.Title, p.URL, p.Refused)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s Store) Delete(ctx context.Context, sub string, id int64) error {
	tag, err := s.DB.Exec(ctx, "DELETE FROM pages WHERE sub = $1 AND id = $2", sub, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// Reorder sets the order; ids must be exactly the person's pages.
func (s Store) Reorder(ctx context.Context, sub string, ids []int64) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, "SELECT id FROM pages WHERE sub = $1 FOR UPDATE", sub)
		if err != nil {
			return err
		}
		have, err := pgx.CollectRows(rows, pgx.RowTo[int64])
		if err != nil {
			return err
		}
		if !sameSet(have, ids) {
			return ErrOrder
		}
		for i, id := range ids {
			if _, err := tx.Exec(ctx, "UPDATE pages SET position = $3 WHERE sub = $1 AND id = $2", sub, id, i+1); err != nil {
				return err
			}
		}
		return nil
	})
}

func sameSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[int64]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		if seen[x] != 1 {
			return false
		}
		seen[x]--
	}
	return true
}
