# 新增衣服图片识别回填 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增或替换衣服主图后，调用图片识别接口，把识别字段只回填到空表单字段。

**Architecture:** 后端在 wardrobe 领域增加图片识别接口和可注入的识别器抽象，接口只返回候选字段，不创建或更新单品。小程序把上传区域前置，上传成功后调用识别接口，并用前端合并函数保护用户已填写内容。

**Tech Stack:** Go/Gin 后端，微信小程序 JavaScript/WXML，现有 Node 验证脚本。

---

### Task 1: 后端识别领域行为

**Files:**
- Modify: `server/internal/domain/wardrobe/model.go`
- Modify: `server/internal/domain/wardrobe/service.go`
- Modify: `server/internal/domain/wardrobe/service_test.go`

- [ ] **Step 1: 写失败测试**

在 `service_test.go` 增加测试：调用 `RecognizeItemImage` 时会修剪字段、归一化场景、保留置信度。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/domain/wardrobe -run TestRecognizeItemImage -count=1`

Expected: FAIL，提示 `RecognizeItemImage` 或相关类型不存在。

- [ ] **Step 3: 最小实现**

增加 `RecognizeImageInput`、`RecognizedItemFields`、`WardrobeImageRecognizer`，在 `Service` 上增加 `SetImageRecognizer` 和 `RecognizeItemImage`。实现只做参数校验、调用识别器、字段修剪和场景去空。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./internal/domain/wardrobe -run TestRecognizeItemImage -count=1`

Expected: PASS。

### Task 2: 后端识别路由

**Files:**
- Modify: `server/internal/domain/wardrobe/handler.go`
- Modify: `server/internal/domain/wardrobe/routes.go`
- Modify: `server/internal/domain/wardrobe/routes_test.go`

- [ ] **Step 1: 写失败路由测试**

新增测试覆盖 `POST /api/user/wardrobe/items/recognize` 成功返回字段，以及缺少 `asset_public_id` 返回 `wardrobe.invalid_primary_asset`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/domain/wardrobe -run 'TestRecognizeItemImageRoute|TestRecognizeItemImageRejectsMissingAsset' -count=1`

Expected: FAIL，提示路由 404 或 handler 不存在。

- [ ] **Step 3: 最小实现**

在 `Handler` 增加 `RecognizeItemImage`，在 `RegisterUserRoutesWithService` 注册 `POST /items/recognize`。错误映射复用 `ErrInvalidPrimaryAsset` 和 `ErrRepositoryUnsupported`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./internal/domain/wardrobe -run 'TestRecognizeItemImageRoute|TestRecognizeItemImageRejectsMissingAsset' -count=1`

Expected: PASS。

### Task 3: 小程序 API 和回填合并

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/utils/wardrobe.js`
- Modify: `miniapp/scripts/verify-api-client.js`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: 写失败验证**

验证 API 客户端存在 `recognizeWardrobeItemImage`，并断言该方法请求 `/api/user/wardrobe/items/recognize`。验证衣橱工具存在合并函数，且识别结果不会覆盖非空 `draft` 字段。

- [ ] **Step 2: 运行验证确认失败**

Run: `cd miniapp && npm run verify:api-client && npm run verify:api-integration`

Expected: FAIL，提示 API 方法或合并函数不存在。

- [ ] **Step 3: 最小实现**

在 `api.js` 增加 `recognizeWardrobeItemImage(assetPublicID)`。在 `wardrobe.js` 增加并导出 `mergeRecognizedFieldsIntoDraft(draft, recognized)`，只填空字段，`scene_tags` 只在 `sceneText` 和 `scene_tags` 都为空时回填。

- [ ] **Step 4: 运行验证确认通过**

Run: `cd miniapp && npm run verify:api-client && npm run verify:api-integration`

Expected: PASS。

### Task 4: 小程序上传后识别和布局前置

**Files:**
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
- Modify: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: 写失败验证**

验证 `wardrobePageConfig.data` 包含 `imageRecognizing` 和 `imageRecognizeError`。模拟上传成功后断言调用识别 API，并且已有 `draft.name` 不被覆盖。验证 WXML 中上传字段出现在名称输入前。

- [ ] **Step 2: 运行验证确认失败**

Run: `cd miniapp && npm run verify:api-integration`

Expected: FAIL，提示识别状态、调用或布局顺序缺失。

- [ ] **Step 3: 最小实现**

新增 `recognizeUploadedWardrobeImage(uploaded)` 方法，在上传成功后调用。识别期间设置 `imageRecognizing`，失败写 `imageRecognizeError`。调整 WXML，把上传块移动到 `form-grid` 前，并显示识别状态和错误。

- [ ] **Step 4: 运行验证确认通过**

Run: `cd miniapp && npm run verify:api-integration`

Expected: PASS。

### Task 5: 全量验证

**Files:**
- Verify only

- [ ] **Step 1: 运行后端领域测试**

Run: `cd server && go test ./internal/domain/wardrobe -count=1`

Expected: PASS。

- [ ] **Step 2: 运行小程序验证**

Run: `cd miniapp && npm run verify:api-client && npm run verify:api-integration`

Expected: PASS。

- [ ] **Step 3: 检查工作区**

Run: `git status --short`

Expected: 包含本次 spec、plan、wardrobe 后端、小程序 API/工具/页面/验证脚本；不新增无关未跟踪文件。
