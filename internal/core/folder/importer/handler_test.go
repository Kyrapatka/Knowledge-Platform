package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeImportService struct {
	importResult Result
	importErr    error
	importCalled bool
}

func (s *fakeImportService) Validate(data []byte) Preview {
	_, preview := Parse(data)
	return preview
}

func (s *fakeImportService) Import(_ context.Context, _ uuid.UUID, data []byte) (Result, error) {
	s.importCalled = true
	if s.importErr != nil {
		return Result{}, s.importErr
	}
	if _, preview := Parse(data); !preview.Valid {
		return Result{}, &InvalidImportError{Preview: preview}
	}
	return s.importResult, nil
}

func TestValidateHandlerReturnsPreviewAndClearItemErrors(t *testing.T) {
	service := &fakeImportService{}
	router := importRouter(service)

	valid := envelopeJSON(TemplateEnglishWords, "English", `{"word":"maintain","translation":"поддерживать"}`)
	response := performUpload(t, router, "/folders/import/validate", "english.json", valid)
	if response.Code != http.StatusOK {
		t.Fatalf("valid status = %d, body = %s", response.Code, response.Body.String())
	}
	var preview Preview
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Valid || preview.ItemsCount != 1 {
		t.Fatalf("unexpected preview: %+v", preview)
	}

	invalid := envelopeJSON(TemplateInterviewQuestions, "Interview", `{"question":"Question?"}`)
	response = performUpload(t, router, "/folders/import/validate", "interview.json", invalid)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Errors []FieldError `json:"errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Item != 1 || body.Errors[0].Field != "short_answer" {
		t.Fatalf("unclear validation response: %+v", body.Errors)
	}
}

func TestImportHandlerUsesAuthenticatedOwner(t *testing.T) {
	folderID := uuid.New()
	service := &fakeImportService{importResult: Result{FolderID: folderID, FolderName: "English", Template: TemplateEnglishWords, ItemsCreated: 1, Warnings: []string{}}}
	router := importRouter(service)
	response := performUpload(t, router, "/folders/import", "english.json", envelopeJSON(TemplateEnglishWords, "English", `{"word":"word","translation":"слово"}`))
	if response.Code != http.StatusCreated || !service.importCalled {
		t.Fatalf("status=%d called=%v body=%s", response.Code, service.importCalled, response.Body.String())
	}
}

func TestUploadLimitsAndExtension(t *testing.T) {
	service := &fakeImportService{}
	router := importRouter(service)
	response := performUpload(t, router, "/folders/import/validate", "folder.txt", []byte(`{}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("extension status = %d", response.Code)
	}

	response = performUpload(t, router, "/folders/import/validate", "folder.json", make([]byte, MaxFileSize+1))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large status = %d, body=%s", response.Code, response.Body.String())
	}
}

func importRouter(service ImportService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(service)
	router := gin.New()
	user := uuid.New()
	router.POST("/folders/import/validate", func(c *gin.Context) {
		c.Set("user_id", user)
		handler.Validate(c)
	})
	router.POST("/folders/import", func(c *gin.Context) {
		c.Set("user_id", user)
		handler.Import(c)
	})
	return router
}

func performUpload(t *testing.T, router http.Handler, path, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
