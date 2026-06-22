package account

import (
	"net/http"

	"hestia/server/internal/common/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) DevLogin(c *gin.Context) {
	var input DevLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "account.invalid_request", "请求参数不正确")
		return
	}
	result, err := h.service.DevLogin(c.Request.Context(), input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "account.dev_login_failed", "登录失败")
		return
	}
	response.OK(c, result)
}
