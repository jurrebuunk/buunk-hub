package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type ctxKey struct{}

// Identity is who is signed in, for handlers behind Require.
type Identity struct {
	User  User
	Roles Roles
}

// FromContext returns the signed-in person; ok is false outside Require.
func FromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// Require lets only signed-in people through. Pages send everyone else to
// sign in and back; /api/ answers 401, and the app reloads to sign in.
func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, u, err := a.session(r)
		if err == nil {
			s, u, err = a.keepFresh(w, r, s, u)
		}
		if err != nil {
			if !errors.Is(err, ErrNoSession) {
				a.cfg.Log.Error("session", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			a.clearCookie(w, a.sessionCookie)
			if strings.HasPrefix(r.URL.Path, "/api/") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"signed out"}` + "\n"))
				return
			}
			http.Redirect(w, r, "/auth/login?"+url.Values{"next": {r.URL.RequestURI()}}.Encode(), http.StatusFound)
			return
		}
		w.Header().Set("Cache-Control", "private, no-cache")
		ctx := context.WithValue(r.Context(), ctxKey{}, Identity{User: u, Roles: s.Roles})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// session looks up the browser's session; ErrNoSession if there is none.
func (a *Auth) session(r *http.Request) (Session, User, error) {
	ck, err := r.Cookie(a.sessionCookie)
	if err != nil || ck.Value == "" {
		return Session{}, User{}, ErrNoSession
	}
	return a.cfg.Store.Session(r.Context(), hashID(ck.Value))
}

// keepFresh slides the session's expiry and re-checks the roles with
// Keycloak when they are older than rolesTTL. A refused refresh (account
// disabled, session revoked) ends the session; Keycloak being unreachable
// doesn't, the roles are tried again on the next request.
func (a *Auth) keepFresh(w http.ResponseWriter, r *http.Request, s Session, u User) (Session, User, error) {
	now := a.now()
	if now.Sub(s.RolesCheckedAt) >= rolesTTL {
		// Requests from one browser come in bunches; refresh once.
		v, err, _ := a.refreshes.Do(string(s.ID), func() (any, error) {
			s, u, err := a.refresh(r.Context(), s)
			return [2]any{s, u}, err
		})
		switch {
		case errors.Is(err, errRefused):
			a.cfg.Log.Info("session ended by keycloak", "sub", s.Sub, "err", err)
			if err := a.cfg.Store.DeleteSession(r.Context(), s.ID); err != nil {
				return s, u, err
			}
			return s, u, ErrNoSession
		case err != nil:
			a.cfg.Log.Warn("refresh roles", "sub", s.Sub, "err", err)
		default:
			pair := v.([2]any)
			s, u = pair[0].(Session), pair[1].(User)
		}
	}
	if s.ExpiresAt.Before(now.Add(sessionTTL - extendEvery)) {
		s.ExpiresAt = now.Add(sessionTTL)
		if err := a.cfg.Store.ExtendSession(r.Context(), s.ID, s.ExpiresAt); err != nil {
			return s, u, err
		}
		if ck, err := r.Cookie(a.sessionCookie); err == nil {
			a.setCookie(w, a.sessionCookie, ck.Value, sessionTTL)
		}
	}
	return s, u, nil
}

// errRefused means Keycloak said no to the refresh token.
var errRefused = errors.New("refresh refused")

func (a *Auth) refresh(ctx context.Context, s Session) (Session, User, error) {
	rt, err := a.seal.open(purposeRefreshToken, s.RefreshToken)
	if err != nil {
		return s, User{}, errors.Join(errRefused, err)
	}
	// Not tied to the request: one browser's cancelled request shouldn't
	// break the refresh for the others waiting on it.
	ctx = context.WithValue(context.WithoutCancel(ctx), oauth2.HTTPClient, a.client)
	tok, err := a.oauth.TokenSource(ctx, &oauth2.Token{RefreshToken: string(rt)}).Token()
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) && (re.Response.StatusCode == http.StatusBadRequest || re.Response.StatusCode == http.StatusUnauthorized) {
			return s, User{}, errors.Join(errRefused, err)
		}
		return s, User{}, err
	}
	_, c, err := a.verify(ctx, tok)
	if err != nil {
		return s, User{}, errors.Join(errRefused, err)
	}
	if c.Subject != s.Sub {
		return s, User{}, errors.Join(errRefused, errors.New("subject changed"))
	}
	s.Roles = c.roles()
	s.RefreshToken = a.seal.seal(purposeRefreshToken, []byte(tok.RefreshToken))
	s.IDToken = a.seal.seal(purposeIDToken, []byte(tok.Extra("id_token").(string)))
	s.RolesCheckedAt = a.now()
	u := c.user()
	if err := a.cfg.Store.RefreshSession(ctx, u, s); err != nil {
		return s, u, err
	}
	return s, u, nil
}

// CheckOrigin refuses requests that change something (anything but GET,
// HEAD and OPTIONS) unless they come from the hub's own pages. With
// SameSite=Lax cookies this stops cross-site request forgery.
func (a *Auth) CheckOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get("Origin") != a.origin {
				a.cfg.Log.Warn("refused: wrong origin", "origin", r.Header.Get("Origin"), "path", r.URL.Path)
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Cleanup removes expired sessions every hour until ctx ends.
func (a *Auth) Cleanup(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if err := a.cfg.Store.DeleteExpired(ctx); err != nil && ctx.Err() == nil {
			a.cfg.Log.Warn("delete expired sessions", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// WithIdentity puts id in ctx, as Require does; for tests elsewhere.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}
