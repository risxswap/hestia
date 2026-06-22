package account

import (
	"errors"
	"log/slog"
	"net/http"

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

func (h *Handler) DevLogin(c *gin.Context) {
	var input DevLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "account.invalid_request", "请求参数不正确")
		return
	}
	result, err := h.service.DevLogin(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			response.Error(c, http.StatusBadRequest, "account.validation_failed", "请求参数不正确")
			return
		}
		h.logger.Error("dev login failed", "error", err)
		response.Error(c, http.StatusInternalServerError, "account.dev_login_failed", "登录失败")
		return
	}
	response.OK(c, result)
}
