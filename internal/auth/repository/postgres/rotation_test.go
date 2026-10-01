package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository/postgres"
)

func TestPostgresRefreshRotationHasOneWinner(t *testing.T) {
	db := prepareTestDatabase(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(db)
	sessions := postgres.NewSessionRepository(db)
	user := createTestUser(t, ctx, users)
	now := time.Now().UTC().Truncate(time.Microsecond)
	session := createTestSession(t, ctx, sessions, user.ID, "old-hash", now, now.Add(time.Hour))
	results := make(chan error, 2)
	for _, hash := range []string{"next-a", "next-b"} {
		go func(next string) {
			results <- sessions.RotateRefreshToken(ctx, session.ID, "old-hash", next, now.Add(time.Second))
		}(hash)
	}
	winners := 0
	for range 2 {
		err := <-results
		if err == nil {
			winners++
		} else if !errors.Is(err, autherrors.ErrSessionNotFound) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("rotation winners: %d", winners)
	}
	if _, err := sessions.GetByTokenHash(ctx, "old-hash"); !errors.Is(err, autherrors.ErrSessionNotFound) {
		t.Fatal("old credential remains usable", err)
	}
	if err := sessions.Revoke(ctx, session.ID, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"next-a", "next-b"} {
		if err := sessions.RotateRefreshToken(ctx, session.ID, hash, "after-logout", now.Add(3*time.Second)); !errors.Is(err, autherrors.ErrSessionNotFound) {
			t.Fatal("rotation after logout", err)
		}
	}
}
