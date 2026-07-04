package wardrobe

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

func (h *Handler) ListItems(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	filter, ok := wardrobeListFilter(c)
	if !ok {
		response.Error(c, http.StatusBadRequest, "wardrobe.invalid_request", "请求参数不正确")
		return
	}
	items, err := h.service.ListItems(c.Request.Context(), user.UserID, filter)
	if err != nil {
		h.writeError(c, err, "list wardrobe items failed", user.UserID, "")
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *Handler) GetOptions(c *gin.Context) {
	if _, ok := auth.UserFromContext(c); !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	options, err := h.service.ListWardrobeOptions(c.Request.Context())
	if err != nil {
		h.writeError(c, err, "list wardrobe options failed", 0, "")
		return
	}
	response.OK(c, options)
}

func (h *Handler) CreateItem(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "wardrobe.invalid_request", "请求参数不正确")
		return
	}
	item, err := h.service.CreateItem(c.Request.Context(), user.UserID, input)
	if err != nil {
		h.writeError(c, err, "create wardrobe item failed", user.UserID, "")
		return
	}
	response.OK(c, item)
}

func (h *Handler) UpdateItem(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var input UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "wardrobe.invalid_request", "请求参数不正确")
		return
	}
	publicID := c.Param("public_id")
	item, err := h.service.UpdateItem(c.Request.Context(), user.UserID, publicID, input)
	if err != nil {
		h.writeError(c, err, "update wardrobe item failed", user.UserID, publicID)
		return
	}
	response.OK(c, item)
}

func (h *Handler) DeleteItem(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	publicID := c.Param("public_id")
	if err := h.service.SoftDeleteItem(c.Request.Context(), user.UserID, publicID); err != nil {
		h.writeError(c, err, "delete wardrobe item failed", user.UserID, publicID)
		return
	}
	response.OK(c, gin.H{"public_id": strings.TrimSpace(publicID)})
}

func (h *Handler) RecognizeItemImage(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var input RecognizeImageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "wardrobe.invalid_request", "请求参数不正确")
		return
	}
	result, err := h.service.RecognizeItemImage(c.Request.Context(), user.UserID, input)
	if err != nil {
		h.writeError(c, err, "recognize wardrobe item image failed", user.UserID, "")
		return
	}
	response.OK(c, result)
}

func wardrobeListFilter(c *gin.Context) (ListFilter, bool) {
	filter := ListFilter{
		Category:             strings.TrimSpace(c.Query("category")),
		RecommendationStatus: strings.TrimSpace(c.Query("recommendation_status")),
	}
	if raw := strings.TrimSpace(c.Query("is_core")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return ListFilter{}, false
		}
		filter.IsCore = &value
	}
	return filter, true
}

func (h *Handler) writeError(c *gin.Context, err error, logMessage string, userID int64, publicID string) {
	switch {
	case errors.Is(err, ErrInvalidRecommendationStatus):
		response.Error(c, http.StatusBadRequest, "wardrobe.invalid_recommendation_status", "不支持的推荐状态")
	case errors.Is(err, ErrInvalidItemName):
		response.Error(c, http.StatusBadRequest, "wardrobe.invalid_item", "单品名称不能为空")
	case errors.Is(err, ErrInvalidPrimaryAsset):
		response.Error(c, http.StatusBadRequest, "wardrobe.invalid_primary_asset", "主图资产不可用于衣橱")
	case errors.Is(err, ErrInvalidWardrobeOption):
		response.Error(c, http.StatusBadRequest, "wardrobe.invalid_option", "衣服字段选项不支持")
	case errors.Is(err, ErrImageRecognizerUnavailable):
		response.Error(c, http.StatusInternalServerError, "wardrobe.image_recognizer_unavailable", "图片识别暂不可用")
	case errors.Is(err, ErrItemNotFound):
		response.Error(c, http.StatusNotFound, "wardrobe.item_not_found", "单品不存在")
	case errors.Is(err, ErrRepositoryUnsupported):
		h.logger.Error(logMessage, "error", err, "user_id", userID, "item_public_id", publicID)
		response.Error(c, http.StatusInternalServerError, "wardrobe.request_failed", "衣橱请求失败")
	default:
		h.logger.Error(logMessage, "error", err, "user_id", userID, "item_public_id", publicID)
		response.Error(c, http.StatusInternalServerError, "wardrobe.request_failed", "衣橱请求失败")
	}
}
