package profile_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/profile"

	"github.com/gin-gonic/gin"
)

func TestSummaryReturnsDashboardData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generatedAt := time.Date(2026, 6, 27, 6, 0, 0, 0, time.UTC)
	repo := &routeProfileRepo{
		summary: profile.Summary{
			User: profile.UserSummary{
				UserPublicID:     "usr_test",
				Nickname:         "明明",
				OnboardingStatus: "completed",
			},
			Profile: &profile.ProfileSummary{
				ProfilePublicID:    "prf_test",
				Gender:             "female",
				HeightCM:           intPtr(165),
				LifestyleScenarios: []string{"通勤", "周末见朋友"},
				StyleGoalSummary:   "更利落",
			},
			MemorySummary: profile.MemorySummary{
				FactCount:                4,
				PreferenceCount:          2,
				AvoidanceCount:           1,
				InferenceCount:           3,
				PendingConfirmationCount: 1,
			},
			LatestReport: &profile.LatestReportSummary{
				PublicID:    "rpt_test",
				Title:       "初版个人形象报告",
				Status:      "ready",
				GeneratedAt: &generatedAt,
			},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodGet, "/api/user/profile/summary", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string          `json:"code"`
		Data profile.Summary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected code ok, got %q", body.Code)
	}
	if body.Data.User.UserPublicID != "usr_test" {
		t.Fatalf("expected user summary, got %#v", body.Data.User)
	}
	if body.Data.Profile == nil || body.Data.Profile.ProfilePublicID != "prf_test" {
		t.Fatalf("expected profile summary, got %#v", body.Data.Profile)
	}
	if body.Data.LatestReport == nil || body.Data.LatestReport.PublicID != "rpt_test" {
		t.Fatalf("expected latest report summary, got %#v", body.Data.LatestReport)
	}
	assertQuickEntries(t, body.Data.QuickEntries, []profile.QuickEntry{
		{Key: "profile", Title: "我的档案", Summary: "已记录 2 个常见场景"},
		{Key: "preferences", Title: "偏好与禁忌", Summary: "2 个风格目标、1 个禁忌"},
		{Key: "report", Title: "报告与路线", Summary: "初版报告已生成"},
		{Key: "privacy", Title: "隐私与数据", Summary: "照片、档案、反馈可管理"},
	})
}

func TestSummaryReturnsEmptyStateWithoutProfileOrReport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{
		summary: profile.Summary{
			User: profile.UserSummary{
				UserPublicID:     "usr_test",
				Nickname:         "明明",
				OnboardingStatus: "not_started",
			},
			MemorySummary: profile.MemorySummary{},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodGet, "/api/user/profile/summary", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data profile.Summary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Profile != nil {
		t.Fatalf("expected nil profile, got %#v", body.Data.Profile)
	}
	if body.Data.LatestReport != nil {
		t.Fatalf("expected nil report, got %#v", body.Data.LatestReport)
	}
	assertQuickEntries(t, body.Data.QuickEntries, []profile.QuickEntry{
		{Key: "profile", Title: "我的档案", Summary: "还没有记录常见场景"},
		{Key: "preferences", Title: "偏好与禁忌", Summary: "0 个风格目标、0 个禁忌"},
		{Key: "report", Title: "报告与路线", Summary: "暂无初版报告"},
		{Key: "privacy", Title: "隐私与数据", Summary: "照片、档案、反馈可管理"},
	})
}

func TestMySQLSummaryFiltersActiveProfile(t *testing.T) {
	source, err := os.ReadFile("repo.go")
	if err != nil {
		t.Fatalf("read repo source: %v", err)
	}
	if !strings.Contains(string(source), "p.status = 'active'") {
		t.Fatalf("expected summary query to filter active profile")
	}
}

func newProfileRouteTestRouter(repo *routeProfileRepo) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	profile.RegisterUserRoutesWithService(router.Group("/api/user/profile"), profile.NewService(repo), nil)
	return router
}

type routeProfileRepo struct {
	summary profile.Summary
	err     error
}

func (r *routeProfileRepo) Summary(context.Context, int64) (profile.Summary, error) {
	if r.err != nil {
		return profile.Summary{}, r.err
	}
	return r.summary, nil
}

func (*routeProfileRepo) Upsert(context.Context, profile.Profile) (profile.Profile, error) {
	return profile.Profile{}, errors.New("unused")
}

func (*routeProfileRepo) ReplaceFacts(context.Context, int64, int64, []profile.Fact) error {
	return errors.New("unused")
}

func (*routeProfileRepo) ReplacePrefs(context.Context, int64, int64, []profile.Pref) error {
	return errors.New("unused")
}

func (*routeProfileRepo) CreateInferences(context.Context, []profile.Inference) error {
	return errors.New("unused")
}

func (*routeProfileRepo) MarkUserOnboardingCompleted(context.Context, int64) error {
	return errors.New("unused")
}

func intPtr(value int) *int {
	return &value
}

func assertQuickEntries(t *testing.T, actual []profile.QuickEntry, expected []profile.QuickEntry) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("expected %d quick entries, got %#v", len(expected), actual)
	}
	for i := range expected {
		if actual[i].Key != expected[i].Key {
			t.Fatalf("entry %d expected key %q, got %#v", i, expected[i].Key, actual[i])
		}
		if actual[i].Title != expected[i].Title {
			t.Fatalf("entry %d expected title %q, got %#v", i, expected[i].Title, actual[i])
		}
		if actual[i].Summary != expected[i].Summary {
			t.Fatalf("entry %d expected summary %q, got %#v", i, expected[i].Summary, actual[i])
		}
	}
}
