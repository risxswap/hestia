package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hestia/server/internal/common/auth"

	"github.com/gin-gonic/gin"
)

const (
	testAssetPublicID  = "ast_abcdefghijklmnopqrstuvwxyz"
	racedAssetPublicID = "ast_bcdefghijklmnopqrstuvwxyza"
	takenAssetPublicID = "ast_cdefghijklmnopqrstuvwxyzab"
)

type memoryRepo struct {
	nextID                int64
	byID                  map[string]Asset
	duplicateOnNextCreate bool
	missOnNextFind        bool
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{nextID: 1, byID: map[string]Asset{}}
}

func (r *memoryRepo) CreateMany(_ context.Context, items []Asset) ([]Asset, error) {
	for i := range items {
		items[i].ID = r.nextID
		r.nextID++
		r.byID[items[i].PublicID] = items[i]
	}
	return items, nil
}

func (r *memoryRepo) Create(_ context.Context, item Asset) (Asset, error) {
	if r.duplicateOnNextCreate {
		r.duplicateOnNextCreate = false
		return Asset{}, ErrAssetAlreadyExists
	}
	if _, ok := r.byID[item.PublicID]; ok {
		return Asset{}, ErrAssetAlreadyExists
	}
	item.ID = r.nextID
	r.nextID++
	r.byID[item.PublicID] = item
	return item, nil
}

type fakeObjectStatChecker struct {
	items map[string]ObjectStat
}

func (c fakeObjectStatChecker) StatObject(_ context.Context, bucket string, objectKey string) (ObjectStat, error) {
	stat, ok := c.items[bucket+"/"+objectKey]
	if !ok {
		return ObjectStat{}, ErrAssetNotFound
	}
	return stat, nil
}

func (r *memoryRepo) FindByPublicID(_ context.Context, publicID string) (Asset, error) {
	if r.missOnNextFind {
		r.missOnNextFind = false
		return Asset{}, ErrAssetNotFound
	}
	item, ok := r.byID[publicID]
	if !ok {
		return Asset{}, ErrAssetNotFound
	}
	return item, nil
}

func TestUploadTokenGeneratesServerOwnedObjectKeyForSupportedAssetTypes(t *testing.T) {
	router := newAuthenticatedFileRouter(t, newMemoryRepo())

	tests := map[string]string{
		"wardrobe_item_photo": "wardrobe",
		"onboarding_photo":    "onboarding",
		"style_reference":     "style-reference",
		"chat_image":          "chat",
	}
	for assetType, scope := range tests {
		t.Run(assetType, func(t *testing.T) {
			recorder := postJSON(t, router, "/api/user/files/upload-token", map[string]any{
				"asset_type": assetType,
				"mime_type":  "image/jpeg",
				"file_size":  1024,
			})

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
			}
			data := responseData(t, recorder)
			if data["asset_public_id"] == "" {
				t.Fatalf("expected asset_public_id in response: %#v", data)
			}
			if data["file_public_id"] != data["asset_public_id"] {
				t.Fatalf("expected file_public_id compatibility alias, got %#v", data)
			}
			publicID := data["asset_public_id"].(string)
			expectedKey := "users/12/" + scope + "/" + publicID + ".jpg"
			if data["bucket"] != "private-assets" {
				t.Fatalf("expected bucket private-assets, got %#v", data["bucket"])
			}
			if data["object_key"] != expectedKey {
				t.Fatalf("expected object key %q, got %#v", expectedKey, data["object_key"])
			}
			if data["upload_url"] != "https://upload.example.test" {
				t.Fatalf("expected upload url, got %#v", data["upload_url"])
			}
			if data["upload_token"] != "token:"+expectedKey {
				t.Fatalf("expected fake upload token, got %#v", data["upload_token"])
			}
			if data["expires_at"] == "" {
				t.Fatalf("expected expires_at in response: %#v", data)
			}
		})
	}
}

