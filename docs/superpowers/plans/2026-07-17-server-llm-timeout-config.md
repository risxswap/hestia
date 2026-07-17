# 服务端大模型超时配置与诊断日志实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Agent 整轮超时和模型 HTTP 超时改为可校验的环境配置，并让超时失败日志直接显示生效配置、错误链、节点路径和超时来源。

**Architecture:** `infra/config` 负责读取和校验秒数；`infra/llm.Service` 负责把模型请求超时注入 Qwen/OpenAI 兼容客户端；`domain/agent.Service` 负责 Agent 整轮 Context 超时。Agent Runner 元数据携带供应商主机名和两层超时，公共诊断函数只输出结构化、脱敏的信息。

**Tech Stack:** Go、`caarlos0/env`、Eino ADK、标准库 `context`/`errors`/`net/url`/`log/slog`、Go testing

---

## 文件结构

- 修改 `server/internal/infra/config/config.go`：新增超时配置、默认值和启动校验。
- 修改 `server/internal/infra/config/config_test.go`：覆盖默认值、环境覆盖和非法层级。
- 修改 `server/internal/infra/llm/service.go`：保存模型请求超时并注入两类客户端配置。
- 修改 `server/internal/infra/llm/service_test.go`：验证默认值与覆盖值实际进入 Eino 配置。
- 修改 `server/internal/domain/agent/model.go`：扩展运行元数据，承载可安全记录的 provider host 和超时值。
- 修改 `server/internal/domain/agent/service.go`：保存 Agent 超时，应用 Context，并记录结构化诊断字段。
- 新增 `server/internal/domain/agent/error_diagnostics.go`：集中处理错误链脱敏、节点路径和超时来源分类。
- 修改 `server/internal/domain/agent/service_test.go`：验证注入超时及失败日志。
- 新增 `server/internal/domain/agent/error_diagnostics_test.go`：验证诊断解析和敏感信息保护。
- 修改 `server/internal/domain/agent/routes.go`：从全局配置向 LLM Service、Agent Service 和 Runner 元数据传递超时。
- 修改 `server/internal/domain/clothes/routes.go`：图片识别复用同一个模型请求超时配置。

### Task 1: 配置加载与校验

**Files:**
- Modify: `server/internal/infra/config/config_test.go`
- Modify: `server/internal/infra/config/config.go`

- [ ] **Step 1: 写默认值和环境覆盖失败测试**

在 `config_test.go` 增加：

```go
func TestLoadUsesLLMTimeoutDefaults(t *testing.T) {
	unsetenv(t, "AGENT_RUNNER_TIMEOUT_SECONDS")
	unsetenv(t, "LLM_REQUEST_TIMEOUT_SECONDS")
	cfg, err := config.Load()
	if err != nil { t.Fatalf("load config: %v", err) }
	if cfg.AgentRunnerTimeoutSeconds != 75 || cfg.LLMRequestTimeoutSeconds != 60 {
		t.Fatalf("unexpected timeout defaults: %d/%d", cfg.AgentRunnerTimeoutSeconds, cfg.LLMRequestTimeoutSeconds)
	}
}

func TestLoadUsesLLMTimeoutEnvironmentOverrides(t *testing.T) {
	t.Setenv("AGENT_RUNNER_TIMEOUT_SECONDS", "120")
	t.Setenv("LLM_REQUEST_TIMEOUT_SECONDS", "90")
	cfg, err := config.Load()
	if err != nil { t.Fatalf("load config: %v", err) }
	if cfg.AgentRunnerTimeoutSeconds != 120 || cfg.LLMRequestTimeoutSeconds != 90 {
		t.Fatalf("unexpected timeout overrides: %d/%d", cfg.AgentRunnerTimeoutSeconds, cfg.LLMRequestTimeoutSeconds)
	}
}
```

同步更新 `TestConfigExposesOnlyRuntimeFields` 的字段清单。

- [ ] **Step 2: 运行测试确认红灯**

Run: `cd server && go test ./internal/infra/config -run 'TestLoadUsesLLMTimeout|TestConfigExposesOnlyRuntimeFields' -count=1`

Expected: FAIL，提示 `Config` 缺少两个超时字段或字段数量不匹配。

- [ ] **Step 3: 实现配置字段与默认值**

在 `Config` 增加：

```go
AgentRunnerTimeoutSeconds int `env:"AGENT_RUNNER_TIMEOUT_SECONDS" envDefault:"75"`
LLMRequestTimeoutSeconds  int `env:"LLM_REQUEST_TIMEOUT_SECONDS" envDefault:"60"`
```

