package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	workshop "github.com/Kyrapatka/knowledge-platform/internal/core/folder/workshop"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeWorkshopService struct {
	updateConfigResult foldermodel.Folder
	updateConfigErr    error

	lastOwnerID         uuid.UUID
	lastFolderID        uuid.UUID
	lastExpectedVersion int64
	lastConfig          folderconfig.FolderConfig
}

func (f *fakeWorkshopService) UpdateConfig(
	ctx context.Context,
	ownerID uuid.UUID,
	folderID uuid.UUID,
	expectedVersion int64,
	config folderconfig.FolderConfig,
) (foldermodel.Folder, error) {
	f.lastOwnerID = ownerID
	f.lastFolderID = folderID
	f.lastExpectedVersion = expectedVersion
	f.lastConfig = config

	return f.updateConfigResult, f.updateConfigErr
}

func TestHandler_UpdateConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := uuid.New()
	folderID := uuid.New()

	service := &fakeWorkshopService{
		updateConfigResult: foldermodel.Folder{
			ID:      folderID,
			OwnerID: ownerID,

			Config: folderconfig.FolderConfig{
				Schema: folderconfig.MaterialSchema{
					Fields: []folderconfig.FieldDefinition{
						{
							Key:      "foreign",
							Label:    "Word",
							Required: true,
							Active:   true,
						},
						{
							Key:      "native",
							Label:    "Native",
							Required: true,
							Active:   true,
						},
					},
				},

				MetadataSchema: folderconfig.MetadataSchema{
					Fields: []folderconfig.FieldDefinition{},
				},

				Card: folderconfig.CardConfig{
					QuestionFields: []string{
						"foreign",
					},

					AnswerFields: []string{
						"native",
					},
				},
			},

			ConfigVersion: 2,
		},
	}

	handler := NewHandler(service)

	router := gin.New()

	router.PATCH(
		"/folders/:folderID/workshop",
		func(c *gin.Context) {
			c.Set("user_id", ownerID)
			handler.UpdateConfig(c)
		},
	)

	requestBody := `{
		"expected_version": 1,
		"config": {
			"schema": {
				"fields": [
					{
						"key": "foreign",
						"label": "Word",
						"required": true,
						"active": true
					},
					{
						"key": "native",
						"label": "Native",
						"required": true,
						"active": true
					}
				]
			},
			"metadata_schema": {
				"fields": []
			},
			"card": {
				"question_fields": [
					"foreign"
				],
				"answer_fields": [
					"native"
				]
			}
		}
	}`

	request := httptest.NewRequest(
		http.MethodPatch,
		"/folders/"+folderID.String()+"/workshop",
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

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if service.lastOwnerID != ownerID {
		t.Fatalf(
			"owner ID = %s, want %s",
			service.lastOwnerID,
			ownerID,
		)
	}

	if service.lastFolderID != folderID {
		t.Fatalf(
			"folder ID = %s, want %s",
			service.lastFolderID,
			folderID,
		)
	}

	if service.lastExpectedVersion != 1 {
		t.Fatalf(
			"expected version = %d, want 1",
			service.lastExpectedVersion,
		)
	}

	if len(service.lastConfig.Schema.Fields) != 2 {
		t.Fatalf(
			"schema fields = %d, want 2",
			len(service.lastConfig.Schema.Fields),
		)
	}
}

func TestHandler_UpdateConfig_InvalidFolderID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := uuid.New()

	service := &fakeWorkshopService{}

	handler := NewHandler(service)

	router := gin.New()

	router.PATCH(
		"/folders/:folderID/workshop",
		func(c *gin.Context) {
			c.Set("user_id", ownerID)
			handler.UpdateConfig(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/folders/not-a-uuid/workshop",
		strings.NewReader(`{
			"expected_version": 1,
			"config": {}
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
}

func TestHandler_UpdateConfig_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := uuid.New()
	folderID := uuid.New()

	service := &fakeWorkshopService{}

	handler := NewHandler(service)

	router := gin.New()

	router.PATCH(
		"/folders/:folderID/workshop",
		func(c *gin.Context) {
			c.Set("user_id", ownerID)
			handler.UpdateConfig(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/folders/"+folderID.String()+"/workshop",
		strings.NewReader(`{
			"expected_version":
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
}

func TestHandler_UpdateConfig_FolderNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := uuid.New()
	folderID := uuid.New()

	service := &fakeWorkshopService{
		updateConfigErr: folder.ErrNotFound,
	}

	handler := NewHandler(service)

	router := gin.New()

	router.PATCH(
		"/folders/:folderID/workshop",
		func(c *gin.Context) {
			c.Set("user_id", ownerID)
			handler.UpdateConfig(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/folders/"+folderID.String()+"/workshop",
		strings.NewReader(validRequestBody()),
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

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNotFound,
		)
	}
}

func TestHandler_UpdateConfig_InvalidConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := uuid.New()
	folderID := uuid.New()

	service := &fakeWorkshopService{
		updateConfigErr: workshop.ErrInvalidConfig,
	}

	handler := NewHandler(service)

	router := gin.New()

	router.PATCH(
		"/folders/:folderID/workshop",
		func(c *gin.Context) {
			c.Set("user_id", ownerID)
			handler.UpdateConfig(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/folders/"+folderID.String()+"/workshop",
		strings.NewReader(validRequestBody()),
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusBadRequest,
		)
	}
}

func TestHandler_UpdateConfig_Conflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := uuid.New()
	folderID := uuid.New()

	service := &fakeWorkshopService{
		updateConfigErr: workshop.ErrConflict,
	}

	handler := NewHandler(service)

	router := gin.New()

	router.PATCH(
		"/folders/:folderID/workshop",
		func(c *gin.Context) {
			c.Set("user_id", ownerID)
			handler.UpdateConfig(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/folders/"+folderID.String()+"/workshop",
		strings.NewReader(validRequestBody()),
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

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusConflict,
		)
	}
}

func TestHandler_UpdateConfig_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ownerID := uuid.New()
	folderID := uuid.New()

	service := &fakeWorkshopService{
		updateConfigErr: errors.New(
			"service error",
		),
	}

	handler := NewHandler(service)

	router := gin.New()

	router.PATCH(
		"/folders/:folderID/workshop",
		func(c *gin.Context) {
			c.Set("user_id", ownerID)
			handler.UpdateConfig(c)
		},
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/folders/"+folderID.String()+"/workshop",
		strings.NewReader(validRequestBody()),
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

func TestHandler_UpdateConfig_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	folderID := uuid.New()

	service := &fakeWorkshopService{}

	handler := NewHandler(service)

	router := gin.New()

	router.PATCH(
		"/folders/:folderID/workshop",
		handler.UpdateConfig,
	)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/folders/"+folderID.String()+"/workshop",
		strings.NewReader(validRequestBody()),
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

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusUnauthorized,
		)
	}
}

func validRequestBody() string {
	return `{
		"expected_version": 1,
		"config": {
			"schema": {
				"fields": [
					{
						"key": "foreign",
						"label": "Foreign",
						"required": true,
						"active": true
					}
				]
			},
			"metadata_schema": {
				"fields": []
			},
			"card": {
				"question_fields": [
					"foreign"
				],
				"answer_fields": []
			}
		}
	}`
}
