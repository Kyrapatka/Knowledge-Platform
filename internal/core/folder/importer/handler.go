package importer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	authhandler "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ImportService interface {
	Validate(data []byte) Preview
	Import(ctx context.Context, ownerID uuid.UUID, data []byte) (Result, error)
}

type Handler struct {
	service ImportService
}

func NewHandler(service ImportService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Validate(c *gin.Context) {
	if _, ok := authenticatedUser(c); !ok {
		return
	}
	data, ok := readUpload(c)
	if !ok {
		return
	}
	preview := h.service.Validate(data)
	if !preview.Valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_folder_import", "message": firstError(preview), "validation": preview, "errors": preview.Errors})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (h *Handler) Import(c *gin.Context) {
	ownerID, ok := authenticatedUser(c)
	if !ok {
		return
	}
	data, ok := readUpload(c)
	if !ok {
		return
	}
	result, err := h.service.Import(c.Request.Context(), ownerID, data)
	if err != nil {
		var invalid *InvalidImportError
		if errors.As(err, &invalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_folder_import", "message": firstError(invalid.Preview), "validation": invalid.Preview, "errors": invalid.Preview.Errors})
			return
		}
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "folder_import_failed", "message": "The folder could not be imported. No data was saved."})
		return
	}
	c.JSON(http.StatusCreated, result)
}

func authenticatedUser(c *gin.Context) (uuid.UUID, bool) {
	ownerID, ok := authhandler.UserIDFromContext(c)
	if !ok || ownerID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, false
	}
	return ownerID, true
}

func readUpload(c *gin.Context) ([]byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxFileSize+64*1024)
	if err := c.Request.ParseMultipartForm(64 * 1024); err != nil {
		status := http.StatusBadRequest
		message := "Choose a JSON file to import."
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			status = http.StatusRequestEntityTooLarge
			message = fmt.Sprintf("The file is larger than %d MB.", MaxFileSize/(1024*1024))
		}
		c.JSON(status, uploadError(message))
		return nil, false
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, uploadError("Choose a JSON file to import."))
		return nil, false
	}
	defer file.Close()
	if strings.ToLower(filepath.Ext(header.Filename)) != ".json" {
		c.JSON(http.StatusBadRequest, uploadError("Only .json files are supported."))
		return nil, false
	}
	data, err := readLimited(file, header)
	if err != nil {
		if errors.Is(err, errFileTooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, uploadError(fmt.Sprintf("The file is larger than %d MB.", MaxFileSize/(1024*1024))))
		} else {
			c.JSON(http.StatusBadRequest, uploadError("The file could not be read."))
		}
		return nil, false
	}
	return data, true
}

var errFileTooLarge = errors.New("file too large")

func readLimited(file multipart.File, header *multipart.FileHeader) ([]byte, error) {
	if header.Size > MaxFileSize {
		return nil, errFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileSize {
		return nil, errFileTooLarge
	}
	return data, nil
}

func uploadError(message string) gin.H {
	return gin.H{"error": "invalid_folder_import", "message": message, "errors": []FieldError{{Field: "file", Message: message}}}
}

func firstError(preview Preview) string {
	if len(preview.Errors) == 0 {
		return "The file cannot be imported."
	}
	error := preview.Errors[0]
	if error.Item > 0 {
		return fmt.Sprintf("Item %d: %s", error.Item, error.Message)
	}
	return error.Message
}
