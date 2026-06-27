# 用户资产七牛图片上传 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建公共用户资产图片上传能力，并把衣橱主图新增/编辑接入该能力。

**Architecture:** 后端新增 `asset` 用户路由，负责签发七牛上传凭证、确认资产、生成私有短期下载 URL；衣橱服务通过公共资产 URL 签名能力返回 `primary_image.url`。小程序新增公共上传封装，衣橱新增/编辑弹窗用 TDesign `t-upload` 调用公共上传逻辑，保存时继续提交现有 `primary_asset_public_id`。

**Tech Stack:** Go 1.25、Gin、sqlx、MySQL、七牛 Go SDK `github.com/qiniu/go-sdk/v7`、微信小程序、TDesign MiniProgram、Node 验证脚本。

---

## 文件结构

- Modify: `server/go.mod`, `server/go.sum`，新增七牛官方 Go SDK。
- Modify: `server/internal/infra/config/config.go`，增加七牛配置字段和 TOML 解析。
- Modify: `server/internal/domain/asset/model.go`，增加上传 token、确认、URL 签名相关输入输出类型。
- Modify: `server/internal/domain/asset/service.go`，增加公共上传凭证、确认资产、短期 URL 生成能力。
- Modify: `server/internal/domain/asset/repo.go`，增加按 `public_id` 查找、幂等确认、单资产创建。
- Create: `server/internal/domain/asset/routes.go`，注册 `/api/user/assets` 路由。
- Create: `server/internal/domain/asset/handler.go`，处理 `upload-token` 和 `confirm`。
- Create: `server/internal/domain/asset/routes_test.go`，覆盖公共接口契约。
- Modify: `server/internal/app/user/router.go`，挂载公共资产路由。
- Modify: `server/internal/domain/wardrobe/routes.go`, `service.go`, `handler.go` 或同层最小文件，给列表/创建/更新返回的 `primary_image` 注入短期 URL。
- Modify: `server/internal/domain/wardrobe/routes_test.go`，覆盖衣橱列表返回 `primary_image.url`。
- Modify: `miniapp/utils/api.js`，增加公共资产上传 API 和七牛直传封装。
- Modify: `miniapp/pages/wardrobe/wardrobe.json`, `wardrobe.js`, `wardrobe.wxml`, `wardrobe.wxss`，新增主图上传块。
- Modify: `miniapp/pages/wardrobe-detail/wardrobe-detail.json`, `wardrobe-detail.js`, `wardrobe-detail.wxml`, `wardrobe-detail.wxss`，编辑弹窗接入同一上传块。
- Modify: `miniapp/scripts/verify-api-client.js`，覆盖公共上传 API。
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`，覆盖衣橱上传 UI、上传状态、保存阻断和清空主图。

---

### Task 1: 后端公共资产上传接口

**Files:**
- Modify: `server/go.mod`
- Modify: `server/go.sum`
- Modify: `server/internal/infra/config/config.go`
- Modify: `server/internal/domain/asset/model.go`
- Modify: `server/internal/domain/asset/service.go`
- Modify: `server/internal/domain/asset/repo.go`
- Create: `server/internal/domain/asset/handler.go`
- Create: `server/internal/domain/asset/routes.go`
- Create: `server/internal/domain/asset/routes_test.go`
- Modify: `server/internal/app/user/router.go`

- [ ] **Step 1: Add failing service and route tests**

Add tests that prove these behaviors before implementation:

```go
func TestCreateUploadTokenRejectsUnsupportedAssetType(t *testing.T) {
	service := asset.NewService(&captureRepo{})
	_, err := service.CreateUploadToken(context.Background(), 12, asset.UploadTokenInput{
		AssetType: "unknown",
		MimeType:  "image/jpeg",
		FileSize:  1024,
		FileExt:   ".jpg",
	})
	if !errors.Is(err, asset.ErrUnsupportedAssetType) {
		t.Fatalf("expected unsupported asset type, got %v", err)
	}
}

