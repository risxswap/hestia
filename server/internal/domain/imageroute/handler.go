package imageroute

import (
	"errors"
	"log/slog"
	"net/http"

	"hestia/server/internal/common/auth"
	businesslock "hestia/server/internal/common/lock"
	"hestia/server/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *FeedbackService
	logger  *slog.Logger
}

func NewHandler(service *FeedbackService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

func (h *Handler) ApplyFeedback(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var input FeedbackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "image_route.invalid_request", "请求参数不正确")
		return
	}
	result, err := h.service.ApplyFeedback(c.Request.Context(), user.UserID, c.Param("public_id"), input)
	if err != nil {
		if errors.Is(err, businesslock.ErrBusy) {
			response.Error(c, http.StatusConflict, "image_route.feedback_busy", "路线反馈正在处理中，请稍后再试")
			return
		}
		if errors.Is(err, ErrInvalidFeedbackAction) {
			response.Error(c, http.StatusBadRequest, "image_route.invalid_feedback_action", "不支持的路线反馈动作")
			return
		}
		if errors.Is(err, ErrInvalidFeedbackReason) {
			response.Error(c, http.StatusBadRequest, "image_route.invalid_feedback_reason", "反馈原因过长")
			return
		}
		if errors.Is(err, ErrRouteNotFound) {
			response.Error(c, http.StatusNotFound, "image_route.not_found", "路线不存在")
			return
		}
		h.logger.Error("apply image route feedback failed", "error", err, "user_id", user.UserID, "route_public_id", c.Param("public_id"))
		response.Error(c, http.StatusInternalServerError, "image_route.feedback_failed", "路线反馈失败")
		return
	}
	response.OK(c, result)
}
