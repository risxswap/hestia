package agent

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

func (h *Handler) Chat(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	var request ChatRequest
	_ = c.ShouldBindJSON(&request)
	response.StreamHeaders(c)
	c.Status(http.StatusOK)
	_ = response.WriteSSE(c.Writer, "status", h.service.StreamStatus(c.Request.Context(), user.UserID))
	c.Writer.Flush()
	result, err := h.service.Chat(c.Request.Context(), user.UserID, request.Text)
	if err != nil {
		_ = response.WriteSSE(c.Writer, "error", gin.H{"message": "智能体请求失败"})
		c.Writer.Flush()
		return
	}
	_ = response.WriteSSE(c.Writer, "message", result.Message)
	done := StreamDone{MessagePublicID: result.AssistantMessagePublicID}
	if result.Draft != nil {
		_ = response.WriteSSE(c.Writer, "draft", result.Draft)
		done.DraftPublicID = result.Draft.DraftPublicID
	}
	_ = response.WriteSSE(c.Writer, "done", done)
	c.Writer.Flush()
}

func (h *Handler) CurrentDraft(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	card, err := h.service.CurrentDraft(c.Request.Context(), user.UserID)
	if err != nil {
		h.writeError(c, err, "get current advice draft failed", user.UserID, "")
		return
	}
	response.OK(c, card)
}

func (h *Handler) ConfirmDraft(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	advice, err := h.service.ConfirmDraft(c.Request.Context(), user.UserID, c.Param("public_id"))
	if err != nil {
		h.writeError(c, err, "confirm advice draft failed", user.UserID, c.Param("public_id"))
		return
	}
	response.OK(c, gin.H{"advice_public_id": advice.PublicID})
}

func (h *Handler) DiscardDraft(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	publicID := c.Param("public_id")
	if err := h.service.DiscardDraft(c.Request.Context(), user.UserID, publicID); err != nil {
		h.writeError(c, err, "discard advice draft failed", user.UserID, publicID)
		return
	}
	response.OK(c, gin.H{"public_id": publicID})
}

func (h *Handler) writeError(c *gin.Context, err error, logMessage string, userID int64, publicID string) {
	switch {
	case errors.Is(err, ErrDraftNotFound):
		response.Error(c, http.StatusNotFound, "agent.draft_not_found", "草稿不存在")
	case errors.Is(err, ErrDraftNotActive):
		response.Error(c, http.StatusConflict, "agent.draft_not_active", "草稿已确认或废弃")
	case errors.Is(err, ErrDraftIncomplete):
		response.Error(c, http.StatusConflict, "agent.draft_incomplete", "草稿内容不完整")
	case errors.Is(err, ErrRepositoryUnsupported):
		response.Error(c, http.StatusServiceUnavailable, "agent.repository_unavailable", "智能体草稿服务不可用")
	default:
		h.logger.Error(logMessage, "error", err, "user_id", userID, "draft_public_id", publicID)
		response.Error(c, http.StatusInternalServerError, "agent.request_failed", "智能体请求失败")
	}
}
