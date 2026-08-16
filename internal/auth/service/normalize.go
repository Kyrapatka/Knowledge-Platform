package service

import "strings"

func NormalizeNickname(value string) string {
	return strings.ToLower(
		strings.TrimSpace(value),
	)
}
