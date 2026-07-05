package hair_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/hair"

	"github.com/gin-gonic/gin"
)

func TestHairCRUDRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRepo()
	router := newRouter(hair.NewService(repo))

	create := httptest.NewRequest(http.MethodPost, "/api/user/hair", bytes.NewBufferString(`{"name":"锁骨发","length":"中长","scene_tags":["通勤","约会"],"recommendation_status":"preferred"}`))
	create.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, create)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("expected create 200, got %d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	created := decodeData[hair.Item](t, createRecorder)
	if created.PublicID == "" || created.Name != "锁骨发" || created.SceneTags[0] != "通勤" {
		t.Fatalf("unexpected created item: %#v", created)
	}

	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/api/user/hair/"+created.PublicID, nil))
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	got := decodeData[hair.Item](t, getRecorder)
	if got.PublicID != created.PublicID {
		t.Fatalf("expected created item, got %#v", got)
	}

	update := httptest.NewRequest(http.MethodPatch, "/api/user/hair/"+created.PublicID, bytes.NewBufferString(`{"bangs":"空气刘海","scene_tags":["周末"]}`))
	update.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, update)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updated := decodeData[hair.Item](t, updateRecorder)
	if updated.Bangs != "空气刘海" || len(updated.SceneTags) != 1 || updated.SceneTags[0] != "周末" {
		t.Fatalf("unexpected updated item: %#v", updated)
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/user/hair", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	list := decodeData[struct {
		Items []hair.Item `json:"items"`
	}](t, listRecorder)
	if len(list.Items) != 1 {
		t.Fatalf("expected one list item, got %#v", list)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, httptest.NewRequest(http.MethodDelete, "/api/user/hair/"+created.PublicID, nil))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}

func TestHairRejectsInvalidRecommendationStatus(t *testing.T) {
	router := newRouter(hair.NewService(newMemoryRepo()))
	request := httptest.NewRequest(http.MethodPost, "/api/user/hair", bytes.NewBufferString(`{"name":"锁骨发","recommendation_status":"hidden"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func newRouter(service *hair.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, Surface: "user"})
		c.Next()
	})
	hair.RegisterUserRoutesWithService(router.Group("/api/user/hair"), service, nil)
	return router
}

func decodeData[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var body struct {
		Code string `json:"code"`
		Data T      `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected ok response, got %#v", body)
	}
	return body.Data
}

type memoryRepo struct {
	nextID int64
	items  map[string]hair.Item
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{nextID: 1, items: map[string]hair.Item{}}
}

func (r *memoryRepo) ListItems(_ context.Context, userID int64, _ hair.ListFilter) ([]hair.Item, error) {
	items := []hair.Item{}
	for _, item := range r.items {
		if item.UserID == userID && item.Status != hair.StatusDeleted {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *memoryRepo) FindItemForUser(_ context.Context, userID int64, publicID string) (hair.Item, error) {
	item, ok := r.items[publicID]
	if !ok || item.UserID != userID || item.Status == hair.StatusDeleted {
		return hair.Item{}, hair.ErrItemNotFound
	}
	return item, nil
}

func (r *memoryRepo) CreateItem(_ context.Context, item hair.Item, _ string) (hair.Item, error) {
	item.ID = r.nextID
	r.nextID++
	r.items[item.PublicID] = item
	return item, nil
}

func (r *memoryRepo) UpdateItem(_ context.Context, userID int64, publicID string, input hair.UpdateInput) (hair.Item, error) {
	item, err := r.FindItemForUser(context.Background(), userID, publicID)
	if err != nil {
		return hair.Item{}, err
	}
	if input.Name != nil {
		item.Name = *input.Name
	}
	if input.Length != nil {
		item.Length = *input.Length
	}
	if input.Bangs != nil {
		item.Bangs = *input.Bangs
	}
	if input.SceneTags != nil {
		item.SceneTags = *input.SceneTags
	}
	r.items[publicID] = item
	return item, nil
}

func (r *memoryRepo) SoftDeleteItem(_ context.Context, userID int64, publicID string) error {
	item, err := r.FindItemForUser(context.Background(), userID, publicID)
	if err != nil {
		return err
	}
	item.Status = hair.StatusDeleted
	r.items[publicID] = item
	return nil
}
