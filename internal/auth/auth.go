// Package auth signs people in with Keycloak (OIDC code flow with PKCE) and
// keeps them signed in. The server holds the tokens; the browser only gets
// a session cookie, never a token.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
)

const (
	// Sessions last 30 days and slide: every use moves the end, at most
	// once per extendEvery, so not every request writes to the database.
	sessionTTL  = 30 * 24 * time.Hour
	extendEvery = time.Hour
	// Roles (and the account itself) are re-checked with Keycloak this often.
	rolesTTL = time.Hour
	// Time to finish signing in at Keycloak.
	loginTTL = 10 * time.Minute

	purposeLogin        = "login-cookie"
	purposeRefreshToken = "refresh-token"
	purposeIDToken      = "id-token"
)

type Config struct {
	PublicURL    *url.URL
	Issuer       string
	ClientID     string
	ClientSecret string
	SessionKey   []byte
	Store        Store
	Log          *slog.Logger
	// HTTPClient talks to Keycloak; nil means one with a 10 s timeout.
	HTTPClient *http.Client
}

type Auth struct {
	cfg      Config
	client   *http.Client
	verifier *oidc.IDTokenVerifier
	oauth    oauth2.Config
	seal     *sealer
	// From the discovery document.
	endSessionURL, revokeURL string

	secure        bool
	sessionCookie string
	loginCookie   string
	origin        string

	refreshes singleflight.Group
	now       func() time.Time
}

// New reads the issuer's discovery document, so Keycloak must be reachable.
func New(ctx context.Context, cfg Config) (*Auth, error) {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	seal, err := newSealer(cfg.SessionKey)
	if err != nil {
		return nil, err
	}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, client), cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery: %w", err)
	}
	var endpoints struct {
		EndSession string `json:"end_session_endpoint"`
		Revoke     string `json:"revocation_endpoint"`
	}
	if err := provider.Claims(&endpoints); err != nil {
		return nil, err
	}

	endpoint := provider.Endpoint()
	// Keycloak takes the client secret as HTTP Basic. Fixing it stops
	// x/oauth2 from guessing, which on an error retries the same code.
	endpoint.AuthStyle = oauth2.AuthStyleInHeader

	a := &Auth{
		cfg:      cfg,
		client:   client,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     endpoint,
			RedirectURL:  cfg.PublicURL.JoinPath("/auth/callback").String(),
			// offline_access: a Keycloak refresh token normally dies with the
			// SSO session (hours); an offline one lasts while it is used
			// (30 days idle), which the 30-day sliding session needs.
			Scopes: []string{oidc.ScopeOpenID, "profile", "email", oidc.ScopeOfflineAccess},
		},
		seal:          seal,
		endSessionURL: endpoints.EndSession,
		revokeURL:     endpoints.Revoke,
		secure:        cfg.PublicURL.Scheme == "https",
		origin:        cfg.PublicURL.Scheme + "://" + cfg.PublicURL.Host,
		now:           time.Now,
	}
	// __Host- cookies are only sent to exactly this host, over https. Plain
	// http is only for local development.
	a.sessionCookie, a.loginCookie = "hub_session", "hub_login"
	if a.secure {
		a.sessionCookie, a.loginCookie = "__Host-hub_session", "__Host-hub_login"
	}
	return a, nil
}

// Routes adds the /auth/ endpoints to mux. None of them need a session.
func (a *Auth) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/login", a.login)
	mux.HandleFunc("GET /auth/callback", a.callback)
	mux.HandleFunc("POST /auth/logout", a.logout)
	mux.HandleFunc("GET /auth/signed-out", a.signedOut)
}

// loginState is what the browser carries, sealed, from /auth/login to
// /auth/callback: it ties the callback to the browser that started it.
type loginState struct {
	State    string    `json:"s"`
	Nonce    string    `json:"n"`
	Verifier string    `json:"v"`
	Next     string    `json:"next"`
	Expires  time.Time `json:"exp"`
}

func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	ls := loginState{
		State:    randomString(),
		Nonce:    randomString(),
		Verifier: oauth2.GenerateVerifier(),
		Next:     safeNext(r.URL.Query().Get("next")),
		Expires:  a.now().Add(loginTTL),
	}
	plain, _ := json.Marshal(ls)
	a.setCookie(w, a.loginCookie, base64.RawURLEncoding.EncodeToString(a.seal.seal(purposeLogin, plain)), loginTTL)
	http.Redirect(w, r, a.oauth.AuthCodeURL(ls.State,
		oidc.Nonce(ls.Nonce), oauth2.S256ChallengeOption(ls.Verifier)), http.StatusFound)
}

