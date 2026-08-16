package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	if db == nil {
		panic("postgres session repository: db is nil")
	}

	return &SessionRepository{
		db: db,
	}
}

func (r *SessionRepository) Create(
	ctx context.Context,
	params repository.CreateSessionParams,
) (model.AuthSession, error) {
	sessionModel := AuthSessionModel{
		ID:               params.ID,
		UserID:           params.UserID,
		RefreshTokenHash: params.RefreshTokenHash,
		UserAgent:        optionalString(params.UserAgent),
		IPAddress:        ipAddressToString(params.IPAddress),
		CreatedAt:        params.CreatedAt,
		ExpiresAt:        params.ExpiresAt,
		LastUsedAt:       params.LastUsedAt,
	}

	err := r.db.
		WithContext(ctx).
		Create(&sessionModel).
		Error
	if err != nil {
		return model.AuthSession{}, fmt.Errorf(
			"create auth session: %w",
			err,
		)
	}

	session, err := sessionToDomain(sessionModel)
	if err != nil {
		return model.AuthSession{}, fmt.Errorf(
			"convert created auth session: %w",
			err,
		)
	}

	return session, nil
}

func (r *SessionRepository) GetByTokenHash(
	ctx context.Context,
	tokenHash string,
) (model.AuthSession, error) {
	var sessionModel AuthSessionModel

	err := r.db.
		WithContext(ctx).
		Where(
			"refresh_token_hash = ?",
			tokenHash,
		).
		First(&sessionModel).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AuthSession{}, autherrors.ErrSessionNotFound
		}

		return model.AuthSession{}, fmt.Errorf(
			"get auth session by token hash: %w",
			err,
		)
	}

	session, err := sessionToDomain(sessionModel)
	if err != nil {
		return model.AuthSession{}, fmt.Errorf(
			"convert auth session: %w",
			err,
		)
	}

	return session, nil
}

func (r *SessionRepository) Revoke(
	ctx context.Context,
	sessionID uuid.UUID,
	revokedAt time.Time,
) error {
	result := r.db.
		WithContext(ctx).
		Model(&AuthSessionModel{}).
		Where(
			"id = ? AND revoked_at IS NULL",
			sessionID,
		).
		Update("revoked_at", revokedAt)

	if result.Error != nil {
		return fmt.Errorf(
			"revoke auth session: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return autherrors.ErrSessionNotFound
	}

	return nil
}

func (r *SessionRepository) RevokeAllByUserID(
	ctx context.Context,
	userID uuid.UUID,
	revokedAt time.Time,
) error {
	result := r.db.
		WithContext(ctx).
		Model(&AuthSessionModel{}).
		Where(
			"user_id = ? AND revoked_at IS NULL",
			userID,
		).
		Update("revoked_at", revokedAt)

	if result.Error != nil {
		return fmt.Errorf(
			"revoke all user auth sessions: %w",
			result.Error,
		)
	}

	return nil
}

func (r *SessionRepository) UpdateLastUsedAt(
	ctx context.Context,
	sessionID uuid.UUID,
	lastUsedAt time.Time,
) error {
	result := r.db.
		WithContext(ctx).
		Model(&AuthSessionModel{}).
		Where("id = ?", sessionID).
		Update("last_used_at", lastUsedAt)

	if result.Error != nil {
		return fmt.Errorf(
			"update auth session last used time: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return autherrors.ErrSessionNotFound
	}

	return nil
}

func (r *SessionRepository) DeleteExpired(
	ctx context.Context,
	now time.Time,
) (int64, error) {
	result := r.db.
		WithContext(ctx).
		Where("expires_at < ?", now).
		Delete(&AuthSessionModel{})

	if result.Error != nil {
		return 0, fmt.Errorf(
			"delete expired auth sessions: %w",
			result.Error,
		)
	}

	return result.RowsAffected, nil
}

func sessionToDomain(
	session AuthSessionModel,
) (model.AuthSession, error) {
	ipAddress, err := parseIPAddress(session.IPAddress)
	if err != nil {
		return model.AuthSession{}, err
	}

	return model.AuthSession{
		ID:               session.ID,
		UserID:           session.UserID,
		RefreshTokenHash: session.RefreshTokenHash,
		UserAgent:        valueOrEmpty(session.UserAgent),
		IPAddress:        ipAddress,
		CreatedAt:        session.CreatedAt,
		ExpiresAt:        session.ExpiresAt,
		RevokedAt:        session.RevokedAt,
		LastUsedAt:       session.LastUsedAt,
	}, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func ipAddressToString(address *netip.Addr) *string {
	if address == nil {
		return nil
	}

	value := address.String()

	return &value
}

func parseIPAddress(
	value *string,
) (*netip.Addr, error) {
	if value == nil || *value == "" {
		return nil, nil
	}

	address, err := netip.ParseAddr(*value)
	if err != nil {
		return nil, fmt.Errorf(
			"parse auth session IP address: %w",
			err,
		)
	}

	return &address, nil
}

var _ repository.SessionRepository = (*SessionRepository)(nil)
