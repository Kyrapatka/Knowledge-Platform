// Package webui serves the locally built application from the API's origin.
package webui

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// Register installs a SPA fallback after the application's API routes. Assets
// are read at request time, so an initially missing build can be added without
// restarting the API. distDirectory is typically "web/dist" from the repo root.
func Register(router *gin.Engine, distDirectory string) {
	router.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead ||
			requestPath == "/api" || strings.HasPrefix(requestPath, "/api/") ||
			requestPath == "/health" || strings.HasPrefix(requestPath, "/health/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")

		name := strings.TrimPrefix(requestPath, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) || strings.Contains(name, "\\") {
			c.Status(http.StatusNotFound)
			return
		}
		for _, segment := range strings.Split(name, "/") {
			if strings.HasPrefix(segment, ".") {
				c.Status(http.StatusNotFound)
				return
			}
		}
		root, err := os.OpenRoot(distDirectory)
		if err != nil {
			c.Header("Cache-Control", "no-store")
			c.String(http.StatusServiceUnavailable, "The frontend has not been built yet. From the repository root, run: npm --prefix web install && npm --prefix web run build. Then reload this page.")
			return
		}
		defer root.Close()
		file, err := root.Open(name)
		if err != nil && path.Ext(name) == "" && !strings.HasPrefix(name, "assets/") {
			name = "index.html"
			file, err = root.Open(name)
		}
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			c.Status(http.StatusNotFound)
			return
		}
		if name == "index.html" {
			c.Header("Cache-Control", "no-cache")
		} else if strings.HasPrefix(name, "assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-cache")
		}
		// Gin enters NoRoute with status 404; ServeContent must start with 200.
		c.Status(http.StatusOK)
		http.ServeContent(c.Writer, c.Request, name, info.ModTime(), file)
	})
}
