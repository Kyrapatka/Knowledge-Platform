package materialimport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"net/http"
	"unicode/utf8"
)

type ImportService interface {
	Validate(context.Context, uuid.UUID, uuid.UUID, Request) (Result, error)
	Import(context.Context, uuid.UUID, uuid.UUID, Request) (Result, error)
}
type Handler struct{ service ImportService }

func NewHandler(s ImportService) *Handler { return &Handler{s} }
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.POST("/folders/:folderID/materials/import/validate", h.validate)
	api.POST("/folders/:folderID/materials/import", h.commit)
}
func (h *Handler) validate(c *gin.Context) { h.handle(c, false) }
func (h *Handler) commit(c *gin.Context)   { h.handle(c, true) }

// JSON decoding otherwise silently accepts duplicate keys and invalid UTF-8.
func strictObject(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON must be UTF-8")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return fmt.Errorf("JSON too deeply nested")
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return fmt.Errorf("duplicate JSON key")
				}
				seen[k] = true
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case '[':
			for d.More() {
				if err := walk(depth + 1); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter")
		}
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}
func (h *Handler) handle(c *gin.Context, commit bool) {
	user, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}
	folderID, err := uuid.Parse(c.Param("folderID"))
	if err != nil || folderID == uuid.Nil {
		c.JSON(400, gin.H{"error": "invalid_folder_id"})
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, MaxBody))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(413, gin.H{"error": "request_too_large"})
		} else {
			c.JSON(400, gin.H{"error": "invalid_request"})
		}
		return
	}
	if err = strictObject(data); err != nil {
		c.JSON(400, gin.H{"error": "invalid_json", "message": err.Error()})
		return
	}
	var req Request
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid_request", "message": err.Error()})
		return
	}
	var result Result
	if commit {
		result, err = h.service.Import(c.Request.Context(), user, folderID, req)
	} else {
		result, err = h.service.Validate(c.Request.Context(), user, folderID, req)
	}
	var invalid *ValidationError
	switch {
	case err == nil:
		c.JSON(200, result)
	case errors.As(err, &invalid):
		c.JSON(422, gin.H{"error": "invalid_material_import", "message": err.Error(), "result": invalid.Result})
	case errors.Is(err, ErrConflict):
		c.JSON(409, gin.H{"error": "import_command_conflict", "message": err.Error()})
	case errors.Is(err, ErrInvalid):
		c.JSON(400, gin.H{"error": "invalid_material_import", "message": err.Error()})
	case errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, folder.ErrNotFound) || errors.Is(err, material.ErrFolderNotFound):
		c.JSON(404, gin.H{"error": "folder_not_found"})
	default:
		c.Error(err)
		c.JSON(500, gin.H{"error": "internal_error"})
	}
}
