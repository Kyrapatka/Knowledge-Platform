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

var (
	errGetSession = errors.New("get session error")
	errRevoke     = errors.New("revoke session error")
	errRevokeAll  = errors.New("revoke all sessions error")
)

type logoutSessionRepositoryFake struct {
	getByTokenHashFunc func(
		ctx context.Context,
		tokenHash string,
	) (model.AuthSession, error)

	revokeFunc func(
		ctx context.Context,
		sessionID uuid.UUID,
		revokedAt time.Time,
	) error

	revokeAllFunc func(
		ctx context.Context,
		userID uuid.UUID,
		revokedAt time.Time,
	) error
}

func (f *logoutSessionRepositoryFake) Create(
	context.Context,
	repository.CreateSessionParams,
) (model.AuthSession, error) {
	panic("not implemented")
}

func (f *logoutSessionRepositoryFake) GetByTokenHash(
	ctx context.Context,
	tokenHash string,
) (model.AuthSession, error) {
	return f.getByTokenHashFunc(
		ctx,
		tokenHash,
	)
}

func (f *logoutSessionRepositoryFake) Revoke(
	ctx context.Context,
	sessionID uuid.UUID,
	revokedAt time.Time,
) error {
	return f.revokeFunc(
		ctx,
		sessionID,
		revokedAt,
	)
}

func (f *logoutSessionRepositoryFake) RevokeAllByUserID(
	ctx context.Context,
	userID uuid.UUID,
	revokedAt time.Time,
) error {
	return f.revokeAllFunc(
		ctx,
		userID,
		revokedAt,
	)
}

func (f *logoutSessionRepositoryFake) UpdateLastUsedAt(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *logoutSessionRepositoryFake) DeleteExpired(
	context.Context,
	time.Time,
) (int64, error) {
	panic("not implemented")
}

type logoutTokenManagerFake struct {
	hash string
}

func (f logoutTokenManagerFake) CreateAccessToken(
	context.Context,
	uuid.UUID,
	time.Time,
) (token.AccessToken, error) {
	panic("not implemented")
}

func (f logoutTokenManagerFake) ParseAccessToken(
	context.Context,
	string,
) (token.AccessTokenClaims, error) {
	panic("not implemented")
}

func (f logoutTokenManagerFake) CreateRefreshToken(
	context.Context,
) (token.RefreshToken, error) {
	panic("not implemented")
}

func (f logoutTokenManagerFake) HashRefreshToken(
	string,
) string {
	return f.hash
}

func TestService_Logout_Success(t *testing.T) {
	sessionID := uuid.New()

	var receivedHash string
	var revokedSessionID uuid.UUID
	var revokedAt time.Time

	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{
			getByTokenHashFunc: func(
				_ context.Context,
				tokenHash string,
			) (model.AuthSession, error) {
				receivedHash = tokenHash

				return model.AuthSession{
					ID:        sessionID,
					ExpiresAt: time.Now().UTC().Add(time.Hour),
				}, nil
			},
			revokeFunc: func(
				_ context.Context,
				id uuid.UUID,
				at time.Time,
			) error {
				revokedSessionID = id
				revokedAt = at

				return nil
			},
		},
		passwordHasherFake{},
		logoutTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	err := service.Logout(
		context.Background(),
		LogoutInput{
			RefreshToken: "refresh-token",
		},
	)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}

	if receivedHash != "refresh-hash" {
		t.Fatalf(
			"unexpected refresh hash: got %q",
			receivedHash,
		)
	}

	if revokedSessionID != sessionID {
		t.Fatalf(
			"unexpected revoked session ID: got %s, want %s",
			revokedSessionID,
			sessionID,
		)
	}

	if revokedAt.IsZero() {
		t.Fatal("revoked time must not be zero")
	}
}

func TestService_Logout_EmptyToken(t *testing.T) {
	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{},
		passwordHasherFake{},
		logoutTokenManagerFake{},
		time.Hour,
	)

	err := service.Logout(
		context.Background(),
		LogoutInput{},
	)

	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf(
			"expected ErrInvalidRefreshToken, got %v",
			err,
		)
	}
}

