# 大模型配置 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增加数据库驱动的大模型供应商、模型和业务使用配置，让图片识别等业务按 usage key 解析供应商、模型和参数。

**Architecture:** 新增 `llm_providers`、`llm_models` 两张表，并通过 `system_configs(group='llm.usages')` 保存固定业务场景绑定。`infra/llm` 负责配置解析和统一调用接口，业务侧通过 usage key 调用，不直接绑定单一客户端。

**Tech Stack:** Go、MySQL SQL migration、`sqlx`、现有 `system_configs`、现有 `llm.Client` 兼容适配。

---

### Task 1: 数据库迁移和默认配置

**Files:**
- Create: `server/internal/infra/migration/mysql/005_llm_config.sql`
- Modify: `server/internal/infra/migration/migration_test.go`

- [ ] **Step 1: Write failing migration tests**

在 `TestApplyMySQLSchemaExecutesInitialSchemaStatements` 增加断言：

```go
if !containsStatement(exec.queries, "CREATE TABLE IF NOT EXISTS `llm_providers`") {
	t.Fatalf("expected llm providers table migration statement")
}
if !containsStatement(exec.queries, "CREATE TABLE IF NOT EXISTS `llm_models`") {
	t.Fatalf("expected llm models table migration statement")
}
if !containsStatement(exec.queries, "llm.usages") {
	t.Fatalf("expected llm usages system config migration statement")
}
if !containsStatement(exec.queries, "caps_json") {
	t.Fatalf("expected llm model caps_json field")
}
```

在 `TestInitialMySQLSchemaMatchesLogicalDesign` 的 `expectedTables` 增加：

```go
"llm_providers",
"llm_models",
```

- [ ] **Step 2: Verify red**

Run:

```bash
cd server && go test ./internal/infra/migration -run 'TestApplyMySQLSchemaExecutesInitialSchemaStatements|TestInitialMySQLSchemaMatchesLogicalDesign' -count=1
```

Expected: FAIL，提示缺少 `llm_providers` / `llm_models` / `llm.usages`。

- [ ] **Step 3: Implement migration**

新增 `005_llm_config.sql`：

