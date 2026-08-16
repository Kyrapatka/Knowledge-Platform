package service

import (
	"context"
	"errors"
	"testing"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
	"github.com/google/uuid"
)

var errUpdateLastUsedAt = errors.New("update last used error")

type refreshUserRepositoryFake struct {
	getByIDFunc func(
		ctx context.Context,
		id uuid.UUID,
	) (model.User, error)
}

func (f *refreshUserRepositoryFake) Create(
	context.Context,
	repository.CreateUserParams,
) (model.User, error) {
	panic("not implemented")
}

func (f *refreshUserRepositoryFake) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (model.User, error) {
	return f.getByIDFunc(ctx, id)
}

func (f *refreshUserRepositoryFake) GetByNickname(
	context.Context,
	string,
) (model.User, error) {
	panic("not implemented")
}

type refreshSessionRepositoryFake struct {
	getByTokenHashFunc func(
		ctx context.Context,
		hash string,
	) (model.AuthSession, error)

	updateLastUsedAtFunc func(
		ctx context.Context,
		sessionID uuid.UUID,
		lastUsedAt time.Time,
	) error
}

func (f *refreshSessionRepositoryFake) Create(
	context.Context,
	repository.CreateSessionParams,
) (model.AuthSession, error) {
	panic("not implemented")
}

func (f *refreshSessionRepositoryFake) GetByTokenHash(
	ctx context.Context,
	hash string,
) (model.AuthSession, error) {
	return f.getByTokenHashFunc(ctx, hash)
}

func (f *refreshSessionRepositoryFake) Revoke(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *refreshSessionRepositoryFake) RevokeAllByUserID(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *refreshSessionRepositoryFake) UpdateLastUsedAt(
	ctx context.Context,
	sessionID uuid.UUID,
	lastUsedAt time.Time,
) error {
	return f.updateLastUsedAtFunc(
		ctx,
		sessionID,
		lastUsedAt,
	)
}

func (f *refreshSessionRepositoryFake) DeleteExpired(
	context.Context,
	time.Time,
) (int64, error) {
	panic("not implemented")
}

type refreshTokenManagerFake struct {
	hash string
}

func (f refreshTokenManagerFake) CreateAccessToken(
	context.Context,
	uuid.UUID,
	time.Time,
) (token.AccessToken, error) {
	return token.AccessToken{
		Value:     "new-access-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}, nil
}

func (f refreshTokenManagerFake) ParseAccessToken(
	context.Context,
	string,
) (token.AccessTokenClaims, error) {
	panic("not implemented")
}

func (f refreshTokenManagerFake) CreateRefreshToken(
	context.Context,
) (token.RefreshToken, error) {
	panic("not implemented")
}

func (f refreshTokenManagerFake) HashRefreshToken(
	string,
) string {
	return f.hash
}

func TestService_Refresh_Success(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	now := time.Now().UTC()

	var receivedHash string
	var updatedSessionID uuid.UUID

	service := New(
		&refreshUserRepositoryFake{
			getByIDFunc: func(
				_ context.Context,
				id uuid.UUID,
			) (model.User, error) {
				if id != userID {
					t.Fatalf(
						"unexpected user ID: got %s, want %s",
						id,
						userID,
					)
				}

				return model.User{
					ID:       userID,
					Nickname: "nikita",
					Status:   model.UserStatusActive,
				}, nil
			},
		},
		&refreshSessionRepositoryFake{
			getByTokenHashFunc: func(
				_ context.Context,
				hash string,
			) (model.AuthSession, error) {
				receivedHash = hash

				return model.AuthSession{
					ID:        sessionID,
					UserID:    userID,
					CreatedAt: now.Add(-time.Hour),
					ExpiresAt: now.Add(24 * time.Hour),
				}, nil
			},
			updateLastUsedAtFunc: func(
				_ context.Context,
				id uuid.UUID,
				lastUsedAt time.Time,
			) error {
				updatedSessionID = id

				if lastUsedAt.IsZero() {
					t.Fatal("last used time must not be zero")
				}

				return nil
			},
		},
		passwordHasherFake{},
		refreshTokenManagerFake{
			hash: "refresh-hash",
		},
		24*time.Hour,
	)

	result, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: "refresh-token",
		},
	)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if receivedHash != "refresh-hash" {
		t.Fatalf(
			"unexpected refresh hash: got %q",
			receivedHash,
		)
	}

	if updatedSessionID != sessionID {
		t.Fatalf(
			"unexpected updated session ID: got %s, want %s",
			updatedSessionID,
			sessionID,
		)
	}

	if result.User.ID != userID {
		t.Fatalf(
			"unexpected user ID: got %s, want %s",
			result.User.ID,
			userID,
		)
	}

	if result.AccessToken != "new-access-token" {
		t.Fatalf(
			"unexpected access token: %q",
			result.AccessToken,
		)
	}

	if result.RefreshToken != "refresh-token" {
		t.Fatalf(
			"unexpected refresh token: %q",
			result.RefreshToken,
		)
	}
}

