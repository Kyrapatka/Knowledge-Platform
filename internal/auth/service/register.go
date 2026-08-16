package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
	"github.com/google/uuid"
)

func (s *Service) Register(
	ctx context.Context,
	input RegisterInput,
) (AuthResult, error) {
	nickname := NormalizeNickname(input.Nickname)

	if err := validateNickname(nickname); err != nil {
		return AuthResult{}, err
	}

	if err := validatePassword(input.Password); err != nil {
		return AuthResult{}, err
	}

	passwordHash, err := s.passwords.Hash(
		input.Password,
	)
	if err != nil {
		return AuthResult{}, fmt.Errorf(
			"hash password: %w",
			err,
		)
	}

	user, err := s.users.Create(
		ctx,
		repository.CreateUserParams{
			ID:                 uuid.New(),
			Nickname:           nickname,
			NicknameNormalized: nickname,
			PasswordHash:       passwordHash,
		},
	)

	if err != nil {
		if errors.Is(err, autherrors.ErrNicknameTaken) {
			return AuthResult{}, ErrNicknameTaken
		}

		return AuthResult{}, fmt.Errorf(
			"create user: %w",
			err,
		)
	}

	if err != nil {
		return AuthResult{}, fmt.Errorf(
			"create user: %w",
			err,
		)
	}

	now := time.Now().UTC()

	accessToken, err := s.tokens.CreateAccessToken(
		ctx,
		user.ID,
		now,
	)
	if err != nil {
		return AuthResult{}, fmt.Errorf(
			"create access token: %w",
			err,
		)
	}

	refreshToken, err := s.tokens.CreateRefreshToken(
		ctx,
	)
	if err != nil {
		return AuthResult{}, fmt.Errorf(
			"create refresh token: %w",
			err,
		)
	}

	_, err = s.sessions.Create(
		ctx,
		repository.CreateSessionParams{
			ID:               uuid.New(),
			UserID:           user.ID,
			RefreshTokenHash: refreshToken.Hash,
			UserAgent:        input.UserAgent,
			IPAddress:        input.IPAddress,
			CreatedAt:        now,
			ExpiresAt:        now.Add(s.refreshTTL),
			LastUsedAt:       now,
		},
	)
	if err != nil {
		return AuthResult{}, fmt.Errorf(
			"create auth session: %w",
			err,
		)
	}

	return AuthResult{
		User:         user,
		AccessToken:  accessToken.Value,
		RefreshToken: refreshToken.Value,
		ExpiresAt:    accessToken.ExpiresAt,
	}, nil
}

func validateNickname(
	nickname string,
) error {
	if len(nickname) < 3 || len(nickname) > 16 {
		return ErrInvalidNickname
	}

	return nil
}

func validatePassword(
	password string,
) error {
	if len(password) < 8 || len(password) > 128 {
		return ErrInvalidPassword
	}

	return nil
}
