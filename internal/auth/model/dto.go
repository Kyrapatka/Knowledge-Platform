package model

import (
	"time"

	"github.com/google/uuid"
)

type RegisterRequest struct {
	Nickname string `json:"nickname" binding:"required,min=3,max=16"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

type LoginRequest struct {
	Nickname string `json:"nickname" binding:"required,min=3,max=16"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

type UserResponse struct {
	ID        uuid.UUID  `json:"id"`
	Nickname  string     `json:"nickname"`
	Status    UserStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
}

type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type AuthResponse struct {
	User   UserResponse      `json:"user"`
	Tokens TokenPairResponse `json:"tokens"`
}

func ToUserResponse(user User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Nickname:  user.Nickname,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
	}
}