并在 `Load` 的初始值中明确设置 `75` 和 `60`。

- [ ] **Step 4: 运行测试确认绿灯**

Run: `cd server && go test ./internal/infra/config -run 'TestLoadUsesLLMTimeout|TestConfigExposesOnlyRuntimeFields' -count=1`

Expected: PASS。

- [ ] **Step 5: 写非法配置失败测试**

增加表驱动测试，分别覆盖 `0`、负数、非整数以及 Agent 超时不大于模型超时：

```go
func TestLoadRejectsInvalidLLMTimeouts(t *testing.T) {
	tests := []struct{ name, agent, llm string }{
		{"zero agent", "0", "60"},
		{"negative llm", "75", "-1"},
		{"agent not greater", "60", "60"},
		{"non integer", "later", "60"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENT_RUNNER_TIMEOUT_SECONDS", tt.agent)
			t.Setenv("LLM_REQUEST_TIMEOUT_SECONDS", tt.llm)
			if _, err := config.Load(); err == nil { t.Fatal("expected timeout config error") }
		})
	}
}
```

- [ ] **Step 6: 运行非法配置测试确认红灯**

Run: `cd server && go test ./internal/infra/config -run TestLoadRejectsInvalidLLMTimeouts -count=1`

Expected: FAIL，零值/负数/错误层级尚未被拒绝。

- [ ] **Step 7: 实现启动校验**

在 `env.Parse` 后增加私有函数：

```go
func validateTimeouts(cfg *Config) error {
	if cfg.AgentRunnerTimeoutSeconds <= 0 { return errors.New("AGENT_RUNNER_TIMEOUT_SECONDS must be positive") }
	if cfg.LLMRequestTimeoutSeconds <= 0 { return errors.New("LLM_REQUEST_TIMEOUT_SECONDS must be positive") }
	if cfg.AgentRunnerTimeoutSeconds <= cfg.LLMRequestTimeoutSeconds {
		return errors.New("AGENT_RUNNER_TIMEOUT_SECONDS must be greater than LLM_REQUEST_TIMEOUT_SECONDS")
	}
	return nil
}
```

`Load` 返回前调用该函数。

- [ ] **Step 8: 运行配置包全量测试**

Run: `cd server && go test ./internal/infra/config -count=1`

Expected: PASS。

- [ ] **Step 9: 提交配置改动**

```bash
git add server/internal/infra/config/config.go server/internal/infra/config/config_test.go
git commit -m "feat: 增加大模型超时配置"
```

### Task 2: 将超时注入模型与 Agent 调用链

**Files:**
- Modify: `server/internal/infra/llm/service_test.go`
- Modify: `server/internal/infra/llm/service.go`
- Modify: `server/internal/domain/agent/service_test.go`
- Modify: `server/internal/domain/agent/service.go`
- Modify: `server/internal/domain/agent/routes.go`
- Modify: `server/internal/domain/clothes/routes.go`

- [ ] **Step 1: 写模型客户端超时失败测试**

在 `service_test.go` 使用现有 capture client 分别断言 Qwen 和 OpenAI 配置：

```go
service := NewServiceWithTimeout(testResolver(), client, 37*time.Second)
// 调用 NewToolCallingChatModelWithUsage 后：
if capturedQwen.Timeout != 37*time.Second { t.Fatalf("unexpected qwen timeout: %s", capturedQwen.Timeout) }
if capturedOpenAI.Timeout != 37*time.Second { t.Fatalf("unexpected openai timeout: %s", capturedOpenAI.Timeout) }
```

另加旧 `NewService` 构造路径断言默认 `60*time.Second`。

- [ ] **Step 2: 运行模型超时测试确认红灯**

Run: `cd server && go test ./internal/infra/llm -run 'TestService.*RequestTimeout' -count=1`

Expected: FAIL，`NewServiceWithTimeout` 尚不存在。

- [ ] **Step 3: 实现 LLM Service 超时注入**

增加默认值和兼容构造器：

```go
const defaultRequestTimeout = 60 * time.Second

func NewService(resolver *ConfigResolver, client Client) *Service {
	return NewServiceWithTimeout(resolver, client, defaultRequestTimeout)
}

func NewServiceWithTimeout(resolver *ConfigResolver, client Client, timeout time.Duration) *Service {
	if timeout <= 0 { timeout = defaultRequestTimeout }
	return &Service{resolver: resolver, client: client, logger: slog.Default(), requestTimeout: timeout}
}
```

