package agent

import (
	"net/http"

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
	_ = response.WriteSSE(c.Writer, "status", gin.H{"text": "agent stream ready"})
	_ = response.WriteSSE(c.Writer, "done", StreamDone{})
}
