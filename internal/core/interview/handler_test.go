package interview

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestReadRejectsMalformedImports(t *testing.T) {
	for _, body := range []string{`{"domains":[],"unknown":true}`, `{"domains":[]} {}`, "{\"domains\":[\"\xff\"]}", strings.Repeat(" ", 2*1024*1024) + `{"domains":[]}`} {
		r := gin.New()
		r.POST("/", func(c *gin.Context) {
			var value struct {
				Domains []string `json:"domains"`
			}
			if read(c, &value) {
				c.Status(204)
			}
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("invalid import accepted: %d", w.Code)
		}
	}
}
