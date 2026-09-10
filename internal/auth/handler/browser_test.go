package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/auth/model"
	"github.com/Kyrapatka/knowledge-platform/internal/auth/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type browserServiceFake struct {
	*authServiceFake
	refreshBrowserFunc func(context.Context, service.RefreshInput) (service.AuthResult, error)
}

func (f browserServiceFake) RefreshBrowser(ctx context.Context, input service.RefreshInput) (service.AuthResult, error) {
	return f.refreshBrowserFunc(ctx, input)
}

func browserTestRouter(s AuthService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	New(s).RegisterPublicRoutes(router.Group("/api/v1/auth"))
	return router
}

func browserTestRequest(method, endpoint, origin, body string) *http.Request {
	request := httptest.NewRequest(method, "http://localhost:8080/api/v1/auth/browser/"+endpoint, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	return request
}

func TestBrowserLoginKeepsRefreshCredentialOutOfJSON(t *testing.T) {
	result := service.AuthResult{
		User:        model.User{ID: uuid.New(), Nickname: "learner", Status: model.UserStatusActive},
		AccessToken: "access", RefreshToken: "private-refresh", ExpiresAt: time.Now().Add(time.Hour),
		RefreshExpiresAt: time.Now().Add(24 * time.Hour),
	}
	router := browserTestRouter(&authServiceFake{loginFunc: func(_ context.Context, in service.LoginInput) (service.AuthResult, error) {
		if in.Nickname != "learner" || in.Password != "password123" {
			t.Fatal("credentials lost")
		}
		return result, nil
	}})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, browserTestRequest("POST", "login", "http://localhost:8080", `{"nickname":"learner","password":"password123"}`))
	if recorder.Code != 200 {
		t.Fatalf("login: %d %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, exists := body["refresh_token"]; exists || strings.Contains(recorder.Body.String(), "private-refresh") {
		t.Fatal("refresh credential leaked into JSON")
	}
	if body["access_token"] != "access" || body["user"].(map[string]any)["nickname"] != "learner" {
		t.Fatal("missing access token or user")
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies: %v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != browserRefreshCookie || cookie.Value != "private-refresh" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/api/v1/auth/browser" || cookie.MaxAge <= 0 || cookie.Secure {
		t.Fatalf("unexpected cookie attributes: %+v", cookie)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("auth responses must not be cached")
	}
}

func TestBrowserAuthRejectsForeignAndMissingOrigin(t *testing.T) {
	router := browserTestRouter(&authServiceFake{})
	for _, origin := range []string{"", "null", "https://localhost:8080", "http://evil.example", "http://localhost:8080@evil.example", "http://localhost:8080/path"} {
		for _, endpoint := range []string{"login", "register", "refresh", "logout"} {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, browserTestRequest("POST", endpoint, origin, "{}"))
			if recorder.Code != http.StatusForbidden || len(recorder.Result().Cookies()) != 0 {
				t.Fatalf("origin %q endpoint %s: %d", origin, endpoint, recorder.Code)
			}
		}
	}
}

func TestBrowserRefreshReadsCookieAndRotates(t *testing.T) {
	router := browserTestRouter(browserServiceFake{
		authServiceFake: &authServiceFake{},
		refreshBrowserFunc: func(_ context.Context, in service.RefreshInput) (service.AuthResult, error) {
			if in.RefreshToken != "old-cookie" {
				t.Fatal("refresh did not use HttpOnly cookie")
			}
			return service.AuthResult{User: model.User{ID: uuid.New(), Nickname: "learner"}, AccessToken: "new-access", RefreshToken: "rotated-cookie", RefreshExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	})
	request := browserTestRequest("POST", "refresh", "http://localhost:8080", `{"refresh_token":"body-must-be-ignored"}`)
	request.AddCookie(&http.Cookie{Name: browserRefreshCookie, Value: "old-cookie"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Result().Cookies()[0].Value != "rotated-cookie" || strings.Contains(recorder.Body.String(), "rotated-cookie") {
		t.Fatalf("refresh: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestBrowserLogoutRevokesAndClearsCookie(t *testing.T) {
	called := false
	router := browserTestRouter(&authServiceFake{logoutFunc: func(_ context.Context, in service.LogoutInput) error {
		called = true
		if in.RefreshToken != "cookie" {
			t.Fatal("wrong token revoked")
		}
		return nil
	}})
	request := browserTestRequest("POST", "logout", "http://localhost:8080", "")
	request.AddCookie(&http.Cookie{Name: browserRefreshCookie, Value: "cookie"})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || !called || recorder.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout must revoke and expire cookie")
	}
}

func TestBrowserSecureCookieUsesTLSOnly(t *testing.T) {
	router := browserTestRouter(&authServiceFake{loginFunc: func(context.Context, service.LoginInput) (service.AuthResult, error) {
		return service.AuthResult{RefreshToken: "cookie"}, nil
	}})
	request := httptest.NewRequest("POST", "https://localhost/api/v1/auth/browser/login", strings.NewReader(`{"nickname":"learner","password":"password123"}`))
	request.Header.Set("Origin", "https://localhost")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != 200 || !recorder.Result().Cookies()[0].Secure {
		t.Fatal("TLS requires Secure cookie")
	}
}
