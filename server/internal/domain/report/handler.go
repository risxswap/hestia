package report

import (
	"errors"
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

func (h *Handler) Latest(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	result, err := h.service.LatestForUser(c.Request.Context(), user.UserID)
	if err != nil {
		if errors.Is(err, ErrReportNotFound) {
			response.Error(c, http.StatusNotFound, "report.not_found", "报告不存在")
			return
		}
		h.logger.Error("get latest report failed", "error", err, "user_id", user.UserID)
		response.Error(c, http.StatusInternalServerError, "report.get_failed", "读取报告失败")
		return
	}
	response.OK(c, result)
}

func (h *Handler) Get(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	result, err := h.service.GetForUser(c.Request.Context(), user.UserID, c.Param("public_id"))
	if err != nil {
		if errors.Is(err, ErrReportNotFound) {
			response.Error(c, http.StatusNotFound, "report.not_found", "报告不存在")
			return
		}
		h.logger.Error("get report failed", "error", err, "user_id", user.UserID)
		response.Error(c, http.StatusInternalServerError, "report.get_failed", "读取报告失败")
		return
	}
	response.OK(c, result)
}
