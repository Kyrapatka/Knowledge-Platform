package auth

import "errors"

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrNicknameTaken   = errors.New("nickname already taken")
	ErrSessionNotFound = errors.New("auth session not found")
)
