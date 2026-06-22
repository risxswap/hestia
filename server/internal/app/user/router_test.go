package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/user/health", nil)
	response := httptest.NewRecorder()

	NewRouter(nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "ok" {
		t.Fatalf("expected code ok, got %#v", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["surface"] != "user" {
		t.Fatalf("expected surface user, got %#v", data["surface"])
	}
}

func TestMiniappHealthIsNotRegistered(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/miniapp/health", nil)
	response := httptest.NewRecorder()

	NewRouter(nil).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", response.Code)
	}
}
