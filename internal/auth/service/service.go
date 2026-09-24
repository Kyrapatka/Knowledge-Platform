package service

import (
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/password"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/repository"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/token"
)

type Service struct {
	analytics.Emitter
	users      repository.UserRepository
	sessions   repository.SessionRepository
	passwords  password.Hasher
	tokens     token.Manager
	refreshTTL time.Duration
}

func New(
	users repository.UserRepository,
	sessions repository.SessionRepository,
	passwords password.Hasher,
	tokens token.Manager,
	refreshTTL time.Duration,
) *Service {
	if users == nil {
		panic("auth service: user repository is nil")
	}

	if sessions == nil {
		panic("auth service: session repository is nil")
	}

	if passwords == nil {
		panic("auth service: password hasher is nil")
	}

	if tokens == nil {
		panic("auth service: token manager is nil")
	}

	if refreshTTL <= 0 {
		panic("auth service: refresh TTL must be positive")
	}

	return &Service{
		users:      users,
		sessions:   sessions,
		passwords:  passwords,
		tokens:     tokens,
		refreshTTL: refreshTTL,
	}
}