func TestCreateUploadTokenGeneratesUserScopedObjectKey(t *testing.T) {
	signer := &fakeUploadSigner{token: "upload-token"}
	service := asset.NewServiceWithOptions(&captureRepo{}, asset.ServiceOptions{
		Bucket:      "hestia-private",
		UploadHost:  "https://upload.qiniup.com",
		UploadTTL:   time.Hour,
		UploadSigner: signer,
	})
	result, err := service.CreateUploadToken(context.Background(), 12, asset.UploadTokenInput{
		AssetType: "wardrobe_item_photo",
		MimeType:  "image/jpeg",
		FileSize:  1024,
		FileExt:   ".jpg",
	})
	if err != nil {
		t.Fatalf("create upload token: %v", err)
	}
	if !strings.HasPrefix(result.AssetPublicID, "ast_") {
		t.Fatalf("expected ast public id, got %q", result.AssetPublicID)
	}
	if !strings.HasPrefix(result.ObjectKey, "users/12/wardrobe/") {
		t.Fatalf("expected user-scoped wardrobe key, got %q", result.ObjectKey)
	}
	if result.UploadToken != "upload-token" {
		t.Fatalf("expected fake token, got %q", result.UploadToken)
	}
	if signer.scope != "hestia-private:"+result.ObjectKey {
		t.Fatalf("expected scoped signer key, got %q", signer.scope)
	}
}

func TestConfirmUploadIsIdempotentForSameAsset(t *testing.T) {
	repo := newRouteAssetMemoryRepo()
	service := asset.NewServiceWithOptions(repo, asset.ServiceOptions{
		Bucket:        "hestia-private",
		PrivateDomain: "https://assets.example.com",
		DownloadTTL:   time.Minute,
		DownloadSigner: &fakeDownloadSigner{},
	})
	input := asset.ConfirmUploadInput{
		AssetPublicID: "ast_same",
		Bucket:        "hestia-private",
		ObjectKey:     "users/12/wardrobe/ast_same.jpg",
		MimeType:      "image/jpeg",
		FileSize:      1024,
		AssetType:     "wardrobe_item_photo",
	}
	first, err := service.ConfirmUpload(context.Background(), 12, input)
	if err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	second, err := service.ConfirmUpload(context.Background(), 12, input)
	if err != nil {
		t.Fatalf("second confirm: %v", err)
	}
	if first.ID != second.ID || len(repo.items) != 1 {
		t.Fatalf("expected idempotent confirm, first=%#v second=%#v items=%#v", first, second, repo.items)
	}
}
```

Add route tests:

```go
func TestUploadTokenRouteRequiresSupportedImageType(t *testing.T) {
	router := newAssetRouteTestRouter(asset.NewServiceWithOptions(newRouteAssetMemoryRepo(), asset.ServiceOptions{
		Bucket: "hestia-private", UploadHost: "https://upload.qiniup.com",
		UploadSigner: &fakeUploadSigner{token: "upload-token"},
	}))
	body := routeAssetErrorResponse(t, router, http.MethodPost, "/api/user/assets/upload-token", `{"asset_type":"wardrobe_item_photo","mime_type":"text/plain","file_size":10}`, http.StatusBadRequest)
	if body.Code != "asset.invalid_mime_type" {
		t.Fatalf("expected invalid mime type, got %q", body.Code)
	}
}

