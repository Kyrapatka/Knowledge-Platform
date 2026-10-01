package httpmiddleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// OrdinaryBodyLimit fills the uncovered auth/library routes. Training, interview
// and multipart imports retain their existing, separately tested handler limits.
func OrdinaryBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		route := c.FullPath()
		if !strings.HasPrefix(route, "/api/v1/") ||
			strings.HasPrefix(route, "/api/v1/training/") || strings.Contains(route, "/training-config") ||
			strings.Contains(route, "/exercises") || strings.Contains(route, "/interview/") ||
			route == "/api/v1/folders/import" || route == "/api/v1/folders/import/validate" {
			c.Next()
			return
		}
		limit := int64(1 << 20)
		if strings.HasPrefix(route, "/api/v1/auth/") {
			limit = 64 << 10
		}
		if c.Request.ContentLength > limit {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request_too_large"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
