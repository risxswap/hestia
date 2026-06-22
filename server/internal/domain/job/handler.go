package job

import (
	"errors"
	"hestia/server/internal/common/auth"
	"hestia/server/internal/common/response"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(c *gin.Context) {
	response.OK(c, h.service.List())
}

func (h *Handler) GetForUser(c *gin.Context) {
	user, ok := auth.UserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")
		return
	}
	result, err := h.service.GetForUser(c.Request.Context(), user.UserID, c.Param("public_id"))
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			response.Error(c, http.StatusNotFound, "job.not_found", "任务不存在")
			return
		}
		response.Error(c, http.StatusInternalServerError, "job.get_failed", "读取任务失败")
		return
	}
	response.OK(c, result)
}
