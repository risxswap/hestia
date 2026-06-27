# 核心衣橱管理页 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将衣橱页从“报告缺口展示页”升级为核心单品管理页，支持后端单品 CRUD、推荐状态、主图关联和小程序核心单品板。

**Architecture:** 后端沿用现有 domain 分层，在 `server/internal/domain/wardrobe` 内补齐 model/service/repo/handler/routes，并在 user router 注册 `/api/user/wardrobe`。小程序沿用现有 Page + `miniapp/utils/api.js` 模式，衣橱页直接调用 wardrobe API，同时保留从最新报告读取缺口的轻量能力。

**Tech Stack:** Go 1.25、Gin、sqlx/MySQL、微信小程序原生 WXML/WXSS/JS、Node 验证脚本。

---

## File Structure

- Modify: `server/internal/infra/migration/mysql/001_init_schema.sql`
  - 给 `wardrobe_items` 增加 `recommendation_status` 字段。
- Modify: `server/internal/domain/wardrobe/model.go`
  - 扩展 Item/Input/Response 类型，增加推荐状态、场景标签、主图结构和查询/更新参数。
- Modify: `server/internal/domain/wardrobe/service.go`
  - 增加列表、新增、编辑、软删除、建议上下文筛选/排序能力。
- Modify: `server/internal/domain/wardrobe/repo.go`
  - 增加 MySQL 查询、按用户查找、更新、软删除、主图关联。
- Create: `server/internal/domain/wardrobe/handler.go`
  - 处理 HTTP 请求、校验用户上下文、返回统一响应。
- Create: `server/internal/domain/wardrobe/routes.go`
  - 注册 `/items` 列表、新增、编辑、删除路由。
- Create: `server/internal/domain/wardrobe/service_test.go`
  - service 层 TDD 覆盖推荐状态、排序、软删除语义。
- Create/Modify: `server/internal/domain/wardrobe/routes_test.go`
  - route 层 TDD 覆盖请求校验、鉴权隔离、响应结构。
- Modify: `server/internal/app/user/router.go`
  - 注册 wardrobe 路由组。
- Modify: `miniapp/utils/api.js`
  - 增加 `getWardrobeItems`、`createWardrobeItem`、`updateWardrobeItem`、`deleteWardrobeItem`。
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
  - 从缺口展示页改为核心单品板页面逻辑。
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
  - 新页面结构：顶部摘要、筛选、单品卡片、表单、建议补齐。
- Modify: `miniapp/pages/wardrobe/wardrobe.wxss`
  - 页面样式沿用现有温暖浅色卡片体系。
- Modify: `miniapp/scripts/verify-api-client.js`
  - 验证新增 API client。
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`
  - 验证衣橱页加载、筛选、新增、编辑、删除、缺口展示。

---

### Task 1: 后端模型、迁移与服务行为

**Files:**
- Modify: `server/internal/infra/migration/mysql/001_init_schema.sql`
- Modify: `server/internal/domain/wardrobe/model.go`
- Modify: `server/internal/domain/wardrobe/service.go`
- Create: `server/internal/domain/wardrobe/service_test.go`

- [ ] **Step 1: Write failing service tests**

Create `server/internal/domain/wardrobe/service_test.go` with these tests before production changes:

```go
package wardrobe

import (
	"context"
	"testing"
	"time"
)

type captureWardrobeRepo struct {
	created []Item
	items   []Item
}

func (r *captureWardrobeRepo) CreateCoreItems(_ context.Context, items []Item) ([]Item, error) {
	r.created = append(r.created, items...)
	return items, nil
}

