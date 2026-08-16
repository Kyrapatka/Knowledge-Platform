package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/google/uuid"
)

func (s *Service) Logout(
	ctx context.Context,
	input LogoutInput,
) error {
	if input.RefreshToken == "" {
		return ErrInvalidRefreshToken
	}

	tokenHash := s.tokens.HashRefreshToken(
		input.RefreshToken,
	)

	session, err := s.sessions.GetByTokenHash(
		ctx,
		tokenHash,
	)
	if err != nil {
		if errors.Is(err, autherrors.ErrSessionNotFound) {
			return nil
		}

		return fmt.Errorf(
			"get auth session: %w",
			err,
		)
	}

	if session.IsRevoked() {
		return nil
	}

	err = s.sessions.Revoke(
		ctx,
		session.ID,
		time.Now().UTC(),
	)
	if err != nil {
		// Сессия могла быть отозвана параллельным запросом.
		if errors.Is(err, autherrors.ErrSessionNotFound) {
			return nil
		}

		return fmt.Errorf(
			"revoke auth session: %w",
			err,
		)
	}

	return nil
}

func (s *Service) LogoutAll(
	ctx context.Context,
	userID uuid.UUID,
) error {
	if userID == uuid.Nil {
		return ErrInvalidCredentials
	}

	err := s.sessions.RevokeAllByUserID(
		ctx,
		userID,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf(
			"revoke all auth sessions: %w",
			err,
		)
	}

	return nil
}
