package profile

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
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Summary(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	summary, err := h.service.Summary(c.Request.Context(), user.UserID)
	if err != nil {
		h.logger.Error("get profile summary failed", "error", err, "user_id", user.UserID)
		response.Error(c, http.StatusInternalServerError, "profile.summary_failed", "读取档案摘要失败")
		return
	}
	response.OK(c, summary)
}
