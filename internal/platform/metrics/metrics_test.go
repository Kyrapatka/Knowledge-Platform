package metrics

import (
	"github.com/gin-gonic/gin"
	dto "github.com/prometheus/client_model/go"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPMetrics(t *testing.T) {
	m := New()
	r := gin.New()
	r.Use(m.Middleware(), gin.Recovery())
	r.GET("/items/:id", func(c *gin.Context) {
		v := &dto.Metric{}
		_ = m.InFlight.Write(v)
		if v.GetGauge().GetValue() != 1 {
			t.Error("not in flight")
		}
		c.Status(204)
	})
	r.GET("/panic", func(c *gin.Context) { panic("test") })
	r.GET("/metrics", gin.WrapH(m.Handler()))
	for _, path := range []string{"/items/private-id", "/items/another-id", "/missing-secret", "/panic"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	v := &dto.Metric{}
	_ = m.InFlight.Write(v)
	if v.GetGauge().GetValue() != 0 {
		t.Fatal("gauge leaked")
	}
	_ = m.Requests.WithLabelValues("GET", "/items/:id", "204").Write(v)
	if v.GetCounter().GetValue() != 2 {
		t.Fatal("counter")
	}
	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var count uint64
	for _, f := range families {
		if f.GetName() == "http_request_duration_seconds" {
			for _, sample := range f.Metric {
				count += sample.Histogram.GetSampleCount()
			}
		}
	}
	if count != 4 {
		t.Fatalf("observations=%d", count)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `route="/items/:id"`) || strings.Contains(w.Body.String(), "private-id") || strings.Contains(w.Body.String(), "missing-secret") {
		t.Fatal("invalid exposition/cardinality")
	}
	_ = m.Requests.WithLabelValues("GET", "/panic", "500").Write(v)
	if v.GetCounter().GetValue() != 1 {
		t.Fatal("panic not counted")
	}
	// Independent Apps may construct registries repeatedly without global collisions.
	_ = New()
}
