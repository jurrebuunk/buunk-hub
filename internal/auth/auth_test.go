package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// --- a fake Keycloak --------------------------------------------------------

type fakeIdP struct {
	t      *testing.T
	srv    *httptest.Server
	key    *rsa.PrivateKey
	signer jose.Signer

	mu        sync.Mutex
	codes     map[string]codeGrant
	roles     map[string][]string // resource_access for the next token
	nonceFor  func(string) string // to forge the nonce in an ID token
	refuse    bool                // refresh: 400 invalid_grant
	down      bool                // refresh: 503
	refreshes int
	revoked   []string
}

type codeGrant struct{ challenge, nonce, sub string }

func newIdP(t *testing.T) *fakeIdP {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeIdP{t: t, key: key, signer: signer, codes: map[string]codeGrant{},
		roles: map[string][]string{"cloudreve": {"cloudreve-user"}}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", f.discovery)
	mux.HandleFunc("GET /jwks", f.jwks)
	mux.HandleFunc("POST /token", f.token)
	mux.HandleFunc("POST /revoke", f.revoke)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIdP) discovery(w http.ResponseWriter, _ *http.Request) {
	u := f.srv.URL
	json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                u,
		"authorization_endpoint":                u + "/auth",
		"token_endpoint":                        u + "/token",
		"jwks_uri":                              u + "/jwks",
		"end_session_endpoint":                  u + "/logout",
		"revocation_endpoint":                   u + "/revoke",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (f *fakeIdP) jwks(w http.ResponseWriter, _ *http.Request) {
	json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &f.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
}

// authorize plays the person signing in at Keycloak: it reads the redirect
// the hub sent, and returns a code for it.
func (f *fakeIdP) authorize(location, sub string) (state, code string) {
	u, err := url.Parse(location)
	if err != nil {
		f.t.Fatal(err)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		f.t.Fatalf("no PKCE in %s", location)
	}
	if !strings.Contains(q.Get("scope"), "offline_access") {
		f.t.Errorf("scope %q has no offline_access", q.Get("scope"))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	code = randomString()
	f.codes[code] = codeGrant{q.Get("code_challenge"), q.Get("nonce"), sub}
	return q.Get("state"), code
}

func (f *fakeIdP) token(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	f.mu.Lock()
	defer f.mu.Unlock()
	var sub, nonce string
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		g, ok := f.codes[r.Form.Get("code")]
		delete(f.codes, r.Form.Get("code"))
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
			oauthError(w, http.StatusBadRequest, "invalid_grant")
			return
		}
		sub, nonce = g.sub, g.nonce
		if f.nonceFor != nil {
			nonce = f.nonceFor(nonce)
		}
	case "refresh_token":
		f.refreshes++
		if f.down {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		if f.refuse || !strings.HasPrefix(r.Form.Get("refresh_token"), "rt-") {
			oauthError(w, http.StatusBadRequest, "invalid_grant")
			return
		}
		sub = strings.TrimPrefix(r.Form.Get("refresh_token"), "rt-")
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  "at",
		"token_type":    "Bearer",
		"expires_in":    300,
		"refresh_token": "rt-" + sub,
		"id_token":      f.idToken(sub, nonce),
	})
}

func (f *fakeIdP) idToken(sub, nonce string) string {
	access := map[string]any{}
	for client, roles := range f.roles {
		access[client] = map[string]any{"roles": roles}
	}
	now := time.Now()
	claims := map[string]any{
		"iss": f.srv.URL, "aud": "hub", "sub": sub,
		"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
		"name": "Test Person", "email": sub + "@example.test", "locale": "nl",
		"resource_access": access,
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	payload, _ := json.Marshal(claims)
	jws, err := f.signer.Sign(payload)
	if err != nil {
		f.t.Fatal(err)
	}
	s, _ := jws.CompactSerialize()
	return s
}

func (f *fakeIdP) revoke(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	f.mu.Lock()
	f.revoked = append(f.revoked, r.Form.Get("token"))
	f.mu.Unlock()
}

func oauthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// --- an in-memory Store -----------------------------------------------------

type memStore struct {
	mu       sync.Mutex
	now      func() time.Time
	users    map[string]User
	sessions map[string]Session
}

func newMemStore(now func() time.Time) *memStore {
	return &memStore{now: now, users: map[string]User{}, sessions: map[string]Session{}}
}

func (m *memStore) CreateSession(_ context.Context, u User, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.Sub] = u
	m.sessions[string(s.ID)] = s
	return nil
}

func (m *memStore) Session(_ context.Context, id []byte) (Session, User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[string(id)]
	if !ok || !s.ExpiresAt.After(m.now()) {
		return Session{}, User{}, ErrNoSession
	}
	return s, m.users[s.Sub], nil
}

func (m *memStore) ExtendSession(_ context.Context, id []byte, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[string(id)]; ok {
		s.ExpiresAt = expiresAt
		m.sessions[string(id)] = s
	}
	return nil
}

func (m *memStore) RefreshSession(_ context.Context, u User, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.Sub] = u
	if old, ok := m.sessions[string(s.ID)]; ok {
		s.ExpiresAt = old.ExpiresAt
		m.sessions[string(s.ID)] = s
	}
	return nil
}

