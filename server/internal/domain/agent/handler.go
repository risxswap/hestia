package agent

import (
	"net/http"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Stream(c *gin.Context) {
	response.StreamHeaders(c)
	c.Status(http.StatusOK)
	user, _ := auth.UserFromContext(c)
	_ = response.WriteSSE(c.Writer, "status", h.service.StreamStatus(c.Request.Context(), user.UserID))
	_ = response.WriteSSE(c.Writer, "done", StreamDone{})
}
