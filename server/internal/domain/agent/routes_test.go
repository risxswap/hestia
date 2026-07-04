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
	"hestia/server/internal/domain/wardrobe"

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

func TestStreamStatusIncludesActiveWardrobeAdviceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	repo := &routeWardrobeRepo{items: []wardrobe.Item{
		{PublicID: "wdi_shirt", UserID: 12, Name: "米白衬衫", Category: "top", IsCore: true, RecommendationStatus: wardrobe.RecommendationStatusPreferred, Status: wardrobe.StatusActive},
		{PublicID: "wdi_paused", UserID: 12, Name: "黑色长裙", Category: "bottom", IsCore: true, RecommendationStatus: wardrobe.RecommendationStatusPaused, Status: wardrobe.StatusActive},
		{PublicID: "wdi_deleted", UserID: 12, Name: "灰色外套", Category: "outerwear", IsCore: true, RecommendationStatus: wardrobe.RecommendationStatusPreferred, Status: wardrobe.StatusDeleted},
		{PublicID: "wdi_other", UserID: 99, Name: "其他用户西装", Category: "outerwear", IsCore: true, RecommendationStatus: wardrobe.RecommendationStatusPreferred, Status: wardrobe.StatusActive},
	}}
	service := agent.NewServiceWithWardrobe(wardrobe.NewService(repo))
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
		t.Fatalf("expected status to include active wardrobe item name, got %q", body)
	}
	for _, excluded := range []string{"黑色长裙", "灰色外套", "其他用户西装"} {
		if strings.Contains(body, excluded) {
			t.Fatalf("expected status to exclude %s, got %q", excluded, body)
		}
	}
}

type routeWardrobeRepo struct {
	items []wardrobe.Item
}

func (r *routeWardrobeRepo) CreateCoreItems(_ context.Context, items []wardrobe.Item) ([]wardrobe.Item, error) {
	return items, nil
}

func (r *routeWardrobeRepo) ListItems(_ context.Context, userID int64, _ wardrobe.ListFilter) ([]wardrobe.Item, error) {
	result := make([]wardrobe.Item, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID == userID && item.Status != wardrobe.StatusDeleted {
			result = append(result, item)
		}
	}
	return result, nil
}

func (r *routeWardrobeRepo) FindItemForUser(_ context.Context, userID int64, publicID string) (wardrobe.Item, error) {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != wardrobe.StatusDeleted {
			return item, nil
		}
	}
	return wardrobe.Item{}, wardrobe.ErrItemNotFound
}

func (r *routeWardrobeRepo) CreateItem(_ context.Context, item wardrobe.Item, _ string) (wardrobe.Item, error) {
	return item, nil
}

func (r *routeWardrobeRepo) UpdateItem(_ context.Context, _ int64, _ string, _ wardrobe.UpdateInput) (wardrobe.Item, error) {
	return wardrobe.Item{}, wardrobe.ErrItemNotFound
}

func (r *routeWardrobeRepo) SoftDeleteItem(_ context.Context, _ int64, _ string) error {
	return wardrobe.ErrItemNotFound
}
