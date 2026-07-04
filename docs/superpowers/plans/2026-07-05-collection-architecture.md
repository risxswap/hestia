# 私藏独立架构 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `私藏` 从 `wardrobe` 包装页重构为独立 `collection` 聚合页面和接口，同时保持衣服使用现有 `wardrobe` 领域。

**Architecture:** 服务端新增 `collection` 聚合领域，只提供 `/api/user/collection`，内部复用 `wardrobe.Service` 派生衣服数量和最近项。`hair`、`makeup`、`references` 作为 `/api/user` 下一级资源提供轻量空列表接口；小程序新增独立 `pages/collection/collection` tab 页面，`pages/wardrobe/wardrobe` 回到衣服页。

**Tech Stack:** Go + Gin 服务端领域路由，微信小程序 WXML/WXSS/JS，Node.js 验证脚本。

---

## File Structure

- `server/internal/domain/collection/*`
  - 新增聚合领域，提供 `GET /api/user/collection`。
  - 依赖已有 `wardrobe.Service`，不拥有明细数据。

- `server/internal/domain/hair/*`
  - 新增一级领域，第一阶段 `GET /api/user/hair` 返回空列表。

- `server/internal/domain/makeup/*`
  - 新增一级领域，第一阶段 `GET /api/user/makeup` 返回空列表。

- `server/internal/domain/reference/*`
  - 新增一级领域，第一阶段注册 `/api/user/references` 返回空列表。

- `server/internal/app/user/router.go`
  - 注册 `collection`、`hair`、`makeup`、`reference` 路由。

- `miniapp/pages/collection/*`
  - 新增私藏 tab 首页。

- `miniapp/pages/wardrobe/*`
  - 撤回上一轮过渡首页逻辑，恢复为衣服独立页。

- `miniapp/pages/hair/*`、`miniapp/pages/makeup/*`、`miniapp/pages/references/*`
  - 新增轻量空状态页。

- `miniapp/utils/api.js`
  - 新增 `getCollectionSummary()`、`getHairItems()`、`getMakeupItems()`、`getReferenceItems()`。

- `miniapp/scripts/*`
  - 新增或更新验证脚本，覆盖独立页面和独立接口。

---

### Task 1: 服务端 collection 聚合接口

**Files:**
- Create: `server/internal/domain/collection/model.go`
- Create: `server/internal/domain/collection/service.go`
- Create: `server/internal/domain/collection/handler.go`
- Create: `server/internal/domain/collection/routes.go`
- Create: `server/internal/domain/collection/routes_test.go`
- Modify: `server/internal/app/user/router.go`

TDD:

1. 先写 `routes_test.go`，断言 `GET /api/user/collection` 返回四个类型：`wardrobe,hair,makeup,references`，且不出现 `/collection/wardrobe` 这类子路由。
2. 运行：
   `cd server && go test ./internal/domain/collection -count=1`
   预期失败：包或路由不存在。
3. 实现 `collection` model/service/handler/routes。
4. 在 `server/internal/app/user/router.go` 注册 `collection.RegisterUserRoutes(protected.Group("/collection"), deps)`。
5. 运行：
   `cd server && go test ./internal/domain/collection -count=1`
   预期通过。

### Task 2: 服务端 hair/makeup/references 一级接口

**Files:**
- Create: `server/internal/domain/hair/routes.go`
- Create: `server/internal/domain/hair/routes_test.go`
- Create: `server/internal/domain/makeup/routes.go`
- Create: `server/internal/domain/makeup/routes_test.go`
- Create: `server/internal/domain/reference/routes.go`
- Create: `server/internal/domain/reference/routes_test.go`
- Modify: `server/internal/app/user/router.go`

TDD:

1. 写路由测试，分别断言：
   - `GET /api/user/hair`
   - `GET /api/user/makeup`
   - `GET /api/user/references`
   返回 `items: []` 和 `enabled: true`。
2. 运行：
   `cd server && go test ./internal/domain/hair ./internal/domain/makeup ./internal/domain/reference -count=1`
   预期失败。
3. 实现三个轻量领域路由。
4. 在 user router 注册：
   - `hair.RegisterUserRoutes(protected.Group("/hair"), deps)`
   - `makeup.RegisterUserRoutes(protected.Group("/makeup"), deps)`
   - `reference.RegisterUserRoutes(protected.Group("/references"), deps)`
5. 运行相同测试，预期通过。

