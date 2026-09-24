package app

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerProbes shares the App-owned state with the HTTP layer. Probes perform
// no network or database work and add no logs beyond ordinary access logging.
func (a *App) registerProbes(router *gin.Engine) {
	router.GET("/health", live)
	router.GET("/live", live)
	router.GET("/ready", a.ready)
}

func live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (a *App) ready(c *gin.Context) {
	if !a.readiness.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
		return
	}
	// Preserve the existing readiness response contract.
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