func (m *memStore) DeleteSession(_ context.Context, id []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, string(id))
	return nil
}

func (m *memStore) DeleteExpired(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if !s.ExpiresAt.After(m.now()) {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *memStore) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// --- the hub under test -----------------------------------------------------

type hub struct {
	t     *testing.T
	idp   *fakeIdP
	auth  *Auth
	store *memStore
	h     http.Handler
	clock time.Time
}

const publicURL = "https://hub.example.test"

func newHub(t *testing.T) *hub {
	idp := newIdP(t)
	x := &hub{t: t, idp: idp, clock: time.Now()}
	now := func() time.Time { return x.clock }
	x.store = newMemStore(now)
	pub, _ := url.Parse(publicURL)
	key := make([]byte, 32)
	rand.Read(key)
	a, err := New(context.Background(), Config{
		PublicURL: pub, Issuer: idp.srv.URL, ClientID: "hub", ClientSecret: "s",
		SessionKey: key, Store: x.store, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	a.now = now
	x.auth = a

	mux := http.NewServeMux()
	a.Routes(mux)
	mux.Handle("/", a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := FromContext(r.Context())
		json.NewEncoder(w).Encode(id)
	})))
	x.h = a.CheckOrigin(mux)
	return x
}

func (x *hub) do(method, target string, cookies []*http.Cookie, header http.Header) *http.Response {
	r := httptest.NewRequest(method, publicURL+target, nil)
	for _, c := range cookies {
		r.AddCookie(c)
	}
	for k, v := range header {
		r.Header[k] = v
	}
	rec := httptest.NewRecorder()
	x.h.ServeHTTP(rec, r)
	return rec.Result()
}

func cookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// signIn runs the whole flow and returns the session cookie.
func (x *hub) signIn(sub, next string) *http.Cookie {
	x.t.Helper()
	login := x.do("GET", "/auth/login?next="+url.QueryEscape(next), nil, nil)
	if login.StatusCode != http.StatusFound {
		x.t.Fatalf("login: %d", login.StatusCode)
	}
	state, code := x.idp.authorize(login.Header.Get("Location"), sub)
	cb := x.do("GET", "/auth/callback?"+url.Values{"state": {state}, "code": {code}}.Encode(),
		[]*http.Cookie{cookie(login, "__Host-hub_login")}, nil)
	if cb.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(cb.Body)
		x.t.Fatalf("callback: %d %s", cb.StatusCode, body)
	}
	if got := cb.Header.Get("Location"); got != next {
		x.t.Errorf("callback went to %q, want %q", got, next)
	}
	c := cookie(cb, "__Host-hub_session")
	if c == nil {
		x.t.Fatal("no session cookie")
	}
	return c
}

