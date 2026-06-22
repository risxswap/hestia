# Onboarding Initial Report Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现开发登录、混合式 onboarding 草稿、初版报告生成、报告查询和候选形象路线反馈这一条正式但很薄的业务闭环。

**Architecture:** 服务端沿用现有 Gin + 轻量 domain 模块结构，每个模块包含 `routes.go`、`handler.go`、`service.go`、`model.go`、`repo.go`。开发态用户 session 写入 Redis，MySQL 只保存账号和业务数据；报告生成通过 `ReportGenerator` 抽象，默认使用规则生成器。小程序报告页读取 `reports/latest`，失败时保留本地 mock fallback。

**Tech Stack:** Go 1.25、Gin、sqlx、go-redis/v9、MySQL JSON 字段、微信小程序原生页面。

---

## 当前基线

- 当前分支：`codex/server-skeleton`
- 基线命令：`cd /Users/ming/Workspace/hestia/server && go test ./...`
- 基线结果：通过，包含 `app/admin`、`app/user`、`common/response`、`domain/agent`、`domain/job`、`infra/config`、`infra/migration`

## 文件结构

### 需要新增或修改的服务端文件

- Modify: `server/internal/infra/migration/mysql/001_init_schema.sql`  
  新增 `onboarding_drafts` 表。
- Create: `server/internal/common/id/id.go`  
  生成短 `public_id` 和 token。
- Create: `server/internal/common/auth/context.go`  
  用户鉴权上下文读写。
- Create: `server/internal/common/auth/middleware.go`  
  Bearer token Redis session 中间件。
- Create: `server/internal/common/auth/middleware_test.go`  
  验证无 token、无效 token、有效 token。
- Create: `server/internal/domain/account/*`  
  开发登录、用户 upsert、Redis session 写入。
- Create: `server/internal/domain/onboarding/*`  
  草稿读取、保存、提交编排。
- Create: `server/internal/domain/profile/*`  
  onboarding 提交所需的画像、事实、偏好、推断落库能力。
- Create: `server/internal/domain/asset/*`  
  本地模拟资产登记能力。
- Create: `server/internal/domain/wardrobe/*`  
  核心衣橱落库能力。
- Create: `server/internal/domain/report/*`  
  报告生成结果保存、latest 和 detail 查询。
- Create: `server/internal/domain/imageroute/*`  
  候选路线保存、报告关联、路线反馈。
- Create: `server/internal/domain/generator/*`  
  `ReportGenerator` 接口和 `RuleReportGenerator`。
- Modify: `server/internal/domain/job/*`  
  从占位列表扩展为用户侧 job 查询和生成任务记录。
- Modify: `server/internal/app/user/router.go`  
  注册 account、onboarding、job、report、image-route 用户侧路由。
- Modify: `server/cmd/user-server/main.go`  
  按配置初始化 Redis 依赖。
- Add tests next to each new module.

### 需要新增或修改的小程序文件

- Modify: `miniapp/pages/report/report.js`  
  请求 `/api/user/reports/latest`，失败时使用 `utils/mock.js`。
- Modify: `miniapp/pages/report/report.wxml`  
  展示接口返回的报告摘要、行动项和候选路线。
- Modify: `miniapp/pages/report/report.wxss`  
  补充路线区块样式，避免文本拥挤。

## Task 1: 最小账号与 Redis 会话

**Files:**
- Create: `server/internal/common/id/id.go`
- Create: `server/internal/common/id/id_test.go`
- Create: `server/internal/common/auth/context.go`
- Create: `server/internal/common/auth/middleware.go`
- Create: `server/internal/common/auth/middleware_test.go`
- Create: `server/internal/domain/account/model.go`
- Create: `server/internal/domain/account/repo.go`
- Create: `server/internal/domain/account/service.go`
- Create: `server/internal/domain/account/handler.go`
- Create: `server/internal/domain/account/routes.go`
- Create: `server/internal/domain/account/routes_test.go`
- Modify: `server/internal/app/user/router.go`
- Modify: `server/cmd/user-server/main.go`

- [ ] **Step 1: 写失败测试：ID 生成带前缀且非空**

在 `server/internal/common/id/id_test.go` 写：

```go
package id_test

import (
	"strings"
	"testing"

	"hestia/server/internal/common/id"
)

func TestNewPublicIDUsesPrefix(t *testing.T) {
	got := id.NewPublicID("usr")
	if !strings.HasPrefix(got, "usr_") {
		t.Fatalf("expected usr_ prefix, got %q", got)
	}
	if len(got) <= len("usr_") {
		t.Fatalf("expected suffix, got %q", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/common/id`

