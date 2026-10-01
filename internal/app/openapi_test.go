package app

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Kyrapatka/knowledge-platform/config"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"
)

// Compare the registered routes, not a second handwritten endpoint inventory.
func TestOpenAPICoversRegisteredRoutes(t *testing.T) {
	dsn := os.Getenv("TRAINING_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TRAINING_TEST_DATABASE_URL")
	}
	a, err := New(config.Config{DatabaseURL: dsn, HTTPAddress: "127.0.0.1:0", JWTSecret: "test-secret-with-at-least-32-characters", JWTIssuer: "test", AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour}, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	raw, err := os.ReadFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err = yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	actual := map[string]bool{}
	param := regexp.MustCompile(`/:([A-Za-z][A-Za-z0-9_]*)`)
	for _, route := range a.server.Handler.(*gin.Engine).Routes() {
		path := param.ReplaceAllString(route.Path, `/{${1}}`)
		method := strings.ToLower(route.Method)
		actual[method+" "+path] = true
		if _, ok := spec.Paths[path][method]; !ok {
			t.Errorf("undocumented route: %s %s", method, path)
		}
	}
	for path, item := range spec.Paths {
		for method := range item {
			if !actual[method+" "+path] {
				t.Errorf("documented route does not exist: %s %s", method, path)
			}
		}
	}
}
