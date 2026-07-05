package memory_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/memory"

	"github.com/gin-gonic/gin"
)

func TestMemoryCRUDRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRepo()
	repo.items["mem_style"] = memory.Item{
		ID:          1,
		PublicID:    "mem_style",
		UserID:      12,
		MemoryType:  memory.TypePreference,
		MemoryKey:   "style_goal",
		MemoryValue: "通勤更利落",
		Polarity:    memory.PolarityPositive,
		Visibility:  memory.VisibilityVisible,
		Status:      memory.StatusActive,
		UpdatedAt:   time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC),
	}
	router := newRouter(memory.NewService(repo))

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/user/memories", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	list := decodeData[struct {
		Items []memory.Item `json:"items"`
	}](t, listRecorder)
	if len(list.Items) != 1 || list.Items[0].DisplayText != "通勤更利落" || list.Items[0].TypeLabel != "偏好" {
		t.Fatalf("unexpected list: %#v", list)
	}

	update := httptest.NewRequest(http.MethodPatch, "/api/user/memories/mem_style", bytes.NewBufferString(`{"memory_value":"通勤想要更清爽利落","correction_note":"用户手动修正"}`))
	update.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, update)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update 200, got %d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updated := decodeData[memory.Item](t, updateRecorder)
	if updated.MemoryValue != "通勤想要更清爽利落" || updated.DisplayText != "通勤想要更清爽利落" || updated.CorrectionNote != "用户手动修正" || updated.UserCorrectedAt == nil {
		t.Fatalf("unexpected updated item: %#v", updated)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, httptest.NewRequest(http.MethodDelete, "/api/user/memories/mem_style", nil))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("expected delete 200, got %d body=%s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	if repo.items["mem_style"].Status != memory.StatusDeleted {
		t.Fatalf("expected item soft-deleted, got %#v", repo.items["mem_style"])
	}
}

func TestMemoryUpdateRejectsEmptyValue(t *testing.T) {
	router := newRouter(memory.NewService(newMemoryRepo()))
	request := httptest.NewRequest(http.MethodPatch, "/api/user/memories/mem_missing", bytes.NewBufferString(`{"memory_value":"   "}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func newRouter(service *memory.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, Surface: "user"})
		c.Next()
	})
	memory.RegisterUserRoutesWithService(router.Group("/api/user/memories"), service, nil)
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
	items map[string]memory.Item
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{items: map[string]memory.Item{}}
}

func (r *memoryRepo) ListItems(_ context.Context, userID int64) ([]memory.Item, error) {
	items := []memory.Item{}
	for _, item := range r.items {
		if item.UserID == userID && item.Status != memory.StatusDeleted {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *memoryRepo) UpdateItem(_ context.Context, userID int64, publicID string, input memory.UpdateInput) (memory.Item, error) {
	item, ok := r.items[publicID]
	if !ok || item.UserID != userID || item.Status == memory.StatusDeleted {
		return memory.Item{}, memory.ErrItemNotFound
	}
	item.MemoryValue = input.MemoryValue
	item.CorrectionNote = input.CorrectionNote
	now := time.Now().UTC()
	item.UserCorrectedAt = &now
	item.UpdatedAt = now
	r.items[publicID] = item
	return item, nil
}

func (r *memoryRepo) SoftDeleteItem(_ context.Context, userID int64, publicID string) error {
	item, ok := r.items[publicID]
	if !ok || item.UserID != userID || item.Status == memory.StatusDeleted {
		return memory.ErrItemNotFound
	}
	item.Status = memory.StatusDeleted
	now := time.Now().UTC()
	item.UpdatedAt = now
	r.items[publicID] = item
	return nil
}
