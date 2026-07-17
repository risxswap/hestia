# 服务端与大模型调用日志增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为服务端增加可关联的 HTTP 请求日志、panic 恢复日志，并让普通生成及 Agent Tool Calling 的所有异常都输出带脱敏上下文的结构化日志。

**Architecture:** 在 `internal/infra/logger` 提供请求上下文、脱敏摘要和 Gin 中间件，作为日志基础设施；`internal/infra/llm` 负责普通生成和模型初始化日志；`internal/domain/agent` 负责 Agent Runner 执行日志。各层只记录自己首次获得完整失败上下文的异常，避免重复堆栈。

**Tech Stack:** Go、`log/slog`、Gin、Eino、Go `testing`/`httptest`

---

### Task 1: 公共日志上下文与脱敏摘要

**Files:**
- Create: `server/internal/infra/logger/context.go`
- Create: `server/internal/infra/logger/context_test.go`

- [ ] **Step 1: 写请求标识与脱敏摘要的失败测试**

测试 `WithRequestID`/`RequestID` 能通过 Context 传递标识；测试 `SanitizeSummary` 折叠空白、限制 rune 长度，并遮蔽 URL、邮箱、手机号、Bearer token 和 API Key。

```go
func TestSanitizeSummaryRedactsSensitiveValues(t *testing.T) {
	raw := "联系 test@example.com 13800138000 https://example.test/a?token=secret Bearer abc api_key=xyz"
	got := SanitizeSummary(raw)
	for _, leaked := range []string{"test@example.com", "13800138000", "example.test", "abc", "xyz"} {
		if strings.Contains(got, leaked) { t.Fatalf("leaked %q in %q", leaked, got) }
	}
}
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `cd server && go test ./internal/infra/logger -run 'Test(RequestID|SanitizeSummary)' -count=1`

Expected: FAIL，提示 `WithRequestID`、`RequestID` 或 `SanitizeSummary` 未定义。

- [ ] **Step 3: 实现最小公共日志工具**

使用私有 context key 保存 `request_id`；摘要先折叠空白，再按稳定正则依次遮蔽敏感模式，最后截断到固定 rune 数。提供 `ErrorSummary(error) string` 复用同一脱敏逻辑。

```go
func WithRequestID(ctx context.Context, requestID string) context.Context
func RequestID(ctx context.Context) string
func SanitizeSummary(value string) string
func ErrorSummary(err error) string
```

- [ ] **Step 4: 运行测试并确认 GREEN**

Run: `cd server && go test ./internal/infra/logger -count=1`

Expected: PASS。

- [ ] **Step 5: 提交公共日志工具**

```bash
git add server/internal/infra/logger/context.go server/internal/infra/logger/context_test.go
git commit -m "feat: 增加日志上下文与脱敏摘要"
```

### Task 2: HTTP 请求与 panic 中间件

**Files:**
- Create: `server/internal/infra/logger/middleware.go`
- Create: `server/internal/infra/logger/middleware_test.go`
- Modify: `server/internal/app/user/router.go`
- Modify: `server/internal/app/user/router_test.go`

- [ ] **Step 1: 写 HTTP 日志与 panic 恢复失败测试**

测试请求头 `X-Request-ID` 被透传到响应和请求 Context，完成日志包含 method、route、status、duration；测试 panic 返回统一 500，并输出 panic 摘要与 `stack`。同时断言 Authorization、查询字符串和请求体不出现在日志。

```go
router := gin.New()
router.Use(RequestLogging(testLogger))
router.GET("/panic", func(c *gin.Context) { panic("token=secret") })
response := httptest.NewRecorder()
router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
if response.Code != http.StatusInternalServerError { t.Fatalf("got %d", response.Code) }
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `cd server && go test ./internal/infra/logger ./internal/app/user -run 'Test(RequestLogging|RequestID|Panic)' -count=1`

Expected: FAIL，提示 `RequestLogging` 未定义或 Router 未注册中间件。

- [ ] **Step 3: 实现中间件并注册到 Router**

`RequestLogging` 统一回退到 `slog.Default()`，校验或生成 request ID，将其写入 Gin/Go Context 和响应头；用 `defer` 捕获 panic，打印脱敏 panic 摘要和 `debug.Stack()`，通过 `response.Error` 返回 `internal.server_error`。请求结束按状态码选择 INFO/WARN/ERROR。

```go
func RequestLogging(log *slog.Logger) gin.HandlerFunc
```

在 `user.NewRouter` 创建 `gin.Engine` 后立即注册，保证 health 和后续所有路由都覆盖。

- [ ] **Step 4: 运行测试并确认 GREEN**

Run: `cd server && go test ./internal/infra/logger ./internal/app/user -count=1`

Expected: PASS。

- [ ] **Step 5: 提交 HTTP 日志中间件**

```bash
git add server/internal/infra/logger/middleware.go server/internal/infra/logger/middleware_test.go server/internal/app/user/router.go server/internal/app/user/router_test.go
git commit -m "feat: 增加服务端请求与异常日志"
```

### Task 3: 普通 LLM 生成与 Tool Calling 初始化日志

**Files:**
- Modify: `server/internal/infra/llm/service.go`
- Modify: `server/internal/infra/llm/service_test.go`

- [ ] **Step 1: 写 LLM 各失败阶段与脱敏字段的失败测试**

扩充现有日志测试，断言开始/完成日志含 `llm_call_id`、request ID、prompt version、角色计数、输入字符数、排序后的参数键、耗时和输出长度；分别制造 client unavailable、config resolve、model init、model generate 错误，断言均有 `llm call failed`、`error_stage` 和脱敏错误摘要。