func TestUploadTokenRouteReturnsToken(t *testing.T) {
	router := newAssetRouteTestRouter(asset.NewServiceWithOptions(newRouteAssetMemoryRepo(), asset.ServiceOptions{
		Bucket: "hestia-private", UploadHost: "https://upload.qiniup.com",
		UploadSigner: &fakeUploadSigner{token: "upload-token"},
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/user/assets/upload-token", bytes.NewBufferString(`{"asset_type":"wardrobe_item_photo","mime_type":"image/jpeg","file_size":1024,"file_ext":".jpg"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
		Data asset.UploadTokenResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.UploadToken != "upload-token" || body.Data.ObjectKey == "" {
		t.Fatalf("expected upload token response, got %#v", body.Data)
	}
}
```

- [ ] **Step 2: Run tests to verify RED**

Run:

```bash
cd server && go test ./internal/domain/asset
```

Expected: FAIL with missing `CreateUploadToken`, `ConfirmUpload`, `NewServiceWithOptions`, route types, or new errors.

- [ ] **Step 3: Add七牛 SDK dependency**

Run:

```bash
cd server && go get github.com/qiniu/go-sdk/v7@latest
```

Then use:

```go
import (
	"github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
)
```

For upload token:

```go
mac := qbox.NewMac(accessKey, secretKey)
policy := storage.PutPolicy{
	Scope:   bucket + ":" + objectKey,
	Expires: uint64(ttl.Seconds()),
}
token := policy.UploadToken(mac)
```

For private download URL:

```go
deadline := time.Now().Add(ttl).Unix()
url := storage.MakePrivateURL(mac, domain, objectKey, deadline)
```

- [ ] **Step 4: Implement config fields**

Add to `config.Config`:

```go
QiniuAccessKey             string `env:"QINIU_ACCESS_KEY"`
QiniuSecretKey             string `env:"QINIU_SECRET_KEY"`
QiniuBucket                string `env:"QINIU_BUCKET"`
QiniuUploadHost            string `env:"QINIU_UPLOAD_HOST"`
QiniuPrivateDomain         string `env:"QINIU_PRIVATE_DOMAIN"`
QiniuUploadTokenTTLSeconds int    `env:"QINIU_UPLOAD_TOKEN_TTL_SECONDS" envDefault:"3600"`
QiniuDownloadURLTTLSeconds int    `env:"QINIU_DOWNLOAD_URL_TTL_SECONDS" envDefault:"900"`
```

Extend `fileConfig`:

```go
Qiniu qiniuConfig `toml:"qiniu"`
```

Add:

```go
type qiniuConfig struct {
	AccessKey             string `toml:"access_key"`
	SecretKey             string `toml:"secret_key"`
	Bucket                string `toml:"bucket"`
	UploadHost            string `toml:"upload_host"`
	PrivateDomain         string `toml:"private_domain"`
	UploadTokenTTLSeconds int    `toml:"upload_token_ttl_seconds"`
	DownloadURLTTLSeconds int    `toml:"download_url_ttl_seconds"`
}
```

Copy non-empty TOML values into `cfg` before env parsing.

- [ ] **Step 5: Implement asset domain**

Add errors:

```go
var (
	ErrInvalidAssetType      = errors.New("invalid asset type")
	ErrUnsupportedAssetType  = errors.New("unsupported asset type")
	ErrInvalidMimeType       = errors.New("invalid mime type")
	ErrInvalidFileSize       = errors.New("invalid file size")
	ErrAssetStorageNotReady  = errors.New("asset storage not configured")
	ErrAssetOwnership        = errors.New("asset ownership mismatch")
	ErrAssetNotFound         = errors.New("asset not found")
)
```

Add interfaces:

```go
type UploadSigner interface {
	UploadToken(bucket string, objectKey string, ttl time.Duration) (string, error)
}

type DownloadSigner interface {
	PrivateURL(domain string, objectKey string, ttl time.Duration) (string, error)
}
```

Add service constructor:

```go
type ServiceOptions struct {
	Bucket        string
	UploadHost    string
	PrivateDomain string
	UploadTTL     time.Duration
	DownloadTTL   time.Duration
	UploadSigner  UploadSigner
	DownloadSigner DownloadSigner
}

func NewServiceWithOptions(repo Repository, options ServiceOptions) *Service
func NewServiceFromConfig(repo Repository, cfg *config.Config) *Service
```

Keep `NewService(repo)` for existing onboarding tests.

Supported asset types:

```go
var assetScopes = map[string]string{
	"wardrobe_item_photo": "wardrobe",
	"onboarding_photo": "onboarding",
	"style_reference": "style-reference",
	"chat_image": "chat",
}
```

Add max image size `10 * 1024 * 1024`, MIME validation for `image/jpeg`, `image/png`, `image/webp`, `image/heic`, and extension normalization.

- [ ] **Step 6: Implement repository additions**

Extend repository capability with optional interface:

```go
type uploadRepository interface {
	Create(ctx context.Context, item Asset) (Asset, error)
	FindByPublicID(ctx context.Context, publicID string) (Asset, error)
}
```

`ConfirmUpload` should:

1. call `FindByPublicID`;
2. if found and same owner/bucket/key, return existing with URL;
3. if found but mismatch, return `ErrAssetOwnership`;
4. if not found, call `Create`;
5. store metadata with `upload_source`, `confirmed_at`, `width`, `height`.

- [ ] **Step 7: Implement routes**

Create `asset.RegisterUserRoutesWithService(group, service, logger)`:

```go
group.POST("/upload-token", handler.CreateUploadToken)
group.POST("/confirm", handler.ConfirmUpload)
```

Map errors:

- `ErrUnsupportedAssetType` -> 400 `asset.unsupported_asset_type`
- `ErrInvalidMimeType` -> 400 `asset.invalid_mime_type`
- `ErrInvalidFileSize` -> 400 `asset.invalid_file_size`
- `ErrAssetStorageNotReady` -> 500 `asset.storage_not_configured`
- `ErrAssetOwnership` -> 403 `asset.forbidden`
- default -> 500 `asset.request_failed`

Mount in `server/internal/app/user/router.go`:

```go
asset.RegisterUserRoutes(protected.Group("/assets"), deps)
```

- [ ] **Step 8: Run tests to verify GREEN**

Run:

```bash
cd server && go test ./internal/domain/asset ./internal/infra/config
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add server/go.mod server/go.sum server/internal/infra/config/config.go server/internal/domain/asset server/internal/app/user/router.go
git commit -m "feat(server): 添加公共资产七牛上传接口"
```

---

### Task 2: 衣橱返回私有图片短期 URL

**Files:**
- Modify: `server/internal/domain/wardrobe/service.go`
- Modify: `server/internal/domain/wardrobe/routes.go`
- Modify: `server/internal/domain/wardrobe/routes_test.go`

- [ ] **Step 1: Add failing wardrobe route test**

Add a fake URL signer/service dependency and a route test:

```go
func TestListItemsSignsPrimaryImageURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteMemoryWardrobeRepo()
	repo.add(wardrobe.Item{
		PublicID: "wdi_owned", UserID: 12, Name: "米白衬衫", Category: "top",
		RecommendationStatus: wardrobe.RecommendationStatusNormal, Status: wardrobe.StatusActive, IsCore: true,
		PrimaryImage: &wardrobe.Image{AssetPublicID: "ast_img", ObjectKey: "users/12/wardrobe/ast_img.jpg"},
	})
	service := wardrobe.NewService(repo)
	service.SetImageURLSigner(fakeWardrobeImageSigner{})
	router := newWardrobeRouteTestRouterWithService(service)

	request := httptest.NewRequest(http.MethodGet, "/api/user/wardrobe/items", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
		Data struct { Items []wardrobe.Item `json:"items"` } `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Items[0].PrimaryImage.URL != "https://assets.example.com/users/12/wardrobe/ast_img.jpg?e=1&token=fake" {
		t.Fatalf("expected signed URL, got %#v", body.Data.Items[0].PrimaryImage)
	}
}
```

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd server && go test ./internal/domain/wardrobe
```

Expected: FAIL because wardrobe service cannot sign primary image URLs.

- [ ] **Step 3: Implement URL enrichment**

Add a small interface in wardrobe:

```go
type ImageURLSigner interface {
	SignedURL(objectKey string) string
}
```

Add to `Service`:

```go
imageURLSigner ImageURLSigner

func (s *Service) SetImageURLSigner(signer ImageURLSigner) {
	s.imageURLSigner = signer
}
```

In `ListItems`, `CreateItem`, and `UpdateItem`, call:

```go
func (s *Service) withSignedImageURLs(items []Item) []Item
func (s *Service) withSignedImageURL(item Item) Item
```

If signer is nil, return unchanged. If `PrimaryImage.ObjectKey` is empty, return unchanged.

In `wardrobe.RegisterUserRoutes`, build asset service from deps and adapt it:

```go
assetService := asset.NewServiceFromConfig(asset.NewMySQLRepository(deps.DB), deps.Config)
service.SetImageURLSigner(assetService)
```

Asset service should expose:

```go
func (s *Service) SignedURL(objectKey string) string
```

- [ ] **Step 4: Run tests to verify GREEN**

Run:

```bash
cd server && go test ./internal/domain/wardrobe ./internal/domain/asset
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/domain/wardrobe server/internal/domain/asset server/internal/app/user/router.go
git commit -m "feat(server): 为衣橱主图返回短期访问地址"
```

---

### Task 3: 小程序公共资产上传工具

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/scripts/verify-api-client.js`

- [ ] **Step 1: Add failing API client tests**

Extend export assertion:

```js
[
  "createAssetUploadToken",
  "confirmAssetUpload",
  "uploadAssetToQiniu"
].forEach((name) => {
  assert(typeof api[name] === "function", `api.js should export ${name}`);
});
```

Add request tests:

```js
const assetCalls = [];
await withGlobals({
  getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
  wx: {
    getStorageSync() { return "asset_token"; },
    request(options) {
      assetCalls.push(["request", options]);
      options.success({
        statusCode: 200,
        data: { code: "ok", data: { asset_public_id: "ast_1", object_key: "users/12/wardrobe/ast_1.jpg", upload_token: "up", upload_url: "https://upload.qiniup.com" } }
      });
    },
    uploadFile(options) {
      assetCalls.push(["uploadFile", options]);
      options.success({ statusCode: 200, data: JSON.stringify({ key: "users/12/wardrobe/ast_1.jpg" }) });
    }
  }
}, async () => {
  await api.createAssetUploadToken({ asset_type: "wardrobe_item_photo", mime_type: "image/jpeg", file_size: 10, file_ext: ".jpg" });
});
```

Add `uploadAssetToQiniu` test with a `wx.request` sequence: first token response, then confirm response; assert `wx.uploadFile` formData contains `token` and `key`.

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd miniapp && npm run verify:api-client
```

Expected: FAIL because upload functions are not exported.

- [ ] **Step 3: Implement API functions**

Add:

```js
function createAssetUploadToken(data) {
  return authorizedRequest({
    path: "/api/user/assets/upload-token",
    method: "POST",
    data
  });
}

function confirmAssetUpload(data) {
  return authorizedRequest({
    path: "/api/user/assets/confirm",
    method: "POST",
    data
  });
}
```

Add helpers:

```js
function fileExtFromPath(path) {
  const match = String(path || "").match(/\\.[a-zA-Z0-9]+(?=\\?|#|$)/);
  return match ? match[0].toLowerCase() : ".jpg";
}

function mimeTypeFromFile(file) {
  if (file && file.type && file.type.startsWith("image/")) return file.type;
  const ext = fileExtFromPath(file && (file.url || file.path || file.tempFilePath || file.name));
  if (ext === ".png") return "image/png";
  if (ext === ".webp") return "image/webp";
  if (ext === ".heic") return "image/heic";
  return "image/jpeg";
}

function uploadFileToQiniu(uploadURL, filePath, formData) {
  return new Promise((resolve, reject) => {
    wx.uploadFile({
      url: uploadURL,
      filePath,
      name: "file",
      formData,
      success(response) {
        if (response.statusCode >= 200 && response.statusCode < 300) resolve(response);
        else reject(new ApiError("图片上传失败", { code: "asset.upload_failed", statusCode: response.statusCode, data: response.data }));
      },
      fail(error) {
        reject(new ApiError(error && error.errMsg ? error.errMsg : "图片上传失败", { code: "asset.upload_failed", data: error }));
      }
    });
  });
}
```

`uploadAssetToQiniu(file, options)` must require `options.assetType`, call token endpoint, upload with `formData: { token: token.upload_token, key: token.object_key }`, then call confirm.

- [ ] **Step 4: Run test to verify GREEN**

Run:

```bash
cd miniapp && npm run verify:api-client
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add miniapp/utils/api.js miniapp/scripts/verify-api-client.js
git commit -m "feat(miniapp): 添加公共资产上传客户端"
```

---

### Task 4: 衣橱弹窗接入公共上传

**Files:**
- Modify: `miniapp/pages/wardrobe/wardrobe.json`
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxss`
- Modify: `miniapp/pages/wardrobe-detail/wardrobe-detail.json`
- Modify: `miniapp/pages/wardrobe-detail/wardrobe-detail.js`
- Modify: `miniapp/pages/wardrobe-detail/wardrobe-detail.wxml`
- Modify: `miniapp/pages/wardrobe-detail/wardrobe-detail.wxss`
- Modify: `miniapp/utils/wardrobe.js`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: Add failing integration tests**

Update `verify-miniapp-api-integration.js`:

- Replace old assertions that require `primary_asset_public_id` input with assertions that require `t-upload`.
- Assert wardrobe and detail JSON register:

```js
assert(wardrobeJson.usingComponents["t-upload"], "wardrobe should register TDesign upload");
assert(detailJson.usingComponents["t-upload"], "wardrobe detail should register TDesign upload");
```

- Add fake API:

```js
uploadAssetToQiniu: async (file, options) => {
  wardrobeApiCalls.push(["upload", file, options]);
  return { asset_public_id: "ast_uploaded", url: "https://assets.example.com/uploaded.jpg", object_key: "users/12/wardrobe/ast_uploaded.jpg" };
}
```

- Assert:

```js
assert(typeof wardrobe.config.handleWardrobeImageUpload === "function", "wardrobe should upload selected image");
assert(typeof wardrobe.config.handleWardrobeImageRemove === "function", "wardrobe should remove selected image");
```

- Simulate upload:

```js
await wardrobe.config.handleWardrobeImageUpload.call(wardrobeInstance, {
  detail: { files: [{ url: "tmp://shirt.jpg", size: 1234, type: "image" }] }
});
assert(wardrobeInstance.data.draft.primary_asset_public_id === "ast_uploaded", "upload should store asset public id");
assert(wardrobeInstance.data.imageFiles[0].url === "https://assets.example.com/uploaded.jpg", "upload should show returned url");
```

- Set `imageUploading: true`, call `handleSaveItem`, assert create/update not called and `errorMessage` says wait for upload.

- Detail page: open edit with `primary_image.url`, assert image file initialized.

- [ ] **Step 2: Run test to verify RED**

Run:

```bash
cd miniapp && npm run verify:api-integration
```

Expected: FAIL because upload handlers/components do not exist.

- [ ] **Step 3: Implement shared wardrobe image state helpers**

In `miniapp/utils/wardrobe.js`, add:

```js
function imageFilesFromItem(item) {
  const decorated = decorateWardrobeItem(item);
  if (!decorated.primaryImageSrc) return [];
  return [{
    url: decorated.primaryImageSrc,
    type: "image",
    name: decorated.name || "衣服图片",
    status: "done",
    asset_public_id: decorated.primary_image && decorated.primary_image.asset_public_id
  }];
}

function imageFilesFromAsset(asset) {
  if (!asset || !asset.asset_public_id) return [];
  return [{
    url: asset.url || asset.object_key || "",
    type: "image",
    name: "衣服图片",
    status: "done",
    asset_public_id: asset.asset_public_id
  }];
}
```

Export both helpers.

- [ ] **Step 4: Register TDesign upload components**

In both page JSON files:

```json
{
  "usingComponents": {
    "t-upload": "/miniprogram_npm/tdesign-miniprogram/upload/upload"
  }
}
```

Merge with existing JSON, do not remove page style settings.

- [ ] **Step 5: Implement wardrobe page upload state**

Add data:

```js
imageFiles: [],
imageUploading: false,
imageUploadError: ""
```

On `handleOpenCreate`, set `imageFiles: []`, `imageUploading: false`, `imageUploadError: ""`.

On `handleEditItem`, set `imageFiles: imageFilesFromItem(item)`.

On close, clear image state.

Add:

```js
async handleWardrobeImageUpload(event) {
  const selected = event.detail && event.detail.files && event.detail.files[0]
    ? event.detail.files[0]
    : event.detail && event.detail.currentSelectedFiles && event.detail.currentSelectedFiles[0] && event.detail.currentSelectedFiles[0][0];
  if (!selected) return {};
  this.setData({ imageUploading: true, imageUploadError: "", imageFiles: [Object.assign({}, selected, { status: "loading" })] });
  try {
    const uploaded = await api.uploadAssetToQiniu(selected, { assetType: "wardrobe_item_photo" });
    const draft = Object.assign({}, this.data.draft, { primary_asset_public_id: uploaded.asset_public_id });
    this.setData({ draft, imageUploading: false, imageFiles: imageFilesFromAsset(uploaded), imageUploadError: "" });
    return uploaded;
  } catch (error) {
    const message = error && error.message ? error.message : "图片上传失败";
    this.setData({ imageUploading: false, imageUploadError: message, imageFiles: [Object.assign({}, selected, { status: "failed" })] });
    return Promise.reject(error);
  }
}

handleWardrobeImageRemove() {
  this.setData({
    imageFiles: [],
    imageUploadError: "",
    draft: Object.assign({}, this.data.draft, { primary_asset_public_id: "" })
  });
}
```

Before save:

```js
if (this.data.imageUploading) {
  this.setData({ errorMessage: "图片还在上传，请稍后再保存" });
  return;
}
```

- [ ] **Step 6: Implement detail page upload state**

Mirror Step 5 in `wardrobe-detail.js`. `handleOpenEdit` initializes `imageFiles: imageFilesFromItem(this.data.item)`. Save blocking and remove behavior match wardrobe page.

- [ ] **Step 7: Replace WXML input with upload component**

Replace the label group for “主图资产 ID” in both pages with:

```xml
<view class="field-group image-field">
  <text class="field-label">主图</text>
  <t-upload
    files="{{imageFiles}}"
    max="{{1}}"
    mediaType="{{['image']}}"
    source="media"
    gridConfig="{{imageGridConfig}}"
    sizeLimit="{{imageSizeLimit}}"
    bind:success="handleWardrobeImageUpload"
    bind:add="handleWardrobeImageUpload"
    bind:remove="handleWardrobeImageRemove"
  />
  <text wx:if="{{imageUploadError}}" class="error-text">{{imageUploadError}}</text>
</view>
```

If TDesign fires both `add` and `success`, guard in JS against duplicate upload by ignoring a second event while `imageUploading` is true.

- [ ] **Step 8: Add upload styles**

Add compact styling:

```css
.image-field {
  grid-column: 1 / -1;
}

.image-field .error-text {
  margin-top: 8rpx;
}
```

Keep the existing restrained wardrobe visual design.

- [ ] **Step 9: Run tests to verify GREEN**

Run:

```bash
cd miniapp && npm run verify:api-integration && npm run verify:api-client
```

Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add miniapp/pages/wardrobe miniapp/pages/wardrobe-detail miniapp/utils/wardrobe.js miniapp/scripts/verify-miniapp-api-integration.js
git commit -m "feat(miniapp): 衣橱弹窗接入图片上传"
```

---

### Task 5: Full verification and integration cleanup

**Files:**
- Any files touched by Tasks 1-4 if final cleanup is needed.

- [ ] **Step 1: Run backend tests**

```bash
cd server && go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run miniapp verification**

```bash
cd miniapp && npm run verify:api-client
cd miniapp && npm run verify:api-integration
cd miniapp && npm run verify:report-page
cd miniapp && npm run verify:today-ui
cd miniapp && npm run verify:tdesign-icon-font
```

Expected: PASS.

- [ ] **Step 3: Run diff hygiene**

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; status only contains intentional committed changes or is clean.

- [ ] **Step 4: Final commit if cleanup was needed**

If cleanup changed files:

```bash
git add <changed-files>
git commit -m "chore: 完成公共资产上传集成验证"
```

If no cleanup changed files, do not create an empty commit.

