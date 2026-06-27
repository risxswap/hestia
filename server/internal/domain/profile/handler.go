package profile

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

func (h *Handler) UpdateProfile(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var request UpdateProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "profile.validation_failed", "请求参数不正确")
		return
	}
	summary, err := h.service.UpdateProfile(c.Request.Context(), user.UserID, request)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			response.Error(c, http.StatusBadRequest, "profile.validation_failed", "请求参数不正确")
			return
		}
		if errors.Is(err, ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "profile.user_not_found", "用户不存在")
			return
		}
		h.logger.Error("update profile failed", "error", err, "user_id", user.UserID)
		response.Error(c, http.StatusInternalServerError, "profile.update_failed", "保存档案失败")
		return
	}
	response.OK(c, summary)
}

func (h *Handler) UpdatePreferences(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var request UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "profile.validation_failed", "请求参数不正确")
		return
	}
	summary, err := h.service.UpdatePreferences(c.Request.Context(), user.UserID, request)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			response.Error(c, http.StatusBadRequest, "profile.validation_failed", "请求参数不正确")
			return
		}
		if errors.Is(err, ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "profile.user_not_found", "用户不存在")
			return
		}
		h.logger.Error("update profile preferences failed", "error", err, "user_id", user.UserID)
		response.Error(c, http.StatusInternalServerError, "profile.update_preferences_failed", "保存偏好失败")
		return
	}
	response.OK(c, summary)
}
