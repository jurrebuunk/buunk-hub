package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jurrebuunk/buunk-hub/internal/db"
)

// Runs against a real Postgres when HUB_TEST_DATABASE_URL is set (CI, or
// dev/compose.yaml's: postgres://hub:hub@localhost:15432/hub). It uses a
// schema of its own and drops it afterwards.
func TestPGStore(t *testing.T) {
	url := os.Getenv("HUB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("HUB_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	schema := fmt.Sprintf("test_%x", hashID(randomString())[:6])
	setup, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	setup.Close()
	pool, err := db.Open(ctx, url+"?search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		pool.Close()
	})
	// Migrating twice is harmless.
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	st := PGStore{DB: pool}
	now := time.Now().Truncate(time.Microsecond)
	u := User{Sub: "ann", Name: "Ann", Email: "ann@example.test", Locale: "nl"}
	s := Session{ID: hashID("v1"), Sub: "ann", Roles: Roles{"cloudreve": {"cloudreve-user"}},
		RefreshToken: []byte{1, 2}, IDToken: []byte{3}, ExpiresAt: now.Add(time.Hour), RolesCheckedAt: now}
	if err := st.CreateSession(ctx, u, s); err != nil {
		t.Fatal(err)
	}
	gotS, gotU, err := st.Session(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotU != u || gotS.Roles["cloudreve"][0] != "cloudreve-user" || !gotS.ExpiresAt.Equal(s.ExpiresAt) {
		t.Errorf("got %+v %+v", gotS, gotU)
	}

	u.Name = "Ann B"
	s.Roles = Roles{}
	s.RefreshToken = []byte{9}
	if err := st.RefreshSession(ctx, u, s); err != nil {
		t.Fatal(err)
	}
	gotS, gotU, _ = st.Session(ctx, s.ID)
	if gotU.Name != "Ann B" || len(gotS.Roles) != 0 || gotS.RefreshToken[0] != 9 {
		t.Errorf("after refresh %+v %+v", gotS, gotU)
	}

	// Expired sessions are gone for Session, and for DeleteExpired.
	if err := st.ExtendSession(ctx, s.ID, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.Session(ctx, s.ID); !errors.Is(err, ErrNoSession) {
		t.Errorf("expired session: %v", err)
	}
	if err := st.DeleteExpired(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	pool.QueryRow(ctx, "SELECT count(*) FROM sessions").Scan(&n)
	if n != 0 {
		t.Errorf("%d sessions left after DeleteExpired", n)
	}
	if err := st.DeleteSession(ctx, s.ID); err != nil {
		t.Errorf("deleting a missing session: %v", err)
	}
}