func (x *hub) identity(c *http.Cookie) (Identity, *http.Response) {
	resp := x.do("GET", "/", []*http.Cookie{c}, nil)
	var id Identity
	if resp.StatusCode == http.StatusOK {
		json.NewDecoder(resp.Body).Decode(&id)
	}
	return id, resp
}

// --- tests ------------------------------------------------------------------

func TestSignIn(t *testing.T) {
	x := newHub(t)
	c := x.signIn("ann", "/settings?tab=1")
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Errorf("session cookie not HttpOnly/Secure/Lax/Path=/: %+v", c)
	}
	id, resp := x.identity(c)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("signed-in request: %d", resp.StatusCode)
	}
	if id.User.Sub != "ann" || id.User.Name != "Test Person" || id.User.Locale != "nl" {
		t.Errorf("user %+v", id.User)
	}
	if got := id.Roles["cloudreve"]; len(got) != 1 || got[0] != "cloudreve-user" {
		t.Errorf("roles %v", id.Roles)
	}
	// The cookie value isn't stored, only its hash, and no token is plain.
	for _, s := range x.store.sessions {
		if bytes.Contains(s.RefreshToken, []byte("rt-ann")) || bytes.Equal(s.ID, []byte(c.Value)) {
			t.Error("token or cookie value stored in plain text")
		}
	}
}

func TestSignedOutRedirectsOrRefuses(t *testing.T) {
	x := newHub(t)
	resp := x.do("GET", "/pages/3?x=1", nil, nil)
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/auth/login?next=%2Fpages%2F3%3Fx%3D1" {
		t.Errorf("page: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp := x.do("GET", "/api/me", nil, nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("api: %d, want 401", resp.StatusCode)
	}
	bogus := &http.Cookie{Name: "__Host-hub_session", Value: "made-up"}
	if _, resp := x.identity(bogus); resp.StatusCode != http.StatusFound {
		t.Errorf("made-up cookie: %d, want redirect", resp.StatusCode)
	}
}

// The Done-when test: a callback whose state isn't the one this browser
// started with is refused, and no session is made.
func TestCallbackForgedState(t *testing.T) {
	x := newHub(t)
	login := x.do("GET", "/auth/login", nil, nil)
	_, code := x.idp.authorize(login.Header.Get("Location"), "ann")
	resp := x.do("GET", "/auth/callback?"+url.Values{"state": {"forged"}, "code": {code}}.Encode(),
		[]*http.Cookie{cookie(login, "__Host-hub_login")}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("forged state: %d, want 400", resp.StatusCode)
	}
	if cookie(resp, "__Host-hub_session") != nil || x.store.count() != 0 {
		t.Error("forged state made a session")
	}
}

func TestCallbackRefusals(t *testing.T) {
	cases := map[string]func(x *hub, login *http.Response, state, code string) *http.Response{
		"no login cookie": func(x *hub, _ *http.Response, state, code string) *http.Response {
			return x.do("GET", "/auth/callback?state="+state+"&code="+code, nil, nil)
		},
		"cookie from another login": func(x *hub, _ *http.Response, state, code string) *http.Response {
			other := x.do("GET", "/auth/login", nil, nil)
			return x.do("GET", "/auth/callback?state="+state+"&code="+code,
				[]*http.Cookie{cookie(other, "__Host-hub_login")}, nil)
		},
		"tampered cookie": func(x *hub, login *http.Response, state, code string) *http.Response {
			c := cookie(login, "__Host-hub_login")
			b, _ := base64.RawURLEncoding.DecodeString(c.Value)
			b[len(b)-1] ^= 1
			c.Value = base64.RawURLEncoding.EncodeToString(b)
			return x.do("GET", "/auth/callback?state="+state+"&code="+code, []*http.Cookie{c}, nil)
		},
		"too late": func(x *hub, login *http.Response, state, code string) *http.Response {
			x.clock = x.clock.Add(loginTTL + time.Second)
			return x.do("GET", "/auth/callback?state="+state+"&code="+code,
				[]*http.Cookie{cookie(login, "__Host-hub_login")}, nil)
		},
		"error from keycloak": func(x *hub, login *http.Response, state, _ string) *http.Response {
			return x.do("GET", "/auth/callback?state="+state+"&error=access_denied",
				[]*http.Cookie{cookie(login, "__Host-hub_login")}, nil)
		},
		"wrong code": func(x *hub, login *http.Response, state, _ string) *http.Response {
			return x.do("GET", "/auth/callback?state="+state+"&code=nope",
				[]*http.Cookie{cookie(login, "__Host-hub_login")}, nil)
		},
		"forged nonce": func(x *hub, login *http.Response, state, code string) *http.Response {
			x.idp.nonceFor = func(string) string { return "forged" }
			return x.do("GET", "/auth/callback?state="+state+"&code="+code,
				[]*http.Cookie{cookie(login, "__Host-hub_login")}, nil)
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			x := newHub(t)
			login := x.do("GET", "/auth/login", nil, nil)
			state, code := x.idp.authorize(login.Header.Get("Location"), "ann")
			resp := run(x, login, url.QueryEscape(state), url.QueryEscape(code))
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%d, want 400", resp.StatusCode)
			}
			if cookie(resp, "__Host-hub_session") != nil || x.store.count() != 0 {
				t.Error("made a session")
			}
		})
	}
}

