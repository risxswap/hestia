package profile_test

import (
	"context"
	"database/sql"
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
			Preferences: profile.PreferencesSummary{
				StyleGoals:          []string{"更利落"},
				Avoidances:          []string{"过甜"},
				ScenarioPreferences: []string{"通勤更正式"},
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
			ProfilePhotos: []profile.ProfilePhoto{
				{
					PublicID:      "pph_test",
					AssetPublicID: "ast_test",
					PhotoType:     "full_body",
					Angle:         "front",
					Image:         &profile.ProfilePhotoImage{ObjectKey: "users/12/profile/ast_test.jpg"},
				},
			},
		},
	}
	signer := &routeProfileImageSigner{
		urls: map[string]profile.SignedImageURLs{
			"users/12/profile/ast_test.jpg": {PreviewURL: "https://private.example.test/preview.jpg"},
		},
	}
	router := newProfileRouteTestRouterWithSigner(repo, signer)
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
	if len(body.Data.ProfilePhotos) != 1 || body.Data.ProfilePhotos[0].Image == nil || body.Data.ProfilePhotos[0].Image.URL != "https://private.example.test/preview.jpg" {
		t.Fatalf("expected signed profile photo url, got %#v", body.Data.ProfilePhotos)
	}
	if len(body.Data.Preferences.StyleGoals) != 1 || body.Data.Preferences.StyleGoals[0] != "更利落" {
		t.Fatalf("expected style goals in summary, got %#v", body.Data.Preferences)
	}
	if len(body.Data.Preferences.Avoidances) != 1 || body.Data.Preferences.Avoidances[0] != "过甜" {
		t.Fatalf("expected avoidances in summary, got %#v", body.Data.Preferences)
	}
	if len(body.Data.Preferences.ScenarioPreferences) != 1 || body.Data.Preferences.ScenarioPreferences[0] != "通勤更正式" {
		t.Fatalf("expected scenario preferences in summary, got %#v", body.Data.Preferences)
	}
	assertQuickEntries(t, body.Data.QuickEntries, []profile.QuickEntry{
		{Key: "profile", Title: "我的档案", Summary: "已记录 2 个常见场景"},
		{Key: "preferences", Title: "偏好与禁忌", Summary: "2 个偏好、1 个禁忌"},
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
		{Key: "preferences", Title: "偏好与禁忌", Summary: "0 个偏好、0 个禁忌"},
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
	mock.ExpectQuery(`(?s)FROM profile_prefs.*WHERE user_id = \?.*deleted_at IS NULL`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"pref_type", "pref_key", "polarity"}).
			AddRow("style_goal", "更利落", "positive").
			AddRow("avoidance", "过甜", "negative").
			AddRow("scenario_preference", "通勤更正式", "positive"))
	mock.ExpectQuery(`(?s)FROM reports.*report_type = 'initial'.*status = 'ready'.*LIMIT 1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "title", "status", "generated_at"}))
	expectProfilePhotoSummaryQuery(mock, userID)

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
	if len(summary.Preferences.StyleGoals) != 1 || summary.Preferences.StyleGoals[0] != "更利落" {
		t.Fatalf("unexpected style goals: %#v", summary.Preferences)
	}
	if len(summary.Preferences.Avoidances) != 1 || summary.Preferences.Avoidances[0] != "过甜" {
		t.Fatalf("unexpected avoidances: %#v", summary.Preferences)
	}
	if len(summary.Preferences.ScenarioPreferences) != 1 || summary.Preferences.ScenarioPreferences[0] != "通勤更正式" {
		t.Fatalf("unexpected scenario preferences: %#v", summary.Preferences)
	}
	if summary.LatestReport != nil {
		t.Fatalf("expected nil latest report, got %#v", summary.LatestReport)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestMySQLSummaryTreatsNullLifestyleScenariosAsEmpty(t *testing.T) {
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
			nil,
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
	mock.ExpectQuery(`(?s)FROM profile_prefs.*WHERE user_id = \?.*deleted_at IS NULL`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"pref_type", "pref_key", "polarity"}))
	mock.ExpectQuery(`(?s)FROM reports.*report_type = 'initial'.*status = 'ready'.*LIMIT 1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "title", "status", "generated_at"}))
	expectProfilePhotoSummaryQuery(mock, userID)

	summary, err := repo.Summary(context.Background(), userID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.Profile == nil {
		t.Fatalf("expected profile summary")
	}
	if len(summary.Profile.LifestyleScenarios) != 0 {
		t.Fatalf("expected empty scenarios, got %#v", summary.Profile.LifestyleScenarios)
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
	if !repo.lastProfileInput.Nickname.Present || repo.lastProfileInput.Nickname.Value != "明明" ||
		!repo.lastProfileInput.BodyNotes.Present || repo.lastProfileInput.BodyNotes.Value != "希望通勤更利落" {
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
		{Key: "preferences", Title: "偏好与禁忌", Summary: "0 个偏好、0 个禁忌"},
		{Key: "report", Title: "报告与路线", Summary: "暂无初版报告"},
		{Key: "privacy", Title: "隐私与数据", Summary: "照片、档案、反馈可管理"},
	})
}

func TestPatchProfileUpdatesFullArchiveFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{
		summary: profile.Summary{
			User: profile.UserSummary{UserPublicID: "usr_test", Nickname: "明明", OnboardingStatus: "completed"},
			Profile: &profile.ProfileSummary{
				ProfilePublicID: "prf_test",
				HeightCM:        intPtr(165),
				WeightKG:        floatPtr(52.5),
				FaceShape:       "方圆脸",
				UpperBodyNotes:  "肩线偏窄",
				LowerBodyNotes:  "偏好利落裤装",
				SizeNotes:       "上衣 M，鞋码 37",
			},
			MemorySummary: profile.MemorySummary{FactCount: 4},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile", strings.NewReader(`{"weight_kg":52.5,"face_shape":"方圆脸","upper_body_notes":"肩线偏窄","lower_body_notes":"偏好利落裤装","size_notes":"上衣 M，鞋码 37"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	input := repo.lastProfileInput
	if !input.WeightKG.Present || input.WeightKG.Value == nil || *input.WeightKG.Value != 52.5 {
		t.Fatalf("expected weight patch, got %#v", input.WeightKG)
	}
	if !input.FaceShape.Present || input.FaceShape.Value != "方圆脸" {
		t.Fatalf("expected face shape patch, got %#v", input.FaceShape)
	}
	if !input.UpperBodyNotes.Present || input.UpperBodyNotes.Value != "肩线偏窄" {
		t.Fatalf("expected upper body patch, got %#v", input.UpperBodyNotes)
	}
	if !input.LowerBodyNotes.Present || input.LowerBodyNotes.Value != "偏好利落裤装" {
		t.Fatalf("expected lower body patch, got %#v", input.LowerBodyNotes)
	}
	if !input.SizeNotes.Present || input.SizeNotes.Value != "上衣 M，鞋码 37" {
		t.Fatalf("expected size notes patch, got %#v", input.SizeNotes)
	}
	var body struct {
		Data profile.Summary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Profile == nil || body.Data.Profile.WeightKG == nil || *body.Data.Profile.WeightKG != 52.5 {
		t.Fatalf("expected full archive profile in response, got %#v", body.Data.Profile)
	}
}

func TestPatchProfileOnlyUpdatesPresentFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{
		summary: profile.Summary{
			User: profile.UserSummary{UserPublicID: "usr_test", Nickname: "新昵称", OnboardingStatus: "completed"},
			Profile: &profile.ProfileSummary{
				ProfilePublicID:    "prf_test",
				Gender:             "female",
				HeightCM:           intPtr(165),
				BodyNotes:          "保留身形记录",
				SkinNotes:          "保留肤色记录",
				HairNotes:          "保留发型记录",
				LifestyleScenarios: []string{"通勤"},
			},
			MemorySummary: profile.MemorySummary{FactCount: 3},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile", strings.NewReader(`{"nickname":"新昵称"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	input := repo.lastProfileInput
	if !input.Nickname.Present || input.Nickname.Value != "新昵称" {
		t.Fatalf("expected nickname to be present, got %#v", input.Nickname)
	}
	if input.Gender.Present || input.HeightCM.Present || input.BodyNotes.Present || input.SkinNotes.Present || input.HairNotes.Present || input.LifestyleScenarios.Present {
		t.Fatalf("expected omitted profile fields to stay absent, got %#v", input)
	}
}

func TestPatchProfileRejectsInvalidWeightRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
	}{
		{name: "too low", body: `{"weight_kg":19.9}`},
		{name: "too high", body: `{"weight_kg":300.1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &routeProfileRepo{}
			router := newProfileRouteTestRouter(repo)
			request := httptest.NewRequest(http.MethodPatch, "/api/user/profile", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
			}
			assertErrorCode(t, recorder.Body.Bytes(), "profile.validation_failed")
			if repo.lastProfileInput.WeightKG.Present {
				t.Fatalf("expected invalid weight not to reach repo, got %#v", repo.lastProfileInput)
			}
		})
	}
}

func TestCreateProfilePhotoCreatesArchivePhoto(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{
		photoResult: profile.ProfilePhoto{
			PublicID:      "pph_test",
			AssetPublicID: "ast_test",
			PhotoType:     "full_body",
			Angle:         "front",
			Note:          "自然站姿",
			SortOrder:     10,
			Image: &profile.ProfilePhotoImage{
				ObjectKey: "users/12/profile/ast_test.jpg",
				URL:       "https://private.example.test/users/12/profile/ast_test.jpg",
			},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPost, "/api/user/profile/photos", strings.NewReader(`{"asset_public_id":"ast_test","photo_type":"full_body","angle":"front","note":"自然站姿","sort_order":10}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.lastPhotoUserID != 12 {
		t.Fatalf("expected user 12, got %d", repo.lastPhotoUserID)
	}
	if repo.lastCreatePhotoInput.AssetPublicID != "ast_test" || repo.lastCreatePhotoInput.PhotoType != "full_body" || repo.lastCreatePhotoInput.Angle != "front" {
		t.Fatalf("expected captured create photo input, got %#v", repo.lastCreatePhotoInput)
	}
	var body struct {
		Data profile.ProfilePhoto `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.PublicID != "pph_test" || body.Data.Image == nil || body.Data.Image.URL == "" {
		t.Fatalf("expected profile photo response, got %#v", body.Data)
	}
}

func TestCreateProfilePhotoRejectsInvalidTypeAndAngle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
	}{
		{name: "type", body: `{"asset_public_id":"ast_test","photo_type":"wardrobe","angle":"front"}`},
		{name: "angle", body: `{"asset_public_id":"ast_test","photo_type":"full_body","angle":"diagonal"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &routeProfileRepo{}
			router := newProfileRouteTestRouter(repo)
			request := httptest.NewRequest(http.MethodPost, "/api/user/profile/photos", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
			}
			assertErrorCode(t, recorder.Body.Bytes(), "profile.validation_failed")
			if repo.lastCreatePhotoInput.AssetPublicID != "" {
				t.Fatalf("expected invalid photo not to reach repo, got %#v", repo.lastCreatePhotoInput)
			}
		})
	}
}

func TestUpdateProfilePhotoRejectsNullStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile/photos/pph_test", strings.NewReader(`{"status":null}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertErrorCode(t, recorder.Body.Bytes(), "profile.validation_failed")
	if repo.lastUpdatePhotoInput.Status.Present {
		t.Fatalf("expected invalid status not to reach repo, got %#v", repo.lastUpdatePhotoInput)
	}
}

func TestDeleteProfilePhotoSoftDeletesArchivePhoto(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodDelete, "/api/user/profile/photos/pph_test", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.lastDeletePhotoUserID != 12 || repo.lastDeletePhotoPublicID != "pph_test" {
		t.Fatalf("expected delete photo call, got user=%d public=%q", repo.lastDeletePhotoUserID, repo.lastDeletePhotoPublicID)
	}
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
	if repo.lastProfileInput.BodyNotes.Present {
		t.Fatalf("expected invalid input not to reach repo, got %#v", repo.lastProfileInput)
	}
}

func TestPatchProfileRejectsInvalidHeightRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
	}{
		{name: "negative", body: `{"height_cm":-1}`},
		{name: "too high", body: `{"height_cm":251}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &routeProfileRepo{}
			router := newProfileRouteTestRouter(repo)
			request := httptest.NewRequest(http.MethodPatch, "/api/user/profile", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
			}
			assertErrorCode(t, recorder.Body.Bytes(), "profile.validation_failed")
			if repo.lastProfileInput.HeightCM.Present {
				t.Fatalf("expected invalid height not to reach repo, got %#v", repo.lastProfileInput)
			}
		})
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

func TestPatchProfileReturnsNotFoundWhenUserMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{err: profile.ErrUserNotFound}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile", strings.NewReader(`{"nickname":"新昵称"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	assertErrorCode(t, recorder.Body.Bytes(), "profile.user_not_found")
}

func TestMySQLUpdateProfilePhotoRejectsFullGroupMove(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()
	repo := profile.NewMySQLRepository(sqlx.NewDb(db, "sqlmock"))
	userID := int64(12)

	mock.ExpectQuery(`(?s)SELECT photo_type FROM profile_photos.*WHERE user_id = \?.*public_id = \?`).
		WithArgs(userID, "pph_test").
		WillReturnRows(sqlmock.NewRows([]string{"photo_type"}).AddRow("headshot"))
	mock.ExpectQuery(`(?s)SELECT.*COUNT\(\*\).*FROM profile_photos.*WHERE user_id = \?`).
		WithArgs("full_body", userID).
		WillReturnRows(sqlmock.NewRows([]string{"total", "group_count"}).AddRow(6, 6))

	_, err = repo.UpdateProfilePhoto(context.Background(), userID, "pph_test", profile.UpdateProfilePhotoInput{
		PhotoType: profile.PatchString{Present: true, Value: "full_body"},
	})
	if !errors.Is(err, profile.ErrProfilePhotoLimit) {
		t.Fatalf("expected photo limit error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
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
	mock.ExpectQuery(`SELECT id FROM users WHERE id = \? AND deleted_at IS NULL FOR UPDATE`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	mock.ExpectExec(`UPDATE users SET nickname = NULLIF\(\?, ''\) WHERE id = \? AND deleted_at IS NULL`).
		WithArgs("明明", userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
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
		}).AddRow(int64(34), "prf_test", userID, "active", "female", 165, "希望通勤更利落", "中性偏暖", "锁骨发", ""))
	mock.ExpectExec(`(?s)UPDATE profiles.*gender = NULLIF\(\?, ''\).*height_cm = \?.*body_notes = NULLIF\(\?, ''\).*skin_notes = NULLIF\(\?, ''\).*hair_notes = NULLIF\(\?, ''\).*lifestyle_scenarios = CAST\(\? AS JSON\).*WHERE user_id = \?`).
		WithArgs("female", 165, "希望通勤更利落", "中性偏暖", "锁骨发", `["通勤"]`, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE profile_facts SET deleted_at = CURRENT_TIMESTAMP\(3\) WHERE user_id = \? AND profile_id = \? AND deleted_at IS NULL AND fact_key IN \(\?,\?,\?\)`).
		WithArgs(userID, int64(34), "gender", "height_cm", "lifestyle_scenarios").
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
		Nickname:           profile.PatchString{Present: true, Value: "明明"},
		Gender:             profile.PatchString{Present: true, Value: "female"},
		HeightCM:           profile.PatchInt{Present: true, Value: intPtr(165)},
		BodyNotes:          profile.PatchString{Present: true, Value: "希望通勤更利落"},
		SkinNotes:          profile.PatchString{Present: true, Value: "中性偏暖"},
		HairNotes:          profile.PatchString{Present: true, Value: "锁骨发"},
		LifestyleScenarios: profile.PatchStringSlice{Present: true, Value: []string{"通勤"}},
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

func TestMySQLUpdateExplicitProfileNicknameOnlyDoesNotReplaceFacts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()
	repo := profile.NewMySQLRepository(sqlx.NewDb(db, "sqlmock"))
	userID := int64(12)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM users WHERE id = \? AND deleted_at IS NULL FOR UPDATE`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	mock.ExpectExec(`UPDATE users SET nickname = NULLIF\(\?, ''\) WHERE id = \? AND deleted_at IS NULL`).
		WithArgs("新昵称", userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectSummaryQueries(mock, userID)

	_, err = repo.UpdateExplicitProfile(context.Background(), userID, profile.UpdateProfileInput{
		Nickname: profile.PatchString{Present: true, Value: "新昵称"},
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestMySQLUpdateExplicitProfileReturnsUserNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()
	repo := profile.NewMySQLRepository(sqlx.NewDb(db, "sqlmock"))
	userID := int64(12)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM users WHERE id = \? AND deleted_at IS NULL FOR UPDATE`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = repo.UpdateExplicitProfile(context.Background(), userID, profile.UpdateProfileInput{
		Nickname: profile.PatchString{Present: true, Value: "新昵称"},
	})
	if !errors.Is(err, profile.ErrUserNotFound) {
		t.Fatalf("expected user not found, got %v", err)
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
	mock.ExpectQuery(`SELECT id FROM users WHERE id = \? AND deleted_at IS NULL FOR UPDATE`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
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

func TestMySQLCreateProfilePhotoCreatesOwnedAssetReference(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	defer db.Close()
	repo := profile.NewMySQLRepository(sqlx.NewDb(db, "sqlmock"))
	userID := int64(12)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM users WHERE id = \? AND deleted_at IS NULL FOR UPDATE`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
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
			"weight_kg",
			"body_notes",
			"skin_notes",
			"hair_notes",
			"face_shape",
			"upper_body_notes",
			"lower_body_notes",
			"size_notes",
			"style_goal_summary",
		}).AddRow(int64(34), "prf_test", userID, "active", "", nil, nil, "", "", "", "", "", "", "", ""))
	mock.ExpectQuery(`(?s)SELECT.*COUNT\(\*\).*FROM profile_photos.*WHERE user_id = \?`).
		WithArgs("full_body", userID).
		WillReturnRows(sqlmock.NewRows([]string{"total", "group_count"}).AddRow(0, 0))
	mock.ExpectQuery(`(?s)SELECT public_id, object_key.*FROM files.*asset_type = 'profile_photo'`).
		WithArgs("ast_test", userID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "object_key"}).AddRow("ast_test", "users/12/profile/ast_test.jpg"))
	mock.ExpectExec(`INSERT INTO profile_photos`).
		WithArgs(sqlmock.AnyArg(), userID, int64(34), "ast_test", "full_body", "front", "自然站姿", 10, "active").
		WillReturnResult(sqlmock.NewResult(56, 1))
	mock.ExpectQuery(`(?s)FROM profile_photos pp.*WHERE pp\.user_id = \?.*pp\.public_id = \?`).
		WithArgs(userID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "asset_public_id", "photo_type", "angle", "note", "sort_order", "status", "object_key"}).
			AddRow("pph_test", "ast_test", "full_body", "front", "自然站姿", 10, "active", "users/12/profile/ast_test.jpg"))
	mock.ExpectCommit()

	photo, err := repo.CreateProfilePhoto(context.Background(), userID, profile.CreateProfilePhotoInput{
		AssetPublicID: "ast_test",
		PhotoType:     "full_body",
		Angle:         "front",
		Note:          "自然站姿",
		SortOrder:     10,
	})
	if err != nil {
		t.Fatalf("create profile photo: %v", err)
	}
	if photo.PublicID != "pph_test" || photo.Image == nil || photo.Image.ObjectKey != "users/12/profile/ast_test.jpg" {
		t.Fatalf("unexpected profile photo: %#v", photo)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func newProfileRouteTestRouter(repo *routeProfileRepo) *gin.Engine {
	return newProfileRouteTestRouterWithSigner(repo, nil)
}

func newProfileRouteTestRouterWithSigner(repo *routeProfileRepo, signer profile.ImageURLSigner) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	service := profile.NewService(repo)
	service.SetImageURLSigner(signer)
	profile.RegisterUserRoutesWithService(router.Group("/api/user/profile"), service, nil)
	return router
}

type routeProfileImageSigner struct {
	urls map[string]profile.SignedImageURLs
}

func (s *routeProfileImageSigner) PrivateImageURLs(_ context.Context, objectKeys []string) (map[string]profile.SignedImageURLs, error) {
	result := make(map[string]profile.SignedImageURLs, len(objectKeys))
	for _, key := range objectKeys {
		result[key] = s.urls[key]
	}
	return result, nil
}

type routeProfileRepo struct {
	summary                 profile.Summary
	err                     error
	lastProfileUserID       int64
	lastProfileInput        profile.UpdateProfileInput
	lastPreferencesUserID   int64
	lastPreferencesInput    profile.UpdatePreferencesInput
	photoResult             profile.ProfilePhoto
	lastPhotoUserID         int64
	lastCreatePhotoInput    profile.CreateProfilePhotoInput
	lastUpdatePhotoInput    profile.UpdateProfilePhotoInput
	lastDeletePhotoUserID   int64
	lastDeletePhotoPublicID string
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

func (r *routeProfileRepo) CreateProfilePhoto(_ context.Context, userID int64, input profile.CreateProfilePhotoInput) (profile.ProfilePhoto, error) {
	if r.err != nil {
		return profile.ProfilePhoto{}, r.err
	}
	r.lastPhotoUserID = userID
	r.lastCreatePhotoInput = input
	if r.photoResult.PublicID != "" {
		return r.photoResult, nil
	}
	return profile.ProfilePhoto{PublicID: "pph_test", AssetPublicID: input.AssetPublicID, PhotoType: input.PhotoType, Angle: input.Angle, Note: input.Note, SortOrder: input.SortOrder}, nil
}

func (r *routeProfileRepo) UpdateProfilePhoto(_ context.Context, userID int64, publicID string, input profile.UpdateProfilePhotoInput) (profile.ProfilePhoto, error) {
	if r.err != nil {
		return profile.ProfilePhoto{}, r.err
	}
	r.lastPhotoUserID = userID
	r.lastUpdatePhotoInput = input
	if r.photoResult.PublicID != "" {
		return r.photoResult, nil
	}
	return profile.ProfilePhoto{PublicID: publicID, PhotoType: input.PhotoType.Value, Angle: input.Angle.Value, Note: input.Note.Value, SortOrder: 0}, nil
}

func (r *routeProfileRepo) DeleteProfilePhoto(_ context.Context, userID int64, publicID string) error {
	if r.err != nil {
		return r.err
	}
	r.lastDeletePhotoUserID = userID
	r.lastDeletePhotoPublicID = publicID
	return nil
}

func intPtr(value int) *int {
	return &value
}

func floatPtr(value float64) *float64 {
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
	mock.ExpectQuery(`(?s)FROM profile_prefs.*WHERE user_id = \?.*deleted_at IS NULL`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"pref_type", "pref_key", "polarity"}).
			AddRow("style_goal", "更利落", "positive").
			AddRow("avoidance", "过甜", "negative").
			AddRow("scenario_preference", "通勤更正式", "positive"))
	mock.ExpectQuery(`(?s)FROM reports.*report_type = 'initial'.*status = 'ready'.*LIMIT 1`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "title", "status", "generated_at"}))
	expectProfilePhotoSummaryQuery(mock, userID)
}

func expectProfilePhotoSummaryQuery(mock sqlmock.Sqlmock, userID int64) {
	mock.ExpectQuery(`(?s)FROM profile_photos pp.*LEFT JOIN files f.*WHERE pp\.user_id = \?.*ORDER BY pp\.photo_type ASC`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"public_id", "asset_public_id", "photo_type", "angle", "note", "sort_order", "status", "object_key"}))
}
