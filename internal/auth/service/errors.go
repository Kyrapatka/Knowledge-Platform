package service

import "errors"

var (
	ErrNicknameTaken = errors.New("nickname already taken")

	ErrInvalidCredentials = errors.New("invalid credentials")

	ErrUserBlocked = errors.New("user is blocked")

	ErrInvalidRefreshToken = errors.New("invalid refresh token")

	ErrSessionExpired = errors.New("session expired")

	ErrInvalidNickname = errors.New("invalid nickname")

	ErrInvalidPassword = errors.New("invalid password")
)