func (r *captureWardrobeRepo) ListItems(_ context.Context, userID int64, filter ListFilter) ([]Item, error) {
	var result []Item
	for _, item := range r.items {
		if item.UserID != userID || item.Status == StatusDeleted {
			continue
		}
		if filter.Category != "" && item.Category != filter.Category {
			continue
		}
		if filter.RecommendationStatus != "" && item.RecommendationStatus != filter.RecommendationStatus {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *captureWardrobeRepo) CreateItem(_ context.Context, item Item, primaryAssetPublicID string) (Item, error) {
	r.created = append(r.created, item)
	return item, nil
}

func (r *captureWardrobeRepo) UpdateItem(_ context.Context, userID int64, publicID string, input UpdateInput) (Item, error) {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != StatusDeleted {
			if input.RecommendationStatus != nil {
				item.RecommendationStatus = *input.RecommendationStatus
			}
			return item, nil
		}
	}
	return Item{}, ErrItemNotFound
}

func (r *captureWardrobeRepo) SoftDeleteItem(_ context.Context, userID int64, publicID string) error {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID {
			return nil
		}
	}
	return ErrItemNotFound
}

func TestCreateCoreItemsDefaultsRecommendationStatusNormal(t *testing.T) {
	repo := &captureWardrobeRepo{}
	service := NewService(repo)

	_, err := service.CreateCoreItems(context.Background(), 12, []Input{{Name: " 米白衬衫 ", Category: "top"}})
	if err != nil {
		t.Fatalf("create core items: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected one created item, got %#v", repo.created)
	}
	if repo.created[0].RecommendationStatus != RecommendationStatusNormal {
		t.Fatalf("expected normal recommendation status, got %q", repo.created[0].RecommendationStatus)
	}
	if !repo.created[0].IsCore {
		t.Fatal("expected onboarding core item to stay core")
	}
}

func TestCreateItemRejectsInvalidRecommendationStatus(t *testing.T) {
	service := NewService(&captureWardrobeRepo{})
	_, err := service.CreateItem(context.Background(), 12, CreateInput{
		Name: "黑色西装",
		Category: "outerwear",
		RecommendationStatus: "hidden",
	})
	if err == nil {
		t.Fatal("expected invalid recommendation status error")
	}
	if err != ErrInvalidRecommendationStatus {
		t.Fatalf("expected ErrInvalidRecommendationStatus, got %v", err)
	}
}

