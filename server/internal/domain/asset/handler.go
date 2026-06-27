package asset

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

func (h *Handler) UploadToken(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var input UploadTokenInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "asset.invalid_request", "请求参数不正确")
		return
	}
	result, err := h.service.CreateUploadToken(c.Request.Context(), user.UserID, input)
	if err != nil {
		h.writeError(c, err, "create asset upload token failed", user.UserID)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Confirm(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var input ConfirmInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "asset.invalid_request", "请求参数不正确")
		return
	}
	result, err := h.service.ConfirmUpload(c.Request.Context(), user.UserID, input)
	if err != nil {
		h.writeError(c, err, "confirm asset upload failed", user.UserID)
		return
	}
	response.OK(c, result)
}

func (h *Handler) writeError(c *gin.Context, err error, logMessage string, userID int64) {
	switch {
	case errors.Is(err, ErrUnsupportedAssetType):
		response.Error(c, http.StatusBadRequest, "asset.unsupported_asset_type", "不支持的资产类型")
	case errors.Is(err, ErrInvalidMimeType):
		response.Error(c, http.StatusBadRequest, "asset.invalid_mime_type", "仅支持图片格式")
	case errors.Is(err, ErrInvalidFileSize):
		response.Error(c, http.StatusBadRequest, "asset.invalid_file_size", "图片大小不符合要求")
	case errors.Is(err, ErrInvalidAssetRequest):
		response.Error(c, http.StatusBadRequest, "asset.invalid_request", "请求参数不正确")
	case errors.Is(err, ErrAssetStorageNotReady):
		h.logger.Error(logMessage, "error", err, "user_id", userID)
		response.Error(c, http.StatusInternalServerError, "asset.storage_not_configured", "资产存储未配置")
	case errors.Is(err, ErrAssetOwnership):
		response.Error(c, http.StatusForbidden, "asset.forbidden", "无权使用该资产")
	case errors.Is(err, ErrAssetNotFound):
		response.Error(c, http.StatusNotFound, "asset.not_found", "资产不存在")
	default:
		h.logger.Error(logMessage, "error", err, "user_id", userID)
		response.Error(c, http.StatusInternalServerError, "asset.request_failed", "资产请求失败")
	}
}
