package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore is the Store in Postgres (tables from internal/db/migrations).
type PGStore struct{ DB *pgxpool.Pool }

func upsertUser(ctx context.Context, tx pgx.Tx, u User) error {
	_, err := tx.Exec(ctx, `INSERT INTO users (sub, name, email, locale, picture, username)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (sub) DO UPDATE SET name = $2, email = $3, locale = $4,
			picture = $5, username = $6, updated_at = now()`,
		u.Sub, u.Name, u.Email, u.Locale, u.Picture, u.Username)
	return err
}

func (p PGStore) CreateSession(ctx context.Context, u User, s Session) error {
	return pgx.BeginFunc(ctx, p.DB, func(tx pgx.Tx) error {
		if err := upsertUser(ctx, tx, u); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO sessions
			(id, sub, roles, refresh_token, id_token, expires_at, roles_checked_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			s.ID, s.Sub, s.Roles, s.RefreshToken, s.IDToken, s.ExpiresAt, s.RolesCheckedAt)
		return err
	})
}

func (p PGStore) Session(ctx context.Context, id []byte) (Session, User, error) {
	var s Session
	var u User
	err := p.DB.QueryRow(ctx, `SELECT s.id, s.sub, s.roles, s.refresh_token, s.id_token,
			s.expires_at, s.roles_checked_at, u.name, u.username, u.email, u.locale, u.picture
		FROM sessions s JOIN users u USING (sub)
		WHERE s.id = $1 AND s.expires_at > now()`, id).
		Scan(&s.ID, &s.Sub, &s.Roles, &s.RefreshToken, &s.IDToken,
			&s.ExpiresAt, &s.RolesCheckedAt, &u.Name, &u.Username, &u.Email, &u.Locale, &u.Picture)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, User{}, ErrNoSession
	}
	u.Sub = s.Sub
	return s, u, err
}

func (p PGStore) ExtendSession(ctx context.Context, id []byte, expiresAt time.Time) error {
	_, err := p.DB.Exec(ctx, "UPDATE sessions SET expires_at = $2 WHERE id = $1", id, expiresAt)
	return err
}

func (p PGStore) RefreshSession(ctx context.Context, u User, s Session) error {
	return pgx.BeginFunc(ctx, p.DB, func(tx pgx.Tx) error {
		if err := upsertUser(ctx, tx, u); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE sessions SET roles = $2, refresh_token = $3,
			id_token = $4, roles_checked_at = $5 WHERE id = $1`,
			s.ID, s.Roles, s.RefreshToken, s.IDToken, s.RolesCheckedAt)
		return err
	})
}

func (p PGStore) DeleteSession(ctx context.Context, id []byte) error {
	_, err := p.DB.Exec(ctx, "DELETE FROM sessions WHERE id = $1", id)
	return err
}

func (p PGStore) DeleteExpired(ctx context.Context) error {
	_, err := p.DB.Exec(ctx, "DELETE FROM sessions WHERE expires_at <= now()")
	return err
}

// SetPicture updates the cached picture right away; the next sign-in or
// hourly refresh brings the same value from Keycloak.
func (p PGStore) SetPicture(ctx context.Context, sub, picture string) error {
	_, err := p.DB.Exec(ctx, "UPDATE users SET picture = $2, updated_at = now() WHERE sub = $1", sub, picture)
	return err
}
