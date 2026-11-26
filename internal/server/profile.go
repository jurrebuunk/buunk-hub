package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/jurrebuunk/buunk-hub/internal/auth"
	"github.com/jurrebuunk/buunk-hub/internal/kcadmin"
	"github.com/jurrebuunk/buunk-hub/internal/startcss"
)

// Account is the person's account in Keycloak (internal/kcadmin).
type Account interface {
	Profile(ctx context.Context, sub string) (kcadmin.Profile, error)
	SetProfile(ctx context.Context, sub string, p kcadmin.Profile) (emailChanged bool, err error)
	Groups(ctx context.Context, sub string) ([]string, error)
}

// Users is what the hub keeps of a person itself (auth.PGStore).
type Users interface {
	SetProfile(ctx context.Context, sub, name, email string) error
	SetLang(ctx context.Context, sub, lang string) error
	SetWelcomed(ctx context.Context, sub string) error
	StartCSS(ctx context.Context, sub string) (string, error)
	SetStartCSS(ctx context.Context, sub, css string) error
}

type profileAPI struct {
	account Account
	users   Users
	fail    func(w http.ResponseWriter, what string, err error)
	log     *slog.Logger
}

type profile struct {
	FirstName     string   `json:"firstName"`
	LastName      string   `json:"lastName"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"emailVerified"`
	Lang          string   `json:"lang"` // chosen in the hub; "" follows the account
	Groups        []string `json:"groups"`
}

// get answers the person's own account, for Settings and the welcome.
func (a *profileAPI) get(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	p, err := a.account.Profile(r.Context(), id.User.Sub)
	if err != nil {
		a.fail(w, "read profile", err)
		return
	}
	groups, err := a.account.Groups(r.Context(), id.User.Sub)
	if err != nil {
		a.fail(w, "read groups", err)
		return
	}
	writeJSON(w, profile{p.FirstName, p.LastName, p.Email, p.EmailVerified, id.User.Lang, groups})
}

// cleanName checks a first or last name; Keycloak checks the characters.
func cleanName(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && utf8.RuneCountInString(s) <= 255
}

// set changes the name and email in Keycloak and the language in the hub,
// and counts as finishing the welcome.
func (a *profileAPI) set(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	var in profile
	if readJSON(r, &in) != nil {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	first, ok1 := cleanName(in.FirstName)
	last, ok2 := cleanName(in.LastName)
	email := strings.TrimSpace(in.Email)
	addr, err := mail.ParseAddress(email)
	if !ok1 || !ok2 || err != nil || addr.Address != email || len(email) > 255 ||
		(in.Lang != "" && in.Lang != "en" && in.Lang != "nl") {
		apiError(w, http.StatusUnprocessableEntity, "invalid-profile")
		return
	}
	changed, err := a.account.SetProfile(r.Context(), id.User.Sub, kcadmin.Profile{FirstName: first, LastName: last, Email: email})
	mailFailed := false
	switch {
	case errors.Is(err, kcadmin.ErrEmailTaken):
		apiError(w, http.StatusConflict, "email-taken")
		return
	case errors.Is(err, kcadmin.ErrInvalid):
		apiError(w, http.StatusUnprocessableEntity, "invalid-profile")
		return
	case err != nil && !changed:
		a.fail(w, "set profile", err)
		return
	case err != nil:
		// Saved; only the verification mail didn't go out. Keycloak sends
		// it again when they sign in with the unverified address.
		a.log.Warn("send verify email", "err", err)
		mailFailed = true
	}
	name := first + " " + last
	ctx := r.Context()
	if err := errors.Join(a.users.SetProfile(ctx, id.User.Sub, name, email),
		a.users.SetLang(ctx, id.User.Sub, in.Lang), a.users.SetWelcomed(ctx, id.User.Sub)); err != nil {
		a.fail(w, "save profile", err)
		return
	}
	writeJSON(w, map[string]any{"name": name, "email": email, "emailChanged": changed, "mailFailed": mailFailed})
}

func (a *profileAPI) welcomed(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	if err := a.users.SetWelcomed(r.Context(), id.User.Sub); err != nil {
		a.fail(w, "welcomed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *profileAPI) getStyle(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	css, err := a.users.StartCSS(r.Context(), id.User.Sub)
	if err != nil {
		a.fail(w, "read start style", err)
		return
	}
	writeJSON(w, map[string]string{"css": css})
}

// setStyle saves the cleaned style and answers it, so the person sees
// what was removed.
func (a *profileAPI) setStyle(w http.ResponseWriter, r *http.Request) {
	id, _ := auth.FromContext(r.Context())
	var in struct {
		CSS string `json:"css"`
	}
	// JSON escaping can double the text; startcss checks the real size.
	if json.NewDecoder(io.LimitReader(r.Body, 4*startcss.MaxSize)).Decode(&in) != nil {
		apiError(w, http.StatusBadRequest, "bad-request")
		return
	}
	css, err := startcss.Clean(in.CSS)
	if err != nil {
		apiError(w, http.StatusUnprocessableEntity, "style-too-large")
		return
	}
	if err := a.users.SetStartCSS(r.Context(), id.User.Sub, css); err != nil {
		a.fail(w, "save start style", err)
		return
	}
	writeJSON(w, map[string]string{"css": css})
}
