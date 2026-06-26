package report_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/report"

	"github.com/gin-gonic/gin"
)

func TestLatestReportReturnsNullWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	report.RegisterUserRoutesWithService(router.Group("/api/user/reports"), report.NewService(missingReportRepo{}), nil)
	request := httptest.NewRequest(http.MethodGet, "/api/user/reports/latest", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	var body struct {
		Code string `json:"code"`
		Data any    `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected code ok, got %q", body.Code)
	}
	if body.Data != nil {
		t.Fatalf("expected nil data for missing latest report, got %#v", body.Data)
	}
}

type missingReportRepo struct{}

func (missingReportRepo) Create(context.Context, report.Report) (report.Report, error) {
	return report.Report{}, report.ErrReportNotFound
}

func (missingReportRepo) AddRoutes(context.Context, int64, []report.ReportRoute) error {
	return report.ErrReportNotFound
}

func (missingReportRepo) MarkReady(context.Context, int64) error {
	return report.ErrReportNotFound
}

func (missingReportRepo) LatestInitialForUser(context.Context, int64) (report.Report, error) {
	return report.Report{}, report.ErrReportNotFound
}

func (missingReportRepo) FindByPublicIDForUser(context.Context, int64, string) (report.Report, error) {
	return report.Report{}, report.ErrReportNotFound
}

func (missingReportRepo) RoutesByReportID(context.Context, int64) ([]report.ReportRoute, error) {
	return nil, report.ErrReportNotFound
}
