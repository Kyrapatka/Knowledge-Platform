package httpmiddleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAuthLimiterBoundsCleanupAndSharedRoutes(t *testing.T) {
	options := AuthLimits{Login: 2, Register: 1, Refresh: 3, MaxKeys: 2, Window: time.Minute}
	l := &authLimiter{buckets: map[string]authBucket{}, options: options}
	now := time.Now()
	for range 2 {
		if ok, _ := l.allow("login:a", 2, now); !ok {
			t.Fatal("early rejection")
		}
	}
	if ok, retry := l.allow("login:a", 2, now); ok || retry != time.Minute {
		t.Fatal("missing limit")
	}
	l.allow("login:b", 2, now)
	if ok, _ := l.allow("login:c", 2, now); ok || len(l.buckets) != 2 {
		t.Fatal("unbounded keys")
	}
	if ok, _ := l.allow("login:c", 2, now.Add(time.Minute)); !ok || len(l.buckets) != 1 {
		t.Fatal("expired keys retained")
	}
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.Use(AuthRateLimit(options))
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/browser/login"} {
		r.POST(path, func(c *gin.Context) { c.Status(204) })
	}
	for i, path := range []string{"/api/v1/auth/login", "/api/v1/auth/browser/login", "/api/v1/auth/login"} {
		request := httptest.NewRequest("POST", path, nil)
		request.RemoteAddr = "192.0.2.1:1234"
		request.Header.Set("X-Forwarded-For", "198.51.100.1")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if i < 2 && response.Code != 204 {
			t.Fatal(response.Code)
		}
		if i == 2 && (response.Code != 429 || response.Header().Get("Retry-After") == "") {
			t.Fatal("missing 429/retry")
		}
	}
}

func TestProxyOriginPolicy(t *testing.T) {
	for _, tt := range []struct {
		name, peer   string
		trusted      []string
		scheme, host string
	}{
		{"default ignores forged headers", "192.0.2.2:42", nil, "http", "app.test"},
		{"trusted proxy", "192.0.2.2:42", []string{"192.0.2.0/24"}, "https", "public.test"},
		{"outside range", "198.51.100.1:42", []string{"192.0.2.0/24"}, "http", "app.test"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := ProxyPolicy(tt.trusted)
			if err != nil {
				t.Fatal(err)
			}
			r := gin.New()
			r.Use(policy)
			r.GET("/", func(c *gin.Context) {
				scheme, host := RequestOrigin(c)
				if scheme != tt.scheme || host != tt.host {
					t.Fatalf("%s://%s", scheme, host)
				}
			})
			request := httptest.NewRequest("GET", "http://app.test/", nil)
			request.RemoteAddr = tt.peer
			request.Header.Set("X-Forwarded-Proto", "https")
			request.Header.Set("X-Forwarded-Host", "public.test")
			r.ServeHTTP(httptest.NewRecorder(), request)
		})
	}
	if _, err := ProxyPolicy([]string{"*"}); err == nil {
		t.Fatal("accepted invalid proxy")
	}
}
