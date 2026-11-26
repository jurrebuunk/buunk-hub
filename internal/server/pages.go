package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/pages"
)

// Pages keeps people's own pages (internal/pages); every call is for one
// person.
type Pages interface {
	List(ctx context.Context, sub string) ([]pages.Page, error)
	Get(ctx context.Context, sub string, id int64) (pages.Page, error)
	Add(ctx context.Context, sub string, p pages.Page) (pages.Page, error)
	Update(ctx context.Context, sub string, p pages.Page) error
	Delete(ctx context.Context, sub string, id int64) error
	Reorder(ctx context.Context, sub string, ids []int64) error
}

// Framing says which header stops a site from being framed ("" if none).
type Framing interface {
	Check(ctx context.Context, address string) string
}

type pagesAPI struct {
	pages   Pages
	framing Framing
	hub     *url.URL
	fail    func(w http.ResponseWriter, what string, err error)
}

func (a *pagesAPI) list(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	ps, err := a.pages.List(r.Context(), id.User.Sub)
	if err != nil {
		a.fail(w, "list pages", err)
		return
	}
	writeJSON(w, ps)
}

func (a *pagesAPI) get(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	n, ok := linkID(r)
	if !ok {
		apiError(w, http.StatusNotFound, "not-found")
		return
	}
	switch p, err := a.pages.Get(r.Context(), id.User.Sub, n); {
	case errors.Is(err, pages.ErrNotFound):
		apiError(w, http.StatusNotFound, "not-found")
	case err != nil:
		a.fail(w, "get page", err)
	default:
		writeJSON(w, p)
	}
}

// input reads and cleans {title, url}; false when it answered already.
func (a *pagesAPI) input(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	var in linkInput
	if readJSON(r, &in) != nil {
		apiError(w, http.StatusBadRequest, "bad-request")
		return "", "", false
	}
	title, address, err := pages.Clean(in.Title, in.URL, a.hub)
	if err != nil {
		apiError(w, http.StatusUnprocessableEntity, "invalid-page")
		return "", "", false
	}
	return title, address, true
}

func (a *pagesAPI) add(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	title, address, ok := a.input(w, r)
	if !ok {
		return
	}
	p := pages.Page{Title: title, URL: address, Refused: a.framing.Check(r.Context(), address)}
	p, err := a.pages.Add(r.Context(), id.User.Sub, p)
	if errors.Is(err, pages.ErrTooMany) {
		apiError(w, http.StatusUnprocessableEntity, "too-many-pages")
		return
	}
	if err != nil {
		a.fail(w, "add page", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, p)
}

// update renames a page or changes its address; a new address is checked
// for framing again.
func (a *pagesAPI) update(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	n, ok := linkID(r)
	if !ok {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	title, address, ok := a.input(w, r)
	if !ok {
		return
	}
	p, err := a.pages.Get(r.Context(), id.User.Sub, n)
	if errors.Is(err, pages.ErrNotFound) {
		apiError(w, http.StatusNotFound, "not-found")
		return
	}
	if err != nil {
		a.fail(w, "get page", err)
		return
	}
	if address != p.URL {
		p.Refused = a.framing.Check(r.Context(), address)
	}
	p.Title, p.URL = title, address
	switch err := a.pages.Update(r.Context(), id.User.Sub, p); {
	case errors.Is(err, pages.ErrNotFound):
		apiError(w, http.StatusNotFound, "not-found")
	case err != nil:
		a.fail(w, "update page", err)
	default:
		writeJSON(w, p)
	}
}

func (a *pagesAPI) delete(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	n, ok := linkID(r)
	if !ok {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	switch err := a.pages.Delete(r.Context(), id.User.Sub, n); {
	case errors.Is(err, pages.ErrNotFound):
		apiError(w, http.StatusNotFound, "not-found")
	case err != nil:
		a.fail(w, "delete page", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *pagesAPI) reorder(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if readJSON(r, &in) != nil {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	switch err := a.pages.Reorder(r.Context(), id.User.Sub, in.IDs); {
	case errors.Is(err, pages.ErrOrder):
		apiError(w, http.StatusConflict, "order")
	case err != nil:
		a.fail(w, "reorder pages", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
