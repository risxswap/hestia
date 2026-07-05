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

func (h *Handler) CreateProfilePhoto(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var request CreateProfilePhotoRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "profile.validation_failed", "请求参数不正确")
		return
	}
	item, err := h.service.CreateProfilePhoto(c.Request.Context(), user.UserID, request)
	if err != nil {
		h.writePhotoError(c, err, "create profile photo failed", user.UserID)
		return
	}
	response.OK(c, item)
}

func (h *Handler) UpdateProfilePhoto(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var request UpdateProfilePhotoRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "profile.validation_failed", "请求参数不正确")
		return
	}
	item, err := h.service.UpdateProfilePhoto(c.Request.Context(), user.UserID, c.Param("public_id"), request)
	if err != nil {
		h.writePhotoError(c, err, "update profile photo failed", user.UserID)
		return
	}
	response.OK(c, item)
}

func (h *Handler) DeleteProfilePhoto(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	publicID := c.Param("public_id")
	if err := h.service.DeleteProfilePhoto(c.Request.Context(), user.UserID, publicID); err != nil {
		h.writePhotoError(c, err, "delete profile photo failed", user.UserID)
		return
	}
	response.OK(c, gin.H{"public_id": publicID})
}

func (h *Handler) writePhotoError(c *gin.Context, err error, logMessage string, userID int64) {
	switch {
	case errors.Is(err, ErrValidation):
		response.Error(c, http.StatusBadRequest, "profile.validation_failed", "请求参数不正确")
	case errors.Is(err, ErrUserNotFound):
		response.Error(c, http.StatusNotFound, "profile.user_not_found", "用户不存在")
	case errors.Is(err, ErrProfilePhotoNotFound):
		response.Error(c, http.StatusNotFound, "profile.photo_not_found", "档案照片不存在")
	case errors.Is(err, ErrProfileAssetNotFound):
		response.Error(c, http.StatusBadRequest, "profile.photo_asset_invalid", "照片资产不可用于档案")
	case errors.Is(err, ErrProfilePhotoLimit):
		response.Error(c, http.StatusConflict, "profile.photo_limit_reached", "档案照片数量已达上限")
	default:
		h.logger.Error(logMessage, "error", err, "user_id", userID)
		response.Error(c, http.StatusInternalServerError, "profile.photo_request_failed", "档案照片请求失败")
	}
}
