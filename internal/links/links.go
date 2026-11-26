// Package links keeps a person's quick links for their start page. Every
// query is by the person's sub, so nobody reads or changes another's.
package links

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
	MaxLinks = 50
	maxTitle = 80
	maxURL   = 2048
)

var (
	ErrNotFound = errors.New("no such link")
	ErrTooMany  = fmt.Errorf("at most %d links", MaxLinks)
	ErrOrder    = errors.New("the order must name each of your links once")
)

type Link struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Clean checks a title and address and returns them tidied. An address
// without a scheme gets https://; only http and https are allowed, so a
// link can't run script (javascript:) on the start page.
func Clean(title, address string) (string, string, error) {
	title = strings.TrimSpace(title)
	address = strings.TrimSpace(address)
	if title == "" || utf8.RuneCountInString(title) > maxTitle {
		return "", "", fmt.Errorf("a title of 1 to %d characters", maxTitle)
	}
	if !strings.Contains(address, "://") {
		address = "https://" + address
	}
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(address) > maxURL {
		return "", "", errors.New("an http:// or https:// address")
	}
	return title, u.String(), nil
}

type Store struct{ DB *pgxpool.Pool }

func (s Store) List(ctx context.Context, sub string) ([]Link, error) {
	rows, err := s.DB.Query(ctx, "SELECT id, title, url FROM quick_links WHERE sub = $1 ORDER BY position, id", sub)
	if err != nil {
		return nil, err
	}
	out := []Link{}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.Title, &l.URL); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Get returns one of the person's links, or ErrNotFound.
func (s Store) Get(ctx context.Context, sub string, id int64) (Link, error) {
	l := Link{ID: id}
	err := s.DB.QueryRow(ctx, "SELECT title, url FROM quick_links WHERE sub = $1 AND id = $2", sub, id).Scan(&l.Title, &l.URL)
	if errors.Is(err, pgx.ErrNoRows) {
		return l, ErrNotFound
	}
	return l, err
}

// Add puts a new link at the end.
func (s Store) Add(ctx context.Context, sub, title, address string) (Link, error) {
	l := Link{Title: title, URL: address}
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		// One add at a time per person, so the count and position hold.
		if _, err := tx.Exec(ctx, "SELECT 1 FROM users WHERE sub = $1 FOR UPDATE", sub); err != nil {
			return err
		}
		var n, next int
		if err := tx.QueryRow(ctx, "SELECT count(*), coalesce(max(position), 0) + 1 FROM quick_links WHERE sub = $1", sub).Scan(&n, &next); err != nil {
			return err
		}
		if n >= MaxLinks {
			return ErrTooMany
		}
		return tx.QueryRow(ctx, "INSERT INTO quick_links (sub, position, title, url) VALUES ($1, $2, $3, $4) RETURNING id",
			sub, next, title, address).Scan(&l.ID)
	})
	return l, err
}

func (s Store) Update(ctx context.Context, sub string, id int64, title, address string) error {
	tag, err := s.DB.Exec(ctx, "UPDATE quick_links SET title = $3, url = $4 WHERE sub = $1 AND id = $2", sub, id, title, address)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s Store) Delete(ctx context.Context, sub string, id int64) error {
	tag, err := s.DB.Exec(ctx, "DELETE FROM quick_links WHERE sub = $1 AND id = $2", sub, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

// Reorder sets the order; ids must be exactly the person's links.
func (s Store) Reorder(ctx context.Context, sub string, ids []int64) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		var have []int64
		rows, err := tx.Query(ctx, "SELECT id FROM quick_links WHERE sub = $1 FOR UPDATE", sub)
		if err != nil {
			return err
		}
		have, err = pgx.CollectRows(rows, pgx.RowTo[int64])
		if err != nil {
			return err
		}
		if !sameSet(have, ids) {
			return ErrOrder
		}
		for i, id := range ids {
			if _, err := tx.Exec(ctx, "UPDATE quick_links SET position = $3 WHERE sub = $1 AND id = $2", sub, id, i+1); err != nil {
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