func (a *Auth) callback(w http.ResponseWriter, r *http.Request) {
	ls, err := a.readLoginCookie(r)
	// One attempt per cookie, whatever happens next.
	a.clearCookie(w, a.loginCookie)
	if err != nil {
		a.fail(w, r, "login cookie", err)
		return
	}
	q := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(ls.State)) != 1 {
		a.fail(w, r, "state", errors.New("state doesn't match"))
		return
	}
	if e := q.Get("error"); e != "" {
		a.fail(w, r, "keycloak", fmt.Errorf("%s: %s", e, q.Get("error_description")))
		return
	}
	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, a.client)
	tok, err := a.oauth.Exchange(ctx, q.Get("code"), oauth2.VerifierOption(ls.Verifier))
	if err != nil {
		a.fail(w, r, "code exchange", err)
		return
	}
	idToken, c, err := a.verify(ctx, tok)
	if err != nil {
		a.fail(w, r, "id token", err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(ls.Nonce)) != 1 {
		a.fail(w, r, "nonce", errors.New("nonce doesn't match"))
		return
	}
	if tok.RefreshToken == "" {
		a.fail(w, r, "refresh token", errors.New("no refresh token (offline_access not allowed?)"))
		return
	}

	value := randomString()
	now := a.now()
	s := Session{
		ID:             hashID(value),
		Sub:            c.Subject,
		Roles:          c.roles(),
		RefreshToken:   a.seal.seal(purposeRefreshToken, []byte(tok.RefreshToken)),
		IDToken:        a.seal.seal(purposeIDToken, []byte(tok.Extra("id_token").(string))),
		ExpiresAt:      now.Add(sessionTTL),
		RolesCheckedAt: now,
	}
	if err := a.cfg.Store.CreateSession(r.Context(), c.user(), s); err != nil {
		a.cfg.Log.Error("create session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	a.setCookie(w, a.sessionCookie, value, sessionTTL)
	a.cfg.Log.Info("signed in", "sub", c.Subject)
	http.Redirect(w, r, ls.Next, http.StatusFound)
}

func (a *Auth) readLoginCookie(r *http.Request) (loginState, error) {
	var ls loginState
	ck, err := r.Cookie(a.loginCookie)
	if err != nil {
		return ls, errors.New("no login cookie")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(ck.Value)
	if err != nil {
		return ls, err
	}
	plain, err := a.seal.open(purposeLogin, sealed)
	if err != nil {
		return ls, err
	}
	if err := json.Unmarshal(plain, &ls); err != nil {
		return ls, err
	}
	if a.now().After(ls.Expires) {
		return ls, errors.New("login took too long")
	}
	return ls, nil
}

func (a *Auth) logout(w http.ResponseWriter, r *http.Request) {
	a.clearCookie(w, a.sessionCookie)
	done := a.cfg.PublicURL.JoinPath("/auth/signed-out").String()

	s, _, err := a.session(r)
	if err != nil {
		http.Redirect(w, r, done, http.StatusSeeOther)
		return
	}
	if err := a.cfg.Store.DeleteSession(r.Context(), s.ID); err != nil {
		a.cfg.Log.Error("delete session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// The offline token outlives Keycloak's own session, so revoke it too.
	if rt, err := a.seal.open(purposeRefreshToken, s.RefreshToken); err == nil {
		a.revoke(r.Context(), string(rt))
	}
	a.cfg.Log.Info("signed out", "sub", s.Sub)

	// Then end the Keycloak session, which comes back to /auth/signed-out.
	q := url.Values{"client_id": {a.cfg.ClientID}, "post_logout_redirect_uri": {done}}
	if hint, err := a.seal.open(purposeIDToken, s.IDToken); err == nil {
		q.Set("id_token_hint", string(hint))
	}
	http.Redirect(w, r, a.endSessionURL+"?"+q.Encode(), http.StatusSeeOther)
}

func (a *Auth) revoke(ctx context.Context, refreshToken string) {
	if a.revokeURL == "" {
		return
	}
	form := url.Values{"token": {refreshToken}, "token_type_hint": {"refresh_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.revokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(a.cfg.ClientID), url.QueryEscape(a.cfg.ClientSecret))
	resp, err := a.client.Do(req)
	if err != nil {
		a.cfg.Log.Warn("revoke token", "err", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		a.cfg.Log.Warn("revoke token", "status", resp.StatusCode)
	}
}

func (a *Auth) signedOut(w http.ResponseWriter, r *http.Request) {
	renderPage(w, r, http.StatusOK, pageSignedOut)
}

// fail ends a sign-in that went wrong: the reason goes to the log, the
// person gets a page with a way to try again.
func (a *Auth) fail(w http.ResponseWriter, r *http.Request, step string, err error) {
	a.cfg.Log.Warn("sign-in refused", "step", step, "err", err)
	renderPage(w, r, http.StatusBadRequest, pageFailed)
}

// claims are the parts of Keycloak's ID token the hub uses.
type claims struct {
	Subject           string `json:"sub"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Locale            string `json:"locale"`
	Picture           string `json:"picture"`
	ResourceAccess    map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

func (c claims) user() User {
	name := c.Name
	if name == "" {
		name = c.PreferredUsername
	}
	return User{Sub: c.Subject, Name: name, Email: c.Email, Locale: c.Locale, Picture: c.Picture}
}

func (c claims) roles() Roles {
	r := Roles{}
	for client, access := range c.ResourceAccess {
		if len(access.Roles) > 0 {
			r[client] = access.Roles
		}
	}
	return r
}

func (a *Auth) verify(ctx context.Context, tok *oauth2.Token) (*oidc.IDToken, claims, error) {
	var c claims
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return nil, c, errors.New("no id_token in token response")
	}
	idToken, err := a.verifier.Verify(oidc.ClientContext(ctx, a.client), raw)
	if err != nil {
		return nil, c, err
	}
	if err := idToken.Claims(&c); err != nil {
		return nil, c, err
	}
	return idToken, c, nil
}

func (a *Auth) setCookie(w http.ResponseWriter, name, value string, maxAge time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: int(maxAge.Seconds()),
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *Auth) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode,
	})
}

// safeNext keeps only a local path to return to after signing in, so the
// login can't be used to send someone to another site.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") ||
		strings.ContainsAny(next, "\\\r\n\t") || strings.HasPrefix(next, "/auth/") {
		return "/"
	}
	u, err := url.Parse(next)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return "/"
	}
	return next
}

func randomString() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashID(value string) []byte {
	h := sha256.Sum256([]byte(value))
	return h[:]
}
