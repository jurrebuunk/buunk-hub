package auth

import (
	"context"
	"errors"
	"time"
)

// ErrNoSession is returned when a session doesn't exist (or has expired).
var ErrNoSession = errors.New("no such session")

// User is the cached profile of someone who signed in.
type User struct {
	Sub, Name, Username, Email, Locale, Picture string
}

// Roles are client roles from the ID token's resource_access claim:
// client ID → role names.
type Roles map[string][]string

// Session is a signed-in browser. The tokens are sealed before they are
// stored; Store never sees them in plain text.
type Session struct {
	ID             []byte // SHA-256 of the cookie value
	Sub            string
	Roles          Roles
	RefreshToken   []byte // sealed
	IDToken        []byte // sealed, for the logout hint
	ExpiresAt      time.Time
	RolesCheckedAt time.Time
}

// Store keeps users and sessions.
type Store interface {
	// CreateSession saves u (inserting or updating) and the new session.
	CreateSession(ctx context.Context, u User, s Session) error
	// Session returns an unexpired session and its user, or ErrNoSession.
	Session(ctx context.Context, id []byte) (Session, User, error)
	// ExtendSession moves the session's expiry (sliding expiry).
	ExtendSession(ctx context.Context, id []byte, expiresAt time.Time) error
	// RefreshSession saves what a token refresh brought: the user's latest
	// profile, roles and tokens.
	RefreshSession(ctx context.Context, u User, s Session) error
	// DeleteSession removes the session; deleting a missing one is no error.
	DeleteSession(ctx context.Context, id []byte) error
	// DeleteExpired removes sessions past their expiry.
	DeleteExpired(ctx context.Context) error
}