func TestFileRoutesCreateUploadTokenAndConfirmWithoutRecognition(t *testing.T) {
	repo := newMemoryRepo()
	options := defaultServiceOptions()
	router := newAuthenticatedFileRouterWithOptions(t, repo, options)

	tokenRecorder := postJSON(t, router, "/api/user/files/upload-token", map[string]any{
		"asset_type": "wardrobe_item_photo",
		"mime_type":  "image/jpeg",
		"file_size":  2048,
	})
	if tokenRecorder.Code != http.StatusOK {
		t.Fatalf("expected token status 200, got %d body=%s", tokenRecorder.Code, tokenRecorder.Body.String())
	}
	tokenData := responseData(t, tokenRecorder)
	if tokenData["file_public_id"] == "" || tokenData["file_public_id"] != tokenData["asset_public_id"] {
		t.Fatalf("expected file_public_id in token response: %#v", tokenData)
	}

	confirmRecorder := postJSON(t, router, "/api/user/files/confirm", map[string]any{
		"asset_public_id": testAssetPublicID,
		"bucket":          "private-assets",
		"object_key":      "users/12/wardrobe/" + testAssetPublicID + ".jpg",
		"mime_type":       "image/jpeg",
		"file_size":       2048,
		"asset_type":      "wardrobe_item_photo",
	})
	if confirmRecorder.Code != http.StatusOK {
		t.Fatalf("expected confirm status 200, got %d body=%s", confirmRecorder.Code, confirmRecorder.Body.String())
	}
	confirmData := responseData(t, confirmRecorder)
	if confirmData["file_public_id"] != testAssetPublicID || confirmData["file_type"] != "wardrobe_item_photo" {
		t.Fatalf("expected file fields in confirm response: %#v", confirmData)
	}
	if _, ok := confirmData["url"]; ok {
		t.Fatalf("confirm response must not expose expiring url: %#v", confirmData)
	}
	if _, ok := confirmData["recognized_fields"]; ok {
		t.Fatalf("confirm response must not include recognized fields: %#v", confirmData)
	}
}

func TestUploadTokenRequiresAuthentication(t *testing.T) {
	router := newFileRouterWithoutAuth(t, newMemoryRepo(), ServiceOptions{
		Bucket:     "private-assets",
		UploadHost: "https://upload.example.test",
		UploadSigner: UploadSignerFunc(func(_ context.Context, req UploadSignRequest) (string, error) {
			return "token:" + req.ObjectKey, nil
		}),
	})

	recorder := postJSON(t, router, "/api/user/files/upload-token", map[string]any{
		"asset_type": "wardrobe_item_photo",
		"mime_type":  "image/jpeg",
		"file_size":  1024,
	})

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadTokenRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name       string
		options    ServiceOptions
		body       map[string]any
		statusCode int
		code       string
	}{
		{
			name: "unsupported asset type",
			body: map[string]any{
				"asset_type": "avatar",
				"mime_type":  "image/jpeg",
				"file_size":  1024,
			},
			statusCode: http.StatusBadRequest,
			code:       "asset.unsupported_asset_type",
		},
		{
			name: "invalid mime",
			body: map[string]any{
				"asset_type": "wardrobe_item_photo",
				"mime_type":  "text/plain",
				"file_size":  1024,
			},
			statusCode: http.StatusBadRequest,
			code:       "asset.invalid_mime_type",
		},
		{
			name: "invalid file size",
			body: map[string]any{
				"asset_type": "wardrobe_item_photo",
				"mime_type":  "image/jpeg",
				"file_size":  10*1024*1024 + 1,
			},
			statusCode: http.StatusBadRequest,
			code:       "asset.invalid_file_size",
		},
		{
			name: "storage not configured",
			options: ServiceOptions{
				Bucket: "private-assets",
			},
			body: map[string]any{
				"asset_type": "wardrobe_item_photo",
				"mime_type":  "image/jpeg",
				"file_size":  1024,
			},
			statusCode: http.StatusInternalServerError,
			code:       "asset.storage_not_configured",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := tt.options
			if options.Bucket == "" {
				options = defaultServiceOptions()
			}
			router := newAuthenticatedFileRouterWithOptions(t, newMemoryRepo(), options)

			recorder := postJSON(t, router, "/api/user/files/upload-token", tt.body)

			if recorder.Code != tt.statusCode {
				t.Fatalf("expected status %d, got %d body=%s", tt.statusCode, recorder.Code, recorder.Body.String())
			}
			body := responseBody(t, recorder)
			if body["code"] != tt.code {
				t.Fatalf("expected code %s, got %#v", tt.code, body["code"])
			}
		})
	}
}

