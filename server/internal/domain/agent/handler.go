package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

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
	streamCtx, cancelStream := context.WithCancel(c.Request.Context())
	defer cancelStream()
	var streamMu sync.Mutex
	streamFailed := false
	writeEvent := func(event string, data any) error {
		streamMu.Lock()
		if streamFailed {
			streamMu.Unlock()
			return ErrStreamClosed
		}
		if err := response.WriteSSE(c.Writer, event, data); err != nil {
			streamFailed = true
			streamMu.Unlock()
			cancelStream()
			return fmt.Errorf("write %s: %w: %v", event, ErrStreamClosed, err)
		}
		c.Writer.Flush()
		streamMu.Unlock()
		return nil
	}
	hasStreamFailed := func() bool {
		streamMu.Lock()
		defer streamMu.Unlock()
		return streamFailed
	}
	if err := writeEvent("status", h.service.StreamStatus(c.Request.Context(), user.UserID)); err != nil {
		return
	}
	result, err := h.service.ChatStream(streamCtx, user.UserID, request.Text, func(event ChatProcessEvent) {
		_ = writeEvent("process", event)
	}, func(delta StreamDelta) error {
		return writeEvent("delta", delta)
	}, request.AssetRefs...)
	if hasStreamFailed() {
		return
	}
	if err != nil {
		if errors.Is(err, ErrChatStopped) || errors.Is(err, context.Canceled) || c.Request.Context().Err() != nil {
			return
		}
		_ = writeEvent("error", gin.H{"message": "智能体请求失败"})
		return
	}
	done := StreamDone{
		MessagePublicID:   result.AssistantMessagePublicID,
		ResponseStartedAt: result.ResponseStartedAt,
		FinishedAt:        optionalTime(result.FinishedAt),
	}
	if result.Draft != nil {
		if err := writeEvent("draft", result.Draft); err != nil {
			return
		}
		done.DraftPublicID = result.Draft.DraftPublicID
	}
	_ = writeEvent("done", done)
}

func (h *Handler) Messages(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	messages, err := h.service.ChatHistory(c.Request.Context(), user.UserID, 50)
	if err != nil {
		h.writeError(c, err, "list agent chat messages failed", user.UserID, "")
		return
	}
	response.OK(c, gin.H{"messages": messages})
}

func (h *Handler) CurrentDraft(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	card, err := h.service.CurrentDraft(c.Request.Context(), user.UserID)
	if err != nil {
		if errors.Is(err, ErrDraftNotFound) {
			response.OK(c, nil)
			return
		}
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

func (h *Handler) DraftVersions(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	versions, err := h.service.DraftVersions(c.Request.Context(), user.UserID, c.Param("public_id"))
	if err != nil {
		h.writeError(c, err, "list advice draft versions failed", user.UserID, c.Param("public_id"))
		return
	}
	response.OK(c, gin.H{"versions": versions})
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
	case errors.Is(err, ErrDraftInvalid):
		response.Error(c, http.StatusBadRequest, "agent.draft_invalid", "草稿内容不符合结构要求")
	case errors.Is(err, ErrRepositoryUnsupported):
		response.Error(c, http.StatusServiceUnavailable, "agent.repository_unavailable", "智能体草稿服务不可用")
	default:
		h.logger.Error(logMessage, "error", err, "user_id", userID, "draft_public_id", publicID)
		response.Error(c, http.StatusInternalServerError, "agent.request_failed", "智能体请求失败")
	}
}
