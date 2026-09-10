package webui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStaticRoutesKeepAPISeparate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dist := t.TempDir()
	if err := os.Mkdir(filepath.Join(dist, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"index.html": "<!doctype html><title>Knowledge</title>", "assets/app.js": "export const ready=true", ".env": "SECRET"} {
		if err := os.WriteFile(filepath.Join(dist, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	Register(router, dist)
	for _, tc := range []struct {
		method, path string
		status       int
		contains     string
	}{
		{"GET", "/", 200, "Knowledge"},
		{"GET", "/library/folder-id", 200, "Knowledge"},
		{"GET", "/assets/app.js", 200, "export const"},
		{"GET", "/assets/missing.js", 404, ""},
		{"GET", "/api/v1/missing", 404, "not_found"},
		{"GET", "/health", 200, "ok"},
		{"GET", "/.env", 404, ""},
		{"GET", "/../go.mod", 404, ""},
		{"POST", "/library", 404, "not_found"},
		{"HEAD", "/library", 200, ""},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
			if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), tc.contains) {
				t.Fatalf("got %d %q; want %d containing %q", recorder.Code, recorder.Body.String(), tc.status, tc.contains)
			}
			if strings.Contains(recorder.Body.String(), "SECRET") {
				t.Fatal("hidden file exposed")
			}
		})
	}
}

func TestMissingBuildIsRecoverable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dist := filepath.Join(t.TempDir(), "dist")
	router := gin.New()
	Register(router, dist)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "npm --prefix web run build") {
		t.Fatalf("missing build: %d %s", recorder.Code, recorder.Body.String())
	}
	if err := os.Mkdir(dist, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "ready" {
		t.Fatal("build did not become available")
	}
}