在 `Service` 增加 `requestTimeout time.Duration`，并将 `qwenChatModelConfig`、`openAIChatModelConfig` 改为接收 timeout 参数，写入 `Timeout`。

- [ ] **Step 4: 运行 LLM 包测试确认绿灯**

Run: `cd server && go test ./internal/infra/llm -count=1`

Expected: PASS。

- [ ] **Step 5: 写 Agent Context 超时失败测试**

增加一个阻塞到 `ctx.Done()` 的 runner，并通过 `NewServiceWithRunnerTimeout(..., 20*time.Millisecond)` 构造 Service：

```go
type contextDeadlineRunner struct{ deadline time.Time }
func (r *contextDeadlineRunner) Run(ctx context.Context, _ AdviceRunInput) (AdviceRunOutput, error) {
	r.deadline, _ = ctx.Deadline()
	<-ctx.Done()
	return AdviceRunOutput{}, ctx.Err()
}
```

断言调用约 20ms 返回，且 runner 截止时间与注入值一致。

- [ ] **Step 6: 运行 Agent 超时测试确认红灯**

Run: `cd server && go test ./internal/domain/agent -run TestServiceChatUsesConfiguredRunnerTimeout -count=1`

Expected: FAIL，构造器尚不存在或仍等待固定 15 秒。

- [ ] **Step 7: 实现 Agent Service 超时注入**

增加 `runnerTimeout time.Duration` 字段、`75*time.Second` 默认值和兼容构造器：

```go
func NewServiceWithRunnerTimeout(repo Repository, clothes ClothesAdviceService, runner AdviceRunner, timeout time.Duration) *Service {
	service := NewServiceWithRunner(repo, clothes, runner)
	if timeout > 0 { service.runnerTimeout = timeout }
	return service
}

func (s *Service) SetRunnerTimeout(timeout time.Duration) {
	if s != nil && timeout > 0 { s.runnerTimeout = timeout }
}
```

让所有 Service 构造器最终带上默认值；聊天执行处使用 `s.runnerTimeout` 创建 Context，删除固定 `defaultAdviceRunnerTimeout`。

- [ ] **Step 8: 将全局配置传入所有 LLM 使用路径**

在 `agent/routes.go`：

```go
agentTimeout := time.Duration(deps.Config.AgentRunnerTimeoutSeconds) * time.Second
llmTimeout := time.Duration(deps.Config.LLMRequestTimeoutSeconds) * time.Second
service.SetRunnerTimeout(agentTimeout)
llmService := llm.NewServiceWithTimeout(resolver, deps.LLM, llmTimeout)
```

在 `clothes/routes.go` 的图片识别 LLM Service 同样使用 `NewServiceWithTimeout`。对 `deps.Config == nil` 保持默认构造路径，避免测试路由 panic。

- [ ] **Step 9: 运行相关包测试**

Run: `cd server && go test ./internal/domain/agent ./internal/domain/clothes ./internal/infra/llm -count=1`

Expected: PASS。

- [ ] **Step 10: 提交调用链改动**

```bash
git add server/internal/infra/llm/service.go server/internal/infra/llm/service_test.go server/internal/domain/agent/service.go server/internal/domain/agent/service_test.go server/internal/domain/agent/routes.go server/internal/domain/clothes/routes.go
git commit -m "feat: 注入 Agent 与模型请求超时"
```

### Task 3: 结构化超时错误诊断

**Files:**
- Create: `server/internal/domain/agent/error_diagnostics.go`
- Create: `server/internal/domain/agent/error_diagnostics_test.go`
- Modify: `server/internal/domain/agent/model.go`
- Modify: `server/internal/domain/agent/adk_runner.go`
- Modify: `server/internal/domain/agent/routes.go`
- Modify: `server/internal/domain/agent/service.go`
- Modify: `server/internal/domain/agent/service_test.go`

- [ ] **Step 1: 写错误诊断解析失败测试**

构造带 URL、Bearer Token、Eino node path 和 `context.DeadlineExceeded` 的包装错误，断言：

```go
diagnostics := agentErrorDiagnostics(runnerCtx, wrappedErr, metadata)
if diagnostics.TimeoutSource != "agent_context" { t.Fatalf("unexpected source: %s", diagnostics.TimeoutSource) }
if diagnostics.NodePath != "node_1, ChatModel" { t.Fatalf("unexpected node path: %s", diagnostics.NodePath) }
if diagnostics.ProviderHost != "api.siliconflow.cn" { t.Fatalf("unexpected host: %s", diagnostics.ProviderHost) }
if strings.Contains(diagnostics.ErrorChain, "secret-token") || strings.Contains(diagnostics.ErrorChain, "https://") {
	t.Fatalf("diagnostics leaked sensitive value: %s", diagnostics.ErrorChain)
}
```