### Task 3: 小程序 API client 和路由配置

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/app.json`
- Modify: `miniapp/scripts/verify-api-client.js`
- Modify: `miniapp/scripts/verify-private-tab-naming.js`

TDD:

1. 更新验证脚本，断言：
   - tab 第三个入口是 `pages/collection/collection`，文案 `私藏`。
   - `app.json` 注册 `pages/collection/collection`、`pages/hair/hair`、`pages/makeup/makeup`、`pages/references/references`。
   - API client 包含 `getCollectionSummary()`，请求 `/api/user/collection`。
   - API client 包含 `getHairItems()`、`getMakeupItems()`、`getReferenceItems()`，分别请求 `/api/user/hair`、`/api/user/makeup`、`/api/user/references`。
2. 运行：
   `npm --prefix miniapp run verify:api-client`
   `npm --prefix miniapp run verify:private-tab-naming`
   预期失败。
3. 实现 API client 和 `app.json`。
4. 运行相同验证，预期通过。

### Task 4: 小程序 collection 首页

**Files:**
- Create: `miniapp/pages/collection/collection.js`
- Create: `miniapp/pages/collection/collection.wxml`
- Create: `miniapp/pages/collection/collection.wxss`
- Create: `miniapp/pages/collection/collection.json`
- Modify: `miniapp/scripts/verify-private-tab-naming.js`

TDD:

1. 验证脚本断言 collection 页面：
   - 导入 `../../utils/api`。
   - 调用 `getCollectionSummary`。
   - 渲染 `private-type-grid`、`recent-private-section`。
   - 类型入口点击根据 `entry_path` 跳转。
2. 运行 `npm --prefix miniapp run verify:private-tab-naming`，预期失败。
3. 实现 collection 页面。
4. 运行验证，预期通过。

### Task 5: wardrobe 恢复为衣服页

**Files:**
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxss`
- Modify: `miniapp/pages/wardrobe/wardrobe.json`
- Modify: `miniapp/utils/wardrobe.js`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

TDD:

1. 更新验证，断言 wardrobe 页面：
   - 页面标题是 `衣服`。
   - 仍渲染衣服分类筛选和图库。
   - 不再包含 `private-type-grid`、`recent-private-section`、`privateTypeSummaries`、`activePrivateType`。
   - 继续使用 `/api/user/wardrobe/*`。
2. 运行 `npm --prefix miniapp run verify:api-integration`，预期失败。
3. 移除上一轮过渡私藏首页状态和 WXML，保留衣服页功能。
4. 运行验证，预期通过。

### Task 6: hair/makeup/references 小程序空状态页

**Files:**
- Create: `miniapp/pages/hair/hair.js`
- Create: `miniapp/pages/hair/hair.wxml`
- Create: `miniapp/pages/hair/hair.wxss`
- Create: `miniapp/pages/hair/hair.json`
- Create: `miniapp/pages/makeup/makeup.js`
- Create: `miniapp/pages/makeup/makeup.wxml`
- Create: `miniapp/pages/makeup/makeup.wxss`
- Create: `miniapp/pages/makeup/makeup.json`
- Create: `miniapp/pages/references/references.js`
- Create: `miniapp/pages/references/references.wxml`
- Create: `miniapp/pages/references/references.wxss`
- Create: `miniapp/pages/references/references.json`
- Modify: `miniapp/scripts/verify-private-tab-naming.js`

TDD:

1. 验证脚本断言三个页面存在并分别调用对应 API。
2. 运行验证，预期失败。
3. 实现三页空状态。
4. 运行验证，预期通过。

### Task 7: 最终验证

Run:

```bash
cd server && go test ./internal/domain/collection ./internal/domain/hair ./internal/domain/makeup ./internal/domain/reference ./internal/app/user -count=1
npm --prefix miniapp run verify:api-client
npm --prefix miniapp run verify:private-tab-naming
npm --prefix miniapp run verify:api-integration
npm --prefix miniapp run verify:today-ui
```

Expected: all pass.

---

## Self-Review

- Spec coverage:
  - collection 独立聚合接口：Task 1。
  - hair/makeup/references 一级接口：Task 2。
  - 衣服继续 wardrobe：Task 5。
  - 小程序独立 collection tab：Tasks 3-4。
  - 三个新空状态页面：Task 6。

- Placeholder scan:
  - No placeholder markers remain.

- Type consistency:
  - 路由名：`collection`、`wardrobe`、`hair`、`makeup`、`references`。
  - 页面路径：`collection`、`wardrobe`、`hair`、`makeup`、`references`。
