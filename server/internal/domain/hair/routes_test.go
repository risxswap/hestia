package hair_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hestia/server/internal/domain/hair"

	"github.com/gin-gonic/gin"
)

func TestGetHairReturnsEmptyItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	hair.RegisterUserRoutes(router.Group("/api/user/hair"), nil)

	request := httptest.NewRequest(http.MethodGet, "/api/user/hair", nil)
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
		t.Fatalf("expected enabled empty hair response, got %#v", body)
	}
}
