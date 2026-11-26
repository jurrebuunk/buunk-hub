package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/avatar"
	"github.com/jurrebuunk/buunk-hub/internal/links"
)

// Pictures sets profile pictures (internal/picture).
type Pictures interface {
	Upload(ctx context.Context, sub, old string, r io.Reader) (string, error)
	Remove(ctx context.Context, sub, old string) error
	Serve(w http.ResponseWriter, r *http.Request)
}

// Links keeps quick links (internal/links); every call is for one person.
type Links interface {
	List(ctx context.Context, sub string) ([]links.Link, error)
	Add(ctx context.Context, sub, title, url string) (links.Link, error)
	Update(ctx context.Context, sub string, id int64, title, url string) error
	Delete(ctx context.Context, sub string, id int64) error
	Reorder(ctx context.Context, sub string, ids []int64) error
}

type api struct {
	pictures Pictures
	links    Links
	log      *slog.Logger
}

// apiError answers {"error": code}; the app shows the message for code in
// the person's language.
func apiError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func (a *api) fail(w http.ResponseWriter, what string, err error) {
	a.log.Error(what, "err", err)
	apiError(w, http.StatusInternalServerError, "internal")
}

func (a *api) uploadPicture(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	// The file plus a little room for the form around it.
	r.Body = http.MaxBytesReader(w, r.Body, avatar.MaxUpload+64<<10)
	file, _, err := r.FormFile("picture")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			apiError(w, http.StatusRequestEntityTooLarge, "too-large")
			return
		}
		apiError(w, http.StatusBadRequest, "no-file")
		return
	}
	defer file.Close()
	pic, err := a.pictures.Upload(r.Context(), id.User.Sub, id.User.Picture, file)
	switch {
	case errors.Is(err, avatar.ErrTooLarge):
		apiError(w, http.StatusRequestEntityTooLarge, "too-large")
	case errors.Is(err, avatar.ErrFormat):
		apiError(w, http.StatusUnsupportedMediaType, "format")
	case errors.Is(err, avatar.ErrTooBig), errors.Is(err, avatar.ErrTooSmall):
		apiError(w, http.StatusUnprocessableEntity, "dimensions")
	case err != nil:
		a.fail(w, "upload picture", err)
	default:
		writeJSON(w, map[string]string{"picture": pic})
	}
}

func (a *api) removePicture(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	if err := a.pictures.Remove(r.Context(), id.User.Sub, id.User.Picture); err != nil {
		a.fail(w, "remove picture", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listLinks(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	ls, err := a.links.List(r.Context(), id.User.Sub)
	if err != nil {
		a.fail(w, "list links", err)
		return
	}
	writeJSON(w, ls)
}

type linkInput struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(v)
}

func (a *api) addLink(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	var in linkInput
	if readJSON(r, &in) != nil {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	title, url, err := links.Clean(in.Title, in.URL)
	if err != nil {
		apiError(w, http.StatusUnprocessableEntity, "invalid-link")
		return
	}
	l, err := a.links.Add(r.Context(), id.User.Sub, title, url)
	if errors.Is(err, links.ErrTooMany) {
		apiError(w, http.StatusUnprocessableEntity, "too-many")
		return
	}
	if err != nil {
		a.fail(w, "add link", err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, l)
}

func linkID(r *http.Request) (int64, bool) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return n, err == nil && n > 0
}

func (a *api) updateLink(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	n, ok := linkID(r)
	var in linkInput
	if !ok || readJSON(r, &in) != nil {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	title, url, err := links.Clean(in.Title, in.URL)
	if err != nil {
		apiError(w, http.StatusUnprocessableEntity, "invalid-link")
		return
	}
	switch err := a.links.Update(r.Context(), id.User.Sub, n, title, url); {
	case errors.Is(err, links.ErrNotFound):
		apiError(w, http.StatusNotFound, "not-found")
	case err != nil:
		a.fail(w, "update link", err)
	default:
		writeJSON(w, links.Link{ID: n, Title: title, URL: url})
	}
}

func (a *api) deleteLink(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	n, ok := linkID(r)
	if !ok {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	switch err := a.links.Delete(r.Context(), id.User.Sub, n); {
	case errors.Is(err, links.ErrNotFound):
		apiError(w, http.StatusNotFound, "not-found")
	case err != nil:
		a.fail(w, "delete link", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *api) reorderLinks(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if readJSON(r, &in) != nil {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	switch err := a.links.Reorder(r.Context(), id.User.Sub, in.IDs); {
	case errors.Is(err, links.ErrOrder):
		apiError(w, http.StatusConflict, "order")
	case err != nil:
		a.fail(w, "reorder links", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
