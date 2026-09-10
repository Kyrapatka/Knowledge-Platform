package service

import (
	"context"
	"errors"
	"testing"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
	"github.com/google/uuid"
)

type rotatingSessionFake struct {
	*refreshSessionRepositoryFake
	rotate func(context.Context, uuid.UUID, string, string, time.Time) error
}

func (f rotatingSessionFake) RotateRefreshToken(ctx context.Context, id uuid.UUID, oldHash, newHash string, now time.Time) error {
	return f.rotate(ctx, id, oldHash, newHash, now)
}

type browserTokenFake struct{ refreshTokenManagerFake }

func (browserTokenFake) CreateRefreshToken(context.Context) (token.RefreshToken, error) {
	return token.RefreshToken{Value: "new-refresh", Hash: "new-hash"}, nil
}

func TestBrowserRefreshRotationPreservesExpiryAndRejectsOldCredential(t *testing.T) {
	sessionID, userID := uuid.New(), uuid.New()
	expires := time.Now().UTC().Add(3 * time.Hour)
	storedHash := "old-hash"
	rotations := 0
	svc := New(&refreshUserRepositoryFake{getByIDFunc: func(context.Context, uuid.UUID) (model.User, error) {
		return model.User{ID: userID, Status: model.UserStatusActive}, nil
	}}, rotatingSessionFake{
		refreshSessionRepositoryFake: &refreshSessionRepositoryFake{getByTokenHashFunc: func(_ context.Context, hash string) (model.AuthSession, error) {
			if hash != storedHash {
				return model.AuthSession{}, autherrors.ErrSessionNotFound
			}
			return model.AuthSession{ID: sessionID, UserID: userID, ExpiresAt: expires}, nil
		}},
		rotate: func(_ context.Context, id uuid.UUID, oldHash, newHash string, now time.Time) error {
			if id != sessionID || oldHash != storedHash || !now.Before(expires) {
				return autherrors.ErrSessionNotFound
			}
			storedHash = newHash
			rotations++
			return nil
		},
	}, passwordHasherFake{}, browserTokenFake{refreshTokenManagerFake{hash: "old-hash"}}, 24*time.Hour)
	result, err := svc.RefreshBrowser(context.Background(), RefreshInput{RefreshToken: "old-refresh"})
	if err != nil {
		t.Fatal(err)
	}
	if result.RefreshToken != "new-refresh" || !result.RefreshExpiresAt.Equal(expires) || rotations != 1 {
		t.Fatal("rotation must replace the credential and retain original expiry")
	}
	_, err = svc.RefreshBrowser(context.Background(), RefreshInput{RefreshToken: "old-refresh"})
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("old credential accepted: %v", err)
	}
}

func TestBrowserRefreshDoesNotReturnCredentialsWhenRotationLosesRace(t *testing.T) {
	userID := uuid.New()
	svc := New(&refreshUserRepositoryFake{getByIDFunc: func(context.Context, uuid.UUID) (model.User, error) {
		return model.User{ID: userID, Status: model.UserStatusActive}, nil
	}}, rotatingSessionFake{
		refreshSessionRepositoryFake: &refreshSessionRepositoryFake{getByTokenHashFunc: func(context.Context, string) (model.AuthSession, error) {
			return model.AuthSession{ID: uuid.New(), UserID: userID, ExpiresAt: time.Now().Add(time.Hour)}, nil
		}},
		rotate: func(context.Context, uuid.UUID, string, string, time.Time) error {
			return autherrors.ErrSessionNotFound
		},
	}, passwordHasherFake{}, browserTokenFake{refreshTokenManagerFake{hash: "old-hash"}}, 24*time.Hour)
	result, err := svc.RefreshBrowser(context.Background(), RefreshInput{RefreshToken: "old-refresh"})
	if !errors.Is(err, ErrInvalidRefreshToken) || result.AccessToken != "" || result.RefreshToken != "" {
		t.Fatal("failed rotation exposed credentials")
	}
}
