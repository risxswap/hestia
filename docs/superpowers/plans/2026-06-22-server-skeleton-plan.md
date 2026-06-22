# Server Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按服务端架构 spec 搭建第一层可编译、可测试的 Go/Gin 服务端骨架。

**Architecture:** 保留 `user-server` 和 `admin-server` 两个入口，使用 Gin 替换当前 `net/http` mux。业务模块采用轻量 MVC 文件结构，先实现 `agent` 和 `job` 的路由骨架、公共响应/SSE、配置加载、手写 Deps，以及 admin-server 启动时调用的 `infra/migration` 迁移骨架。

**Tech Stack:** Go, Gin, sqlx, go-redis, asynq, caarlos0/env, godotenv, slog.

---

### Task 1: 公共响应和 SSE 基础能力

**Files:**
- Create: `server/internal/common/response/response.go`
- Create: `server/internal/common/response/sse.go`
- Test: `server/internal/common/response/response_test.go`
- Test: `server/internal/common/response/sse_test.go`

- [ ] **Step 1: Write failing JSON response tests**

Add tests that call `response.OK` and `response.Error` through Gin and assert the unified JSON shape:

```go
func TestOKWritesUnifiedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/ok", func(c *gin.Context) {
		response.OK(c, gin.H{"surface": "user"})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ok", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "ok" {
		t.Fatalf("expected code ok, got %#v", body["code"])
	}
	if body["message"] != "" {
		t.Fatalf("expected empty message, got %#v", body["message"])
	}
	data := body["data"].(map[string]any)
	if data["surface"] != "user" {
		t.Fatalf("expected surface user, got %#v", data["surface"])
	}
}

func TestErrorWritesUnifiedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/error", func(c *gin.Context) {
		response.Error(c, http.StatusUnauthorized, "account.unauthorized", "请先登录")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/error", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "account.unauthorized" {
		t.Fatalf("unexpected code %#v", body["code"])
	}
	if body["message"] != "请先登录" {
		t.Fatalf("unexpected message %#v", body["message"])
	}
	if body["data"] != nil {
		t.Fatalf("expected nil data, got %#v", body["data"])
	}
}
```

- [ ] **Step 2: Run tests to verify red**

Run: `cd server && go test ./internal/common/response`

Expected: FAIL because `internal/common/response` does not exist.

- [ ] **Step 3: Implement response helpers and SSE writer**

Implement `OK`, `Error`, `WriteSSE`, and `StreamHeaders` in the response package.

- [ ] **Step 4: Run tests to verify green**

Run: `cd server && go test ./internal/common/response`

Expected: PASS.

### Task 2: Gin app routers and health endpoints

**Files:**
- Modify: `server/internal/app/user/router.go`
- Modify: `server/internal/app/admin/router.go`
- Modify: `server/internal/app/user/router_test.go`
- Modify: `server/internal/app/admin/router_test.go`
- Create: `server/internal/app/deps.go`

- [ ] **Step 1: Write failing router tests**

Update user router tests to assert `GET /api/user/health` returns unified JSON and `GET /api/miniapp/health` no longer exists. Update admin router tests to assert `GET /api/admin/health` returns unified JSON.

- [ ] **Step 2: Run tests to verify red**

Run: `cd server && go test ./internal/app/...`

Expected: FAIL because routers still use `net/http` and old response shape.

- [ ] **Step 3: Implement Gin routers and Deps**

Create `app.Deps`, switch both routers to `*gin.Engine`, register `/api/user/health` and `/api/admin/health`, and use `response.OK`.

- [ ] **Step 4: Run tests to verify green**

Run: `cd server && go test ./internal/app/...`

Expected: PASS.

### Task 3: Agent and Job domain route skeletons

