package logger

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"hestia/server/internal/common/id"
	"hestia/server/internal/common/response"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-ID"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func RequestLogging(log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}
	return func(c *gin.Context) {
		startedAt := time.Now()
		requestID := normalizedRequestID(c.GetHeader(requestIDHeader))
		c.Header(requestIDHeader, requestID)
		c.Request = c.Request.WithContext(WithRequestID(c.Request.Context(), requestID))

		defer func() {
			if recovered := recover(); recovered != nil {
				log.ErrorContext(c.Request.Context(), "http request panic",
					"request_id", requestID,
					"method", c.Request.Method,
					"route", requestRoute(c),
					"panic", SanitizeSummary(fmt.Sprint(recovered)),
					"stack", string(debug.Stack()),
				)
				c.Abort()
				if !c.Writer.Written() {
					response.Error(c, http.StatusInternalServerError, "internal.server_error", "服务暂时不可用")
				}
			}
			logRequestCompleted(log, c, requestID, time.Since(startedAt))
		}()

		c.Next()
	}
}

func normalizedRequestID(value string) string {
	value = strings.TrimSpace(value)
	if validRequestID.MatchString(value) {
		return value
	}
	return id.NewPublicID("req")
}

func requestRoute(c *gin.Context) string {
	if route := strings.TrimSpace(c.FullPath()); route != "" {
		return route
	}
	return "unmatched"
}

func logRequestCompleted(log *slog.Logger, c *gin.Context, requestID string, duration time.Duration) {
	attrs := []any{
		"request_id", requestID,
		"method", c.Request.Method,
		"route", requestRoute(c),
		"status_code", c.Writer.Status(),
		"duration_ms", duration.Milliseconds(),
		"client_ip", c.ClientIP(),
	}
	switch status := c.Writer.Status(); {
	case status >= http.StatusInternalServerError:
		log.ErrorContext(c.Request.Context(), "http request completed", attrs...)
	case status >= http.StatusBadRequest:
		log.WarnContext(c.Request.Context(), "http request completed", attrs...)
	default:
		log.InfoContext(c.Request.Context(), "http request completed", attrs...)
	}
}
