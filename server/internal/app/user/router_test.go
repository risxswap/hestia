package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/infra/config"
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

func TestDevLoginIsHiddenInProduction(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/user/dev-login",
		strings.NewReader(`{"nickname":"测试用户","dev_key":"ming-local"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewRouter(&baseapp.Deps{Config: &config.Config{AppEnv: "production"}}).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", response.Code)
	}
}

func TestDevLoginRouteIsKeptInDevelopment(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/user/dev-login",
		strings.NewReader(`{"nickname":"测试用户","dev_key":"ming-local"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewRouter(&baseapp.Deps{Config: &config.Config{AppEnv: "development"}}).ServeHTTP(response, request)

	if response.Code == http.StatusNotFound {
		t.Fatalf("expected dev-login route in development")
	}
}

func TestLegacyAssetsUploadRoutesAreNotRegistered(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/user/assets/upload-token",
		strings.NewReader(`{"asset_type":"clothes_item_photo","mime_type":"image/jpeg","file_size":1024}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewRouter(nil).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected legacy assets upload route to be removed, got status %d", response.Code)
	}
}

func TestClothesRoutesReplaceWardrobeRoutes(t *testing.T) {
	router := NewRouter(nil)

	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/user/wardrobe/items", nil)
	legacyResponse := httptest.NewRecorder()
	router.ServeHTTP(legacyResponse, legacyRequest)
	if legacyResponse.Code != http.StatusNotFound {
		t.Fatalf("expected legacy wardrobe route to be removed, got status %d", legacyResponse.Code)
	}

	clothesRequest := httptest.NewRequest(http.MethodGet, "/api/user/clothes/items", nil)
	clothesResponse := httptest.NewRecorder()
	router.ServeHTTP(clothesResponse, clothesRequest)
	if clothesResponse.Code == http.StatusNotFound {
		t.Fatalf("expected clothes route to be registered")
	}
}
