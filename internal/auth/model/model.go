package model

import (
	"net/netip"
	"time"

	"github.com/google/uuid"
)

const (
	MinNicknameLength = 3
	MaxNicknameLength = 16

	MinPasswordLength = 8
	MaxPasswordLength = 128
)

type UserStatus string

const (
	UserStatusActive  UserStatus = "active"
	UserStatusBlocked UserStatus = "blocked"
)

type User struct {
	ID                 uuid.UUID
	Nickname           string
	NicknameNormalized string
	PasswordHash       string
	Status             UserStatus
	CreatedAt          time.Time
}

type AuthSession struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	UserAgent        string
	IPAddress        *netip.Addr
	CreatedAt        time.Time
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	LastUsedAt       time.Time
}

func (s AuthSession) IsRevoked() bool {
	return s.RevokedAt != nil
}

func (s AuthSession) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

func (s AuthSession) IsActive(now time.Time) bool {
	return !s.IsRevoked() && !s.IsExpired(now)
}
