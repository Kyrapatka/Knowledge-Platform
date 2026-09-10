package dashboard

import (
	"errors"
	"net/http"
	"strconv"

	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Handler struct{ store *Store }

func NewHandler(db *gorm.DB) *Handler { return &Handler{store: NewStore(db)} }

// RegisterRoutes requires a group protected by AuthMiddleware. Every handler
// additionally checks its authenticated context before reading any data.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/library", h.Library)
	api.GET("/library/folders/:folderID/materials", h.FolderMaterials)
	api.GET("/statistics", h.Statistics)
}

func userID(c *gin.Context) (uuid.UUID, bool) {
	user, ok := auth.UserIDFromContext(c)
	if !ok || user == uuid.Nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, false
	}
	return user, true
}

func respond(c *gin.Context, result any, err error) {
	switch {
	case err == nil:
		c.JSON(http.StatusOK, result)
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "library_not_found"})
	case errors.Is(err, ErrInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_library_query"})
	default:
		_ = c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
	}
}

func (h *Handler) Library(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	out, err := h.store.Library(c.Request.Context(), u)
	respond(c, out, err)
}

func (h *Handler) FolderMaterials(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	folder, err := uuid.Parse(c.Param("folderID"))
	if err != nil || folder == uuid.Nil {
		respond(c, nil, ErrInvalid)
		return
	}
	limit, e1 := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, e2 := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if e1 != nil || e2 != nil {
		respond(c, nil, ErrInvalid)
		return
	}
	query := MaterialsQuery{Topic: c.Query("topic"), Search: c.Query("q"), Sort: c.DefaultQuery("sort", "created_at"),
		Direction: c.DefaultQuery("direction", "desc"), Limit: limit, Offset: offset}
	if raw := c.Query("plan_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			respond(c, nil, ErrInvalid)
			return
		}
		query.PlanID = &id
	}
	out, err := h.store.FolderMaterials(c.Request.Context(), u, folder, query)
	respond(c, out, err)
}

func (h *Handler) Statistics(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	days, err := strconv.Atoi(c.DefaultQuery("days", "30"))
	if err != nil {
		respond(c, nil, ErrInvalid)
		return
	}
	out, err := h.store.Statistics(c.Request.Context(), u, days, c.DefaultQuery("timezone", "UTC"))
	respond(c, out, err)
}