再用未超时 Context 和 `net.Error` timeout 覆盖 `http_transport`，普通错误覆盖 `unknown`。

- [ ] **Step 2: 运行诊断测试确认红灯**

Run: `cd server && go test ./internal/domain/agent -run TestAgentErrorDiagnostics -count=1`

Expected: FAIL，诊断类型和函数尚不存在。

- [ ] **Step 3: 实现独立诊断模块**

`error_diagnostics.go` 定义内部结构：

```go
type errorDiagnostics struct {
	ProviderCode, ModelCode, ProviderHost string
	AgentTimeoutMS, LLMTimeoutMS int64
	ContextDeadline, ContextError string
	ErrorType, ErrorChain, NodePath, TimeoutSource string
}
```

实现：

- 使用 `url.Parse` 只提取 `Hostname()`。
- 使用 `errors.Unwrap` 逐层获取 `%T` 和 `serverlogger.ErrorSummary(current)`，限制链长度为 8。
- 从脱敏后的错误文本解析 `node path: [...]`，只保留方括号内容。
- 若 runner Context 的 cause 是 `DeadlineExceeded`，分类为 `agent_context`；否则若错误链中存在实现 `net.Error` 且 `Timeout()` 为真，分类为 `http_transport`；错误链包含模型客户端 deadline 且 runner Context 未过期时分类为 `llm_client`；其余为 `unknown`。

- [ ] **Step 4: 运行诊断单测确认绿灯**

Run: `cd server && go test ./internal/domain/agent -run TestAgentErrorDiagnostics -count=1`

Expected: PASS。

- [ ] **Step 5: 写 Agent 失败日志失败测试**

用 JSON slog handler 捕获 `agent llm call failed`，runner 返回包装超时错误，断言日志包含：

```text
provider_code, model_code, provider_host,
agent_timeout_ms, llm_timeout_ms,
context_deadline, context_error,
error_type, error_chain, node_path, timeout_source
```

并断言日志不包含完整 provider URL、Bearer Token 和原始敏感错误值。

- [ ] **Step 6: 运行日志测试确认红灯**

Run: `cd server && go test ./internal/domain/agent -run TestServiceChatLogsDetailedAgentFailure -count=1`

Expected: FAIL，现有日志缺少结构化诊断字段。

- [ ] **Step 7: 将安全元数据和诊断字段接入失败日志**

扩展 `AdviceRunMetadata`：

```go
ProviderHost   string
AgentTimeoutMS int64
LLMTimeoutMS   int64
```

在路由装配时从 `resolvedUsage.Provider.APIBaseURL` 提取 host，并写入两层超时。让 `EinoADKAdviceRunner` 暴露只读 `Metadata() AdviceRunMetadata`；Service 在执行前从实现该可选接口的 runner 获取元数据。失败分支调用诊断函数，并把各字段追加到 `runnerLog.ErrorContext`。

必须在 `cancelRunner()` 之前采集 runner Context 的 deadline/cause，避免主动 cancel 覆盖真实原因。

- [ ] **Step 8: 运行 Agent 包全量测试**

Run: `cd server && go test ./internal/domain/agent -count=1`

Expected: PASS，日志断言中没有敏感内容。

- [ ] **Step 9: 运行服务端全量验证**

Run: `cd server && gofmt -w internal/infra/config/config.go internal/infra/config/config_test.go internal/infra/llm/service.go internal/infra/llm/service_test.go internal/domain/agent/model.go internal/domain/agent/service.go internal/domain/agent/service_test.go internal/domain/agent/error_diagnostics.go internal/domain/agent/error_diagnostics_test.go internal/domain/agent/adk_runner.go internal/domain/agent/routes.go internal/domain/clothes/routes.go`

Run: `cd server && go test ./... -count=1`

Expected: 所有包 PASS。

Run: `git diff --check && git status --short`

Expected: 无空白错误；只显示本计划涉及的文件。

- [ ] **Step 10: 提交诊断日志改动**

```bash
git add server/internal/domain/agent/error_diagnostics.go server/internal/domain/agent/error_diagnostics_test.go server/internal/domain/agent/model.go server/internal/domain/agent/adk_runner.go server/internal/domain/agent/routes.go server/internal/domain/agent/service.go server/internal/domain/agent/service_test.go server/internal/domain/clothes/routes.go
git commit -m "feat: 增强 Agent 超时错误诊断"
```
