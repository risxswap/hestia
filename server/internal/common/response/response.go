package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		Code:    "ok",
		Message: "",
		Data:    data,
	})
}

func Error(c *gin.Context, status int, code string, message string) {
	c.JSON(status, Body{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}
