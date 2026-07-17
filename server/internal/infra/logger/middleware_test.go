package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggingPropagatesRequestIDAndLogsResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestLogging(slog.New(slog.NewTextHandler(&logs, nil))))
	router.POST("/items/:id", func(c *gin.Context) {
		if got := RequestID(c.Request.Context()); got != "req_client_123" {
			t.Fatalf("expected request id in request context, got %q", got)
		}
		c.Status(http.StatusCreated)
	})

	request := httptest.NewRequest(http.MethodPost, "/items/42?secret=query", strings.NewReader(`{"token":"body-secret"}`))
	request.Header.Set("X-Request-ID", "req_client_123")
	request.Header.Set("Authorization", "Bearer auth-secret")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if got := response.Header().Get("X-Request-ID"); got != "req_client_123" {
		t.Fatalf("expected response request id, got %q", got)
	}
	output := logs.String()
	for _, expected := range []string{"http request completed", "request_id=req_client_123", "method=POST", "route=/items/:id", "status_code=201", "duration_ms="} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log to contain %q, got %s", expected, output)
		}
	}
	for _, leaked := range []string{"secret=query", "body-secret", "auth-secret"} {
		if strings.Contains(output, leaked) {
			t.Fatalf("expected log not to contain %q, got %s", leaked, output)
		}
	}
}

func TestRequestLoggingRecoversPanicWithStackAndUnifiedError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var logs bytes.Buffer
	router := gin.New()
	router.Use(RequestLogging(slog.New(slog.NewTextHandler(&logs, nil))))
	router.GET("/panic", func(*gin.Context) {
		panic("token=panic-secret")
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", response.Code)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "internal.server_error" {
		t.Fatalf("expected unified error code, got %q", body.Code)
	}
	output := logs.String()
	for _, expected := range []string{"http request panic", "level=ERROR", "panic=\"token=[REDACTED]\"", "stack=", "status_code=500"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected panic log to contain %q, got %s", expected, output)
		}
	}
	if strings.Contains(output, "panic-secret") {
		t.Fatalf("expected panic secret to be redacted, got %s", output)
	}
}

func TestRequestLoggingGeneratesRequestID(t *testing.T) {
	router := gin.New()
	router.Use(RequestLogging(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))))
	router.GET("/ok", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if got := response.Header().Get("X-Request-ID"); !strings.HasPrefix(got, "req_") {
		t.Fatalf("expected generated request id, got %q", got)
	}
}
