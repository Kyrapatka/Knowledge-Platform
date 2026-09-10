package handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
)

const browserRefreshCookie = "kp_refresh"
const browserCookiePathKey = "auth.browser.cookie_path"

type browserAuthResponse struct {
	User        userResponse `json:"user"`
	AccessToken string       `json:"access_token"`
	ExpiresAt   time.Time    `json:"expires_at"`
}

type browserRefresher interface {
	RefreshBrowser(context.Context, service.RefreshInput) (service.AuthResult, error)
}

func (h *Handler) registerBrowserRoutes(router *gin.RouterGroup) {
	group := router.Group("/browser")
	group.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		// This adapter is same-origin only. Do not trust forwarded host/protocol
		// headers: local development proxies must preserve the original Host.
		origin, err := url.Parse(c.GetHeader("Origin"))
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		if err != nil || origin.Scheme != scheme || !strings.EqualFold(origin.Host, c.Request.Host) || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse{Error: "invalid_origin"})
			return
		}
		c.Set(browserCookiePathKey, group.BasePath())
		c.Next()
	})
	group.POST("/login", h.BrowserLogin)
	group.POST("/register", h.BrowserRegister)
	group.POST("/refresh", h.BrowserRefresh)
	group.POST("/logout", h.BrowserLogout)
}

func (h *Handler) BrowserLogin(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid_request"})
		return
	}
	result, err := h.service.Login(c.Request.Context(), service.LoginInput{
		Nickname: request.Nickname, Password: request.Password,
		UserAgent: c.Request.UserAgent(), IPAddress: clientIPAddress(c),
	})
	if err != nil {
		browserError(c, err)
		return
	}
	writeBrowserAuth(c, http.StatusOK, result)
}

func (h *Handler) BrowserRegister(c *gin.Context) {
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid_request"})
		return
	}
	result, err := h.service.Register(c.Request.Context(), service.RegisterInput{
		Nickname: request.Nickname, Password: request.Password,
		UserAgent: c.Request.UserAgent(), IPAddress: clientIPAddress(c),
	})
	if err != nil {
		browserError(c, err)
		return
	}
	writeBrowserAuth(c, http.StatusCreated, result)
}

func (h *Handler) BrowserRefresh(c *gin.Context) {
	value, err := c.Cookie(browserRefreshCookie)
	if err != nil || value == "" {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "invalid_refresh_token"})
		return
	}
	refresher, ok := h.service.(browserRefresher)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, errorResponse{Error: "browser_auth_unavailable"})
		return
	}
	result, err := refresher.RefreshBrowser(c.Request.Context(), service.RefreshInput{RefreshToken: value})
	if err != nil {
		// A late response using an old token must not delete a cookie which a
		// concurrent refresh already rotated successfully.
		browserError(c, err)
		return
	}
	writeBrowserAuth(c, http.StatusOK, result)
}

func (h *Handler) BrowserLogout(c *gin.Context) {
	value, err := c.Cookie(browserRefreshCookie)
	if err == nil && value != "" {
		err = h.service.Logout(c.Request.Context(), service.LogoutInput{RefreshToken: value})
		if err != nil && !errors.Is(err, service.ErrInvalidRefreshToken) {
			browserError(c, err)
			return
		}
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name: browserRefreshCookie, Value: "", Path: c.GetString(browserCookiePathKey),
		MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true,
		Secure: c.Request.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
	c.Status(http.StatusNoContent)
}

func writeBrowserAuth(c *gin.Context, status int, result service.AuthResult) {
	cookie := &http.Cookie{
		Name: browserRefreshCookie, Value: result.RefreshToken,
		Path: c.GetString(browserCookiePathKey), HttpOnly: true,
		Secure: c.Request.TLS != nil, SameSite: http.SameSiteLaxMode,
	}
	if !result.RefreshExpiresAt.IsZero() {
		cookie.Expires = result.RefreshExpiresAt
		cookie.MaxAge = max(1, int(time.Until(result.RefreshExpiresAt).Seconds()))
	}
	http.SetCookie(c.Writer, cookie)
	c.JSON(status, browserAuthResponse{
		User:        userResponse{ID: result.User.ID, Nickname: result.User.Nickname, Status: string(result.User.Status)},
		AccessToken: result.AccessToken, ExpiresAt: result.ExpiresAt,
	})
}

func browserError(c *gin.Context, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		status, code = http.StatusUnauthorized, "invalid_credentials"
	case errors.Is(err, service.ErrInvalidRefreshToken):
		status, code = http.StatusUnauthorized, "invalid_refresh_token"
	case errors.Is(err, service.ErrSessionExpired):
		status, code = http.StatusUnauthorized, "session_expired"
	case errors.Is(err, service.ErrUserBlocked):
		status, code = http.StatusForbidden, "user_blocked"
	case errors.Is(err, service.ErrInvalidNickname):
		status, code = http.StatusBadRequest, "invalid_nickname"
	case errors.Is(err, service.ErrInvalidPassword):
		status, code = http.StatusBadRequest, "invalid_password"
	case errors.Is(err, service.ErrNicknameTaken):
		status, code = http.StatusConflict, "nickname_taken"
	}
	c.JSON(status, errorResponse{Error: code})
}
