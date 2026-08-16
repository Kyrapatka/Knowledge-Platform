package handler

import (
	"net/netip"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type authResponse struct {
	User         userResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
}

type userResponse struct {
	ID       uuid.UUID `json:"id"`
	Nickname string    `json:"nickname"`
	Status   string    `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func toAuthResponse(result service.AuthResult) authResponse {
	return authResponse{
		User: userResponse{
			ID:       result.User.ID,
			Nickname: result.User.Nickname,
			Status:   string(result.User.Status),
		},
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
	}
}

func clientIPAddress(c *gin.Context) *netip.Addr {
	value := c.ClientIP()

	address, err := netip.ParseAddr(value)
	if err != nil {
		return nil
	}

	return &address
}
