package makeup_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hestia/server/internal/domain/makeup"

	"github.com/gin-gonic/gin"
)

func TestGetMakeupReturnsEmptyItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	makeup.RegisterUserRoutes(router.Group("/api/user/makeup"), nil)

	request := httptest.NewRequest(http.MethodGet, "/api/user/makeup", nil)
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
		t.Fatalf("expected enabled empty makeup response, got %#v", body)
	}
}
