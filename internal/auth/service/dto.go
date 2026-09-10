package service

import (
	"net/netip"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
)

type RegisterInput struct {
	Nickname  string
	Password  string
	UserAgent string
	IPAddress *netip.Addr
}

type LoginInput struct {
	Nickname  string
	Password  string
	UserAgent string
	IPAddress *netip.Addr
}

type RefreshInput struct {
	RefreshToken string
}

type LogoutInput struct {
	RefreshToken string
}

type AuthResult struct {
	User             model.User
	AccessToken      string
	RefreshToken     string
	ExpiresAt        time.Time
	RefreshExpiresAt time.Time
}