```sql
CREATE TABLE IF NOT EXISTS `llm_providers` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(64) NOT NULL,
  `name` varchar(128) NOT NULL,
  `api_base_url` varchar(512) NOT NULL,
  `token` varchar(2048) NOT NULL,
  `auth_type` varchar(32) NOT NULL DEFAULT 'bearer',
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_llm_providers_code` (`code`),
  KEY `idx_llm_providers_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE IF NOT EXISTS `llm_models` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `provider_id` bigint unsigned NOT NULL,
  `model_code` varchar(128) NOT NULL,
  `name` varchar(128) NOT NULL,
  `caps_json` json NOT NULL,
  `max_input_tokens` int unsigned DEFAULT NULL,
  `max_output_tokens` int unsigned DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT 'active',
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_llm_models_provider_model` (`provider_id`, `model_code`),
  KEY `idx_llm_models_provider_status` (`provider_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO `system_configs`
  (`group`, `key`, `value`, `value_type`, `description`, `status`)
VALUES
  ('llm.usages', 'wardrobe_image_recognition', CAST('{"provider_code":"qwen","model_code":"qwen-vl-plus","params":{"temperature":0.2,"max_tokens":1200,"response_format":"json_object"},"prompt_version":"v1"}' AS JSON), 'json', '衣服图片识别大模型配置', 'active'),
  ('llm.usages', 'agent_chat', CAST('{"provider_code":"qwen","model_code":"qwen-plus","params":{"temperature":0.7,"max_tokens":2000},"prompt_version":"v1"}' AS JSON), 'json', '聊天大模型配置', 'active'),
  ('llm.usages', 'onboarding_summary', CAST('{"provider_code":"qwen","model_code":"qwen-plus","params":{"temperature":0.3,"max_tokens":1600,"response_format":"json_object"},"prompt_version":"v1"}' AS JSON), 'json', 'onboarding 总结大模型配置', 'active')
ON DUPLICATE KEY UPDATE
  `value` = VALUES(`value`),
  `value_type` = VALUES(`value_type`),
  `description` = VALUES(`description`),
  `status` = VALUES(`status`);
```

同时将两张表加入 `001_init_schema.sql`，保证新库初始 schema 完整。

- [ ] **Step 4: Verify green**

Run:

```bash
cd server && go test ./internal/infra/migration -count=1
```

Expected: PASS。

### Task 2: LLM 配置仓储和 resolver

**Files:**
- Create: `server/internal/infra/llm/config.go`
- Create: `server/internal/infra/llm/repo.go`
- Create: `server/internal/infra/llm/config_test.go`

- [ ] **Step 1: Write failing resolver tests**

覆盖：

- `ResolveUsage(ctx, "wardrobe_image_recognition", []string{"vision","json"})` 返回 provider、model、params。
- 缺少 required caps 返回 `ErrCapabilityNotSupported`。
- 缺少 usage/provider/model 返回对应错误。

- [ ] **Step 2: Verify red**

Run:

```bash
cd server && go test ./internal/infra/llm -count=1
```

Expected: FAIL，缺少类型和函数。

- [ ] **Step 3: Implement models and repository**

定义：

```go
type Provider struct { ID int64; Code, Name, APIBaseURL, Token, AuthType, Status string }
type Model struct { ID, ProviderID int64; ModelCode, Name, Status string; Caps []string; MaxInputTokens, MaxOutputTokens *int }
type Usage struct { Key, ProviderCode, ModelCode, PromptVersion string; Params map[string]any }
type ResolvedUsage struct { Usage Usage; Provider Provider; Model Model }
```

定义 repository 接口：

```go
type ConfigRepository interface {
	FindUsage(ctx context.Context, key string) (Usage, error)
	FindProviderByCode(ctx context.Context, code string) (Provider, error)
	FindModel(ctx context.Context, providerID int64, modelCode string) (Model, error)
}
```

实现 `MySQLConfigRepository` 读取 `system_configs`、`llm_providers`、`llm_models`。

- [ ] **Step 4: Implement resolver**

```go
type ConfigResolver struct { repo ConfigRepository }
func (r *ConfigResolver) ResolveUsage(ctx context.Context, key string, requiredCaps []string) (ResolvedUsage, error)
```

校验 active、trim、caps。

- [ ] **Step 5: Verify green**

Run:

```bash
cd server && go test ./internal/infra/llm -count=1
```

Expected: PASS。

### Task 3: 结构化 LLM 调用接口

**Files:**
- Modify: `server/internal/infra/llm/client.go`
- Create: `server/internal/infra/llm/service.go`
- Modify: `server/internal/infra/llm/config_test.go`

- [ ] **Step 1: Write failing tests**

覆盖：

- `Service.Generate` 按 `UsageKey` 调 resolver。
- 对 legacy `Client` 调用 `Complete`，将 prompt 拼成 input。
- resolver 错误原样返回。

- [ ] **Step 2: Verify red**

Run:

```bash
cd server && go test ./internal/infra/llm -count=1
```

Expected: FAIL。

- [ ] **Step 3: Implement request/response**

```go
type Message struct { Role, Content string }
type Request struct {
	UsageKey string
	RequiredCaps []string
	Messages []Message
	ImageURLs []string
	Params map[string]any
}
type Response struct { Text string; Usage ResolvedUsage }
type Generator interface { Generate(ctx context.Context, req Request) (Response, error) }
```

保留旧 `Client` 接口，并用 `Service` 包装旧 client：

```go
type Service struct { resolver *ConfigResolver; legacy Client }
```

第一版 provider-specific HTTP 客户端不在本任务实现，先通过 legacy client 兼容现有能力。

- [ ] **Step 4: Verify green**

Run:

```bash
cd server && go test ./internal/infra/llm -count=1
```

Expected: PASS。

### Task 4: 接入衣服图片识别和用户路由

**Files:**
- Modify: `server/internal/domain/wardrobe/image_recognizer.go`
- Modify: `server/internal/domain/wardrobe/service_test.go`
- Modify: `server/internal/domain/wardrobe/routes.go`
- Modify: `server/internal/app/user/router.go`

- [ ] **Step 1: Write failing tests**

覆盖 `LLMImageRecognizer` 使用 `Generate`，并传：

```go
UsageKey: "wardrobe_image_recognition"
RequiredCaps: []string{"vision", "json"}
```

- [ ] **Step 2: Verify red**

Run:

```bash
cd server && go test ./internal/domain/wardrobe -run 'TestLLMImageRecognizer' -count=1
```

Expected: FAIL。

- [ ] **Step 3: Implement recognizer against Generator**

`LLMImageRecognizer` 改为依赖 `llm.Generator`，调用 `Generate` 后解析 `Response.Text`。

- [ ] **Step 4: Wire resolver in routes**

在有 DB 的 `deps` 中：

```go
llmRepo := llm.NewMySQLConfigRepository(deps.DB)
llmGenerator := llm.NewService(llm.NewConfigResolver(llmRepo), deps.LLM)
```

图片识别注入 `wardrobe.NewLLMImageRecognizer(llmGenerator)`。

- [ ] **Step 5: Verify green**

Run:

```bash
cd server && go test ./internal/domain/wardrobe ./internal/app/user ./internal/infra/llm -count=1
```

Expected: PASS。

### Final Verification

Run:

```bash
cd server && go test ./... -count=1
cd miniapp && npm run verify:api-client && npm run verify:api-integration
```

Expected: all PASS。