Expected: FAIL，包或函数不存在。

- [ ] **Step 3: 实现最小 ID 工具**

在 `server/internal/common/id/id.go` 实现：

```go
package id

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

func NewPublicID(prefix string) string {
	return prefix + "_" + randomString(16)
}

func NewToken(prefix string) string {
	return prefix + "_" + randomString(32)
}

func randomString(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return strings.ToLower(encoded)
}
```

- [ ] **Step 4: 写失败测试：鉴权中间件从 Redis 读取 session**

在 `server/internal/common/auth/middleware_test.go` 使用 `httptest` 和一个 fake session store，验证无 token 返回 401，有效 token 写入上下文。

关键断言：

```go
if recorder.Code != http.StatusUnauthorized {
	t.Fatalf("expected 401, got %d", recorder.Code)
}
```

以及：

```go
user, ok := auth.UserFromContext(c)
if !ok || user.UserID != 12 || user.UserPublicID != "usr_test" {
	t.Fatalf("expected user context, got %#v", user)
}
```

- [ ] **Step 5: 运行测试确认失败**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/common/auth`

Expected: FAIL，鉴权包或类型不存在。

- [ ] **Step 6: 实现 auth context 和 Redis session middleware**

要求：

- `SessionStore` 接口提供 `Get(ctx, token string) (Session, error)`。
- `RedisSessionStore` 使用 key `hestia:user-session:{token}`。
- 中间件读取 `Authorization: Bearer <token>`。
- 查不到 session 返回 `response.Error(c, http.StatusUnauthorized, "auth.unauthorized", "请先登录")`。

- [ ] **Step 7: 写失败测试：dev-login 创建用户并返回 token**

在 `server/internal/domain/account/routes_test.go` 写路由测试，使用 fake repo 和 fake session writer。请求：

```http
POST /api/user/dev-login
Content-Type: application/json

{"nickname":"测试用户","dev_key":"ming-local"}
```

断言响应 `code == "ok"`，`data.user_public_id` 非空，`data.token` 以 `dev_` 开头。

- [ ] **Step 8: 运行测试确认失败**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/domain/account`

Expected: FAIL，account 模块不存在。

- [ ] **Step 9: 实现 account 模块**

要求：

- `repo.go` 使用 `users` 表按 `dev_key` 映射到 `phone` 或 `wechat_openid` 不合适，本轮使用 `nickname` + `dev_key` 创建测试用户时，`wechat_openid` 写 `dev:{dev_key}`。
- `service.go` upsert 用户，并调用 session writer 写 Redis。
- `handler.go` 返回 `user_public_id`、`token`、`onboarding_status`。
- `routes.go` 注册 `POST /dev-login`。

- [ ] **Step 10: 注册用户侧路由并验证**

修改 `server/internal/app/user/router.go` 注册：

```go
account.RegisterUserRoutes(api, deps)
```

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/domain/account ./internal/common/auth ./internal/app/user`

Expected: PASS。

- [ ] **Step 11: 提交**

```bash
git add server/internal/common/id server/internal/common/auth server/internal/domain/account server/internal/app/user/router.go server/cmd/user-server/main.go
git commit -m "feat(server): 实现开发登录和 Redis 会话"
```

## Task 2: Onboarding 草稿表与读写接口

**Files:**
- Modify: `server/internal/infra/migration/mysql/001_init_schema.sql`
- Create: `server/internal/domain/onboarding/model.go`
- Create: `server/internal/domain/onboarding/repo.go`
- Create: `server/internal/domain/onboarding/service.go`
- Create: `server/internal/domain/onboarding/handler.go`
- Create: `server/internal/domain/onboarding/routes.go`
- Create: `server/internal/domain/onboarding/routes_test.go`
- Modify: `server/internal/app/user/router.go`

- [ ] **Step 1: 写失败测试：迁移包含 onboarding_drafts**

在 `server/internal/infra/migration/migration_test.go` 追加断言 SQL 文本包含：

```text
CREATE TABLE IF NOT EXISTS `onboarding_drafts`
```

以及字段 `content_hash`、`version`、`draft_data`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/infra/migration`

Expected: FAIL，SQL 未包含草稿表。

- [ ] **Step 3: 新增 onboarding_drafts 表**

在 `users` 后或画像域前加入：

