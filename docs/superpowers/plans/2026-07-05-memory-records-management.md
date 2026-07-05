# Memory Records Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将“我的 > 记忆”改为真实的逐条记忆记录列表，并支持编辑、删除。

**Architecture:** 后端新增 `memory` domain，围绕已有 `memories` 表提供用户态列表、更新和软删除接口。小程序通过 `utils/api.js` 调用接口，`pages/memory/index` 负责列表、底部编辑弹层和删除确认。

**Tech Stack:** Go + Gin + sqlx + MySQL，小程序原生 WXML/WXSS/JS，Node 验证脚本。

---

### Task 1: 后端记忆 CRUD

**Files:**
- Create: `server/internal/domain/memory/model.go`
- Create: `server/internal/domain/memory/service.go`
- Create: `server/internal/domain/memory/repo.go`
- Create: `server/internal/domain/memory/handler.go`
- Create: `server/internal/domain/memory/routes.go`
- Create: `server/internal/domain/memory/routes_test.go`
- Modify: `server/internal/app/user/router.go`

- [ ] 写失败测试：覆盖列表、编辑和删除。
- [ ] 运行 `cd server && go test ./internal/domain/memory`，确认因 package 或接口缺失失败。
- [ ] 实现 model/service/repo/handler/routes。
- [ ] 在用户路由挂载 `/api/user/memories`。
- [ ] 运行 `cd server && go test ./internal/domain/memory ./internal/app/user`。

### Task 2: 小程序 API 和记忆页

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/pages/memory/index.js`
- Modify: `miniapp/pages/memory/index.wxml`
- Modify: `miniapp/pages/memory/index.wxss`
- Create: `miniapp/scripts/verify-memory-page.js`
- Modify: `miniapp/package.json`

- [ ] 写失败验证脚本：断言页面调用 `getMemoryItems`、能打开编辑弹层、保存调用 `updateMemoryItem`、删除调用 `deleteMemoryItem`。
- [ ] 运行 `cd miniapp && node scripts/verify-memory-page.js`，确认因函数缺失失败。
- [ ] 在 `utils/api.js` 增加 `getMemoryItems`、`updateMemoryItem`、`deleteMemoryItem`。
- [ ] 将记忆页改为记录列表、编辑弹层和删除确认。
- [ ] 运行 `cd miniapp && node scripts/verify-memory-page.js && npm run verify:profile-page && npm run verify:api-integration`。

### Task 3: 最终验证

**Files:**
- Check all modified files.

- [ ] 运行 `git status --short`，确认没有无关文件被纳入。
- [ ] 运行相关 Go 和 miniapp 验证命令。
- [ ] 总结接口、页面行为和验证结果。
