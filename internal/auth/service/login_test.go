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
	errGetUser = errors.New("get user error")
	errCompare = errors.New("compare password error")
)

type loginUserRepositoryFake struct {
	getByNicknameFunc func(
		ctx context.Context,
		nickname string,
	) (model.User, error)
}

func (f *loginUserRepositoryFake) Create(
	context.Context,
	repository.CreateUserParams,
) (model.User, error) {
	panic("not implemented")
}

func (f *loginUserRepositoryFake) GetByID(
	context.Context,
	uuid.UUID,
) (model.User, error) {
	panic("not implemented")
}

func (f *loginUserRepositoryFake) GetByNickname(
	ctx context.Context,
	nickname string,
) (model.User, error) {
	return f.getByNicknameFunc(ctx, nickname)
}

type loginPasswordHasherFake struct {
	compareFunc func(
		password string,
		hash string,
	) (bool, error)
}

func (f *loginPasswordHasherFake) Hash(
	string,
) (string, error) {
	panic("not implemented")
}

func (f *loginPasswordHasherFake) Compare(
	password string,
	hash string,
) (bool, error) {
	return f.compareFunc(password, hash)
}

type loginSessionRepositoryFake struct {
	createFunc func(
		ctx context.Context,
		params repository.CreateSessionParams,
	) (model.AuthSession, error)
}

func (f *loginSessionRepositoryFake) Create(
	ctx context.Context,
	params repository.CreateSessionParams,
) (model.AuthSession, error) {
	return f.createFunc(ctx, params)
}

func (f *loginSessionRepositoryFake) GetByTokenHash(
	context.Context,
	string,
) (model.AuthSession, error) {
	panic("not implemented")
}

func (f *loginSessionRepositoryFake) Revoke(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *loginSessionRepositoryFake) RevokeAllByUserID(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *loginSessionRepositoryFake) UpdateLastUsedAt(
	context.Context,
	uuid.UUID,
	time.Time,
) error {
	panic("not implemented")
}

func (f *loginSessionRepositoryFake) DeleteExpired(
	context.Context,
	time.Time,
) (int64, error) {
	panic("not implemented")
}

type loginTokenManagerFake struct{}

