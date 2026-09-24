package analytics

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClickHouseHTTPBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "writer" || password != "secret" {
			t.Error("authentication")
		}
		if r.URL.Query().Get("query") != "INSERT INTO analytics_events FORMAT JSONEachRow" || r.URL.Query().Get("database") != "analytics" {
			t.Error("query")
		}
		decoder := json.NewDecoder(r.Body)
		for range 2 {
			var e Event
			if err := decoder.Decode(&e); err != nil {
				t.Error(err)
			}
			if e.EventName != UserRegistered {
				t.Error(e.EventName)
			}
		}
	}))
	defer srv.Close()
	c, err := NewClickHouse(ClickHouseConfig{Address: srv.URL, Database: "analytics", User: "writer", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = c.Write(context.Background(), []Event{New(UserRegistered, uuid.New()), New(UserRegistered, uuid.New())}); err != nil {
		t.Fatal(err)
	}
}
func TestClickHouseErrorsAreSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret SQL payload", 500) }))
	defer srv.Close()
	c, err := NewClickHouse(ClickHouseConfig{Address: srv.URL, Database: "analytics"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err = c.Write(context.Background(), []Event{New(LoginFailed, uuid.Nil)}); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	if _, err = NewClickHouse(ClickHouseConfig{Address: "http://user:secret@localhost", Database: "bad; DROP"}); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
}
