package agent_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/agent"
	"hestia/server/internal/domain/clothes"

	"github.com/gin-gonic/gin"
)

func TestStreamReturnsSSEEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	agent.RegisterUserRoutes(router.Group("/api/user/agent"), &baseapp.Deps{})
	request := httptest.NewRequest(http.MethodPost, "/api/user/agent/stream", strings.NewReader(`{"text":"今天怎么穿"}`))
	request.Header.Set("Accept", "text/event-stream")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: status\n") {
		t.Fatalf("expected status event, got %q", body)
	}
	if !strings.Contains(body, "event: done\n") {
		t.Fatalf("expected done event, got %q", body)
	}
}

func TestStreamStatusIncludesActiveClothesAdviceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	repo := &routeClothesRepo{items: []clothes.Item{
		{PublicID: "wdi_shirt", UserID: 12, Name: "米白衬衫", Category: "top", IsCore: true, RecommendationStatus: clothes.RecommendationStatusPreferred, Status: clothes.StatusActive},
		{PublicID: "wdi_paused", UserID: 12, Name: "黑色长裙", Category: "bottom", IsCore: true, RecommendationStatus: clothes.RecommendationStatusPaused, Status: clothes.StatusActive},
		{PublicID: "wdi_deleted", UserID: 12, Name: "灰色外套", Category: "outerwear", IsCore: true, RecommendationStatus: clothes.RecommendationStatusPreferred, Status: clothes.StatusDeleted},
		{PublicID: "wdi_other", UserID: 99, Name: "其他用户西装", Category: "outerwear", IsCore: true, RecommendationStatus: clothes.RecommendationStatusPreferred, Status: clothes.StatusActive},
	}}
	service := agent.NewServiceWithClothes(clothes.NewService(repo))
	agent.RegisterUserRoutesWithService(router.Group("/api/user/agent"), service)
	request := httptest.NewRequest(http.MethodPost, "/api/user/agent/stream", strings.NewReader(`{"text":"今天怎么穿"}`))
	request.Header.Set("Accept", "text/event-stream")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "米白衬衫") {
		t.Fatalf("expected status to include active clothes item name, got %q", body)
	}
	for _, excluded := range []string{"黑色长裙", "灰色外套", "其他用户西装"} {
		if strings.Contains(body, excluded) {
			t.Fatalf("expected status to exclude %s, got %q", excluded, body)
		}
	}
}

type routeClothesRepo struct {
	items []clothes.Item
}

func (r *routeClothesRepo) CreateCoreItems(_ context.Context, items []clothes.Item) ([]clothes.Item, error) {
	return items, nil
}

func (r *routeClothesRepo) ListItems(_ context.Context, userID int64, _ clothes.ListFilter) ([]clothes.Item, error) {
	result := make([]clothes.Item, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID == userID && item.Status != clothes.StatusDeleted {
			result = append(result, item)
		}
	}
	return result, nil
}

func (r *routeClothesRepo) FindItemForUser(_ context.Context, userID int64, publicID string) (clothes.Item, error) {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != clothes.StatusDeleted {
			return item, nil
		}
	}
	return clothes.Item{}, clothes.ErrItemNotFound
}

func (r *routeClothesRepo) CreateItem(_ context.Context, item clothes.Item, _ string) (clothes.Item, error) {
	return item, nil
}

func (r *routeClothesRepo) UpdateItem(_ context.Context, _ int64, _ string, _ clothes.UpdateInput) (clothes.Item, error) {
	return clothes.Item{}, clothes.ErrItemNotFound
}

func (r *routeClothesRepo) SoftDeleteItem(_ context.Context, _ int64, _ string) error {
	return clothes.ErrItemNotFound
}
