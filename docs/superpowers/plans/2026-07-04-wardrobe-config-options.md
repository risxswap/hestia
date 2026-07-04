# 衣橱枚举配置下拉 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将衣橱分类、材质、季节、廓形改为由 `system_configs` 驱动的下拉枚举，并在后端保存时强校验。

**Architecture:** 后端在 wardrobe 领域增加选项模型、配置仓库读取和 `/options` 接口；`CreateItem`/`UpdateItem` 复用同一套选项校验。小程序加载 options 后渲染 picker，识别回填通过枚举匹配把 label/value 归一为保存 value。

**Tech Stack:** Go/Gin/MySQL migration，微信小程序 WXML/JavaScript，现有 Node 验证脚本。

---

### Task 1: 默认枚举配置 migration

**Files:**
- Create: `server/internal/infra/migration/mysql/003_wardrobe_item_options.sql`
- Modify: `server/internal/infra/migration/migration_test.go`

- [ ] 写失败测试：断言迁移包含 `wardrobe.item_options` 和四个 key。
- [ ] 运行 `cd server && go test ./internal/infra/migration -run TestApplyMySQLSchemaExecutesInitialSchemaStatements -count=1`，预期失败。
- [ ] 增加 migration，写入 categories/materials/seasons/silhouettes 默认 JSON。
- [ ] 运行迁移测试，预期通过。

### Task 2: 后端 options 读取和强校验

**Files:**
- Modify: `server/internal/domain/wardrobe/model.go`
- Modify: `server/internal/domain/wardrobe/repo.go`
- Modify: `server/internal/domain/wardrobe/service.go`
- Modify: `server/internal/domain/wardrobe/service_test.go`

- [ ] 写失败测试：`CreateItem` 拒绝不在配置中的 category；允许空 material/season/silhouette；拒绝非空无效 material。
- [ ] 写失败测试：配置仓库从 `system_configs` JSON 读取四组 options。
- [ ] 运行 `cd server && go test ./internal/domain/wardrobe -run 'TestCreateItemRejectsInvalidConfiguredOption|TestMySQLRepositoryListWardrobeOptions' -count=1`，预期失败。
- [ ] 实现 `WardrobeOptions`、`OptionItem`、`ListWardrobeOptions`、默认兜底和 `validateConfiguredOptions`。
- [ ] 运行上述测试，预期通过。

### Task 3: 后端 options 路由

**Files:**
- Modify: `server/internal/domain/wardrobe/handler.go`
- Modify: `server/internal/domain/wardrobe/routes.go`
- Modify: `server/internal/domain/wardrobe/routes_test.go`

- [ ] 写失败测试：`GET /api/user/wardrobe/options` 返回四组选项；保存无效枚举返回 `wardrobe.invalid_option`。
- [ ] 运行 `cd server && go test ./internal/domain/wardrobe -run 'TestGetWardrobeOptionsRoute|TestCreateItemRejectsInvalidOptionRoute' -count=1`，预期失败。
- [ ] 实现 handler 和路由错误映射。
- [ ] 运行上述测试，预期通过。

### Task 4: 小程序 API、工具和 picker

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/utils/wardrobe.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
- Modify: `miniapp/scripts/verify-api-client.js`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] 写失败验证：`getWardrobeOptions` 路径、picker 结构、选项加载、选择 handler、识别 label/value 匹配。
- [ ] 运行 `cd miniapp && npm run verify:api-client && npm run verify:api-integration`，预期失败。
- [ ] 实现 API、默认 options、picker label 辅助、选择 handler 和识别回填枚举匹配。
- [ ] 运行小程序验证，预期通过。

### Task 5: 全量验证

- [ ] 运行 `cd server && go test ./internal/domain/wardrobe ./internal/infra/migration -count=1`。
- [ ] 运行 `cd miniapp && npm run verify:api-client && npm run verify:api-integration`。
- [ ] 运行 `git status --short`，确认只新增/修改本次相关文件，且不回滚已有用户改动。
