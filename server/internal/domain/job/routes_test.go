package job_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/domain/job"

	"github.com/gin-gonic/gin"
)

func TestListJobsReturnsEmptyList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	job.RegisterAdminRoutes(router.Group("/api/admin/jobs"), &baseapp.Deps{})
	request := httptest.NewRequest(http.MethodGet, "/api/admin/jobs", nil)
	recorder := httptest.NewRecorder()

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
	data := body["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("expected empty items, got %#v", items)
	}
}
