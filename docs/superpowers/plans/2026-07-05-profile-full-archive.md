# 完整用户档案 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将“我的档案”扩展为包含可选体重、形象要素字段和多角度本人照片的完整档案能力。

**Architecture:** 后端继续沿用 `profile` 域承载用户明确资料，新增 `profile_photos` 表保存照片引用与语义，图片上传复用现有 `files`/asset 上传链路。小程序 `pages/profile/edit` 拆成基础信息、照片档案和形象要素三个分区，通过 `api.js` 调用新增照片 CRUD。

**Tech Stack:** Go + Gin + sqlx + sqlmock；微信小程序原生 WXML/WXSS/JS；Node 验证脚本。

---

### Task 1: 后端档案字段与照片模型

**Files:**
- Modify: `server/internal/domain/profile/model.go`
- Modify: `server/internal/domain/profile/service.go`
- Modify: `server/internal/domain/profile/repo.go`
- Modify: `server/internal/domain/profile/handler.go`
- Modify: `server/internal/domain/profile/routes.go`
- Modify: `server/internal/domain/profile/routes_test.go`
- Modify: `server/internal/domain/asset/service.go`
- Create: `server/internal/infra/migration/mysql/010_profile_full_archive.sql`

- [ ] **Step 1: Write failing backend tests**

Add route/service/repository tests covering:
- `PATCH /api/user/profile` accepts `weight_kg`, `face_shape`, `upper_body_notes`, `lower_body_notes`, `size_notes`.
- weight below 20 or above 300 returns validation error.
- `POST /api/user/profile/photos` creates a profile photo from owned asset.
- invalid `photo_type` or `angle` returns validation error.
- `DELETE /api/user/profile/photos/:public_id` soft-deletes the photo.
- `GET /api/user/profile/summary` includes `profile_photos`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd server && go test ./internal/domain/profile ./internal/domain/asset`

Expected: FAIL because new fields, methods and routes are missing.

- [ ] **Step 3: Implement minimal backend**

Add patch types, validation, repository methods and routes. Add `profile_photo` to asset type normalization with scope `profile`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd server && go test ./internal/domain/profile ./internal/domain/asset`

Expected: PASS.

### Task 2: 小程序档案编辑页与 API

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/pages/profile/edit.js`
- Modify: `miniapp/pages/profile/edit.wxml`
- Modify: `miniapp/pages/profile/edit.wxss`
- Modify: `miniapp/scripts/verify-profile-page.js`

- [ ] **Step 1: Write failing miniapp verification**

Extend `verify-profile-page.js` to assert:
- draft hydrates `weight_kg`, `face_shape`, `upper_body_notes`, `lower_body_notes`, `size_notes`.
- payload includes new fields and sends `weight_kg` as number or `null`.
- profile photos group into `headshot`, `half_body`, `full_body`.
- API exports `createProfilePhoto`, `updateProfilePhoto`, `deleteProfilePhoto`, and upload flow can use `assetType: "profile_photo"`.

- [ ] **Step 2: Run verification to see it fail**

Run: `cd miniapp && npm run verify:profile-page`

Expected: FAIL because the new UI state and API methods are missing.

- [ ] **Step 3: Implement miniapp changes**

Update the edit page to render three sections and add photo group handlers using existing `uploadFileToQiniu`.

- [ ] **Step 4: Run verification to see it pass**

Run: `cd miniapp && npm run verify:profile-page`

Expected: PASS.

### Task 3: Final verification

**Files:**
- All files above.

- [ ] **Step 1: Run focused backend tests**

Run: `cd server && go test ./internal/domain/profile ./internal/domain/asset ./internal/infra/migration`

Expected: PASS.

- [ ] **Step 2: Run focused miniapp verification**

Run: `cd miniapp && npm run verify:api-client && npm run verify:profile-page`

Expected: PASS.

- [ ] **Step 3: Check git status**

Run: `git status --short`

Expected: Only current task files plus pre-existing unrelated memory changes are present.