```sql
CREATE TABLE IF NOT EXISTS `onboarding_drafts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `public_id` varchar(32) NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'draft',
  `current_step` varchar(64) DEFAULT NULL,
  `draft_data` json DEFAULT NULL,
  `content_hash` varchar(64) NOT NULL,
  `version` int unsigned NOT NULL DEFAULT 1,
  `submitted_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_onboarding_drafts_public_id` (`public_id`),
  KEY `idx_onboarding_drafts_user_status` (`user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
```

- [ ] **Step 4: 写失败测试：GET/PUT onboarding 草稿**

在 `server/internal/domain/onboarding/routes_test.go` 写测试：

- 未登录访问返回 401。
- 登录上下文下 `PUT /api/user/onboarding` 保存 `step=style_goal` 和 `data.style_goal.goals`。
- `GET /api/user/onboarding` 返回同一份草稿、`current_step`、`version`。

- [ ] **Step 5: 运行测试确认失败**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/domain/onboarding`

Expected: FAIL，onboarding 模块不存在。

- [ ] **Step 6: 实现 onboarding 草稿读写**

要求：

- `model.go` 定义 `DraftData`，包含 `photos`、`basic`、`wardrobe`、`style_goal`、`reference_style`。
- `service.SaveDraft` 合并局部 `data` 到当前草稿，递增 `version`，更新 `content_hash`。
- `service.GetDraft` 没有草稿时返回空草稿和 `status=not_started`。
- `handler` 从 auth context 获取用户，不接受请求体传 user_id。

- [ ] **Step 7: 注册路由并验证**

在用户 router 的受保护分组中注册：

```go
onboarding.RegisterUserRoutes(api.Group("/onboarding"), deps)
```

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/infra/migration ./internal/domain/onboarding ./internal/app/user`

Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add server/internal/infra/migration/mysql/001_init_schema.sql server/internal/infra/migration/migration_test.go server/internal/domain/onboarding server/internal/app/user/router.go
git commit -m "feat(server): 实现 onboarding 草稿读写"
```

## Task 3: Onboarding 提交、规则报告生成与查询

**Files:**
- Create: `server/internal/domain/generator/model.go`
- Create: `server/internal/domain/generator/rule.go`
- Create: `server/internal/domain/generator/rule_test.go`
- Create: `server/internal/domain/profile/*`
- Create: `server/internal/domain/asset/*`
- Create: `server/internal/domain/wardrobe/*`
- Create: `server/internal/domain/report/*`
- Create: `server/internal/domain/imageroute/*`
- Modify: `server/internal/domain/job/*`
- Modify: `server/internal/domain/onboarding/*`
- Modify: `server/internal/app/user/router.go`

- [ ] **Step 1: 写失败测试：规则生成器输出行动型报告**

在 `server/internal/domain/generator/rule_test.go` 写测试，给定包含风格目标、禁忌、核心衣橱的 `InitialReportInput`，断言：

- 至少 1 条 route。
- `ActionItems` 非空。
- `WardrobeGaps` 不含商品链接字段。
- `ReferenceStyleLogic` 使用“参考造型逻辑”表达，不出现“你像”。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/domain/generator`

Expected: FAIL，generator 模块不存在。

- [ ] **Step 3: 实现 generator 接口和 RuleReportGenerator**

定义：

```go
type ReportGenerator interface {
	GenerateInitialReport(ctx context.Context, input InitialReportInput) (*InitialReportResult, error)
}
```

结果必须包含 `Summary`、`Routes`、`HairStrategy`、`MakeupStrategy`、`OutfitStrategy`、`Avoidances`、`WardrobeCombinations`、`WardrobeGaps`、`ActionItems`、`ReferenceStyleLogic`、`PrivacyNote`。

- [ ] **Step 4: 写失败测试：submit 草稿生成报告**

在 `server/internal/domain/onboarding/routes_test.go` 增加集成式 handler 测试，使用 fake repos 或内存 service：

- 保存包含 `basic`、`style_goal`、`wardrobe` 的草稿。
- 调用 `POST /api/user/onboarding/submit`。
- 断言返回 `job_public_id` 和 `report_public_id`。
- 查询 `GET /api/user/reports/latest` 返回同一个报告。

- [ ] **Step 5: 运行测试确认失败**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/domain/onboarding ./internal/domain/report`

Expected: FAIL，提交和报告模块尚未实现。

- [ ] **Step 6: 实现最小沉淀 service/repo**

要求：

- `profile` upsert `profiles`，写 `profile_facts`、`profile_prefs`、`profile_inferences`。
- `asset` 为 `client_ref` 或 `asset_public_id` 登记本地模拟资产。
- `wardrobe` 保存 `is_core=1` 的核心单品。
- `job` 创建 `initial_report_generation`，成功后写 `status=succeeded`、`output_summary`、`finished_at`。
- `imageroute` 写 `image_routes` 和 `report_image_routes`。
- `report` 写 `reports`，支持 latest/detail 查询。
- `onboarding.Submit` 用事务编排，生成失败时 job 标记 failed。

- [ ] **Step 7: 注册查询路由并验证**

注册：

```go
job.RegisterUserRoutes(api.Group("/jobs"), deps)
report.RegisterUserRoutes(api.Group("/reports"), deps)
imageroute.RegisterUserRoutes(api.Group("/image-routes"), deps)
```

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/domain/generator ./internal/domain/onboarding ./internal/domain/report ./internal/domain/job ./internal/app/user`

Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add server/internal/domain/generator server/internal/domain/profile server/internal/domain/asset server/internal/domain/wardrobe server/internal/domain/report server/internal/domain/imageroute server/internal/domain/job server/internal/domain/onboarding server/internal/app/user/router.go
git commit -m "feat(server): 生成 onboarding 初版报告"
```

## Task 4: 形象路线反馈

**Files:**
- Modify: `server/internal/domain/imageroute/model.go`
- Modify: `server/internal/domain/imageroute/repo.go`
- Modify: `server/internal/domain/imageroute/service.go`
- Modify: `server/internal/domain/imageroute/handler.go`
- Modify: `server/internal/domain/imageroute/routes.go`
- Modify: `server/internal/domain/imageroute/routes_test.go`

- [ ] **Step 1: 写失败测试：路线 like/dislike/adjust 更新状态并记录事件**

测试三种动作：

- `like` 后路线 `status=active`，`activated_at` 非空。
- `dislike` 后路线 `status=archived`。
- `adjust` 后路线 `status=refinement`，事件 JSON 包含 reason。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/domain/imageroute`

Expected: FAIL，反馈接口未实现。

- [ ] **Step 3: 实现反馈接口**

接口：

```text
POST /api/user/image-routes/:public_id/feedback
```

请求：

```json
{"action":"adjust","reason":"想更轻松一点"}
```

响应：

```json
{"public_id":"irt_xxx","status":"refinement"}
```

未知 action 返回 `400 image_route.invalid_feedback_action`。

- [ ] **Step 4: 验证并提交**

Run: `cd /Users/ming/Workspace/hestia/server && go test ./internal/domain/imageroute ./internal/app/user`

Expected: PASS。

```bash
git add server/internal/domain/imageroute server/internal/app/user/router.go
git commit -m "feat(server): 实现形象路线反馈"
```

## Task 5: 小程序报告页接入 latest report

**Files:**
- Modify: `miniapp/pages/report/report.js`
- Modify: `miniapp/pages/report/report.wxml`
- Modify: `miniapp/pages/report/report.wxss`

- [ ] **Step 1: 写验证脚本或轻量测试**

如果小程序目录现有脚本不覆盖报告页数据映射，则新增 `miniapp/scripts/verify-report-page.js`，验证 `normalizeReportResponse` 能把服务端 `content_json.action_items` 转为页面 `actionItems`。

Run: `cd /Users/ming/Workspace/hestia/miniapp && node scripts/verify-report-page.js`

Expected: FAIL，函数或脚本不存在。

- [ ] **Step 2: 实现报告页数据请求和 fallback**

要求：

- `onLoad` 调 `wx.request` 请求 `/api/user/reports/latest`。
- 从本地 storage 读取 token，并设置 `Authorization`。
- 请求成功且 `code=ok` 时展示真实报告。
- 请求失败时保留 `../../utils/mock` fallback。
- 页面展示 `summary`、`actionItems`、`routes`。

- [ ] **Step 3: 更新 WXML/WXSS**

新增路线展示区，不使用嵌套卡片；控制文字行距和间距，避免与现有视觉冲突。

- [ ] **Step 4: 验证并提交**

Run: `cd /Users/ming/Workspace/hestia/miniapp && node scripts/verify-report-page.js`

Expected: PASS。

```bash
git add miniapp/pages/report miniapp/scripts/verify-report-page.js
git commit -m "feat(miniapp): 报告页接入初版报告接口"
```

## Final Verification

全部任务完成后运行：

```bash
cd /Users/ming/Workspace/hestia/server && go test ./...
```

```bash
cd /Users/ming/Workspace/hestia/miniapp && node scripts/verify-report-page.js
```

最后检查：

```bash
cd /Users/ming/Workspace/hestia && git status --short
```

验收输出必须说明每个命令的退出状态。不能只说“应该通过”。
