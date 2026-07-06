# 编辑档案固定照片位 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将编辑档案页从自由添加照片组改为固定照片位，并把每组文字备注作为对应照片观察的补充说明。

**Architecture:** 后端继续使用现有 `profile_photos.photo_type + angle + note` 结构，不新增数据库表。小程序页面定义固定观察组和照片位，上传时按照片位传入固定 `photo_type` 与 `angle`，文字备注继续写入现有 profile 字段。

**Tech Stack:** 微信小程序 WXML/WXSS/JS，现有 `miniapp/scripts/verify-profile-page.js` Node 验证脚本。

---

### Task 1: 固定照片位验证

**Files:**
- Modify: `miniapp/scripts/verify-profile-page.js`

- [ ] **Step 1: 写失败验证**

在 profile edit markup 断言中加入：

```js
assert(!profileEditMarkup.includes("按角度补充"), "profile edit should not use generic angle guidance copy");
assert(profileEditMarkup.includes("脸型与五官比例"), "profile edit should render face observation group");
assert(profileEditMarkup.includes("身形与比例"), "profile edit should render body observation group");
assert(profileEditMarkup.includes("肤色、妆发与发型"), "profile edit should render beauty observation group");
assert(profileEditMarkup.includes("正面头肩照"), "profile edit should render front headshot slot");
assert(profileEditMarkup.includes("左 45 度头肩照"), "profile edit should render left 45 headshot slot");
assert(profileEditMarkup.includes("右 45 度头肩照"), "profile edit should render right 45 headshot slot");
assert(profileEditMarkup.includes("侧面头肩照"), "profile edit should render side headshot slot");
assert(profileEditMarkup.includes("正面全身照"), "profile edit should render front full body slot");
assert(profileEditMarkup.includes("侧面全身照"), "profile edit should render side full body slot");
assert(profileEditMarkup.includes("背面全身照"), "profile edit should render back full body slot");
assert(profileEditMarkup.includes("日常站姿全身照"), "profile edit should render natural full body slot");
assert(profileEditMarkup.includes("自然光近照"), "profile edit should render natural light close-up slot");
assert(profileEditMarkup.includes("日常妆发照"), "profile edit should render daily makeup slot");
assert(profileEditMarkup.includes("发型侧面照"), "profile edit should render side hair slot");
assert(profileEditMarkup.includes("发型背面照"), "profile edit should render back hair slot");
```

- [ ] **Step 2: 跑验证确认失败**

Run: `cd miniapp && npm run verify:profile-page`

Expected: FAIL，提示缺少固定观察组或仍存在旧文案。

### Task 2: 页面数据结构和上传参数

**Files:**
- Modify: `miniapp/pages/profile/edit.js`

- [ ] **Step 1: 定义固定观察组**

新增 `photoObservationMeta`，每个 slot 包含 `key`、`label`、`photoType`、`angle`。

- [ ] **Step 2: 将已有照片映射到 slot**

新增 `buildPhotoObservations(summary)`，按 `photo_type + angle` 找到对应照片，供 WXML 直接渲染。

- [ ] **Step 3: 上传固定 slot**

调整 `handleChoosePhoto` 和 `handlePhotoUpload` 支持 `data-photo-type`、`data-angle`，上传后刷新固定观察组。

### Task 3: WXML/WXSS 改版

**Files:**
- Modify: `miniapp/pages/profile/edit.wxml`
- Modify: `miniapp/pages/profile/edit.wxss`

- [ ] **Step 1: 用观察组替代旧照片组**

三组固定渲染：脸型与五官比例、身形与比例、肤色妆发与发型。

- [ ] **Step 2: 每个照片位一个上传入口**

每个照片位显示固定 label、已有图片、上传/替换、删除。

- [ ] **Step 3: 每组备注跟随照片组**

备注文案表达为“可不填，不确定可由系统识别”，不要求用户自我诊断。

### Task 4: 验证

**Files:**
- Test: `miniapp/scripts/verify-profile-page.js`

- [ ] **Step 1: 跑 profile 页面验证**

Run: `cd miniapp && npm run verify:profile-page`

Expected: `profile page verification passed`

- [ ] **Step 2: 跑 API client 验证**

Run: `cd miniapp && npm run verify:api-client`

Expected: `api client verification passed`

- [ ] **Step 3: 检查 diff 空白问题**

Run: `git diff --check -- miniapp/pages/profile/edit.js miniapp/pages/profile/edit.wxml miniapp/pages/profile/edit.wxss miniapp/scripts/verify-profile-page.js`

Expected: exit 0。
