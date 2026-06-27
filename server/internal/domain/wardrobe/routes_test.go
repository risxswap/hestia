package wardrobe_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/wardrobe"

	"github.com/gin-gonic/gin"
)

func TestListItemsReturnsOnlyCurrentUserItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{PublicID: "wdi_owned", UserID: 12, Name: "米白衬衫", Category: "top", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
	repo.add(wardrobe.Item{PublicID: "wdi_other", UserID: 99, Name: "黑色西装", Category: "outerwear", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
	router := newWardrobeRouteTestRouter(repo)

	request := httptest.NewRequest(http.MethodGet, "/api/user/wardrobe/items", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
		Data struct {
			Items []wardrobe.Item `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected code ok, got %q", body.Code)
	}
	if len(body.Data.Items) != 1 {
		t.Fatalf("expected one current user item, got %#v", body.Data.Items)
	}
	if body.Data.Items[0].PublicID != "wdi_owned" {
		t.Fatalf("expected owned item only, got %#v", body.Data.Items)
	}
}

func TestCreateItemRejectsInvalidRecommendationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newWardrobeRouteTestRouter(newRouteMemoryWardrobeRepo())

	body := routeWardrobeErrorResponse(t, router, http.MethodPost, "/api/user/wardrobe/items", `{"name":"黑色西装","category":"outerwear","recommendation_status":"hidden"}`, http.StatusBadRequest)

	if body.Code != "wardrobe.invalid_recommendation_status" {
		t.Fatalf("expected invalid recommendation status code, got %q", body.Code)
	}
}

func TestCreateItemRejectsEmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newWardrobeRouteTestRouter(newRouteMemoryWardrobeRepo())

	body := routeWardrobeErrorResponse(t, router, http.MethodPost, "/api/user/wardrobe/items", `{"name":"   ","category":"top"}`, http.StatusBadRequest)

	if body.Code != "wardrobe.invalid_item" {
		t.Fatalf("expected invalid item code, got %q", body.Code)
	}
}

func TestPatchItemUpdatesRecommendationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{PublicID: "wdi_owned", UserID: 12, Name: "米白衬衫", Category: "top", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
	router := newWardrobeRouteTestRouter(repo)

	request := httptest.NewRequest(http.MethodPatch, "/api/user/wardrobe/items/wdi_owned", bytes.NewBufferString(`{"recommendation_status":"paused"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string        `json:"code"`
		Data wardrobe.Item `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected code ok, got %q", body.Code)
	}
	if body.Data.RecommendationStatus != wardrobe.RecommendationStatusPaused {
		t.Fatalf("expected paused recommendation status, got %#v", body.Data)
	}
}

func TestDeleteItemSoftDeletesCurrentUserItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{PublicID: "wdi_owned", UserID: 12, Name: "米白衬衫", Category: "top", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
	router := newWardrobeRouteTestRouter(repo)

	request := httptest.NewRequest(http.MethodDelete, "/api/user/wardrobe/items/wdi_owned", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
		Data struct {
			PublicID string `json:"public_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected code ok, got %q", body.Code)
	}
	if body.Data.PublicID != "wdi_owned" {
		t.Fatalf("expected deleted public id, got %q", body.Data.PublicID)
	}
	if repo.items["wdi_owned"].Status != wardrobe.StatusDeleted {
		t.Fatalf("expected repo item soft deleted, got %#v", repo.items["wdi_owned"])
	}
}

func TestDeleteItemReturnsNotFoundForOtherUserItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{PublicID: "wdi_other", UserID: 99, Name: "黑色西装", Category: "outerwear", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
	router := newWardrobeRouteTestRouter(repo)

	body := routeWardrobeErrorResponse(t, router, http.MethodDelete, "/api/user/wardrobe/items/wdi_other", "", http.StatusNotFound)

	if body.Code != "wardrobe.item_not_found" {
		t.Fatalf("expected item not found code, got %q", body.Code)
	}
}

func newWardrobeRouteTestRouter(repo *routeMemoryWardrobeRepo) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{
			UserID:       12,
			UserPublicID: "usr_test",
			Surface:      "user",
		})
		c.Next()
	})
	wardrobe.RegisterUserRoutesWithService(router.Group("/api/user/wardrobe"), wardrobe.NewService(repo), nil)
	return router
}

