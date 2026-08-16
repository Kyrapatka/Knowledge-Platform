package service

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
	"github.com/google/uuid"
)

var (
	errHashPassword  = errors.New("hash password error")
	errCreateUser    = errors.New("create user error")
	errCreateSession = errors.New("create session error")
)

type userRepositoryFake struct {
	createFunc func(
		ctx context.Context,
		params repository.CreateUserParams,
	) (model.User, error)
}

func (f *userRepositoryFake) Create(
	ctx context.Context,
	params repository.CreateUserParams,
) (model.User, error) {
	return f.createFunc(ctx, params)
}

func (f *userRepositoryFake) GetByID(
	context.Context,
	uuid.UUID,
) (model.User, error) {
	panic("not implemented")
}

func (f *userRepositoryFake) GetByNickname(
	context.Context,
	string,
) (model.User, error) {
	panic("not implemented")
}

type sessionRepositoryFake struct {
	createFunc func(
		ctx context.Context,
		params repository.CreateSessionParams,
	) (model.AuthSession, error)
}

func (f *sessionRepositoryFake) Create(
	ctx context.Context,
	params repository.CreateSessionParams,
) (model.AuthSession, error) {
	return f.createFunc(ctx, params)
}

func (f *sessionRepositoryFake) GetByTokenHash(
	context.Context,
	string,
) (model.AuthSession, error) {
	panic("not implemented")
}

func (f *sessionRepositoryFake) Revoke(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *sessionRepositoryFake) RevokeAllByUserID(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *sessionRepositoryFake) UpdateLastUsedAt(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *sessionRepositoryFake) DeleteExpired(
	context.Context,
	time.Time,
) (int64, error) {
	panic("not implemented")
}

type passwordHasherFake struct{}

func (passwordHasherFake) Hash(
	password string,
) (string, error) {
	return "hashed-" + password, nil
}

func (passwordHasherFake) Compare(
	string,
	string,
) (bool, error) {
	return false, nil
}

type tokenManagerFake struct{}

func (tokenManagerFake) CreateAccessToken(
	context.Context,
	uuid.UUID,
	time.Time,
) (token.AccessToken, error) {
	return token.AccessToken{
		Value:     "access-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func (tokenManagerFake) ParseAccessToken(
	context.Context,
	string,
) (token.AccessTokenClaims, error) {
	panic("not implemented")
}

func (tokenManagerFake) CreateRefreshToken(
	context.Context,
) (token.RefreshToken, error) {
	return token.RefreshToken{
		Value: "refresh-token",
		Hash:  "refresh-hash",
	}, nil
}

func (tokenManagerFake) HashRefreshToken(
	value string,
) string {
	return "hash-" + value
}

func TestService_Register(t *testing.T) {
	userID := uuid.New()

	var createdUserParams repository.CreateUserParams
	var createdSessionParams repository.CreateSessionParams

	service := New(
		&userRepositoryFake{
			createFunc: func(
				_ context.Context,
				params repository.CreateUserParams,
			) (model.User, error) {
				createdUserParams = params

				return model.User{
					ID:                 userID,
					Nickname:           params.Nickname,
					NicknameNormalized: params.NicknameNormalized,
					PasswordHash:       params.PasswordHash,
					Status:             model.UserStatusActive,
				}, nil
			},
		},
		&sessionRepositoryFake{
			createFunc: func(
				_ context.Context,
				params repository.CreateSessionParams,
			) (model.AuthSession, error) {
				createdSessionParams = params

				return model.AuthSession{
					ID:     params.ID,
					UserID: params.UserID,
				}, nil
			},
		},
		passwordHasherFake{},
		tokenManagerFake{},
		24*time.Hour,
	)

	ip := netip.MustParseAddr("127.0.0.1")

	result, err := service.Register(
		context.Background(),
		RegisterInput{
			Nickname:  "  Nikita  ",
			Password:  "password123",
			UserAgent: "test-agent",
			IPAddress: &ip,
		},
	)
	if err != nil {
		t.Fatalf(
			"register user: %v",
			err,
		)
	}

	if result.User.ID != userID {
		t.Fatalf(
			"unexpected user id: got %s",
			result.User.ID,
		)
	}

	if createdUserParams.Nickname != "nikita" {
		t.Fatalf(
			"nickname was not normalized: %q",
			createdUserParams.Nickname,
		)
	}

	if createdUserParams.NicknameNormalized != "nikita" {
		t.Fatalf(
			"normalized nickname mismatch: %q",
			createdUserParams.NicknameNormalized,
		)
	}

	if createdUserParams.PasswordHash != "hashed-password123" {
		t.Fatalf(
			"password was not hashed correctly: %q",
			createdUserParams.PasswordHash,
		)
	}

	if createdSessionParams.UserID != userID {
		t.Fatalf(
			"session user id mismatch",
		)
	}

	if createdSessionParams.RefreshTokenHash != "refresh-hash" {
		t.Fatalf(
			"refresh token hash mismatch",
		)
	}

	if result.AccessToken != "access-token" {
		t.Fatalf(
			"access token mismatch",
		)
	}

	if result.RefreshToken != "refresh-token" {
		t.Fatalf(
			"refresh token mismatch",
		)
	}
}

func TestService_Register_InvalidNickname(t *testing.T) {
	service := New(
		&userRepositoryFake{},
		&sessionRepositoryFake{},
		passwordHasherFake{},
		tokenManagerFake{},
		time.Hour,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Nickname: "a",
			Password: "password123",
		},
	)

	if !errors.Is(err, ErrInvalidNickname) {
		t.Fatalf(
			"expected ErrInvalidNickname, got %v",
			err,
		)
	}
}