func TestService_Logout_SessionNotFound(t *testing.T) {
	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{},
					autherrors.ErrSessionNotFound
			},
		},
		passwordHasherFake{},
		logoutTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	err := service.Logout(
		context.Background(),
		LogoutInput{
			RefreshToken: "unknown-token",
		},
	)

	if err != nil {
		t.Fatalf(
			"logout must be idempotent, got %v",
			err,
		)
	}
}

func TestService_Logout_AlreadyRevoked(t *testing.T) {
	revokedAt := time.Now().UTC()

	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{
					ID:        uuid.New(),
					RevokedAt: &revokedAt,
				}, nil
			},
			revokeFunc: func(
				context.Context,
				uuid.UUID,
				time.Time,
			) error {
				t.Fatal(
					"revoke must not be called for revoked session",
				)

				return nil
			},
		},
		passwordHasherFake{},
		logoutTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	err := service.Logout(
		context.Background(),
		LogoutInput{
			RefreshToken: "refresh-token",
		},
	)
	if err != nil {
		t.Fatalf(
			"logout revoked session: %v",
			err,
		)
	}
}

func TestService_Logout_GetSessionError(t *testing.T) {
	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{},
					errGetSession
			},
		},
		passwordHasherFake{},
		logoutTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	err := service.Logout(
		context.Background(),
		LogoutInput{
			RefreshToken: "refresh-token",
		},
	)

	if !errors.Is(err, errGetSession) {
		t.Fatalf(
			"expected get session error, got %v",
			err,
		)
	}
}

func TestService_Logout_RevokeError(t *testing.T) {
	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{
			getByTokenHashFunc: func(
				context.Context,
				string,
			) (model.AuthSession, error) {
				return model.AuthSession{
					ID: uuid.New(),
				}, nil
			},
			revokeFunc: func(
				context.Context,
				uuid.UUID,
				time.Time,
			) error {
				return errRevoke
			},
		},
		passwordHasherFake{},
		logoutTokenManagerFake{
			hash: "refresh-hash",
		},
		time.Hour,
	)

	err := service.Logout(
		context.Background(),
		LogoutInput{
			RefreshToken: "refresh-token",
		},
	)

	if !errors.Is(err, errRevoke) {
		t.Fatalf(
			"expected revoke error, got %v",
			err,
		)
	}
}

func TestService_LogoutAll_Success(t *testing.T) {
	userID := uuid.New()

	var receivedUserID uuid.UUID
	var revokedAt time.Time

	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{
			revokeAllFunc: func(
				_ context.Context,
				id uuid.UUID,
				at time.Time,
			) error {
				receivedUserID = id
				revokedAt = at

				return nil
			},
		},
		passwordHasherFake{},
		logoutTokenManagerFake{},
		time.Hour,
	)

	err := service.LogoutAll(
		context.Background(),
		userID,
	)
	if err != nil {
		t.Fatalf(
			"logout all: %v",
			err,
		)
	}

	if receivedUserID != userID {
		t.Fatalf(
			"unexpected user ID: got %s, want %s",
			receivedUserID,
			userID,
		)
	}

	if revokedAt.IsZero() {
		t.Fatal("revoked time must not be zero")
	}
}

func TestService_LogoutAll_EmptyUserID(t *testing.T) {
	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{},
		passwordHasherFake{},
		logoutTokenManagerFake{},
		time.Hour,
	)

	err := service.LogoutAll(
		context.Background(),
		uuid.Nil,
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}

func TestService_LogoutAll_RepositoryError(t *testing.T) {
	service := New(
		emptyUserRepositoryFake(),
		&logoutSessionRepositoryFake{
			revokeAllFunc: func(
				context.Context,
				uuid.UUID,
				time.Time,
			) error {
				return errRevokeAll
			},
		},
		passwordHasherFake{},
		logoutTokenManagerFake{},
		time.Hour,
	)

	err := service.LogoutAll(
		context.Background(),
		uuid.New(),
	)

	if !errors.Is(err, errRevokeAll) {
		t.Fatalf(
			"expected revoke all error, got %v",
			err,
		)
	}
}