func TestAdviceContextExcludesPausedAndSortsPreferredFirst(t *testing.T) {
	now := time.Now()
	repo := &captureWardrobeRepo{items: []Item{
		{PublicID: "wdi_normal", UserID: 12, Name: "蓝色牛仔裤", Category: "bottom", RecommendationStatus: RecommendationStatusNormal, Status: StatusActive, IsCore: true, UpdatedAt: now.Add(-time.Hour)},
		{PublicID: "wdi_paused", UserID: 12, Name: "红色长裙", Category: "dress", RecommendationStatus: RecommendationStatusPaused, Status: StatusActive, IsCore: true, UpdatedAt: now},
		{PublicID: "wdi_preferred", UserID: 12, Name: "米白衬衫", Category: "top", RecommendationStatus: RecommendationStatusPreferred, Status: StatusActive, IsCore: true, SceneTags: []string{"通勤"}, UpdatedAt: now.Add(-2 * time.Hour)},
	}}
	service := NewService(repo)

	items, err := service.AdviceContextItems(context.Background(), 12, AdviceContextFilter{Scene: "通勤", Limit: 10})
	if err != nil {
		t.Fatalf("advice context: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected paused item excluded, got %#v", items)
	}
	if items[0].PublicID != "wdi_preferred" {
		t.Fatalf("expected preferred scene match first, got %#v", items)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd server
go test ./internal/domain/wardrobe
```

Expected: FAIL because `ListFilter`、`CreateItem`、`CreateInput`、`RecommendationStatusNormal`、`AdviceContextItems` and related symbols do not exist.

- [ ] **Step 3: Implement minimal model and service**

Update `server/internal/domain/wardrobe/model.go` with:

```go
package wardrobe

import "time"

type Item struct {
	ID                   int64     `json:"-"`
	PublicID             string    `json:"public_id"`
	UserID               int64     `json:"-"`
	Name                 string    `json:"name"`
	Category             string    `json:"category"`
	Color                string    `json:"color,omitempty"`
	Silhouette           string    `json:"silhouette,omitempty"`
	Material             string    `json:"material,omitempty"`
	Season               string    `json:"season,omitempty"`
	SceneTags            []string  `json:"scene_tags,omitempty"`
	UserNotes            string    `json:"user_notes,omitempty"`
	IsCore               bool      `json:"is_core"`
	RecommendationStatus string    `json:"recommendation_status"`
	Status               string    `json:"status"`
	PrimaryImage         *Image    `json:"primary_image,omitempty"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

type Image struct {
	AssetPublicID string `json:"asset_public_id"`
	ObjectKey     string `json:"object_key,omitempty"`
	URL           string `json:"url,omitempty"`
}

type Input struct {
	Name       string
	Category   string
	Color      string
	Silhouette string
	Material   string
	Season     string
	Notes      string
}

type CreateInput struct {
	Name                 string   `json:"name"`
	Category             string   `json:"category"`
	Color                string   `json:"color"`
	Silhouette           string   `json:"silhouette"`
	Material             string   `json:"material"`
	Season               string   `json:"season"`
	SceneTags            []string `json:"scene_tags"`
	UserNotes            string   `json:"user_notes"`
	IsCore               *bool    `json:"is_core"`
	RecommendationStatus string   `json:"recommendation_status"`
	PrimaryAssetPublicID string   `json:"primary_asset_public_id"`
}

type UpdateInput struct {
	Name                 *string   `json:"name"`
	Category             *string   `json:"category"`
	Color                *string   `json:"color"`
	Silhouette           *string   `json:"silhouette"`
	Material             *string   `json:"material"`
	Season               *string   `json:"season"`
	SceneTags            *[]string `json:"scene_tags"`
	UserNotes            *string   `json:"user_notes"`
	IsCore               *bool     `json:"is_core"`
	RecommendationStatus *string   `json:"recommendation_status"`
	PrimaryAssetPublicID *string   `json:"primary_asset_public_id"`
}

type ListFilter struct {
	Category             string
	RecommendationStatus string
	IsCore               *bool
}

type AdviceContextFilter struct {
	Scene string
	Limit int
}
```

Update `server/internal/domain/wardrobe/service.go` so it defines:

```go
const (
	StatusActive  = "active"
	StatusDeleted = "deleted"

	RecommendationStatusPreferred = "preferred"
	RecommendationStatusNormal    = "normal"
	RecommendationStatusPaused    = "paused"
)
```

and adds `CreateItem`、`ListItems`、`UpdateItem`、`SoftDeleteItem`、`AdviceContextItems` with status validation, trimming, default `normal`, default `is_core=true`, and sorting preferred/core/scene matches first.

- [ ] **Step 4: Update migration**

In `server/internal/infra/migration/mysql/001_init_schema.sql`, add this column after `is_core`:

```sql
  `recommendation_status` varchar(32) NOT NULL DEFAULT 'normal',
```

- [ ] **Step 5: Run tests and verify GREEN**

Run:

```bash
cd server
go test ./internal/domain/wardrobe
```

Expected: PASS.

- [ ] **Step 6: Commit Task 1**

```bash
git add server/internal/infra/migration/mysql/001_init_schema.sql server/internal/domain/wardrobe/model.go server/internal/domain/wardrobe/service.go server/internal/domain/wardrobe/service_test.go
git commit -m "feat(server): 扩展核心衣橱服务模型"
```

---

### Task 2: 后端仓储、路由与 HTTP 行为

**Files:**
- Modify: `server/internal/domain/wardrobe/repo.go`
- Create: `server/internal/domain/wardrobe/handler.go`
- Create: `server/internal/domain/wardrobe/routes.go`
- Create/Modify: `server/internal/domain/wardrobe/routes_test.go`
- Modify: `server/internal/app/user/router.go`

- [ ] **Step 1: Write failing route tests**

Create `server/internal/domain/wardrobe/routes_test.go` with tests for:

```go
func TestListItemsReturnsOnlyCurrentUserItems(t *testing.T)
func TestCreateItemRejectsInvalidRecommendationStatus(t *testing.T)
func TestPatchItemUpdatesRecommendationStatus(t *testing.T)
func TestDeleteItemSoftDeletesCurrentUserItem(t *testing.T)
func TestDeleteItemReturnsNotFoundForOtherUserItem(t *testing.T)
```

Use a memory repo with users `12` and `99`, `auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})`, and assert response codes:

- list: `200`, body `data.items` excludes user 99.
- invalid create: `400`, body `code == "wardrobe.invalid_recommendation_status"`.
- patch: `200`, body `data.recommendation_status == "paused"`.
- delete own item: `200`, body `data.public_id == "wdi_owned"`.
- delete other user's item: `404`, body `code == "wardrobe.item_not_found"`.

- [ ] **Step 2: Run route tests and verify RED**

Run:

```bash
cd server
go test ./internal/domain/wardrobe -run 'Test(ListItems|CreateItem|PatchItem|DeleteItem)'
```

Expected: FAIL because routes/handler and repository methods are missing.

- [ ] **Step 3: Implement routes and handler**

Create `routes.go`:

```go
package wardrobe

import (
	"log/slog"

	baseapp "hestia/server/internal/app"

	"github.com/gin-gonic/gin"
)

func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo Repository
	var logger *slog.Logger
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
		logger = deps.Logger
	}
	RegisterUserRoutesWithService(group, NewService(repo), logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.GET("/items", handler.ListItems)
	group.POST("/items", handler.CreateItem)
	group.PATCH("/items/:public_id", handler.UpdateItem)
	group.DELETE("/items/:public_id", handler.DeleteItem)
}
```

Create `handler.go` with `ListItems`、`CreateItem`、`UpdateItem`、`DeleteItem`; use `auth.UserFromContext` and `response.OK/Error`. Map errors:

- `ErrItemNotFound` -> 404 `wardrobe.item_not_found`
- `ErrInvalidRecommendationStatus` -> 400 `wardrobe.invalid_recommendation_status`
- empty/invalid name -> 400 `wardrobe.invalid_item`
- unknown internal -> 500 `wardrobe.request_failed`

- [ ] **Step 4: Implement repository methods**

Update `repo.go` to:

- Insert `recommendation_status` and `scene_tags`.
- List by `user_id`, `deleted_at IS NULL`, `status <> 'deleted'`.
- Update only allowed fields.
- Soft delete with `status='deleted', deleted_at=CURRENT_TIMESTAMP(3)`.
- Manage `wardrobe_item_assets` primary relation when `primary_asset_public_id` is present.

Use JSON helpers similar to other repos. Do not build SQL from untrusted values without fixed field names.

- [ ] **Step 5: Register user route**

In `server/internal/app/user/router.go`, import wardrobe and register:

```go
wardrobe.RegisterUserRoutes(protected.Group("/wardrobe"), deps)
```

- [ ] **Step 6: Run tests and verify GREEN**

Run:

```bash
cd server
go test ./internal/domain/wardrobe ./internal/app/user
```

Expected: PASS.

- [ ] **Step 7: Commit Task 2**

```bash
git add server/internal/domain/wardrobe server/internal/app/user/router.go
git commit -m "feat(server): 添加核心衣橱用户接口"
```

---

### Task 3: 小程序 API client 与页面状态逻辑

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/scripts/verify-api-client.js`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: Write failing API client verification**

Extend `miniapp/scripts/verify-api-client.js` to assert these exported functions exist and call paths:

```js
assert(typeof api.getWardrobeItems === "function", "api should export getWardrobeItems");
assert(typeof api.createWardrobeItem === "function", "api should export createWardrobeItem");
assert(typeof api.updateWardrobeItem === "function", "api should export updateWardrobeItem");
assert(typeof api.deleteWardrobeItem === "function", "api should export deleteWardrobeItem");
```

The expected request paths are:

- `GET /api/user/wardrobe/items`
- `POST /api/user/wardrobe/items`
- `PATCH /api/user/wardrobe/items/wdi_test`
- `DELETE /api/user/wardrobe/items/wdi_test`

- [ ] **Step 2: Write failing wardrobe page verification**

Extend `miniapp/scripts/verify-miniapp-api-integration.js` wardrobe section to assert:

```js
assert(typeof wardrobe.config.loadWardrobe === "function", "wardrobe should load core wardrobe items");
assert(typeof wardrobe.config.handleCategoryFilter === "function", "wardrobe should filter by category");
assert(typeof wardrobe.config.handleOpenCreate === "function", "wardrobe should open create form");
assert(typeof wardrobe.config.handleSaveItem === "function", "wardrobe should save item");
assert(typeof wardrobe.config.handleDeleteItem === "function", "wardrobe should delete item");
```

Use fake API methods returning:

```js
getWardrobeItems: async () => ({ items: [
  { public_id: "wdi_shirt", name: "米白衬衫", category: "top", color: "米白", is_core: true, recommendation_status: "preferred", scene_tags: ["通勤"] },
  { public_id: "wdi_jeans", name: "直筒牛仔裤", category: "bottom", color: "蓝色", is_core: true, recommendation_status: "normal", scene_tags: ["日常"] }
] }),
getLatestReport: async () => ({ content_json: { wardrobe_gaps: ["浅色短外套"] } }),
createWardrobeItem: async (payload) => Object.assign({ public_id: "wdi_new", status: "active" }, payload),
updateWardrobeItem: async (publicID, payload) => Object.assign({ public_id: publicID, status: "active" }, payload),
deleteWardrobeItem: async () => ({ public_id: "wdi_shirt" })
```

Assert:

- `loadWardrobe` populates `items.length === 2`.
- preferred item appears in `priorityItems`.
- filtering category `top` leaves one visible item.
- save sends default `recommendation_status === "normal"` for new item.
- delete removes item from local list after API success.

- [ ] **Step 3: Run verification and verify RED**

Run:

```bash
cd miniapp
npm run verify:api-client
npm run verify:api-integration
```

Expected: FAIL because wardrobe API functions and page handlers do not exist.

- [ ] **Step 4: Implement API client**

In `miniapp/utils/api.js`, add:

```js
function getWardrobeItems(filters) {
  const params = filters || {};
  const query = Object.keys(params)
    .filter((key) => params[key] !== undefined && params[key] !== null && params[key] !== "")
    .map((key) => `${encodeURIComponent(key)}=${encodeURIComponent(params[key])}`)
    .join("&");
  return authorizedRequest({
    path: `/api/user/wardrobe/items${query ? `?${query}` : ""}`
  });
}

function createWardrobeItem(data) {
  return authorizedRequest({
    path: "/api/user/wardrobe/items",
    method: "POST",
    data
  });
}

function updateWardrobeItem(publicID, data) {
  return authorizedRequest({
    path: `/api/user/wardrobe/items/${publicID}`,
    method: "PATCH",
    data
  });
}

function deleteWardrobeItem(publicID) {
  return authorizedRequest({
    path: `/api/user/wardrobe/items/${publicID}`,
    method: "DELETE"
  });
}
```

Export all four functions.

- [ ] **Step 5: Implement wardrobe page state logic**

In `miniapp/pages/wardrobe/wardrobe.js`, replace gap-only logic with:

- `data.items`
- `data.visibleItems`
- `data.priorityItems`
- `data.gaps`
- `data.categoryOptions`
- `data.activeCategory`
- `data.editorVisible`
- `data.draft`
- `data.saving`
- `data.errorMessage`

Add pure helpers:

- `normalizeWardrobeItems(response)`
- `decorateWardrobeItem(item)`
- `filterItems(items, category)`
- `priorityItems(items)`
- `normalizeWardrobeGaps(report)`
- `buildPayload(draft)`

Handlers:

- `loadWardrobe`
- `handleCategoryFilter`
- `handleOpenCreate`
- `handleEditItem`
- `handleDraftInput`
- `handleSceneInput`
- `handleRecommendationStatus`
- `handleCoreToggle`
- `handleSaveItem`
- `handleDeleteItem`

Keep `loadWardrobeGaps` as a compatibility wrapper if existing verification still calls it.

- [ ] **Step 6: Run verification and verify GREEN**

Run:

```bash
cd miniapp
npm run verify:api-client
npm run verify:api-integration
```

Expected: PASS.

- [ ] **Step 7: Commit Task 3**

```bash
git add miniapp/utils/api.js miniapp/pages/wardrobe/wardrobe.js miniapp/scripts/verify-api-client.js miniapp/scripts/verify-miniapp-api-integration.js
git commit -m "feat(miniapp): 添加核心衣橱页面逻辑"
```

---

### Task 4: 小程序衣橱页面 WXML/WXSS

**Files:**
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxss`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: Write failing markup verification**

Extend `miniapp/scripts/verify-miniapp-api-integration.js` to read `pages/wardrobe/wardrobe.wxml` and assert it includes:

```js
assert(wardrobeMarkup.includes("核心衣橱"), "wardrobe page should show core wardrobe title");
assert(wardrobeMarkup.includes("priorityItems"), "wardrobe page should render priority item summary");
assert(wardrobeMarkup.includes("visibleItems"), "wardrobe page should render filtered wardrobe cards");
assert(wardrobeMarkup.includes("建议补齐"), "wardrobe page should keep wardrobe gaps section");
assert(wardrobeMarkup.includes("handleSaveItem"), "wardrobe page should bind save item action");
```

- [ ] **Step 2: Run verification and verify RED**

Run:

```bash
cd miniapp
npm run verify:api-integration
```

Expected: FAIL until WXML is updated.

- [ ] **Step 3: Implement WXML**

Update `miniapp/pages/wardrobe/wardrobe.wxml` with these sections:

- top hero with title, count and add button.
- priority summary using `priorityItems`.
- category chip row using `categoryOptions`.
- empty state when `!visibleItems.length`.
- card grid using `visibleItems`, primary image when available, status labels, scene tags.
- editor panel/form when `editorVisible`.
- gaps section using `gaps`.

Use existing miniapp primitives only; do not add a new UI library.

- [ ] **Step 4: Implement WXSS**

Update `miniapp/pages/wardrobe/wardrobe.wxss` using existing app palette:

- background remains inherited from `app.wxss`.
- cards use `#fffdf8`, border `#e6ded0`, radius around `12rpx`.
- stable grid: `grid-template-columns: repeat(2, minmax(0, 1fr))`.
- no oversized marketing hero.
- text must wrap inside buttons/cards on small screens.

- [ ] **Step 5: Run verification and verify GREEN**

Run:

```bash
cd miniapp
npm run verify:api-integration
```

Expected: PASS.

- [ ] **Step 6: Commit Task 4**

```bash
git add miniapp/pages/wardrobe/wardrobe.wxml miniapp/pages/wardrobe/wardrobe.wxss miniapp/scripts/verify-miniapp-api-integration.js
git commit -m "feat(miniapp): 完善核心衣橱页面"
```

---

### Task 5: 全量验证与收尾

**Files:**
- Modify only if verification exposes defects in touched files.

- [ ] **Step 1: Run full backend tests**

Run:

```bash
cd server
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run full miniapp verification**

Run:

```bash
cd miniapp
npm run verify:api-client
npm run verify:api-integration
npm run verify:report-page
npm run verify:today-ui
```

Expected: PASS.

- [ ] **Step 3: Inspect git status**

Run:

```bash
git status --short
```

Expected: no unrelated untracked files. If verification fixes were necessary, commit only the actual touched implementation/test files. For example, if the final fix only touches wardrobe page logic and its verifier, run:

```bash
git add miniapp/pages/wardrobe/wardrobe.js miniapp/scripts/verify-miniapp-api-integration.js
git commit -m "fix: 修复核心衣橱验证问题"
```

- [ ] **Step 4: Final review**

Review these requirements against code:

- `recommendation_status=paused` is visible in wardrobe page but excluded from advice context.
- `status=deleted` is excluded from list and advice context.
- wardrobe gaps remain separate from owned items.
- miniapp does not promise shopping links or guaranteed improvement.
- existing onboarding core item creation still works.

Run final commands again if any fix was made.
