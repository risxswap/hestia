package collection

import (
	"log/slog"
	"net/http"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(service *Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Summary(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok || user.UserID == 0 {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	summary, err := h.service.Summary(c.Request.Context(), user.UserID)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("collection summary failed", "error", err)
		}
		response.Error(c, http.StatusInternalServerError, "collection.request_failed", "读取私藏失败")
		return
	}
	response.OK(c, summary)
}
