package profile_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/profile"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
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
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()
	repo := profile.NewMySQLRepository(sqlx.NewDb(db, "sqlmock"))
	userID := int64(12)

	mock.ExpectQuery(`(?s)FROM users u.*LEFT JOIN profiles p.*p\.status = 'active'.*WHERE u\.id = \?`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_public_id",
			"nickname",
			"onboarding_status",
			"profile_id",
			"profile_public_id",
			"gender",
			"height_cm",
			"body_notes",
			"skin_notes",
			"hair_notes",
			"lifestyle_scenarios",
			"style_goal_summary",
		}).AddRow(
			"usr_test",
			"明明",
			"completed",
			int64(34),
			"prf_test",
			"female",
			165,
			"肩颈偏窄",
			"中性偏暖",
			"锁骨发",
			[]byte(`["通勤","周末见朋友"]`),
			"更利落",
		))
	mock.ExpectQuery(`(?s)SELECT.*fact_count.*preference_count.*avoidance_count.*inference_count.*pending_confirmation_count`).
		WithArgs(userID, userID, userID, userID, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"fact_count",
			"preference_count",
			"avoidance_count",
			"inference_count",
			"pending_confirmation_count",
		}).AddRow(4, 2, 1, 3, 1))
	mock.ExpectQuery(`(?s)FROM reports.*report_type = 'initial'.*status = 'ready'.*LIMIT 1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "title", "status", "generated_at"}))

	summary, err := repo.Summary(context.Background(), userID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.User.UserPublicID != "usr_test" || summary.User.Nickname != "明明" || summary.User.OnboardingStatus != "completed" {
		t.Fatalf("unexpected user summary: %#v", summary.User)
	}
	if summary.Profile == nil {
		t.Fatalf("expected profile summary")
	}
	if summary.Profile.ProfilePublicID != "prf_test" || summary.Profile.Gender != "female" {
		t.Fatalf("unexpected profile summary: %#v", summary.Profile)
	}
	if summary.Profile.HeightCM == nil || *summary.Profile.HeightCM != 165 {
		t.Fatalf("unexpected height: %#v", summary.Profile.HeightCM)
	}
	if summary.Profile.BodyNotes != "肩颈偏窄" || summary.Profile.SkinNotes != "中性偏暖" || summary.Profile.HairNotes != "锁骨发" {
		t.Fatalf("unexpected profile notes: %#v", summary.Profile)
	}
	if len(summary.Profile.LifestyleScenarios) != 2 || summary.Profile.LifestyleScenarios[0] != "通勤" || summary.Profile.LifestyleScenarios[1] != "周末见朋友" {
		t.Fatalf("unexpected scenarios: %#v", summary.Profile.LifestyleScenarios)
	}
	if summary.Profile.StyleGoalSummary != "更利落" {
		t.Fatalf("unexpected style goal: %q", summary.Profile.StyleGoalSummary)
	}
	if summary.MemorySummary.FactCount != 4 ||
		summary.MemorySummary.PreferenceCount != 2 ||
		summary.MemorySummary.AvoidanceCount != 1 ||
		summary.MemorySummary.InferenceCount != 3 ||
		summary.MemorySummary.PendingConfirmationCount != 1 {
		t.Fatalf("unexpected memory summary: %#v", summary.MemorySummary)
	}
	if summary.LatestReport != nil {
		t.Fatalf("expected nil latest report, got %#v", summary.LatestReport)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestPatchProfileUpdatesExplicitProfileFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{
		summary: profile.Summary{
			User: profile.UserSummary{UserPublicID: "usr_test", Nickname: "明明", OnboardingStatus: "completed"},
			Profile: &profile.ProfileSummary{
				ProfilePublicID:    "prf_test",
				Gender:             "female",
				HeightCM:           intPtr(165),
				BodyNotes:          "希望通勤更利落",
				SkinNotes:          "中性偏暖",
				HairNotes:          "锁骨发",
				LifestyleScenarios: []string{"通勤"},
			},
			MemorySummary: profile.MemorySummary{FactCount: 4},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile", strings.NewReader(`{"nickname":"明明","gender":"female","height_cm":165,"body_notes":"希望通勤更利落","skin_notes":"中性偏暖","hair_notes":"锁骨发","lifestyle_scenarios":["通勤"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.lastProfileUserID != 12 {
		t.Fatalf("expected user 12, got %d", repo.lastProfileUserID)
	}
	if repo.lastProfileInput.Nickname != "明明" || repo.lastProfileInput.BodyNotes != "希望通勤更利落" {
		t.Fatalf("expected captured profile input, got %#v", repo.lastProfileInput)
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
	assertQuickEntries(t, body.Data.QuickEntries, []profile.QuickEntry{
		{Key: "profile", Title: "我的档案", Summary: "已记录 1 个常见场景"},
		{Key: "preferences", Title: "偏好与禁忌", Summary: "0 个风格目标、0 个禁忌"},
		{Key: "report", Title: "报告与路线", Summary: "暂无初版报告"},
		{Key: "privacy", Title: "隐私与数据", Summary: "照片、档案、反馈可管理"},
	})
}

func TestPatchPreferencesUpdatesExplicitPrefs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{
		summary: profile.Summary{
			User:          profile.UserSummary{UserPublicID: "usr_test"},
			MemorySummary: profile.MemorySummary{PreferenceCount: 2, AvoidanceCount: 1},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile/preferences", strings.NewReader(`{"style_goals":["更利落"],"avoidances":["过甜"],"scenario_preferences":["通勤"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.lastPreferencesUserID != 12 {
		t.Fatalf("expected user 12, got %d", repo.lastPreferencesUserID)
	}
	if len(repo.lastPreferencesInput.StyleGoals) != 1 || repo.lastPreferencesInput.StyleGoals[0] != "更利落" {
		t.Fatalf("expected captured preferences, got %#v", repo.lastPreferencesInput)
	}
	if len(repo.lastPreferencesInput.Avoidances) != 1 || repo.lastPreferencesInput.Avoidances[0] != "过甜" {
		t.Fatalf("expected captured avoidances, got %#v", repo.lastPreferencesInput)
	}
	if len(repo.lastPreferencesInput.ScenarioPreferences) != 1 || repo.lastPreferencesInput.ScenarioPreferences[0] != "通勤" {
		t.Fatalf("expected captured scenario preferences, got %#v", repo.lastPreferencesInput)
	}
}

func TestPatchProfileRejectsInvalidInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile", strings.NewReader(`{"body_notes":"`+strings.Repeat("太", 221)+`"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertErrorCode(t, recorder.Body.Bytes(), "profile.validation_failed")
	if repo.lastProfileInput.BodyNotes != "" {
		t.Fatalf("expected invalid input not to reach repo, got %#v", repo.lastProfileInput)
	}
}

func TestPatchPreferencesRejectsInvalidInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile/preferences", strings.NewReader(`{"style_goals":["`+strings.Repeat("长", 61)+`"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertErrorCode(t, recorder.Body.Bytes(), "profile.validation_failed")
	if len(repo.lastPreferencesInput.StyleGoals) != 0 {
		t.Fatalf("expected invalid input not to reach repo, got %#v", repo.lastPreferencesInput)
	}
}

func TestMySQLUpdateExplicitProfileWritesUserProfileAndFacts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()
	repo := profile.NewMySQLRepository(sqlx.NewDb(db, "sqlmock"))
	userID := int64(12)

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE users SET nickname = NULLIF\(\?, ''\) WHERE id = \? AND deleted_at IS NULL`).
		WithArgs("明明", userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO profiles.*ON DUPLICATE KEY UPDATE`).
		WithArgs(sqlmock.AnyArg(), userID, "active", "female", 165, "希望通勤更利落", "中性偏暖", "锁骨发", `["通勤"]`).
		WillReturnResult(sqlmock.NewResult(34, 1))
	mock.ExpectQuery(`(?s)SELECT.*FROM profiles.*WHERE user_id = \?`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"public_id",
			"user_id",
			"status",
			"gender",
			"height_cm",
			"body_notes",
			"skin_notes",
			"hair_notes",
			"style_goal_summary",
		}).AddRow(int64(34), "prf_test", userID, "active", "female", 165, "希望通勤更利落", "中性偏暖", "锁骨发", ""))
	mock.ExpectExec(`UPDATE profile_facts SET deleted_at = CURRENT_TIMESTAMP\(3\) WHERE user_id = \? AND profile_id = \? AND deleted_at IS NULL`).
		WithArgs(userID, int64(34)).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`INSERT INTO profile_facts`).
		WithArgs(userID, int64(34), "gender", `"female"`, "user").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO profile_facts`).
		WithArgs(userID, int64(34), "height_cm", "165", "user").
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(`INSERT INTO profile_facts`).
		WithArgs(userID, int64(34), "lifestyle_scenarios", `["通勤"]`, "user").
		WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()
	expectSummaryQueries(mock, userID)

	summary, err := repo.UpdateExplicitProfile(context.Background(), userID, profile.UpdateProfileInput{
		Nickname:           "明明",
		Gender:             "female",
		HeightCM:           intPtr(165),
		BodyNotes:          "希望通勤更利落",
		SkinNotes:          "中性偏暖",
		HairNotes:          "锁骨发",
		LifestyleScenarios: []string{"通勤"},
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if summary.User.UserPublicID != "usr_test" {
		t.Fatalf("expected refreshed summary, got %#v", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestMySQLUpdateExplicitPreferencesReplacesPrefsWithUserSource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()
	repo := profile.NewMySQLRepository(sqlx.NewDb(db, "sqlmock"))
	userID := int64(12)

	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO profiles.*ON DUPLICATE KEY UPDATE`).
		WithArgs(sqlmock.AnyArg(), userID, "active").
		WillReturnResult(sqlmock.NewResult(34, 1))
	mock.ExpectQuery(`(?s)SELECT.*FROM profiles.*WHERE user_id = \?`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"public_id",
			"user_id",
			"status",
			"gender",
			"height_cm",
			"body_notes",
			"skin_notes",
			"hair_notes",
			"style_goal_summary",
		}).AddRow(int64(34), "prf_test", userID, "active", "", nil, "", "", "", ""))
	mock.ExpectExec(`UPDATE profile_prefs SET deleted_at = CURRENT_TIMESTAMP\(3\) WHERE user_id = \? AND profile_id = \? AND deleted_at IS NULL`).
		WithArgs(userID, int64(34)).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec(`INSERT INTO profile_prefs`).
		WithArgs(userID, int64(34), "style_goal", "更利落", `"更利落"`, "positive", "user", nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO profile_prefs`).
		WithArgs(userID, int64(34), "avoidance", "过甜", `"过甜"`, "negative", "user", nil).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(`INSERT INTO profile_prefs`).
		WithArgs(userID, int64(34), "scenario_preference", "通勤", `"通勤"`, "positive", "user", nil).
		WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()
	expectSummaryQueries(mock, userID)

	_, err = repo.UpdateExplicitPreferences(context.Background(), userID, profile.UpdatePreferencesInput{
		StyleGoals:          []string{"更利落"},
		Avoidances:          []string{"过甜"},
		ScenarioPreferences: []string{"通勤"},
	})
	if err != nil {
		t.Fatalf("update preferences: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
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
	summary               profile.Summary
	err                   error
	lastProfileUserID     int64
	lastProfileInput      profile.UpdateProfileInput
	lastPreferencesUserID int64
	lastPreferencesInput  profile.UpdatePreferencesInput
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

func (r *routeProfileRepo) UpdateExplicitProfile(_ context.Context, userID int64, input profile.UpdateProfileInput) (profile.Summary, error) {
	if r.err != nil {
		return profile.Summary{}, r.err
	}
	r.lastProfileUserID = userID
	r.lastProfileInput = input
	return r.summary, nil
}

func (r *routeProfileRepo) UpdateExplicitPreferences(_ context.Context, userID int64, input profile.UpdatePreferencesInput) (profile.Summary, error) {
	if r.err != nil {
		return profile.Summary{}, r.err
	}
	r.lastPreferencesUserID = userID
	r.lastPreferencesInput = input
	return r.summary, nil
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

func assertErrorCode(t *testing.T, raw []byte, expected string) {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Code != expected {
		t.Fatalf("expected error code %q, got %q body=%s", expected, body.Code, string(raw))
	}
}

func expectSummaryQueries(mock sqlmock.Sqlmock, userID int64) {
	mock.ExpectQuery(`(?s)FROM users u.*LEFT JOIN profiles p.*WHERE u\.id = \?`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_public_id",
			"nickname",
			"onboarding_status",
			"profile_id",
			"profile_public_id",
			"gender",
			"height_cm",
			"body_notes",
			"skin_notes",
			"hair_notes",
			"lifestyle_scenarios",
			"style_goal_summary",
		}).AddRow(
			"usr_test",
			"明明",
			"completed",
			int64(34),
			"prf_test",
			"female",
			165,
			"希望通勤更利落",
			"中性偏暖",
			"锁骨发",
			[]byte(`["通勤"]`),
			"更利落",
		))
	mock.ExpectQuery(`(?s)SELECT.*fact_count.*preference_count.*avoidance_count.*inference_count.*pending_confirmation_count`).
		WithArgs(userID, userID, userID, userID, userID).
		WillReturnRows(sqlmock.NewRows([]string{
			"fact_count",
			"preference_count",
			"avoidance_count",
			"inference_count",
			"pending_confirmation_count",
		}).AddRow(4, 2, 1, 3, 1))
	mock.ExpectQuery(`(?s)FROM reports.*report_type = 'initial'.*status = 'ready'.*LIMIT 1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "title", "status", "generated_at"}))
}