func routeWardrobeErrorResponse(t *testing.T, router *gin.Engine, method string, target string, requestBody string, expectedStatus int) struct {
	Code string `json:"code"`
} {
	t.Helper()
	var bodyReader *bytes.Buffer
	if requestBody == "" {
		bodyReader = bytes.NewBuffer(nil)
	} else {
		bodyReader = bytes.NewBufferString(requestBody)
	}
	request := httptest.NewRequest(method, target, bodyReader)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d, body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}

type routeMemoryWardrobeRepo struct {
	items map[string]wardrobe.Item
}

func newRouteMemoryWardrobeRepo() *routeMemoryWardrobeRepo {
	return &routeMemoryWardrobeRepo{items: map[string]wardrobe.Item{}}
}

func (r *routeMemoryWardrobeRepo) add(item wardrobe.Item) {
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}
	r.items[item.PublicID] = item
}

func (r *routeMemoryWardrobeRepo) CreateCoreItems(_ context.Context, items []wardrobe.Item) ([]wardrobe.Item, error) {
	for _, item := range items {
		r.add(item)
	}
	return items, nil
}

func (r *routeMemoryWardrobeRepo) ListItems(_ context.Context, userID int64, filter wardrobe.ListFilter) ([]wardrobe.Item, error) {
	result := make([]wardrobe.Item, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID != userID || item.Status == wardrobe.StatusDeleted {
			continue
		}
		if filter.Category != "" && item.Category != filter.Category {
			continue
		}
		if filter.RecommendationStatus != "" && item.RecommendationStatus != filter.RecommendationStatus {
			continue
		}
		if filter.IsCore != nil && item.IsCore != *filter.IsCore {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *routeMemoryWardrobeRepo) CreateItem(_ context.Context, item wardrobe.Item, _ string) (wardrobe.Item, error) {
	r.add(item)
	return item, nil
}

func (r *routeMemoryWardrobeRepo) UpdateItem(_ context.Context, userID int64, publicID string, input wardrobe.UpdateInput) (wardrobe.Item, error) {
	item, ok := r.items[publicID]
	if !ok || item.UserID != userID || item.Status == wardrobe.StatusDeleted {
		return wardrobe.Item{}, wardrobe.ErrItemNotFound
	}
	if input.Name != nil {
		item.Name = *input.Name
	}
	if input.Category != nil {
		item.Category = *input.Category
	}
	if input.Color != nil {
		item.Color = *input.Color
	}
	if input.Silhouette != nil {
		item.Silhouette = *input.Silhouette
	}
	if input.Material != nil {
		item.Material = *input.Material
	}
	if input.Season != nil {
		item.Season = *input.Season
	}
	if input.SceneTags != nil {
		item.SceneTags = *input.SceneTags
	}
	if input.UserNotes != nil {
		item.UserNotes = *input.UserNotes
	}
	if input.IsCore != nil {
		item.IsCore = *input.IsCore
	}
	if input.RecommendationStatus != nil {
		item.RecommendationStatus = *input.RecommendationStatus
	}
	item.UpdatedAt = time.Now().UTC()
	r.items[publicID] = item
	return item, nil
}

func (r *routeMemoryWardrobeRepo) SoftDeleteItem(_ context.Context, userID int64, publicID string) error {
	item, ok := r.items[publicID]
	if !ok || item.UserID != userID {
		return wardrobe.ErrItemNotFound
	}
	item.Status = wardrobe.StatusDeleted
	r.items[publicID] = item
	return nil
}
