package httpmiddleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOrdinaryBodyLimitsKnownAndChunked(t *testing.T) {
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/folders"} {
		for _, known := range []bool{true, false} {
			r := gin.New()
			r.Use(OrdinaryBodyLimit())
			called := false
			r.POST(path, func(c *gin.Context) {
				var body map[string]string
				if c.ShouldBindJSON(&body) != nil {
					c.Status(400)
					return
				}
				called = true
				c.Status(204)
			})
			size := 1 << 20
			if strings.Contains(path, "/auth/") {
				size = 64 << 10
			}
			request := httptest.NewRequest("POST", path, strings.NewReader(`{"value":"`+strings.Repeat("x", size)+`"}`))
			request.Header.Set("Content-Type", "application/json")
			if !known {
				request.ContentLength = -1
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, request)
			if called || (w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest) {
				t.Fatalf("unbounded %s known=%v status=%d", path, known, w.Code)
			}
		}
	}
}
func TestOrdinaryLimitPreservesLargeImportAllowance(t *testing.T) {
	r := gin.New()
	r.Use(OrdinaryBodyLimit())
	r.POST("/api/v1/folders/import", func(c *gin.Context) { c.Status(204) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/folders/import", strings.NewReader(strings.Repeat("x", 2<<20))))
	if w.Code != 204 {
		t.Fatal("ordinary limit shadowed import-specific limit")
	}
}