func TestConfirmCreatesAssetAndIsIdempotent(t *testing.T) {
	repo := newMemoryRepo()
	router := newAuthenticatedFileRouter(t, repo)

	body := map[string]any{
		"asset_public_id": "ast_abcdefghijklmnopqrstuvwxyz",
		"bucket":          "private-assets",
		"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png",
		"mime_type":       "image/png",
		"file_size":       2048,
		"width":           640,
		"height":          480,
		"asset_type":      "onboarding_photo",
	}
	first := postJSON(t, router, "/api/user/files/confirm", body)
	if first.Code != http.StatusOK {
		t.Fatalf("expected first confirm status 200, got %d body=%s", first.Code, first.Body.String())
	}
	firstData := responseData(t, first)
	if firstData["asset_public_id"] != "ast_abcdefghijklmnopqrstuvwxyz" ||
		firstData["object_key"] != "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png" ||
		firstData["asset_type"] != "onboarding_photo" {
		t.Fatalf("unexpected confirm response data: %#v", firstData)
	}
	if _, ok := firstData["url"]; ok {
		t.Fatalf("confirm response must not expose expiring url: %#v", firstData)
	}
	for _, internalKey := range []string{"owner_user_id", "metadata", "status", "review_status", "source", "bucket"} {
		if _, ok := firstData[internalKey]; ok {
			t.Fatalf("confirm response exposed internal key %s: %#v", internalKey, firstData)
		}
	}
	second := postJSON(t, router, "/api/user/files/confirm", body)
	if second.Code != http.StatusOK {
		t.Fatalf("expected idempotent confirm status 200, got %d body=%s", second.Code, second.Body.String())
	}
	if len(repo.byID) != 1 {
		t.Fatalf("expected one persisted asset, got %#v", repo.byID)
	}
	created := repo.byID["ast_abcdefghijklmnopqrstuvwxyz"]
	if created.OwnerUserID != 12 {
		t.Fatalf("expected owner 12, got %d", created.OwnerUserID)
	}
	if created.Source != SourceMiniappUpload || created.Status != StatusActive || created.ReviewStatus != ReviewStatusPending {
		t.Fatalf("expected upload lifecycle fields, got %#v", created)
	}
	if created.Metadata["upload_source"] != SourceMiniappUpload {
		t.Fatalf("expected upload_source metadata, got %#v", created.Metadata)
	}
	if created.Metadata["width"] != 640 || created.Metadata["height"] != 480 {
		t.Fatalf("expected dimensions metadata, got %#v", created.Metadata)
	}
	if created.Metadata["confirmed_at"] == "" {
		t.Fatalf("expected confirmed_at metadata, got %#v", created.Metadata)
	}
}

func TestConfirmRejectsInvalidRequestFields(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
	}{
		{
			name: "missing asset public id",
			body: map[string]any{
				"bucket":     "private-assets",
				"object_key": "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png",
				"mime_type":  "image/png",
				"file_size":  2048,
				"asset_type": "onboarding_photo",
			},
		},
		{
			name: "short asset public id",
			body: map[string]any{
				"asset_public_id": "bad",
				"bucket":          "private-assets",
				"object_key":      "users/12/onboarding/bad.png",
				"mime_type":       "image/png",
				"file_size":       2048,
				"asset_type":      "onboarding_photo",
			},
		},
		{
			name: "overlong asset public id",
			body: map[string]any{
				"asset_public_id": "ast_abcdefghijklmnopqrstuvwxyz2",
				"bucket":          "private-assets",
				"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz2.png",
				"mime_type":       "image/png",
				"file_size":       2048,
				"asset_type":      "onboarding_photo",
			},
		},
		{
			name: "slash asset public id",
			body: map[string]any{
				"asset_public_id": "ast_abcdefghijklmnopqrstuvwxy/",
				"bucket":          "private-assets",
				"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxy/.png",
				"mime_type":       "image/png",
				"file_size":       2048,
				"asset_type":      "onboarding_photo",
			},
		},
		{
			name: "invalid width",
			body: map[string]any{
				"asset_public_id": "ast_abcdefghijklmnopqrstuvwxyz",
				"bucket":          "private-assets",
				"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png",
				"mime_type":       "image/png",
				"file_size":       2048,
				"width":           0,
				"asset_type":      "onboarding_photo",
			},
		},
		{
			name: "invalid height",
			body: map[string]any{
				"asset_public_id": "ast_abcdefghijklmnopqrstuvwxyz",
				"bucket":          "private-assets",
				"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png",
				"mime_type":       "image/png",
				"file_size":       2048,
				"height":          -1,
				"asset_type":      "onboarding_photo",
			},
		},
		{
			name: "oversized dimension",
			body: map[string]any{
				"asset_public_id": "ast_abcdefghijklmnopqrstuvwxyz",
				"bucket":          "private-assets",
				"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png",
				"mime_type":       "image/png",
				"file_size":       2048,
				"width":           20001,
				"asset_type":      "onboarding_photo",
			},
		},
	}
	router := newAuthenticatedFileRouter(t, newMemoryRepo())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := postJSON(t, router, "/api/user/files/confirm", tt.body)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d body=%s", recorder.Code, recorder.Body.String())
			}
			body := responseBody(t, recorder)
			if body["code"] != "asset.invalid_request" {
				t.Fatalf("expected asset.invalid_request, got %#v", body["code"])
			}
		})
	}
}

