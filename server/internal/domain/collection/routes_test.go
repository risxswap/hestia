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
	"hestia/server/internal/domain/collection"
	"hestia/server/internal/domain/wardrobe"
	"hestia/server/internal/infra/config"

	"github.com/gin-gonic/gin"
)

func TestGetCollectionSummaryUsesTopLevelResources(t *testing.T) {
	gin.SetMode(gin.TestMode)
	updatedAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	service := collection.NewService(&fakeWardrobeLister{
		items: []wardrobe.Item{
			{
				PublicID:  "wdi_shirt",
				Name:      "米白衬衫",
				Category:  "上装",
				Color:     "米白",
				Status:    wardrobe.StatusActive,
				UpdatedAt: updatedAt,
				PrimaryImage: &wardrobe.Image{
					PreviewURL: "https://cdn.example.com/shirt.webp",
				},
			},
		},
	})
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
	if body.Code != "ok" {
		t.Fatalf("expected ok response, got %q", body.Code)
	}
	if got := len(body.Data.Types); got != 4 {
		t.Fatalf("expected four collection types, got %d", got)
	}
	if body.Data.Types[0].Type != "wardrobe" || body.Data.Types[0].Label != "衣橱" || body.Data.Types[0].EntryPath != "/pages/wardrobe/wardrobe" {
		t.Fatalf("expected wardrobe top-level entry, got %#v", body.Data.Types[0])
	}
	if body.Data.Types[1].Type != "hair" || body.Data.Types[1].EntryPath != "/pages/hair/hair" {
		t.Fatalf("expected hair top-level entry, got %#v", body.Data.Types[1])
	}
	if body.Data.Types[2].Type != "makeup" || body.Data.Types[2].EntryPath != "/pages/makeup/makeup" {
		t.Fatalf("expected makeup top-level entry, got %#v", body.Data.Types[2])
	}
	if body.Data.Types[3].Type != "references" || body.Data.Types[3].EntryPath != "/pages/references/references" {
		t.Fatalf("expected references top-level entry, got %#v", body.Data.Types[3])
	}
	if body.Data.Types[0].Count != 1 {
		t.Fatalf("expected wardrobe count from wardrobe service, got %#v", body.Data.Types[0])
	}
	if len(body.Data.RecentItems) != 1 {
		t.Fatalf("expected one recent wardrobe item, got %#v", body.Data.RecentItems)
	}
	recent := body.Data.RecentItems[0]
	if recent.Type != "wardrobe" || recent.EntryPath != "/pages/wardrobe-detail/wardrobe-detail?public_id=wdi_shirt" {
		t.Fatalf("expected recent wardrobe item detail entry, got %#v", recent)
	}
	if recent.Image == nil || recent.Image.PreviewURL != "https://cdn.example.com/shirt.webp" {
		t.Fatalf("expected recent image preview, got %#v", recent.Image)
	}
}

func TestRegisterUserRoutesSignsRecentWardrobeImageFromObjectKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new sqlmock: %v", err)
	}
	defer sqlDB.Close()

	updatedAt := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM wardrobe_items wi")).
		WithArgs(int64(12), wardrobe.StatusDeleted).
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
			wardrobe.RecommendationStatusNormal,
			wardrobe.RecognitionStatusSucceeded,
			wardrobe.StatusActive,
			updatedAt,
			updatedAt,
			sql.NullInt64{Int64: 10, Valid: true},
			sql.NullInt64{Int64: 0, Valid: true},
			sql.NullString{String: "ast_shirt", Valid: true},
			sql.NullString{String: "users/12/wardrobe/ast_shirt.jpg", Valid: true},
		))

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
		t.Fatalf("expected one recent wardrobe item, got %#v", body.Data.RecentItems)
	}
	recent := body.Data.RecentItems[0]
	if recent.Image == nil || recent.Image.PreviewURL == "" {
		t.Fatalf("expected recent image preview signed from object key, got %#v", recent.Image)
	}
	if !regexp.MustCompile(`^https://private\.example\.test/users/12/wardrobe/ast_shirt\.jpg\?`).MatchString(recent.Image.PreviewURL) {
		t.Fatalf("expected private preview url, got %q", recent.Image.PreviewURL)
	}
	if !regexp.MustCompile(`(^|[?&])imageView2/2/w/360/h/360/q/80/format/webp(&|$)`).MatchString(recent.Image.PreviewURL) {
		t.Fatalf("expected preview transform query, got %q", recent.Image.PreviewURL)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
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

type fakeWardrobeLister struct {
	items []wardrobe.Item
}

func (f *fakeWardrobeLister) ListItems(_ context.Context, _ int64, _ wardrobe.ListFilter) ([]wardrobe.Item, error) {
	return f.items, nil
}
