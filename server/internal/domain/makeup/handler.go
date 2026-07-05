package makeup

import (
	"errors"
	"log/slog"
	"net/http"
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
	items, err := h.service.ListItems(c.Request.Context(), user.UserID, ListFilter{})
	if err != nil {
		h.writeError(c, err, "list makeup items failed", user.UserID, "")
		return
	}
	response.OK(c, gin.H{"items": items})
}

func (h *Handler) GetItem(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	publicID := c.Param("public_id")
	item, err := h.service.GetItem(c.Request.Context(), user.UserID, publicID)
	if err != nil {
		h.writeError(c, err, "get makeup item failed", user.UserID, publicID)
		return
	}
	response.OK(c, item)
}

func (h *Handler) CreateItem(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "makeup.invalid_request", "请求参数不正确")
		return
	}
	item, err := h.service.CreateItem(c.Request.Context(), user.UserID, input)
	if err != nil {
		h.writeError(c, err, "create makeup item failed", user.UserID, "")
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
		response.Error(c, http.StatusBadRequest, "makeup.invalid_request", "请求参数不正确")
		return
	}
	publicID := c.Param("public_id")
	item, err := h.service.UpdateItem(c.Request.Context(), user.UserID, publicID, input)
	if err != nil {
		h.writeError(c, err, "update makeup item failed", user.UserID, publicID)
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
		h.writeError(c, err, "delete makeup item failed", user.UserID, publicID)
		return
	}
	response.OK(c, gin.H{"public_id": strings.TrimSpace(publicID)})
}

func (h *Handler) writeError(c *gin.Context, err error, logMessage string, userID int64, publicID string) {
	switch {
	case errors.Is(err, ErrInvalidRecommendationStatus):
		response.Error(c, http.StatusBadRequest, "makeup.invalid_recommendation_status", "不支持的推荐状态")
	case errors.Is(err, ErrInvalidItemName):
		response.Error(c, http.StatusBadRequest, "makeup.invalid_item", "妆容名称不能为空")
	case errors.Is(err, ErrInvalidPrimaryAsset):
		response.Error(c, http.StatusBadRequest, "makeup.invalid_primary_asset", "主图资产不可用于妆容")
	case errors.Is(err, ErrItemNotFound):
		response.Error(c, http.StatusNotFound, "makeup.item_not_found", "妆容不存在")
	default:
		h.logger.Error(logMessage, "error", err, "user_id", userID, "item_public_id", publicID)
		response.Error(c, http.StatusInternalServerError, "makeup.request_failed", "妆容请求失败")
	}
}
