package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
)

func (s *Service) Refresh(
	ctx context.Context,
	input RefreshInput,
) (AuthResult, error) {
	return s.refresh(ctx, input, false)
}

// RefreshBrowser rotates the browser's credential atomically while retaining
// the session's original expiry. JSON API clients retain their existing contract.
func (s *Service) RefreshBrowser(ctx context.Context, input RefreshInput) (AuthResult, error) {
	return s.refresh(ctx, input, true)
}

func (s *Service) refresh(ctx context.Context, input RefreshInput, rotate bool) (AuthResult, error) {
	if input.RefreshToken == "" {
		return AuthResult{}, ErrInvalidRefreshToken
	}

	now := time.Now().UTC()

	refreshTokenHash := s.tokens.HashRefreshToken(
		input.RefreshToken,
	)

	session, err := s.sessions.GetByTokenHash(
		ctx,
		refreshTokenHash,
	)
	if err != nil {
		if errors.Is(err, autherrors.ErrSessionNotFound) {
			return AuthResult{}, ErrInvalidRefreshToken
		}

		return AuthResult{}, fmt.Errorf(
			"get auth session: %w",
			err,
		)
	}

	if session.IsRevoked() {
		return AuthResult{}, ErrInvalidRefreshToken
	}

	if session.IsExpired(now) {
		return AuthResult{}, ErrSessionExpired
	}

	user, err := s.users.GetByID(
		ctx,
		session.UserID,
	)
	if err != nil {
		if errors.Is(err, autherrors.ErrUserNotFound) {
			return AuthResult{}, ErrInvalidRefreshToken
		}

		return AuthResult{}, fmt.Errorf(
			"get user: %w",
			err,
		)
	}

	if user.Status == model.UserStatusBlocked {
		return AuthResult{}, ErrUserBlocked
	}

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

	refreshValue := input.RefreshToken
	if rotate {
		rotator, ok := s.sessions.(repository.BrowserSessionRotator)
		if !ok {
			return AuthResult{}, fmt.Errorf("browser refresh rotation is unavailable")
		}
		next, createErr := s.tokens.CreateRefreshToken(ctx)
		if createErr != nil {
			return AuthResult{}, fmt.Errorf("create refresh token: %w", createErr)
		}
		err = rotator.RotateRefreshToken(ctx, session.ID, refreshTokenHash, next.Hash, now)
		if errors.Is(err, autherrors.ErrSessionNotFound) {
			return AuthResult{}, ErrInvalidRefreshToken
		}
		refreshValue = next.Value
	} else {
		err = s.sessions.UpdateLastUsedAt(ctx, session.ID, now)
	}
	if err != nil {
		return AuthResult{}, fmt.Errorf(
			"update auth session: %w",
			err,
		)
	}

	return AuthResult{
		User:             user,
		AccessToken:      accessToken.Value,
		RefreshToken:     refreshValue,
		ExpiresAt:        accessToken.ExpiresAt,
		RefreshExpiresAt: session.ExpiresAt,
	}, nil
}