func TestService_Refresh_EmptyToken(t *testing.T) {
	service := New(
		&refreshUserRepositoryFake{},
		&refreshSessionRepositoryFake{},
		passwordHasherFake{},
		refreshTokenManagerFake{},
		time.Hour,
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{},
	)

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"expected ErrInvalidRefreshToken, got %v",
			err,
		)
	}
}

func TestService_Refresh_SessionNotFound(t *testing.T) {
	service := New(
		&refreshUserRepositoryFake{},
		&refreshSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{},
					autherrors.ErrSessionNotFound
			},
		},
		passwordHasherFake{},
		refreshTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: "unknown-token",
		},
	)

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"expected ErrInvalidRefreshToken, got %v",
			err,
		)
	}
}

func TestService_Refresh_RevokedSession(t *testing.T) {
	revokedAt := time.Now().UTC()

	service := New(
		&refreshUserRepositoryFake{},
		&refreshSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					ExpiresAt: time.Now().UTC().Add(time.Hour),
					RevokedAt: &revokedAt,
				}, nil
			},
		},
		passwordHasherFake{},
		refreshTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: "refresh-token",
		},
	)

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"expected ErrInvalidRefreshToken, got %v",
			err,
		)
	}
}

func TestService_Refresh_ExpiredSession(t *testing.T) {
	service := New(
		&refreshUserRepositoryFake{},
		&refreshSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{
					ID:        uuid.New(),
					UserID:    uuid.New(),
					ExpiresAt: time.Now().UTC().Add(-time.Hour),
				}, nil
			},
		},
		passwordHasherFake{},
		refreshTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: "refresh-token",
		},
	)

	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf(
			"expected ErrSessionExpired, got %v",
			err,
		)
	}
}

func TestService_Refresh_UserBlocked(t *testing.T) {
	userID := uuid.New()

	service := New(
		&refreshUserRepositoryFake{
			getByIDFunc: func(
				context.Context,
				uuid.UUID,
			) (model.User, error) {
				return model.User{
					ID:     userID,
					Status: model.UserStatusBlocked,
				}, nil
			},
		},
		&refreshSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{
					ID:        uuid.New(),
					UserID:    userID,
					ExpiresAt: time.Now().UTC().Add(time.Hour),
				}, nil
			},
		},
		passwordHasherFake{},
		refreshTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: "refresh-token",
		},
	)

	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf(
			"expected ErrUserBlocked, got %v",
			err,
		)
	}
}

func TestService_Refresh_UserNotFound(t *testing.T) {
	userID := uuid.New()

	service := New(
		&refreshUserRepositoryFake{
			getByIDFunc: func(
				context.Context,
				uuid.UUID,
			) (model.User, error) {
				return model.User{},
					autherrors.ErrUserNotFound
			},
		},
		&refreshSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{
					ID:        uuid.New(),
					UserID:    userID,
					ExpiresAt: time.Now().UTC().Add(time.Hour),
				}, nil
			},
		},
		passwordHasherFake{},
		refreshTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: "refresh-token",
		},
	)

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"expected ErrInvalidRefreshToken, got %v",
			err,
		)
	}
}

func TestService_Refresh_UpdateLastUsedError(t *testing.T) {
	userID := uuid.New()

	service := New(
		&refreshUserRepositoryFake{
			getByIDFunc: func(
				context.Context,
				uuid.UUID,
			) (model.User, error) {
				return model.User{
					ID:     userID,
					Status: model.UserStatusActive,
				}, nil
			},
		},
		&refreshSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{
					ID:        uuid.New(),
					UserID:    userID,
					ExpiresAt: time.Now().UTC().Add(time.Hour),
				}, nil
			},
			updateLastUsedAtFunc: func(
				context.Context,
				uuid.UUID,
				time.Time,
			) error {
				return errUpdateLastUsedAt
			},
		},
		passwordHasherFake{},
		refreshTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	_, err := service.Refresh(
		context.Background(),
		RefreshInput{
			RefreshToken: "refresh-token",
		},
	)

	if !errors.Is(err, errUpdateLastUsedAt) {
		t.Fatalf(
			"expected update error, got %v",
			err,
		)
	}
}
