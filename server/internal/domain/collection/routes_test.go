package collection_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/clothes"
	"hestia/server/internal/domain/collection"
	"hestia/server/internal/domain/hair"
	"hestia/server/internal/domain/makeup"
	"hestia/server/internal/infra/config"

	"github.com/gin-gonic/gin"
)

func TestGetCollectionSummaryUsesThreePrivateDomains(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	service := collection.NewServiceWithDomains(
		&fakeClothesLister{items: []clothes.Item{{
			PublicID:  "wdi_shirt",
			Name:      "米白衬衫",
			Category:  "上装",
			Color:     "米白",
			Status:    clothes.StatusActive,
			UpdatedAt: now.Add(-time.Hour),
			PrimaryImage: &clothes.Image{
				PreviewURL: "https://cdn.example.com/shirt.webp",
			},
		}}},
		&fakeHairLister{items: []hair.Item{{
			PublicID:  "hai_bob",
			Name:      "锁骨发",
			Length:    "中长",
			Status:    hair.StatusActive,
			UpdatedAt: now,
		}}},
		&fakeMakeupLister{items: []makeup.Item{{
			PublicID:   "mku_daily",
			Name:       "清透通勤妆",
			MakeupType: "日常",
			Status:     makeup.StatusActive,
			UpdatedAt:  now.Add(-2 * time.Hour),
		}}},
	)
	router := newCollectionRouter(service)

	request := httptest.NewRequest(http.MethodGet, "/api/user/collection", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string             `json:"code"`
		Data collection.Summary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := len(body.Data.Types); got != 3 {
		t.Fatalf("expected three collection types, got %d", got)
	}
	expectedTypes := []collection.TypeSummary{
		{Type: "clothes", Label: "衣服", Count: 1, EntryPath: "/pages/clothes/list"},
		{Type: "hair", Label: "发型", Count: 1, EntryPath: "/pages/hair/list"},
		{Type: "makeup", Label: "妆容", Count: 1, EntryPath: "/pages/makeup/list"},
	}
	for index, expected := range expectedTypes {
		got := body.Data.Types[index]
		if got.Type != expected.Type || got.Label != expected.Label || got.Count != expected.Count || got.EntryPath != expected.EntryPath {
			t.Fatalf("unexpected type summary at %d: %#v", index, got)
		}
	}
	if len(body.Data.RecentItems) != 3 {
		t.Fatalf("expected three recent items, got %#v", body.Data.RecentItems)
	}
	if body.Data.RecentItems[0].Type != "hair" || body.Data.RecentItems[0].EntryPath != "/pages/hair/detail?public_id=hai_bob" {
		t.Fatalf("expected hair to be newest recent item, got %#v", body.Data.RecentItems[0])
	}
	if body.Data.RecentItems[1].Type != "clothes" || body.Data.RecentItems[1].EntryPath != "/pages/clothes/detail?public_id=wdi_shirt" {
		t.Fatalf("expected clothes detail entry, got %#v", body.Data.RecentItems[1])
	}
	if body.Data.RecentItems[1].Image == nil || body.Data.RecentItems[1].Image.PreviewURL != "https://cdn.example.com/shirt.webp" {
		t.Fatalf("expected clothes image preview, got %#v", body.Data.RecentItems[1].Image)
	}
}

func TestRegisterUserRoutesSignsRecentClothesImageFromObjectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer sqlDB.Close()

	updatedAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM clothes wi")).
		WithArgs(int64(12), clothes.StatusDeleted).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"public_id",
			"user_id",
			"name",
			"category",
			"color",
			"silhouette",
			"material",
			"season",
			"scene_tags",
			"user_notes",
			"is_core",
			"recommendation_status",
			"recognition_status",
			"status",
			"created_at",
			"updated_at",
			"primary_asset_relation_id",
			"primary_asset_sort_order",
			"primary_asset_public_id",
			"primary_object_key",
		}).AddRow(
			int64(1),
			"wdi_shirt",
			int64(12),
			"米白衬衫",
			"上装",
			"米白",
			sql.NullString{},
			sql.NullString{},
			sql.NullString{},
			[]byte(`[]`),
			sql.NullString{},
			true,
			clothes.RecommendationStatusNormal,
			clothes.RecognitionStatusSucceeded,
			clothes.StatusActive,
			updatedAt,
			updatedAt,
			sql.NullInt64{Int64: 10, Valid: true},
			sql.NullInt64{Int64: 0, Valid: true},
			sql.NullString{String: "ast_shirt", Valid: true},
			sql.NullString{String: "users/12/clothes/ast_shirt.jpg", Valid: true},
		))
	expectEmptyHairList(mock)
	expectEmptyMakeupList(mock)

	router := newCollectionRouterWithDeps(&baseapp.Deps{
		DB: sqlx.NewDb(sqlDB, "sqlmock"),
		Config: &config.Config{
			QiniuPrivateDomain:         "private.example.test",
			QiniuAccessKey:             "test-ak",
			QiniuSecretKey:             "test-sk",
			QiniuDownloadURLTTLSeconds: 900,
			QiniuUploadTokenTTLSeconds: 3600,
		},
	})
	request := httptest.NewRequest(http.MethodGet, "/api/user/collection", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string             `json:"code"`
		Data collection.Summary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.RecentItems) != 1 {
		t.Fatalf("expected one recent clothes item, got %#v", body.Data.RecentItems)
	}
	recent := body.Data.RecentItems[0]
	if recent.Image == nil || recent.Image.PreviewURL == "" {
		t.Fatalf("expected recent image preview signed from object key, got %#v", recent.Image)
	}
	if !regexp.MustCompile(`^https://private\.example\.test/users/12/clothes/ast_shirt\.jpg\?`).MatchString(recent.Image.PreviewURL) {
		t.Fatalf("expected private preview url, got %q", recent.Image.PreviewURL)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func expectEmptyHairList(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta("FROM hair h")).
		WithArgs(int64(12), hair.StatusDeleted).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "user_id", "name", "length", "shape", "bangs", "color", "care_time",
			"suitability_notes", "avoidance_notes", "user_notes", "recommendation_status", "status", "created_at", "updated_at",
			"primary_asset_public_id", "primary_object_key", "scene_tag", "scene_tag_sort_order",
		}))
}

