package interview

import (
	"errors"
	auth "github.com/Kyrapatka/knowledge-platform/internal/auth/handler"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
)

type Handler struct {
	analytics.Emitter
	store *Store
}

func NewHandler(db *gorm.DB) *Handler { return &Handler{store: NewStore(db)} }
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	api.GET("/interview/seed", h.seedInfo)
	api.POST("/interview/seed/import", h.importSeed)
	api.GET("/interview/concepts", h.concepts)
	api.PUT("/interview/concepts/:slug", h.saveConcept)
	api.GET("/folders/:folderID/interview/questions/:materialID/profile", h.profile)
	api.PUT("/folders/:folderID/interview/questions/:materialID/profile", h.saveProfile)
	api.POST("/folders/:folderID/interview/questions:bulk", h.bulk)
}
func userID(c *gin.Context) (uuid.UUID, bool) {
	u, ok := auth.UserIDFromContext(c)
	if !ok {
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
	}
	return u, ok
}
func paramID(c *gin.Context, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(key))
	if err != nil || id == uuid.Nil {
		c.AbortWithStatusJSON(400, gin.H{"error": "invalid_id"})
		return id, false
	}
	return id, true
}
func write(c *gin.Context, v any, err error) {
	if err == nil {
		c.JSON(200, v)
		return
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(404, gin.H{"error": "interview_not_found"})
	case errors.Is(err, ErrConflict):
		c.JSON(409, gin.H{"error": "interview_conflict", "message": err.Error()})
	case errors.Is(err, ErrInvalid):
		c.JSON(400, gin.H{"error": "invalid_interview_request", "message": err.Error()})
	default:
		c.Error(err)
		c.JSON(500, gin.H{"error": "internal_error"})
	}
}
func read(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)
	if err := c.ShouldBindJSON(v); err != nil {
		c.JSON(400, gin.H{"error": "invalid_request"})
		return false
	}
	return true
}
func (h *Handler) seedInfo(c *gin.Context) {
	if _, ok := userID(c); !ok {
		return
	}
	b, e := ReadSeed()
	write(c, gin.H{"version": b.Version, "revision": b.Version, "domains": b.Domains, "question_count": len(b.Questions), "profiles": InterviewProfiles, "validation": b.Source.Report}, e)
}
func (h *Handler) importSeed(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	var req struct {
		Domains []string `json:"domains"`
	}
	if !read(c, &req) {
		return
	}
	v, e := h.store.ImportSeed(c.Request.Context(), u, req.Domains)
	if e == nil {
		h.publishCreatedMaterials(c, u, v)
	}
	if e == nil && (v.Created > 0 || v.Updated > 0) {
		for _, id := range v.FolderIDs {
			event := analytics.New(analytics.FolderImported, u)
			event.FolderID, event.Template = id.String(), "interview_questions"
			h.Publish(c.Request.Context(), event)
		}
	}
	write(c, v, e)
}
func (h *Handler) concepts(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	v, e := h.store.Catalog(c.Request.Context(), u)
	write(c, v, e)
}
func (h *Handler) saveConcept(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	var req Concept
	if !read(c, &req) {
		return
	}
	req.Slug = c.Param("slug")
	v, e := h.store.SaveConcept(c.Request.Context(), u, req)
	write(c, v, e)
}
func (h *Handler) profile(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	f, ok := paramID(c, "folderID")
	if !ok {
		return
	}
	m, ok := paramID(c, "materialID")
	if !ok {
		return
	}
	v, e := h.store.Profile(c.Request.Context(), u, f, m)
	write(c, v, e)
}
func (h *Handler) saveProfile(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	f, ok := paramID(c, "folderID")
	if !ok {
		return
	}
	m, ok := paramID(c, "materialID")
	if !ok {
		return
	}
	var req ProfileRequest
	if !read(c, &req) {
		return
	}
	v, e := h.store.SaveProfile(c.Request.Context(), u, f, m, req)
	write(c, v, e)
}
func (h *Handler) bulk(c *gin.Context) {
	u, ok := userID(c)
	if !ok {
		return
	}
	f, ok := paramID(c, "folderID")
	if !ok {
		return
	}
	var req struct {
		Questions []BulkQuestion `json:"questions"`
	}
	if !read(c, &req) {
		return
	}
	v, e := h.store.Bulk(c.Request.Context(), u, f, req.Questions)
	if e == nil {
		h.publishCreatedMaterials(c, u, v)
	}
	write(c, v, e)
}

func (h *Handler) publishCreatedMaterials(c *gin.Context, user uuid.UUID, result ImportResult) {
	for _, m := range result.CreatedMaterials {
		e := analytics.New(analytics.MaterialCreated, user)
		e.MaterialID, e.FolderID, e.Template = m.ID.String(), m.FolderID.String(), "interview_questions"
		e.Difficulty = analytics.Ptr("medium")
		h.Publish(c.Request.Context(), e)
	}
}
