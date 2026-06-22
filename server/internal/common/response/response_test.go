package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hestia/server/internal/common/response"

	"github.com/gin-gonic/gin"
)

func TestOKWritesUnifiedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ok", func(c *gin.Context) {
		response.OK(c, gin.H{"surface": "user"})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ok", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "ok" {
		t.Fatalf("expected code ok, got %#v", body["code"])
	}
	if body["message"] != "" {
		t.Fatalf("expected empty message, got %#v", body["message"])
	}
	data := body["data"].(map[string]any)
	if data["surface"] != "user" {
		t.Fatalf("expected surface user, got %#v", data["surface"])
	}
}

func TestErrorWritesUnifiedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/error", func(c *gin.Context) {
		response.Error(c, http.StatusUnauthorized, "account.unauthorized", "请先登录")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/error", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "account.unauthorized" {
		t.Fatalf("unexpected code %#v", body["code"])
	}
	if body["message"] != "请先登录" {
		t.Fatalf("unexpected message %#v", body["message"])
	}
	if body["data"] != nil {
		t.Fatalf("expected nil data, got %#v", body["data"])
	}
}