func expectEmptyMakeupList(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(regexp.QuoteMeta("FROM makeup h")).
		WithArgs(int64(12), makeup.StatusDeleted).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "user_id", "name", "makeup_type", "focus", "color_palette", "finish",
			"suitability_notes", "avoidance_notes", "user_notes", "recommendation_status", "status", "created_at", "updated_at",
			"primary_asset_public_id", "primary_object_key", "scene_tag", "scene_tag_sort_order",
		}))
}

func newCollectionRouter(service *collection.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	collection.RegisterUserRoutesWithService(router.Group("/api/user/collection"), service)
	return router
}

func newCollectionRouterWithDeps(deps *baseapp.Deps) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	collection.RegisterUserRoutes(router.Group("/api/user/collection"), deps)
	return router
}

type fakeClothesLister struct {
	items []clothes.Item
}

func (f *fakeClothesLister) ListItems(_ context.Context, _ int64, _ clothes.ListFilter) ([]clothes.Item, error) {
	return f.items, nil
}

type fakeHairLister struct {
	items []hair.Item
}

func (f *fakeHairLister) ListItems(_ context.Context, _ int64, _ hair.ListFilter) ([]hair.Item, error) {
	return f.items, nil
}

type fakeMakeupLister struct {
	items []makeup.Item
}

func (f *fakeMakeupLister) ListItems(_ context.Context, _ int64, _ makeup.ListFilter) ([]makeup.Item, error) {
	return f.items, nil
}
