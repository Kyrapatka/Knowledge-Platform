package postgres

import (
	"context"
	"errors"
	"fmt"

	autherrors "github.com/Kyrapatka/knowledge-platform/internal/auth"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	if db == nil {
		panic("postgres user repository: db is nil")
	}

	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) Create(
	ctx context.Context,
	params repository.CreateUserParams,
) (model.User, error) {
	userModel := UserModel{
		ID:                 params.ID,
		Nickname:           params.Nickname,
		NicknameNormalized: params.NicknameNormalized,
		PasswordHash:       params.PasswordHash,
		Status:             string(model.UserStatusActive),
	}

	err := r.db.
		WithContext(ctx).
		Create(&userModel).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.User{}, autherrors.ErrNicknameTaken
		}

		return model.User{}, fmt.Errorf(
			"create user: %w",
			err,
		)
	}

	return userToDomain(userModel), nil
}

func (r *UserRepository) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (model.User, error) {
	var userModel UserModel

	err := r.db.
		WithContext(ctx).
		Where("id = ?", id).
		First(&userModel).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, autherrors.ErrUserNotFound
		}

		return model.User{}, fmt.Errorf(
			"get user by ID: %w",
			err,
		)
	}

	return userToDomain(userModel), nil
}

func (r *UserRepository) GetByNickname(
	ctx context.Context,
	normalizedNickname string,
) (model.User, error) {
	var userModel UserModel

	err := r.db.
		WithContext(ctx).
		Where(
			"nickname_normalized = ?",
			normalizedNickname,
		).
		First(&userModel).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, autherrors.ErrUserNotFound
		}

		return model.User{}, fmt.Errorf(
			"get user by nickname: %w",
			err,
		)
	}

	return userToDomain(userModel), nil
}

func userToDomain(user UserModel) model.User {
	return model.User{
		ID:                 user.ID,
		Nickname:           user.Nickname,
		NicknameNormalized: user.NicknameNormalized,
		PasswordHash:       user.PasswordHash,
		Status:             model.UserStatus(user.Status),
		CreatedAt:          user.CreatedAt,
	}
}

var _ repository.UserRepository = (*UserRepository)(nil)
