package httpmiddleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestDiagnosticPriorityAndNoDuplicateRecords(t *testing.T) {
	for _, tt := range []struct {
		path, reason, level string
		status              int
		err                 error
	}{
		{"/api/missing", "unmatched", "WARN", 404, nil},
		{"/api/known", "api_not_found", "WARN", 404, nil},
		{"/slow", "slow", "WARN", 200, nil},
		{"/failure", "server_error", "ERROR", 503, nil},
		{"/canceled", "context_canceled", "INFO", 500, context.Canceled},
		{"/deadline", "deadline_exceeded", "WARN", 500, context.DeadlineExceeded},
	} {
		t.Run(tt.reason, func(t *testing.T) {
			var buf bytes.Buffer
			l := slog.New(slog.NewJSONHandler(&buf, nil))
			r := gin.New()
			r.Use(RequestID(l), AccessLog(l, time.Nanosecond), Recovery())
			if tt.reason != "unmatched" {
				r.GET(tt.path, func(c *gin.Context) {
					if tt.reason == "slow" {
						time.Sleep(2 * time.Millisecond)
					}
					if tt.err != nil {
						_ = c.Error(tt.err)
					}
					c.Status(tt.status)
				})
			}
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", tt.path+"?token=secret", nil))
			if strings.Count(strings.TrimSpace(buf.String()), "\n") != 0 || strings.Contains(buf.String(), "secret") {
				t.Fatal(buf.String())
			}
			var row map[string]any
			if err := json.Unmarshal(buf.Bytes(), &row); err != nil {
				t.Fatal(err)
			}
			if row["reason"] != tt.reason || row["level"] != tt.level {
				t.Fatal(row)
			}
			if tt.reason == "unmatched" && row["path"] != tt.path {
				t.Fatal(row)
			}
		})
	}
}