func (loginTokenManagerFake) CreateAccessToken(
	context.Context,
	uuid.UUID,
	time.Time,
) (token.AccessToken, error) {
	return token.AccessToken{
		Value:     "access-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}, nil
}

func (loginTokenManagerFake) ParseAccessToken(
	context.Context,
	string,
) (token.AccessTokenClaims, error) {
	panic("not implemented")
}

func (loginTokenManagerFake) CreateRefreshToken(
	context.Context,
) (token.RefreshToken, error) {
	return token.RefreshToken{
		Value: "refresh-token",
		Hash:  "refresh-hash",
	}, nil
}

func (loginTokenManagerFake) HashRefreshToken(
	value string,
) string {
	return "hash-" + value
}

func TestService_Login_Success(t *testing.T) {
	userID := uuid.New()

	var receivedNickname string
	var createdSession repository.CreateSessionParams

	service := New(
		&loginUserRepositoryFake{
			getByNicknameFunc: func(
				_ context.Context,
				nickname string,
			) (model.User, error) {
				receivedNickname = nickname

				return model.User{
					ID:                 userID,
					Nickname:           "nikita",
					NicknameNormalized: "nikita",
					PasswordHash:       "stored-hash",
					Status:             model.UserStatusActive,
				}, nil
			},
		},
		&loginSessionRepositoryFake{
			createFunc: func(
				_ context.Context,
				params repository.CreateSessionParams,
			) (model.AuthSession, error) {
				createdSession = params

				return model.AuthSession{
					ID:     params.ID,
					UserID: params.UserID,
				}, nil
			},
		},
		&loginPasswordHasherFake{
			compareFunc: func(
				password string,
				hash string,
			) (bool, error) {
				if password != "password123" {
					t.Fatalf(
						"unexpected password: %q",
						password,
					)
				}

				if hash != "stored-hash" {
					t.Fatalf(
						"unexpected hash: %q",
						hash,
					)
				}

				return true, nil
			},
		},
		loginTokenManagerFake{},
		24*time.Hour,
	)

	result, err := service.Login(
		context.Background(),
		LoginInput{
			Nickname:  "  NiKiTa  ",
			Password:  "password123",
			UserAgent: "test-agent",
		},
	)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if receivedNickname != "nikita" {
		t.Fatalf(
			"nickname was not normalized: %q",
			receivedNickname,
		)
	}

	if result.User.ID != userID {
		t.Fatalf(
			"unexpected user ID: got %s",
			result.User.ID,
		)
	}

	if result.AccessToken != "access-token" {
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

	if createdSession.UserID != userID {
		t.Fatalf(
			"unexpected session user ID: got %s",
			createdSession.UserID,
		)
	}

	if createdSession.RefreshTokenHash != "refresh-hash" {
		t.Fatalf(
			"unexpected refresh hash: %q",
			createdSession.RefreshTokenHash,
		)
	}
}

func TestService_Login_UserNotFound(t *testing.T) {
	service := New(
		&loginUserRepositoryFake{
			getByNicknameFunc: func(
				context.Context,
				string,
			) (model.User, error) {
				return model.User{}, autherrors.ErrUserNotFound
			},
		},
		emptySessionRepositoryFake(),
		&loginPasswordHasherFake{
			compareFunc: func(
				string,
				string,
			) (bool, error) {
				panic("compare must not be called")
			},
		},
		loginTokenManagerFake{},
		time.Hour,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Nickname: "unknown",
			Password: "password123",
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}

func TestService_Login_WrongPassword(t *testing.T) {
	service := New(
		&loginUserRepositoryFake{
			getByNicknameFunc: func(
				context.Context,
				string,
			) (model.User, error) {
				return model.User{
					ID:           uuid.New(),
					PasswordHash: "stored-hash",
					Status:       model.UserStatusActive,
				}, nil
			},
		},
		emptySessionRepositoryFake(),
		&loginPasswordHasherFake{
			compareFunc: func(
				string,
				string,
			) (bool, error) {
				return false, nil
			},
		},
		loginTokenManagerFake{},
		time.Hour,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Nickname: "nikita",
			Password: "wrong-password",
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}

func TestService_Login_UserBlocked(t *testing.T) {
	service := New(
		&loginUserRepositoryFake{
			getByNicknameFunc: func(
				context.Context,
				string,
			) (model.User, error) {
				return model.User{
					ID:           uuid.New(),
					PasswordHash: "stored-hash",
					Status:       model.UserStatusBlocked,
				}, nil
			},
		},
		emptySessionRepositoryFake(),
		&loginPasswordHasherFake{
			compareFunc: func(
				string,
				string,
			) (bool, error) {
				return true, nil
			},
		},
		loginTokenManagerFake{},
		time.Hour,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Nickname: "nikita",
			Password: "password123",
		},
	)

	if !errors.Is(err, ErrUserBlocked) {
		t.Fatalf(
			"expected ErrUserBlocked, got %v",
			err,
		)
	}
}

func TestService_Login_CompareError(t *testing.T) {
	service := New(
		&loginUserRepositoryFake{
			getByNicknameFunc: func(
				context.Context,
				string,
			) (model.User, error) {
				return model.User{
					ID:           uuid.New(),
					PasswordHash: "stored-hash",
					Status:       model.UserStatusActive,
				}, nil
			},
		},
		emptySessionRepositoryFake(),
		&loginPasswordHasherFake{
			compareFunc: func(
				string,
				string,
			) (bool, error) {
				return false, errCompare
			},
		},
		loginTokenManagerFake{},
		time.Hour,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Nickname: "nikita",
			Password: "password123",
		},
	)

	if !errors.Is(err, errCompare) {
		t.Fatalf(
			"expected compare error, got %v",
			err,
		)
	}
}

func TestService_Login_GetUserError(t *testing.T) {
	service := New(
		&loginUserRepositoryFake{
			getByNicknameFunc: func(
				context.Context,
				string,
			) (model.User, error) {
				return model.User{}, errGetUser
			},
		},
		emptySessionRepositoryFake(),
		&loginPasswordHasherFake{
			compareFunc: func(
				string,
				string,
			) (bool, error) {
				panic("compare must not be called")
			},
		},
		loginTokenManagerFake{},
		time.Hour,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Nickname: "nikita",
			Password: "password123",
		},
	)

	if !errors.Is(err, errGetUser) {
		t.Fatalf(
			"expected get user error, got %v",
			err,
		)
	}
}

func TestService_Login_CreateSessionError(t *testing.T) {
	service := New(
		&loginUserRepositoryFake{
			getByNicknameFunc: func(
				context.Context,
				string,
			) (model.User, error) {
				return model.User{
					ID:           uuid.New(),
					PasswordHash: "stored-hash",
					Status:       model.UserStatusActive,
				}, nil
			},
		},
		&loginSessionRepositoryFake{
			createFunc: func(
				context.Context,
				repository.CreateSessionParams,
			) (model.AuthSession, error) {
				return model.AuthSession{}, errCreateSession
			},
		},
		&loginPasswordHasherFake{
			compareFunc: func(
				string,
				string,
			) (bool, error) {
				return true, nil
			},
		},
		loginTokenManagerFake{},
		time.Hour,
	)

	_, err := service.Login(
		context.Background(),
		LoginInput{
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