**Files:**
- Create: `server/internal/domain/agent/routes.go`
- Create: `server/internal/domain/agent/handler.go`
- Create: `server/internal/domain/agent/service.go`
- Create: `server/internal/domain/agent/model.go`
- Create: `server/internal/domain/job/routes.go`
- Create: `server/internal/domain/job/handler.go`
- Create: `server/internal/domain/job/service.go`
- Create: `server/internal/domain/job/model.go`
- Modify: `server/internal/app/user/router.go`
- Modify: `server/internal/app/admin/router.go`
- Test: `server/internal/domain/agent/routes_test.go`
- Test: `server/internal/domain/job/routes_test.go`

- [ ] **Step 1: Write failing route tests**

Add tests that assert:

```text
POST /api/user/agent/stream returns text/event-stream and a done SSE event.
GET /api/admin/jobs returns unified JSON with an empty list.
```

- [ ] **Step 2: Run tests to verify red**

Run: `cd server && go test ./internal/domain/... ./internal/app/...`

Expected: FAIL because `domain/agent` and `domain/job` do not exist.

- [ ] **Step 3: Implement minimal route skeletons**

Implement `agent.RegisterUserRoutes`, `job.RegisterAdminRoutes`, placeholder services, and register them from app routers.

- [ ] **Step 4: Run tests to verify green**

Run: `cd server && go test ./internal/domain/... ./internal/app/...`

Expected: PASS.

### Task 4: Infra config, logger, and server entrypoints

**Files:**
- Create: `server/internal/infra/config/config.go`
- Create: `server/internal/infra/logger/logger.go`
- Modify: `server/cmd/user-server/main.go`
- Modify: `server/cmd/admin-server/main.go`
- Test: `server/internal/infra/config/config_test.go`

- [ ] **Step 1: Write failing config tests**

Add tests for default ports and environment variable override:

```text
USER_SERVER_PORT defaults to 8080.
ADMIN_SERVER_PORT defaults to 8081.
USER_SERVER_PORT=18080 overrides the default.
```

- [ ] **Step 2: Run tests to verify red**

Run: `cd server && go test ./internal/infra/...`

Expected: FAIL because infra config package does not exist.

- [ ] **Step 3: Implement config and logger skeletons**

Implement config loading with environment variables and optional `.env` loading in development. Implement slog logger construction. Update server entrypoints to use Gin routers and ports from config.

- [ ] **Step 4: Run tests to verify green**

Run: `cd server && go test ./internal/infra/... ./cmd/...`

Expected: PASS.

### Task 5: Admin startup migration skeleton and final verification

**Files:**
- Modify: `server/cmd/admin-server/main.go`
- Create: `server/internal/infra/migration/migration.go`
- Test: `server/internal/infra/migration/migration_test.go`
- Test: `server/cmd/admin-server/main_test.go`

- [ ] **Step 1: Write failing startup migration tests**

Add tests for startup migration:

```text
RunOnStartup calls Runner.Up once.
RunOnStartup returns Runner.Up errors.
prepareAdminServer calls migration runner before returning the router.
```

- [ ] **Step 2: Run tests to verify red**

Run: `cd server && go test ./internal/infra/migration ./cmd/admin-server`

Expected: FAIL because migration package and prepare helper do not exist.

- [ ] **Step 3: Implement startup migration skeleton**

Implement `infra/migration.RunOnStartup`, a `Runner` interface, a no-op runner for the first skeleton pass, and admin-server startup wiring. Do not expose migration over HTTP and do not keep a `cmd/admin-server/migrate.go` CLI helper.

- [ ] **Step 4: Run full verification**

Run:

```bash
cd server
go test ./...
```

Expected: PASS.

---

## Self-Review

- Spec coverage: covers Gin app routers, `/api/user` and `/api/admin` surfaces, response/SSE, domain route registration, `infra` config/logger, and admin-server startup migration skeleton.
- Intentional gaps: real MySQL/sqlx repositories, Redis/asynq worker execution, Qiniu, WeChat login, LLM/Eino runtime, and database migration execution are not implemented in this first skeleton pass.
- Placeholder scan: no task depends on an undefined business implementation; skeleton methods return deterministic placeholder responses where necessary.
