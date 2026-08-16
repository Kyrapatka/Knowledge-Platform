package token

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AccessToken struct {
	Value     string
	ExpiresAt time.Time
}

type AccessTokenClaims struct {
	UserID uuid.UUID
}

type RefreshToken struct {
	Value string
	Hash  string
}

type Manager interface {
	CreateAccessToken(
		ctx context.Context,
		userID uuid.UUID,
		issuedAt time.Time,
	) (AccessToken, error)

	ParseAccessToken(
		ctx context.Context,
		token string,
	) (AccessTokenClaims, error)

	CreateRefreshToken(
		ctx context.Context,
	) (RefreshToken, error)

	HashRefreshToken(token string) string
}
