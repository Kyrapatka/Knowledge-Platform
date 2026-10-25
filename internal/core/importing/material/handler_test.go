package materialimport

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/httpmiddleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeService struct {
	commits, validations int
	user                 uuid.UUID
	err                  error
}

func (s *fakeService) Validate(_ context.Context, u, f uuid.UUID, r Request) (Result, error) {
	s.validations++
	s.user = u
	return validate(english(), r, nil), s.err
}
func (s *fakeService) Import(_ context.Context, u, f uuid.UUID, r Request) (Result, error) {
	s.commits++
	s.user = u
	return Result{Valid: true, Created: len(r.Items)}, s.err
}
func routerFor(s *fakeService, user uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(httpmiddleware.OrdinaryBodyLimit())
	api := r.Group("/api/v1")
	if user != uuid.Nil {
		api.Use(func(c *gin.Context) { c.Set("user_id", user) })
	}
	NewHandler(s).RegisterRoutes(api)
	return r
}
func TestHandlerAuthStrictJSONAndLimits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		data   []byte
		auth   bool
		status int
	}{
		{"auth", []byte(`{}`), false, 401}, {"duplicate", []byte(`{"items":[],"items":[]}`), true, 400}, {"utf8", []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}, true, 400}, {"unknown", []byte(`{"unknown":1}`), true, 400}, {"trailing", []byte(`{} {}`), true, 400}, {"large", []byte(strings.Repeat(" ", MaxBody+1)), true, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeService{}
			u := uuid.Nil
			if tc.auth {
				u = uuid.New()
			}
			r := routerFor(s, u)
			w := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/api/v1/folders/"+uuid.NewString()+"/materials/import", bytes.NewReader(tc.data))
			request.ContentLength = -1
			r.ServeHTTP(w, request)
			if w.Code != tc.status || s.commits != 0 {
				t.Fatalf("status %d calls %d", w.Code, s.commits)
			}
		})
	}
}
func TestValidateIsReadOnlyAndLargeImportIsNotShadowed(t *testing.T) {
	s := &fakeService{}
	user := uuid.New()
	r := routerFor(s, user)
	body := Request{CommandID: uuid.New(), Items: []Item{item("a", "run")}}
	b, _ := json.Marshal(body)
	b = append([]byte(strings.Repeat(" ", 2<<20)), b...)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/folders/"+uuid.NewString()+"/materials/import/validate", bytes.NewReader(b)))
	if w.Code != 200 || s.commits != 0 || s.validations != 1 || s.user != user {
		t.Fatal(w.Code, s)
	}
}
func TestHandlerErrorContracts(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{ErrConflict, 409}, {&ValidationError{Result{Valid: false}}, 422}} {
		s := &fakeService{err: tc.err}
		r := routerFor(s, uuid.New())
		b, _ := json.Marshal(Request{CommandID: uuid.New(), Items: []Item{item("a", "run")}})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/folders/"+uuid.NewString()+"/materials/import", bytes.NewReader(b)))
		if w.Code != tc.status {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
