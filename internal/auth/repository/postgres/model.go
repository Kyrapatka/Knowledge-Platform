package postgres

import (
	"time"

	"github.com/google/uuid"
)

type UserModel struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	Nickname           string    `gorm:"type:varchar(16);not null"`
	NicknameNormalized string    `gorm:"type:varchar(16);not null;uniqueIndex:ux_users_nickname_normalized"`
	PasswordHash       string    `gorm:"type:text;not null"`
	Status             string    `gorm:"type:varchar(16);not null;default:active"`
	CreatedAt          time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

func (UserModel) TableName() string {
	return "users"
}

type AuthSessionModel struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	UserID uuid.UUID `gorm:"type:uuid;not null;index:ix_auth_sessions_user_id"`

	User UserModel `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`

	RefreshTokenHash string `gorm:"type:text;not null;uniqueIndex:ux_auth_sessions_refresh_token_hash;check:auth_sessions_refresh_token_hash_not_empty_check,refresh_token_hash <> ''"`

	UserAgent *string `gorm:"type:text"`

	IPAddress *string `gorm:"type:inet"`

	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`

	ExpiresAt time.Time `gorm:"type:timestamptz;not null;index:ix_auth_sessions_expires_at;check:auth_sessions_expiration_check,expires_at > created_at"`

	RevokedAt *time.Time `gorm:"type:timestamptz;check:auth_sessions_revoked_at_check,revoked_at IS NULL OR revoked_at >= created_at"`

	LastUsedAt time.Time `gorm:"type:timestamptz;not null;check:auth_sessions_last_used_at_check,last_used_at >= created_at"`
}

func (AuthSessionModel) TableName() string {
	return "auth_sessions"
}
