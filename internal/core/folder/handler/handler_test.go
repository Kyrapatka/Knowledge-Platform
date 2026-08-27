package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	"github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeFolderService struct {
	createResult model.Folder
	createErr    error

	getResult model.Folder
	getErr    error

	listResult []model.Folder
	listErr    error

	updateResult model.Folder
	updateErr    error

	deleteErr error
}

func (f *fakeFolderService) Create(
	ctx context.Context,
	ownerID uuid.UUID,
	title string,
	description string,
	templateKey string,
) (model.Folder, error) {
	return f.createResult, f.createErr
}

func (f *fakeFolderService) GetByID(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
) (model.Folder, error) {
	return f.getResult, f.getErr
}

func (f *fakeFolderService) List(
	ctx context.Context,
	ownerID uuid.UUID,
) ([]model.Folder, error) {
	return f.listResult, f.listErr
}

func (f *fakeFolderService) Update(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	title string,
	description string,
) (model.Folder, error) {
	return f.updateResult, f.updateErr
}

func (f *fakeFolderService) Delete(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
) error {
	return f.deleteErr
}

func TestHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	folderID := uuid.New()

	service := &fakeFolderService{
		createResult: model.Folder{
			ID:          folderID,
			OwnerID:     userID,
			Title:       "English B2",
			Description: "My words",
			TemplateKey: "english_words",
		},
	}

	handler := NewHandler(service)

	router := gin.New()

	router.POST(
		"/folders",
		func(c *gin.Context) {
			c.Set("user_id", userID)
			handler.Create(c)
		},
	)

	requestBody := `{
		"title":"English B2",
		"description":"My words",
		"template_key":"english_words"
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(requestBody),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusCreated,
		)
	}
}

func TestHandler_GetByID_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	folderID := uuid.New()

	service := &fakeFolderService{
		getErr: folder.ErrNotFound,
	}

	handler := NewHandler(service)

	router := gin.New()

	router.GET(
		"/folders/:folderID",
		func(c *gin.Context) {
			c.Set("user_id", userID)
			handler.GetByID(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/folders/"+folderID.String(),
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNotFound,
		)
	}
}

func TestHandler_GetByID_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()

	service := &fakeFolderService{}

	handler := NewHandler(service)

	router := gin.New()

	router.GET(
		"/folders/:folderID",
		func(c *gin.Context) {
			c.Set("user_id", userID)
			handler.GetByID(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/folders/not-a-uuid",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
}

func TestHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()

	service := &fakeFolderService{
		listResult: []model.Folder{
			{
				ID:      uuid.New(),
				OwnerID: userID,
				Title:   "English",
			},
			{
				ID:      uuid.New(),
				OwnerID: userID,
				Title:   "Go",
			},
		},
	}

	handler := NewHandler(service)

	router := gin.New()

	router.GET(
		"/folders",
		func(c *gin.Context) {
			c.Set("user_id", userID)
			handler.List(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/folders",
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}
}

func TestHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	folderID := uuid.New()

	service := &fakeFolderService{}

	handler := NewHandler(service)

	router := gin.New()

	router.DELETE(
		"/folders/:folderID",
		func(c *gin.Context) {
			c.Set("user_id", userID)
			handler.Delete(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/folders/"+folderID.String(),
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}
}

func TestHandler_Delete_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	folderID := uuid.New()

	service := &fakeFolderService{
		deleteErr: folder.ErrNotFound,
	}

	handler := NewHandler(service)

	router := gin.New()

	router.DELETE(
		"/folders/:folderID",
		func(c *gin.Context) {
			c.Set("user_id", userID)
			handler.Delete(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/folders/"+folderID.String(),
		nil,
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNotFound,
		)
	}
}

func TestHandler_Create_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()

	service := &fakeFolderService{
		createErr: errors.New("service error"),
	}

	handler := NewHandler(service)

	router := gin.New()

	router.POST(
		"/folders",
		func(c *gin.Context) {
			c.Set("user_id", userID)
			handler.Create(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/folders",
		strings.NewReader(`{
			"title":"English",
			"template_key":"english_words"
		}`),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	router.ServeHTTP(
		recorder,
		request,
	)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusInternalServerError,
		)
	}
}
