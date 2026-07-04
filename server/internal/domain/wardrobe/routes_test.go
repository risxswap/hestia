package wardrobe_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/wardrobe"

	"github.com/gin-gonic/gin"
)

func TestListItemsReturnsOnlyCurrentUserItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{PublicID: "wdi_owned", UserID: 12, Name: "米白衬衫", Category: "上装", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
	repo.add(wardrobe.Item{PublicID: "wdi_other", UserID: 99, Name: "黑色西装", Category: "外套", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
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

func TestListItemsInjectsPrimaryImagePreviewAndOriginalURLsInBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	newerUpdatedAt := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	olderUpdatedAt := newerUpdatedAt.Add(-time.Minute)
	repo.add(wardrobe.Item{
		PublicID:             "wdi_owned",
		UserID:               12,
		Name:                 "米白衬衫",
		Category:             "上装",
		RecommendationStatus: wardrobe.RecommendationStatusNormal,
		Status:               wardrobe.StatusActive,
		IsCore:               true,
		UpdatedAt:            newerUpdatedAt,
		PrimaryImage: &wardrobe.Image{
			AssetPublicID: "ast_owned",
			ObjectKey:     "users/12/wardrobe/ast_owned.jpg",
		},
	})
	repo.add(wardrobe.Item{
		PublicID:             "wdi_owned_two",
		UserID:               12,
		Name:                 "黑色西装",
		Category:             "外套",
		RecommendationStatus: wardrobe.RecommendationStatusNormal,
		Status:               wardrobe.StatusActive,
		IsCore:               true,
		UpdatedAt:            olderUpdatedAt,
		PrimaryImage: &wardrobe.Image{
			AssetPublicID: "ast_owned_two",
			ObjectKey:     "users/12/wardrobe/ast_owned_two.jpg",
		},
	})
	service := wardrobe.NewService(repo)
	signer := &routeBatchImageURLSigner{}
	service.SetImageURLSigner(signer)
	router := newWardrobeRouteTestRouterWithService(service)

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
	if len(body.Data.Items) != 2 {
		t.Fatalf("expected one item, got %#v", body.Data.Items)
	}
	if body.Data.Items[0].PrimaryImage == nil {
		t.Fatalf("expected primary image, got nil")
	}
	if signer.batchCalls != 1 || signer.singleCalls != 0 {
		t.Fatalf("expected one batch signing call and no single calls, got batch=%d single=%d", signer.batchCalls, signer.singleCalls)
	}
	expectedOriginalURL := "https://private.example.test/users/12/wardrobe/ast_owned.jpg?token=original"
	expectedPreviewURL := "https://private.example.test/users/12/wardrobe/ast_owned.jpg?imageView2/2/w/360/h/360/q/80/format/webp&token=preview"
	if body.Data.Items[0].PrimaryImage.OriginalURL != expectedOriginalURL ||
		body.Data.Items[0].PrimaryImage.PreviewURL != expectedPreviewURL {
		t.Fatalf("expected primary image URLs, got %#v", body.Data.Items[0].PrimaryImage)
	}
	var raw map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	data := raw["data"].(map[string]any)
	items := data["items"].([]any)
	primaryImage := items[0].(map[string]any)["primary_image"].(map[string]any)
	if _, ok := primaryImage["url"]; ok {
		t.Fatalf("primary image must not expose legacy url field: %#v", primaryImage)
	}
}

func TestListItemsReturnsRequestFailedWhenRepositoryUnsupported(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newWardrobeRouteTestRouterWithService(wardrobe.NewService(routeCoreOnlyWardrobeRepo{}))

	body := routeWardrobeErrorResponse(t, router, http.MethodGet, "/api/user/wardrobe/items", "", http.StatusInternalServerError)

	if body.Code != "wardrobe.request_failed" {
		t.Fatalf("expected request failed code, got %q", body.Code)
	}
}

func TestGetWardrobeOptionsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.options = wardrobe.WardrobeOptions{
		Categories:  []string{"上装"},
		Colors:      []string{"雾霾蓝"},
		Materials:   []string{"棉"},
		Seasons:     []string{"春秋"},
		Silhouettes: []string{"微宽松"},
	}
	router := newWardrobeRouteTestRouter(repo)

	request := httptest.NewRequest(http.MethodGet, "/api/user/wardrobe/options", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string                   `json:"code"`
		Data wardrobe.WardrobeOptions `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" || body.Data.Categories[0] != "上装" || body.Data.Colors[0] != "雾霾蓝" || body.Data.Materials[0] != "棉" {
		t.Fatalf("expected options response, got %#v", body)
	}
}

func TestCreateItemAllowsCustomSuggestedFieldsRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.options = wardrobe.WardrobeOptions{
		Categories: []string{"上装"},
	}
	router := newWardrobeRouteTestRouter(repo)

	request := httptest.NewRequest(http.MethodPost, "/api/user/wardrobe/items", bytes.NewBufferString(`{"name":"黑色西装","category":"定制分类","color":"雾霾蓝","material":"丝绒","season":"梅雨季","silhouette":"茧型"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data wardrobe.Item `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Category != "定制分类" || body.Data.Material != "丝绒" || body.Data.Season != "梅雨季" || body.Data.Silhouette != "茧型" {
		t.Fatalf("expected custom suggested fields saved, got %#v", body.Data)
	}
}

func TestCreateItemRejectsInvalidRecommendationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newWardrobeRouteTestRouter(newRouteMemoryWardrobeRepo())

	body := routeWardrobeErrorResponse(t, router, http.MethodPost, "/api/user/wardrobe/items", `{"name":"黑色西装","category":"外套","recommendation_status":"hidden"}`, http.StatusBadRequest)

	if body.Code != "wardrobe.invalid_recommendation_status" {
		t.Fatalf("expected invalid recommendation status code, got %q", body.Code)
	}
}

func TestCreateItemRejectsEmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newWardrobeRouteTestRouter(newRouteMemoryWardrobeRepo())

	body := routeWardrobeErrorResponse(t, router, http.MethodPost, "/api/user/wardrobe/items", `{"name":"   ","category":"上装"}`, http.StatusBadRequest)

	if body.Code != "wardrobe.invalid_item" {
		t.Fatalf("expected invalid item code, got %q", body.Code)
	}
}

func TestCreateItemInjectsPrimaryImagePreviewAndOriginalURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	service := wardrobe.NewService(repo)
	service.SetImageURLSigner(&routeBatchImageURLSigner{})
	router := newWardrobeRouteTestRouterWithService(service)

	request := httptest.NewRequest(http.MethodPost, "/api/user/wardrobe/items", bytes.NewBufferString(`{"name":"米白衬衫","category":"上装","primary_asset_public_id":"ast_created"}`))
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
	if body.Data.PrimaryImage == nil {
		t.Fatalf("expected primary image, got nil")
	}
	expectedOriginalURL := "https://private.example.test/users/12/wardrobe/ast_created.jpg?token=original"
	expectedPreviewURL := "https://private.example.test/users/12/wardrobe/ast_created.jpg?imageView2/2/w/360/h/360/q/80/format/webp&token=preview"
	if body.Data.PrimaryImage.OriginalURL != expectedOriginalURL ||
		body.Data.PrimaryImage.PreviewURL != expectedPreviewURL {
		t.Fatalf("expected signed create URLs, got %#v", body.Data.PrimaryImage)
	}
}

func TestCreateItemWithMultipleAssetsUsesFirstAsPrimary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	service := wardrobe.NewService(repo)
	service.SetImageURLSigner(&routeBatchImageURLSigner{})
	router := newWardrobeRouteTestRouterWithService(service)

	request := httptest.NewRequest(http.MethodPost, "/api/user/wardrobe/items", bytes.NewBufferString(`{"asset_public_ids":["ast_first","ast_second"],"name":"识别中","category":"其他"}`))
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
	if body.Data.PrimaryImage == nil || body.Data.PrimaryImage.AssetPublicID != "ast_first" {
		t.Fatalf("expected first uploaded asset as primary, got %#v", body.Data.PrimaryImage)
	}
	if body.Data.RecognitionStatus != wardrobe.RecognitionStatusPending {
		t.Fatalf("expected pending recognition status, got %#v", body.Data)
	}
}

func TestPatchItemUpdatesRecommendationStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{PublicID: "wdi_owned", UserID: 12, Name: "米白衬衫", Category: "上装", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
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

func TestPatchItemRejectsRecognitionPendingItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{
		PublicID:             "wdi_pending",
		UserID:               12,
		Name:                 "识别中",
		Category:             "其他",
		RecognitionStatus:    wardrobe.RecognitionStatusPending,
		RecommendationStatus: wardrobe.RecommendationStatusNormal,
		Status:               wardrobe.StatusActive,
		IsCore:               true,
	})
	router := newWardrobeRouteTestRouter(repo)

	body := routeWardrobeErrorResponse(t, router, http.MethodPatch, "/api/user/wardrobe/items/wdi_pending", `{"name":"米白衬衫"}`, http.StatusConflict)

	if body.Code != "wardrobe.recognition_pending" {
		t.Fatalf("expected recognition pending code, got %q", body.Code)
	}
}

func TestPatchItemInjectsPrimaryImagePreviewAndOriginalURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{PublicID: "wdi_owned", UserID: 12, Name: "米白衬衫", Category: "上装", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
	service := wardrobe.NewService(repo)
	service.SetImageURLSigner(&routeBatchImageURLSigner{})
	router := newWardrobeRouteTestRouterWithService(service)

	request := httptest.NewRequest(http.MethodPatch, "/api/user/wardrobe/items/wdi_owned", bytes.NewBufferString(`{"primary_asset_public_id":"ast_updated"}`))
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
	if body.Data.PrimaryImage == nil {
		t.Fatalf("expected primary image, got nil")
	}
	expectedOriginalURL := "https://private.example.test/users/12/wardrobe/ast_updated.jpg?token=original"
	expectedPreviewURL := "https://private.example.test/users/12/wardrobe/ast_updated.jpg?imageView2/2/w/360/h/360/q/80/format/webp&token=preview"
	if body.Data.PrimaryImage.OriginalURL != expectedOriginalURL ||
		body.Data.PrimaryImage.PreviewURL != expectedPreviewURL {
		t.Fatalf("expected signed patch URLs, got %#v", body.Data.PrimaryImage)
	}
}

func TestListItemsKeepsResponseWhenPrimaryImageSigningFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{
		PublicID:             "wdi_owned",
		UserID:               12,
		Name:                 "米白衬衫",
		Category:             "上装",
		RecommendationStatus: wardrobe.RecommendationStatusNormal,
		Status:               wardrobe.StatusActive,
		IsCore:               true,
		PrimaryImage: &wardrobe.Image{
			AssetPublicID: "ast_owned",
			ObjectKey:     "users/12/wardrobe/ast_owned.jpg",
		},
	})
	service := wardrobe.NewService(repo)
	signErr := errors.New("sign failed")
	service.SetImageURLSigner(routeImageURLSignerFunc(func(_ context.Context, _ string) (string, error) {
		return "", signErr
	}))
	var capturedObjectKey string
	var capturedErr error
	service.SetImageURLSignErrorHandler(func(_ context.Context, objectKey string, err error) {
		capturedObjectKey = objectKey
		capturedErr = err
	})
	router := newWardrobeRouteTestRouterWithService(service)

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
	if body.Data.Items[0].PrimaryImage == nil ||
		body.Data.Items[0].PrimaryImage.PreviewURL != "" ||
		body.Data.Items[0].PrimaryImage.OriginalURL != "" {
		t.Fatalf("expected empty URL after signing failure, got %#v", body.Data.Items[0].PrimaryImage)
	}
	if capturedObjectKey != "users/12/wardrobe/ast_owned.jpg" || !errors.Is(capturedErr, signErr) {
		t.Fatalf("expected signing error handler to capture failure, key=%q err=%v", capturedObjectKey, capturedErr)
	}
}

func TestDeleteItemSoftDeletesCurrentUserItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{PublicID: "wdi_owned", UserID: 12, Name: "米白衬衫", Category: "上装", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
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
	repo.add(wardrobe.Item{PublicID: "wdi_other", UserID: 99, Name: "黑色西装", Category: "外套", RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true})
	router := newWardrobeRouteTestRouter(repo)

	body := routeWardrobeErrorResponse(t, router, http.MethodDelete, "/api/user/wardrobe/items/wdi_other", "", http.StatusNotFound)

	if body.Code != "wardrobe.item_not_found" {
		t.Fatalf("expected item not found code, got %q", body.Code)
	}
}

func TestRecognizeItemImageRouteReturnsRecognizedFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := wardrobe.NewService(newRouteMemoryWardrobeRepo())
	service.SetImageURLSigner(&routeBatchImageURLSigner{})
	service.SetImageRecognizer(routeImageRecognizerFunc(func(_ context.Context, userID int64, input wardrobe.RecognizeImageInput) (wardrobe.RecognizedItemFields, error) {
		if userID != 12 || input.AssetPublicID != "ast_primary" || input.ImageURL == "" {
			t.Fatalf("expected current user and asset id, user=%d input=%#v", userID, input)
		}
		return wardrobe.RecognizedItemFields{
			Name:       "米白针织开衫",
			Category:   "外套",
			Color:      "米白",
			Silhouette: "微宽松",
			Material:   "针织",
			Season:     "春秋",
			SceneTags:  []string{"通勤", "周末"},
			UserNotes:  "建议内搭简洁上衣",
			Confidence: 0.78,
		}, nil
	}))
	router := newWardrobeRouteTestRouterWithService(service)

	request := httptest.NewRequest(http.MethodPost, "/api/user/wardrobe/items/recognize", bytes.NewBufferString(`{"asset_public_id":" ast_primary "}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string                        `json:"code"`
		Data wardrobe.RecognizedItemFields `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected code ok, got %q", body.Code)
	}
	if body.Data.Name != "米白针织开衫" || body.Data.Category != "外套" || len(body.Data.SceneTags) != 2 {
		t.Fatalf("expected recognized fields response, got %#v", body.Data)
	}
}

func TestRecognizeItemImageRejectsMissingAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := wardrobe.NewService(newRouteMemoryWardrobeRepo())
	service.SetImageRecognizer(routeImageRecognizerFunc(func(_ context.Context, _ int64, _ wardrobe.RecognizeImageInput) (wardrobe.RecognizedItemFields, error) {
		return wardrobe.RecognizedItemFields{}, nil
	}))
	router := newWardrobeRouteTestRouterWithService(service)

	body := routeWardrobeErrorResponse(t, router, http.MethodPost, "/api/user/wardrobe/items/recognize", `{"asset_public_id":" "}`, http.StatusBadRequest)

	if body.Code != "wardrobe.invalid_primary_asset" {
		t.Fatalf("expected invalid primary asset code, got %q", body.Code)
	}
}

func TestRecognizeItemImageReturnsUnavailableWithoutRecognizer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newWardrobeRouteTestRouter(newRouteMemoryWardrobeRepo())

	body := routeWardrobeErrorResponse(t, router, http.MethodPost, "/api/user/wardrobe/items/recognize", `{"asset_public_id":"ast_primary"}`, http.StatusInternalServerError)

	if body.Code != "wardrobe.image_recognizer_unavailable" {
		t.Fatalf("expected image recognizer unavailable code, got %q", body.Code)
	}
}

func newWardrobeRouteTestRouter(repo *routeMemoryWardrobeRepo) *gin.Engine {
	return newWardrobeRouteTestRouterWithService(wardrobe.NewService(repo))
}

func newWardrobeRouteTestRouterWithService(service *wardrobe.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{
			UserID:       12,
			UserPublicID: "usr_test",
			Surface:      "user",
		})
		c.Next()
	})
	wardrobe.RegisterUserRoutesWithService(router.Group("/api/user/wardrobe"), service, nil)
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
	items   map[string]wardrobe.Item
	options wardrobe.WardrobeOptions
}

type routeCoreOnlyWardrobeRepo struct{}

type routeImageURLSignerFunc func(ctx context.Context, objectKey string) (string, error)

func (f routeImageURLSignerFunc) PrivateDownloadURL(ctx context.Context, objectKey string) (string, error) {
	return f(ctx, objectKey)
}

type routeBatchImageURLSigner struct {
	batchCalls  int
	singleCalls int
}

func (s *routeBatchImageURLSigner) PrivateDownloadURL(_ context.Context, objectKey string) (string, error) {
	s.singleCalls++
	return fmt.Sprintf("https://private.example.test/%s?token=original", objectKey), nil
}

func (s *routeBatchImageURLSigner) PrivateImageURLs(_ context.Context, objectKeys []string) (map[string]wardrobe.SignedImageURLs, error) {
	s.batchCalls++
	result := make(map[string]wardrobe.SignedImageURLs, len(objectKeys))
	for _, objectKey := range objectKeys {
		result[objectKey] = wardrobe.SignedImageURLs{
			PreviewURL:  fmt.Sprintf("https://private.example.test/%s?imageView2/2/w/360/h/360/q/80/format/webp&token=preview", objectKey),
			OriginalURL: fmt.Sprintf("https://private.example.test/%s?token=original", objectKey),
		}
	}
	return result, nil
}

type routeImageRecognizerFunc func(ctx context.Context, userID int64, input wardrobe.RecognizeImageInput) (wardrobe.RecognizedItemFields, error)

func (f routeImageRecognizerFunc) RecognizeWardrobeItemImage(ctx context.Context, userID int64, input wardrobe.RecognizeImageInput) (wardrobe.RecognizedItemFields, error) {
	return f(ctx, userID, input)
}

func (routeCoreOnlyWardrobeRepo) CreateCoreItems(_ context.Context, items []wardrobe.Item) ([]wardrobe.Item, error) {
	return items, nil
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
	sort.SliceStable(result, func(i, j int) bool {
		if !result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].UpdatedAt.After(result[j].UpdatedAt)
		}
		return result[i].PublicID < result[j].PublicID
	})
	return result, nil
}

func (r *routeMemoryWardrobeRepo) FindItemForUser(_ context.Context, userID int64, publicID string) (wardrobe.Item, error) {
	item, ok := r.items[publicID]
	if !ok || item.UserID != userID || item.Status == wardrobe.StatusDeleted {
		return wardrobe.Item{}, wardrobe.ErrItemNotFound
	}
	return item, nil
}

func (r *routeMemoryWardrobeRepo) ListWardrobeOptions(context.Context) (wardrobe.WardrobeOptions, error) {
	if len(r.options.Categories) == 0 &&
		len(r.options.Materials) == 0 &&
		len(r.options.Seasons) == 0 &&
		len(r.options.Silhouettes) == 0 {
		return wardrobe.DefaultWardrobeOptions(), nil
	}
	return r.options, nil
}

func (r *routeMemoryWardrobeRepo) CreateItem(_ context.Context, item wardrobe.Item, primaryAssetPublicID string) (wardrobe.Item, error) {
	if primaryAssetPublicID != "" {
		item.PrimaryImage = routePrimaryImage(primaryAssetPublicID)
	}
	r.add(item)
	return item, nil
}

func (r *routeMemoryWardrobeRepo) CreateItemWithAssets(_ context.Context, item wardrobe.Item, assetPublicIDs []string) (wardrobe.Item, error) {
	if len(assetPublicIDs) > 0 {
		item.PrimaryImage = routePrimaryImage(assetPublicIDs[0])
	}
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
	if input.RecognitionStatus != nil {
		item.RecognitionStatus = *input.RecognitionStatus
	}
	if input.PrimaryAssetPublicID != nil {
		if *input.PrimaryAssetPublicID == "" {
			item.PrimaryImage = nil
		} else {
			item.PrimaryImage = routePrimaryImage(*input.PrimaryAssetPublicID)
		}
	}
	item.UpdatedAt = time.Now().UTC()
	r.items[publicID] = item
	return item, nil
}

func (r *routeMemoryWardrobeRepo) FindRecognizableAsset(_ context.Context, userID int64, assetPublicID string) (wardrobe.Image, error) {
	if userID != 12 || assetPublicID == "" {
		return wardrobe.Image{}, wardrobe.ErrInvalidPrimaryAsset
	}
	return *routePrimaryImage(assetPublicID), nil
}

func routePrimaryImage(assetPublicID string) *wardrobe.Image {
	return &wardrobe.Image{
		AssetPublicID: assetPublicID,
		ObjectKey:     fmt.Sprintf("users/12/wardrobe/%s.jpg", assetPublicID),
	}
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
