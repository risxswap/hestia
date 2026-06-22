package onboarding

import (
	"encoding/json"
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

func (h *Handler) Get(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	result, err := h.service.GetDraft(c.Request.Context(), user.UserID)
	if err != nil {
		h.logger.Error("get onboarding draft failed", "error", err, "user_id", user.UserID)
		response.Error(c, http.StatusInternalServerError, "onboarding.get_failed", "读取 onboarding 草稿失败")
		return
	}
	response.OK(c, result)
}

func (h *Handler) Save(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}

	var request SaveDraftRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "onboarding.invalid_request", "请求参数不正确")
		return
	}
	data, err := parseDraftData(request.Data)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "onboarding.invalid_request", "请求参数不正确")
		return
	}

	result, err := h.service.SaveDraft(c.Request.Context(), user.UserID, SaveDraftInput{
		Step: request.Step,
		Data: data,
	})
	if err != nil {
		if errors.Is(err, ErrValidation) {
			response.Error(c, http.StatusBadRequest, "onboarding.validation_failed", "请求参数不正确")
			return
		}
		h.logger.Error("save onboarding draft failed", "error", err, "user_id", user.UserID)
		response.Error(c, http.StatusInternalServerError, "onboarding.save_failed", "保存 onboarding 草稿失败")
		return
	}
	response.OK(c, result)
}

func (h *Handler) Submit(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	result, err := h.service.Submit(c.Request.Context(), user.UserID)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			response.Error(c, http.StatusBadRequest, "onboarding.validation_failed", "请先完成必要的 onboarding 信息")
			return
		}
		h.logger.Error("submit onboarding failed", "error", err, "user_id", user.UserID)
		response.Error(c, http.StatusInternalServerError, "onboarding.submit_failed", "生成初版报告失败，请稍后再试")
		return
	}
	response.OK(c, result)
}

func parseDraftData(raw json.RawMessage) (DraftData, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, ValidationError{Field: "data", Message: "required"}
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]json.RawMessage{}
	}
	return DraftData(data), nil
}
