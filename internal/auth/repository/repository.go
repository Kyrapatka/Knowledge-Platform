package repository

import (
	"context"
	"net/netip"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/google/uuid"
)

type CreateUserParams struct {
	ID                 uuid.UUID
	Nickname           string
	NicknameNormalized string
	PasswordHash       string
}

type UserRepository interface {
	Create(
		ctx context.Context,
		params CreateUserParams,
	) (model.User, error)

	GetByID(
		ctx context.Context,
		id uuid.UUID,
	) (model.User, error)

	GetByNickname(
		ctx context.Context,
		normalizedNickname string,
	) (model.User, error)
}

type CreateSessionParams struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	UserAgent        string
	IPAddress        *netip.Addr
	CreatedAt        time.Time
	ExpiresAt        time.Time
	LastUsedAt       time.Time
}

type SessionRepository interface {
	Create(
		ctx context.Context,
		params CreateSessionParams,
	) (model.AuthSession, error)

	GetByTokenHash(
		ctx context.Context,
		tokenHash string,
	) (model.AuthSession, error)

	Revoke(
		ctx context.Context,
		sessionID uuid.UUID,
		revokedAt time.Time,
	) error

	RevokeAllByUserID(
		ctx context.Context,
		userID uuid.UUID,
		revokedAt time.Time,
	) error

	UpdateLastUsedAt(
		ctx context.Context,
		sessionID uuid.UUID,
		lastUsedAt time.Time,
	) error

	DeleteExpired(
		ctx context.Context,
		now time.Time,
	) (int64, error)
}