func TestLoginCookieUsedOnce(t *testing.T) {
	x := newHub(t)
	login := x.do("GET", "/auth/login", nil, nil)
	state, code := x.idp.authorize(login.Header.Get("Location"), "ann")
	resp := x.do("GET", "/auth/callback?state="+url.QueryEscape(state)+"&code="+url.QueryEscape(code),
		[]*http.Cookie{cookie(login, "__Host-hub_login")}, nil)
	if c := cookie(resp, "__Host-hub_login"); c == nil || c.MaxAge >= 0 {
		t.Error("login cookie not cleared after the callback")
	}
}

func TestSafeNext(t *testing.T) {
	for in, want := range map[string]string{
		"":                       "/",
		"/":                      "/",
		"/settings?a=b":          "/settings?a=b",
		"//evil.test/x":          "/",
		"/\\evil.test":           "/",
		"https://evil.test":      "/",
		"evil":                   "/",
		"/auth/login":            "/",
		"/ok\r\nSet-Cookie: x=y": "/",
	} {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSessionSlidesAndExpires(t *testing.T) {
	x := newHub(t)
	c := x.signIn("ann", "/")
	// Used every 20 days, it stays signed in.
	for i := 0; i < 3; i++ {
		x.clock = x.clock.Add(20 * 24 * time.Hour)
		if _, resp := x.identity(c); resp.StatusCode != http.StatusOK {
			t.Fatalf("after %d×20 days: %d", i+1, resp.StatusCode)
		}
	}
	// 30 days unused, it has ended.
	x.clock = x.clock.Add(30*24*time.Hour + time.Second)
	if _, resp := x.identity(c); resp.StatusCode != http.StatusFound {
		t.Errorf("after 30 idle days: %d, want redirect to sign in", resp.StatusCode)
	}
}

func TestRolesRefreshHourly(t *testing.T) {
	x := newHub(t)
	c := x.signIn("ann", "/")
	x.idp.roles = map[string][]string{"cloudreve": {"cloudreve-admin"}}

	x.clock = x.clock.Add(30 * time.Minute)
	id, _ := x.identity(c)
	if x.idp.refreshes != 0 || id.Roles["cloudreve"][0] != "cloudreve-user" {
		t.Errorf("refreshed within the hour: %d, roles %v", x.idp.refreshes, id.Roles)
	}
	x.clock = x.clock.Add(31 * time.Minute)
	id, _ = x.identity(c)
	if x.idp.refreshes != 1 || id.Roles["cloudreve"][0] != "cloudreve-admin" {
		t.Errorf("after an hour: %d refreshes, roles %v", x.idp.refreshes, id.Roles)
	}
}

func TestRefreshRefusedEndsSession(t *testing.T) {
	x := newHub(t)
	c := x.signIn("ann", "/")
	x.idp.refuse = true // account disabled in Keycloak
	x.clock = x.clock.Add(2 * time.Hour)
	if _, resp := x.identity(c); resp.StatusCode != http.StatusFound {
		t.Errorf("refused refresh: %d, want redirect to sign in", resp.StatusCode)
	}
	if x.store.count() != 0 {
		t.Error("session not deleted")
	}
}

func TestKeycloakDownKeepsSession(t *testing.T) {
	x := newHub(t)
	c := x.signIn("ann", "/")
	x.idp.down = true
	x.clock = x.clock.Add(2 * time.Hour)
	if id, resp := x.identity(c); resp.StatusCode != http.StatusOK || len(id.Roles["cloudreve"]) != 1 {
		t.Errorf("Keycloak down: %d, roles %v", resp.StatusCode, id.Roles)
	}
	x.idp.down = false
	x.identity(c)
	if x.idp.refreshes != 2 {
		t.Errorf("not retried: %d refreshes", x.idp.refreshes)
	}
}

func TestCheckOrigin(t *testing.T) {
	x := newHub(t)
	c := x.signIn("ann", "/")
	for origin, want := range map[string]int{
		"":                         http.StatusForbidden,
		"null":                     http.StatusForbidden,
		"https://evil.test":        http.StatusForbidden,
		"http://hub.example.test":  http.StatusForbidden,
		"https://hub.example.test": http.StatusOK,
	} {
		h := http.Header{}
		if origin != "" {
			h.Set("Origin", origin)
		}
		if resp := x.do("POST", "/api/x", []*http.Cookie{c}, h); resp.StatusCode != want {
			t.Errorf("POST with Origin %q: %d, want %d", origin, resp.StatusCode, want)
		}
	}
	if _, resp := x.identity(c); resp.StatusCode != http.StatusOK {
		t.Errorf("GET without Origin: %d", resp.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	x := newHub(t)
	c := x.signIn("ann", "/")
	origin := http.Header{"Origin": {publicURL}}

	resp := x.do("POST", "/auth/logout", []*http.Cookie{c}, origin)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	q := loc.Query()
	if loc.Path != "/logout" || q.Get("id_token_hint") == "" ||
		q.Get("post_logout_redirect_uri") != publicURL+"/auth/signed-out" || q.Get("client_id") != "hub" {
		t.Errorf("logout went to %s", loc)
	}
	if sc := cookie(resp, "__Host-hub_session"); sc == nil || sc.MaxAge >= 0 {
		t.Error("session cookie not cleared")
	}
	if x.store.count() != 0 {
		t.Error("session not deleted")
	}
	if len(x.idp.revoked) != 1 || x.idp.revoked[0] != "rt-ann" {
		t.Errorf("revoked %v", x.idp.revoked)
	}
	if _, resp := x.identity(c); resp.StatusCode != http.StatusFound {
		t.Errorf("old cookie after logout: %d", resp.StatusCode)
	}
	// Cross-site logout is refused (it would sign people out from elsewhere).
	c2 := x.signIn("ann", "/")
	if resp := x.do("POST", "/auth/logout", []*http.Cookie{c2}, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("logout without Origin: %d", resp.StatusCode)
	}
}

func TestPages(t *testing.T) {
	x := newHub(t)
	resp := x.do("GET", "/auth/signed-out", nil, http.Header{"Accept-Language": {"nl-NL,nl;q=0.9"}})
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Je bent uitgelogd") ||
		!strings.Contains(string(body), `href="/auth/login"`) {
		t.Errorf("signed-out page: %d %s", resp.StatusCode, body)
	}
}

func TestSealPurpose(t *testing.T) {
	s, _ := newSealer(make([]byte, 32))
	sealed := s.seal(purposeLogin, []byte("x"))
	if _, err := s.open(purposeRefreshToken, sealed); err == nil {
		t.Error("opened with another purpose")
	}
	if got, err := s.open(purposeLogin, sealed); err != nil || string(got) != "x" {
		t.Errorf("open: %q %v", got, err)
	}
}
