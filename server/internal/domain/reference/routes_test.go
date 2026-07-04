package reference_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hestia/server/internal/domain/reference"

	"github.com/gin-gonic/gin"
)

func TestGetReferencesReturnsEmptyItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	reference.RegisterUserRoutes(router.Group("/api/user/references"), nil)

	request := httptest.NewRequest(http.MethodGet, "/api/user/references", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
		Data struct {
			Items   []any `json:"items"`
			Enabled bool  `json:"enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" || !body.Data.Enabled || len(body.Data.Items) != 0 {
		t.Fatalf("expected enabled empty references response, got %#v", body)
	}
}