func TestConfirmRequiresConfiguredBucket(t *testing.T) {
	options := defaultServiceOptions()
	options.Bucket = ""
	router := newAuthenticatedFileRouterWithOptions(t, newMemoryRepo(), options)

	recorder := postJSON(t, router, "/api/user/files/confirm", map[string]any{
		"asset_public_id": "ast_abcdefghijklmnopqrstuvwxyz",
		"bucket":          "private-assets",
		"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png",
		"mime_type":       "image/png",
		"file_size":       2048,
		"asset_type":      "onboarding_photo",
	})

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := responseBody(t, recorder)
	if body["code"] != "asset.storage_not_configured" {
		t.Fatalf("expected asset.storage_not_configured, got %#v", body["code"])
	}
}

func TestConfirmDoesNotReturnPrivateDownloadURL(t *testing.T) {
	options := defaultServiceOptions()
	options.PrivateDomain = "private.example.test"
	options.DownloadSigner = DownloadSignerFunc(func(_ context.Context, req DownloadSignRequest) (string, error) {
		return req.PrivateDomain + "/" + req.ObjectKey, nil
	})
	router := newAuthenticatedFileRouterWithOptions(t, newMemoryRepo(), options)

	recorder := postJSON(t, router, "/api/user/files/confirm", map[string]any{
		"asset_public_id": "ast_abcdefghijklmnopqrstuvwxyz",
		"bucket":          "private-assets",
		"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png",
		"mime_type":       "image/png",
		"file_size":       2048,
		"asset_type":      "onboarding_photo",
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	data := responseData(t, recorder)
	if _, ok := data["url"]; ok {
		t.Fatalf("confirm response must not expose expiring url: %#v", data)
	}
}

func TestConfirmChecksObjectStatWhenCheckerIsConfigured(t *testing.T) {
	tests := []struct {
		name       string
		checker    ObjectStatChecker
		statusCode int
		code       string
	}{
		{
			name: "exists",
			checker: fakeObjectStatChecker{items: map[string]ObjectStat{
				"private-assets/users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png": {FileSize: 2048, MimeType: "image/png"},
			}},
			statusCode: http.StatusOK,
			code:       "ok",
		},
		{
			name: "size mismatch",
			checker: fakeObjectStatChecker{items: map[string]ObjectStat{
				"private-assets/users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png": {FileSize: 1024, MimeType: "image/png"},
			}},
			statusCode: http.StatusBadRequest,
			code:       "asset.invalid_file_size",
		},
		{
			name:       "missing",
			checker:    fakeObjectStatChecker{items: map[string]ObjectStat{}},
			statusCode: http.StatusNotFound,
			code:       "asset.not_found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMemoryRepo()
			options := defaultServiceOptions()
			options.ObjectStatChecker = tt.checker
			router := newAuthenticatedFileRouterWithOptions(t, repo, options)

			recorder := postJSON(t, router, "/api/user/files/confirm", map[string]any{
				"asset_public_id": "ast_abcdefghijklmnopqrstuvwxyz",
				"bucket":          "private-assets",
				"object_key":      "users/12/onboarding/ast_abcdefghijklmnopqrstuvwxyz.png",
				"mime_type":       "image/png",
				"file_size":       2048,
				"asset_type":      "onboarding_photo",
			})

			if recorder.Code != tt.statusCode {
				t.Fatalf("expected status %d, got %d body=%s", tt.statusCode, recorder.Code, recorder.Body.String())
			}
			body := responseBody(t, recorder)
			if body["code"] != tt.code {
				t.Fatalf("expected code %s, got %#v", tt.code, body["code"])
			}
		})
	}
}

func TestConfirmTreatsDuplicateCreateAsIdempotent(t *testing.T) {
	repo := newMemoryRepo()
	existing := Asset{
		PublicID:    "ast_bcdefghijklmnopqrstuvwxyza",
		OwnerUserID: 12,
		Bucket:      "private-assets",
		ObjectKey:   "users/12/onboarding/ast_bcdefghijklmnopqrstuvwxyza.png",
		MimeType:    "image/png",
		FileSize:    2048,
		AssetType:   "onboarding_photo",
	}
	repo.duplicateOnNextCreate = true
	repo.missOnNextFind = true
	router := newAuthenticatedFileRouterWithOptions(t, repo, defaultServiceOptions())
	repo.byID[existing.PublicID] = existing

	recorder := postJSON(t, router, "/api/user/files/confirm", map[string]any{
		"asset_public_id": "ast_bcdefghijklmnopqrstuvwxyza",
		"bucket":          "private-assets",
		"object_key":      "users/12/onboarding/ast_bcdefghijklmnopqrstuvwxyza.png",
		"mime_type":       "image/png",
		"file_size":       2048,
		"asset_type":      "onboarding_photo",
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected duplicate create to be idempotent, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	data := responseData(t, recorder)
	if data["asset_public_id"] != "ast_bcdefghijklmnopqrstuvwxyza" {
		t.Fatalf("expected raced asset response, got %#v", data)
	}
}

func TestConfirmRejectsConflictingAssetOwnership(t *testing.T) {
	repo := newMemoryRepo()
	_, err := repo.Create(context.Background(), Asset{
		PublicID:    "ast_cdefghijklmnopqrstuvwxyzab",
		OwnerUserID: 99,
		Bucket:      "private-assets",
		ObjectKey:   "users/99/onboarding/ast_cdefghijklmnopqrstuvwxyzab.png",
	})
	if err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	router := newAuthenticatedFileRouter(t, repo)

	recorder := postJSON(t, router, "/api/user/files/confirm", map[string]any{
		"asset_public_id": "ast_cdefghijklmnopqrstuvwxyzab",
		"bucket":          "private-assets",
		"object_key":      "users/12/onboarding/ast_cdefghijklmnopqrstuvwxyzab.png",
		"mime_type":       "image/png",
		"file_size":       2048,
		"asset_type":      "onboarding_photo",
	})

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := responseBody(t, recorder)
	if body["code"] != "asset.forbidden" {
		t.Fatalf("expected asset.forbidden, got %#v", body["code"])
	}
}

func newAuthenticatedFileRouter(t *testing.T, repo Repository) *gin.Engine {
	t.Helper()
	return newAuthenticatedFileRouterWithOptions(t, repo, defaultServiceOptions())
}

func newAuthenticatedFileRouterWithOptions(t *testing.T, repo Repository, options ServiceOptions) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	group := router.Group("/api/user/files")
	group.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	service := NewServiceWithOptions(repo, options)
	RegisterFileRoutesWithService(group, service, nil)
	return router
}

func newFileRouterWithoutAuth(t *testing.T, repo Repository, options ServiceOptions) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	RegisterFileRoutesWithService(router.Group("/api/user/files"), NewServiceWithOptions(repo, options), nil)
	return router
}

func defaultServiceOptions() ServiceOptions {
	return ServiceOptions{
		Bucket:        "private-assets",
		UploadHost:    "https://upload.example.test",
		PrivateDomain: "private.example.test",
		UploadTTL:     time.Hour,
		DownloadTTL:   15 * time.Minute,
		UploadSigner: UploadSignerFunc(func(_ context.Context, req UploadSignRequest) (string, error) {
			return "token:" + req.ObjectKey, nil
		}),
		DownloadSigner: DownloadSignerFunc(func(_ context.Context, req DownloadSignRequest) (string, error) {
			return "https://download.example.test/" + req.ObjectKey, nil
		}),
	}
}

func postJSON(t *testing.T, router *gin.Engine, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func responseData(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	body := responseBody(t, recorder)
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected response data object, got %#v", body["data"])
	}
	return data
}

func responseBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}