```go
ctx := logger.WithRequestID(context.Background(), "req_test")
_, err := service.Generate(ctx, Request{UsageKey: "missing", Messages: []Message{{Role: "user", Content: "secret@example.com"}}})
if err == nil { t.Fatal("expected resolver error") }
if !strings.Contains(logs.String(), "error_stage=config_resolve") { t.Fatal(logs.String()) }
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `cd server && go test ./internal/infra/llm -run 'TestService(GenerateLogs|GenerateLogsFailures|ToolCallingLogs)' -count=1`

Expected: FAIL，因为提前返回分支没有日志，且新字段不存在。

- [ ] **Step 3: 实现统一 LLM 调用日志属性与失败记录**

在进入 `Generate` 和 `NewToolCallingChatModelWithUsage` 时生成调用 ID、开始时间与基础属性。配置解析后追加 provider/model/prompt version；所有返回错误经过统一 `logCallError`。消息统计只记录角色数量、总字符数和脱敏摘要；参数仅记录排序后的 key。

```go
func (s *Service) logCallError(ctx context.Context, startedAt time.Time, attrs []any, stage string, err error)
func requestLogAttrs(ctx context.Context, callID string, request Request) []any
func resolvedLogAttrs(resolved ResolvedUsage) []any
```

- [ ] **Step 4: 运行测试并确认 GREEN**

Run: `cd server && go test ./internal/infra/llm -count=1`

Expected: PASS，且敏感值断言继续通过。

- [ ] **Step 5: 提交 LLM 服务日志**

```bash
git add server/internal/infra/llm/service.go server/internal/infra/llm/service_test.go
git commit -m "feat: 完善大模型调用日志"
```

### Task 4: Agent Runner 执行与装配异常日志

**Files:**
- Modify: `server/internal/domain/agent/adk_runner.go`
- Modify: `server/internal/domain/agent/adk_runner_test.go`
- Modify: `server/internal/domain/agent/routes.go`
- Modify: `server/internal/domain/agent/routes_test.go`

- [ ] **Step 1: 写 Agent Runner 与装配错误失败测试**

为 `EinoADKAdviceRunner` 注入 logger，断言 Run 开始、成功和迭代事件错误分别记录模型元数据、输入/输出字符数、耗时和 `error_stage=agent_run`。路由装配测试制造 Tool Calling 初始化错误，断言错误不会被静默忽略。

```go
runner := NewEinoADKAdviceRunnerWithMetadataAndLogger(fakeRunner, metadata, testLogger)
_, err := runner.Run(logger.WithRequestID(context.Background(), "req_agent"), input)
if err == nil { t.Fatal("expected runner error") }
if !strings.Contains(logs.String(), "error_stage=agent_run") { t.Fatal(logs.String()) }
```

- [ ] **Step 2: 运行测试并确认 RED**

Run: `cd server && go test ./internal/domain/agent -run 'Test(EinoADKAdviceRunnerLogs|RegisterUserRoutesLogs)' -count=1`

Expected: FAIL，因为 runner 尚无 logger，路由装配错误仍被 `err == nil` 分支吞掉。

- [ ] **Step 3: 实现 Agent 日志与装配错误输出**

新增兼容现有构造函数的带 logger 构造方式；在 `Run` 中记录开始、完成和每个错误出口。`RegisterUserRoutes` 对模型初始化、工具创建、runner 创建失败分别输出 ERROR，并使用脱敏错误摘要；成功创建 runner 时传入同一个 logger。

```go
func NewEinoADKAdviceRunnerWithMetadataAndLogger(runner *adk.Runner, metadata AdviceRunMetadata, log *slog.Logger) *EinoADKAdviceRunner
```

- [ ] **Step 4: 运行测试并确认 GREEN**

Run: `cd server && go test ./internal/domain/agent -count=1`

Expected: PASS。

- [ ] **Step 5: 提交 Agent 日志**

```bash
git add server/internal/domain/agent/adk_runner.go server/internal/domain/agent/adk_runner_test.go server/internal/domain/agent/routes.go server/internal/domain/agent/routes_test.go
git commit -m "feat: 补齐智能体模型异常日志"
```

### Task 5: 全量验证与收尾

**Files:**
- Modify only if verification reveals a directly related defect.

- [ ] **Step 1: 格式化修改过的 Go 文件**

Run: `cd server && gofmt -w internal/infra/logger/*.go internal/infra/llm/service.go internal/infra/llm/service_test.go internal/app/user/router.go internal/app/user/router_test.go internal/domain/agent/adk_runner.go internal/domain/agent/adk_runner_test.go internal/domain/agent/routes.go internal/domain/agent/routes_test.go`

Expected: 命令成功且无输出。

- [ ] **Step 2: 运行服务端全量测试**

Run: `cd server && go test ./...`

Expected: PASS；如外部环境依赖导致失败，单独运行不依赖外部服务的相关包并记录原因。

- [ ] **Step 3: 检查日志安全和差异质量**

Run: `rg -n 'APIKey|Token|ImageURLs|Messages' server/internal/infra/logger server/internal/infra/llm/service.go server/internal/domain/agent/adk_runner.go`

Expected: 仅出现配置读取或安全测试代码，不存在把敏感字段直接传给 `slog` 的调用。

Run: `git diff --check && git status --short`

Expected: 无空白错误；状态只包含本任务明确修改的文件。

- [ ] **Step 4: 必要时提交验证修正**

```bash
git add <only-related-files>
git commit -m "test: 完善服务端日志验证"
```
