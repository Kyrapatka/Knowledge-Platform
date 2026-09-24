package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
	"github.com/google/uuid"
)

func (s *Service) Login(
	ctx context.Context,
	input LoginInput,
) (result AuthResult, resultErr error) {
	defer func() {
		if errors.Is(resultErr, ErrInvalidCredentials) || errors.Is(resultErr, ErrUserBlocked) {
			s.Publish(ctx, analytics.New(analytics.LoginFailed, uuid.Nil))
		}
	}()
	nickname := NormalizeNickname(input.Nickname)

	if nickname == "" || input.Password == "" {
		return AuthResult{}, ErrInvalidCredentials
	}

	user, err := s.users.GetByNickname(
		ctx,
		nickname,
	)
	if err != nil {
		if errors.Is(err, autherrors.ErrUserNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}

		return AuthResult{}, fmt.Errorf(
			"get user by nickname: %w",
			err,
		)
	}

	match, err := s.passwords.Compare(
		input.Password,
		user.PasswordHash,
	)
	if err != nil {
		return AuthResult{}, fmt.Errorf(
			"compare password: %w",
			err,
		)
	}

	if !match {
		return AuthResult{}, ErrInvalidCredentials
	}

	if user.Status == model.UserStatusBlocked {
		return AuthResult{}, ErrUserBlocked
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

	refreshToken, err := s.tokens.CreateRefreshToken(ctx)
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

	s.Publish(ctx, analytics.New(analytics.UserLoggedIn, user.ID))
	return AuthResult{
		User:             user,
		AccessToken:      accessToken.Value,
		RefreshToken:     refreshToken.Value,
		ExpiresAt:        accessToken.ExpiresAt,
		RefreshExpiresAt: now.Add(s.refreshTTL),
	}, nil
}