func emptyUserRepositoryFake() *userRepositoryFake {
	return &userRepositoryFake{
		createFunc: func(
			context.Context,
			repository.CreateUserParams,
		) (model.User, error) {
			return model.User{}, errors.New("should not be called")
		},
	}
}

func emptySessionRepositoryFake() *sessionRepositoryFake {
	return &sessionRepositoryFake{
		createFunc: func(
			context.Context,
			repository.CreateSessionParams,
		) (model.AuthSession, error) {
			return model.AuthSession{}, errors.New("should not be called")
		},
	}
}

func TestService_Register_InvalidPassword(t *testing.T) {
	service := New(
		emptyUserRepositoryFake(),
		emptySessionRepositoryFake(),
		passwordHasherFake{},
		tokenManagerFake{},
		time.Hour,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Nickname: "nikita",
			Password: "123",
		},
	)

	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf(
			"expected ErrInvalidPassword, got %v",
			err,
		)
	}
}

type passwordHasherErrorFake struct{}

func (passwordHasherErrorFake) Hash(
	string,
) (string, error) {
	return "", errHashPassword
}

func (passwordHasherErrorFake) Compare(
	string,
	string,
) (bool, error) {
	return false, nil
}

func TestService_Register_HashPasswordError(t *testing.T) {
	service := New(
		emptyUserRepositoryFake(),
		emptySessionRepositoryFake(),
		passwordHasherErrorFake{},
		tokenManagerFake{},
		time.Hour,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Nickname: "nikita",
			Password: "password123",
		},
	)

	if !errors.Is(err, errHashPassword) {
		t.Fatalf(
			"expected hash error, got %v",
			err,
		)
	}
}

func userRepositoryCreateErrorFake() *userRepositoryFake {
	return &userRepositoryFake{
		createFunc: func(
			context.Context,
			repository.CreateUserParams,
		) (model.User, error) {
			return model.User{}, errCreateUser
		},
	}
}

func TestService_Register_CreateUserError(t *testing.T) {
	service := New(
		userRepositoryCreateErrorFake(),
		emptySessionRepositoryFake(),
		passwordHasherFake{},
		tokenManagerFake{},
		time.Hour,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Nickname: "nikita",
			Password: "password123",
		},
	)

	if !errors.Is(err, errCreateUser) {
		t.Fatalf(
			"expected create user error, got %v",
			err,
		)
	}
}

func sessionRepositoryCreateErrorFake() *sessionRepositoryFake {
	return &sessionRepositoryFake{
		createFunc: func(
			context.Context,
			repository.CreateSessionParams,
		) (model.AuthSession, error) {
			return model.AuthSession{}, errCreateSession
		},
	}
}

func TestService_Register_CreateSessionError(t *testing.T) {
	service := New(
		&userRepositoryFake{
			createFunc: func(
				_ context.Context,
				params repository.CreateUserParams,
			) (model.User, error) {
				return model.User{
					ID:                 uuid.New(),
					Nickname:           params.Nickname,
					NicknameNormalized: params.NicknameNormalized,
					PasswordHash:       params.PasswordHash,
					Status:             model.UserStatusActive,
				}, nil
			},
		},
		sessionRepositoryCreateErrorFake(),
		passwordHasherFake{},
		tokenManagerFake{},
		time.Hour,
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Nickname: "nikita",
			Password: "password123",
		},
	)

	if !errors.Is(err, errCreateSession) {
		t.Fatalf(
			"expected create session error, got %v",
			err,
		)
	}
}
