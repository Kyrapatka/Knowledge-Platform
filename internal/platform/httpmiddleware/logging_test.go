package httpmiddleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestRequestLogging(t *testing.T) {
	const secret = "SECRET_MUST_NOT_BE_LOGGED"
	for _, tc := range []struct {
		path, route, level string
		status             int
	}{
		{"/items/" + secret, "/items/:id", "INFO", 200},
		{"/unauthorized", "/unauthorized", "INFO", 401},
		{"/" + secret, "unmatched", "INFO", 404},
		{"/failure", "/failure", "ERROR", 503},
		{"/panic", "/panic", "ERROR", 500},
		{"/partial-panic", "/partial-panic", "ERROR", 200},
	} {
		t.Run(tc.route, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil)).With("service", "test")
			r := gin.New()
			r.Use(RequestID(logger), AccessLog(logger), Recovery())
			r.GET("/items/:id", func(c *gin.Context) {
				if ID(c.Request.Context()) != c.Writer.Header().Get(RequestIDHeader) {
					t.Error("context/header mismatch")
				}
				Logger(c.Request.Context(), nil).Info("handler event")
				c.String(200, "ok")
			})
			r.GET("/unauthorized", func(c *gin.Context) { c.AbortWithStatus(401) })
			r.GET("/failure", func(c *gin.Context) { _ = c.Error(errors.New(secret)); c.Status(503) })
			r.GET("/panic", func(c *gin.Context) { panic(secret) })
			r.GET("/partial-panic", func(c *gin.Context) { c.String(200, "partial"); panic(secret) })
			req := httptest.NewRequest("GET", tc.path+"?token="+secret, strings.NewReader(secret))
			for _, key := range []string{RequestIDHeader, "Authorization", "Cookie", "User-Agent"} {
				req.Header.Set(key, secret)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			id := w.Header().Get(RequestIDHeader)
			if _, err := uuid.Parse(id); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status {
				t.Fatalf("status=%d", w.Code)
			}
			if strings.Contains(output.String(), secret) {
				t.Fatal("secret leaked")
			}
			count := 0
			for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
				var record map[string]any
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}
				if record["request_id"] != id || record["service"] != "test" {
					t.Fatal(record)
				}
				if record["msg"] != "http request completed" {
					continue
				}
				count++
				if record["status"] != float64(tc.status) || record["route"] != tc.route || record["level"] != tc.level || record["method"] != "GET" {
					t.Fatal(record)
				}
				if record["duration_ms"].(float64) < 0 || record["response_bytes"].(float64) < 0 {
					t.Fatal(record)
				}
				if strings.Contains(tc.path, "panic") && (record["panic_recovered"] != true || record["stack"] == "") {
					t.Fatal(record)
				}
			}
			if count != 1 {
				t.Fatalf("completion records=%d", count)
			}
		})
	}
}

func TestConcurrentIDsAndLevelFiltering(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelError}))
	if ID(context.Background()) != "" || Logger(context.Background(), logger) != logger {
		t.Fatal("fallback")
	}
	r := gin.New()
	r.Use(RequestID(logger), AccessLog(logger), Recovery())
	r.GET("/ok", func(c *gin.Context) { c.Status(204) })
	ids := make(chan string, 40)
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/ok", nil)
			req.Header.Set(RequestIDHeader, "b17e62f8-9251-4e64-9153-2cd29c565d3c")
			r.ServeHTTP(w, req)
			ids <- w.Header().Get(RequestIDHeader)
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			t.Fatal(err)
		}
		if seen[id] || id == "b17e62f8-9251-4e64-9153-2cd29c565d3c" {
			t.Fatal("reused/client ID")
		}
		seen[id] = true
	}
	if output.Len() != 0 {
		t.Fatal("INFO bypassed ERROR threshold")
	}
}
