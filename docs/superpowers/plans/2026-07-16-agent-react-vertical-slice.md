# Agent ReAct 端到端纵切 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将现有建议草稿编排器替换为数据库动态 system prompt 驱动的标准 ReAct Agent，新增主动视觉工具，并以 ADK 原生事件作为审计和前端处理过程的唯一来源。

**Architecture:** 每次聊天请求由 `AdviceAgentFactory` 从 `system_configs(agent.prompts/system)` 和 LLM 配置创建 Agent；8 个工具稳定暴露，Agent 自主执行 Action/Observation。视觉工具内部解析私有素材并调用 `image_analysis` 模型；Runner 直接解析 Eino assistant/tool 消息事件，删除最终 JSON 和工具自报协议。

**Tech Stack:** Go 1.24、Gin、sqlx、MySQL、CloudWeGo Eino ADK、微信小程序原生 JavaScript。

---

## 文件职责

- 新增 `server/internal/domain/agent/prompt.go`：读取、解析和校验唯一 system prompt。
- 新增 `server/internal/domain/agent/factory.go`：按请求创建 ReAct Runner，不持有启动期固定模型。
- 新增 `server/internal/domain/agent/image_tool.go`：视觉工具输入、输出和视觉模型适配。
- 重写 `server/internal/domain/agent/adk_runner.go`：自然语言最终回复和 ADK 原生事件采集。
- 修改 `server/internal/domain/agent/tools.go`：固定 8 工具及统一业务错误结构。
- 修改 `server/internal/domain/agent/service.go`：请求级 Runner、原生审计持久化和真实草稿投影。
- 修改 `server/internal/domain/agent/routes.go`：只装配 Factory 依赖，删除静默初始化。
- 修改 `server/internal/domain/agent/model.go`：删除旧自报协议模型，新增原生事件结果模型。
- 修改 `server/internal/infra/llm/repo.go`：复用 `system_configs` 读取字符串配置。
- 新增 `server/internal/infra/migration/mysql/013_agent_react_prompt.sql`：写入唯一 XML system prompt。
- 修改 `miniapp/pages/advisor/advisor.js` 和验证脚本：删除 JSON 回复兼容解析，展示真实读取步骤。

### Task 1: 唯一动态 System Prompt

**Files:**
- Create: `server/internal/domain/agent/prompt.go`
- Create: `server/internal/domain/agent/prompt_test.go`
- Modify: `server/internal/infra/llm/repo.go`
- Modify: `server/internal/infra/llm/service_test.go`
- Create: `server/internal/infra/migration/mysql/013_agent_react_prompt.sql`

- [ ] **Step 1: 写 prompt 读取和 XML 校验失败测试**

在 `prompt_test.go` 定义最小仓储桩并覆盖：有效 XML、配置缺失、非激活、空内容、非法 XML、缺少六个一级节点。

```go
func TestSystemPromptServiceLoad(t *testing.T) {
    repo := &stubSystemPromptRepo{value: validSystemPromptXML()}
    prompt, err := NewSystemPromptService(repo).Load(context.Background())
    if err != nil || prompt != validSystemPromptXML() {
        t.Fatalf("load system prompt: prompt=%q err=%v", prompt, err)
    }
}

func TestValidateSystemPromptRejectsMissingToolPolicy(t *testing.T) {
    err := validateSystemPrompt(`<agent><identity>x</identity></agent>`)
    if !errors.Is(err, ErrInvalidSystemPrompt) {
        t.Fatalf("expected invalid prompt, got %v", err)
    }
}
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `cd server && go test ./internal/domain/agent -run 'Test(SystemPrompt|ValidateSystemPrompt)' -count=1`

Expected: FAIL，提示 `NewSystemPromptService` 或 `validateSystemPrompt` 未定义。

- [ ] **Step 3: 实现配置仓储和 XML 校验**

```go
type SystemPromptRepository interface {
    ActiveString(ctx context.Context, group, key string) (string, error)
}

func (s *SystemPromptService) Load(ctx context.Context) (string, error) {
    value, err := s.repo.ActiveString(ctx, "agent.prompts", "system")
    if err != nil { return "", err }
    value = strings.TrimSpace(value)
    if err := validateSystemPrompt(value); err != nil { return "", err }
    return value, nil
}
```

`validateSystemPrompt` 使用 `encoding/xml` 解码并验证 `identity`、`operating_mode`、`decision_principles`、`tool_policy`、`image_boundaries`、`response_contract`。

- [ ] **Step 4: 写入唯一 seed 配置**

`013_agent_react_prompt.sql` 使用 MySQL upsert 写入完整 XML：

```sql
SET @agent_system_prompt = '<agent>
  <identity>你是 Hestia，持续了解用户的个人 AI 形象顾问。你的目标是提供中性、具体、可执行、可调整的建议。</identity>
  <operating_mode>使用 ReAct 工作：理解目标，判断信息是否充分，自主选择工具，根据工具观察继续行动，直到可以回答或需要向用户追问。不向用户展示隐藏推理过程。</operating_mode>
  <decision_principles>
    <principle priority="1">用户本轮明确要求优先。</principle>
    <principle priority="2">用户确认的事实和真实反馈优先于 AI 推断。</principle>
    <principle priority="3">信息不足且不同答案会显著改变建议时，先追问。</principle>
    <principle priority="4">普通聊天或常识回答不必调用工具。</principle>
    <principle priority="5">不要为了展示能力而调用无关工具。</principle>
  </decision_principles>
  <tool_policy>
    <inspect_user_image>回答依赖图片中可观察的信息时调用；未观察图片时不得声称根据照片得出结论。</inspect_user_image>
    <get_profile_context>建议依赖稳定档案、生活场景或风格目标时调用。</get_profile_context>
    <get_memory_context>建议依赖偏好、禁忌、历史反馈或过去信息时调用；查询使用短关键词。</get_memory_context>
    <get_wardrobe_context>用户要求使用现有衣物或引用具体衣物时调用。</get_wardrobe_context>
    <get_current_advice_draft>修改、解释、比较、保存或放弃当前方案时调用。</get_current_advice_draft>
    <create_advice_draft>用户需要完整可调整方案且信息充分时调用；普通问答和单点建议不创建草稿。</create_advice_draft>
    <update_advice_draft>用户明确修改已有草稿时调用，并先读取当前草稿。</update_advice_draft>
    <discard_advice_draft>用户明确放弃当前草稿时调用，意图不清时先确认。</discard_advice_draft>
  </tool_policy>
  <image_boundaries>只描述可观察的造型、比例、廓形、色彩、发型和单品结构。区分观察事实与推断，不做人脸身份识别、相貌匹配、医疗判断或羞辱性评价。</image_boundaries>
  <response_contract>默认自然对话，不输出 JSON、工具名或内部步骤。建议说明适合原因、不建议项和替代方案；工具失败时如实说明。</response_contract>
</agent>';

INSERT INTO system_configs (`group`, `key`, `value`, `value_type`, `description`, `status`)
VALUES ('agent.prompts', 'system', JSON_QUOTE(@agent_system_prompt), 'string', 'Hestia Agent system prompt', 'active')
ON DUPLICATE KEY UPDATE value = VALUES(value), value_type = VALUES(value_type), status = 'active';
```

XML 正文采用已确认 spec 的完整内容，不使用占位文字。

- [ ] **Step 5: 运行测试确认 GREEN**

Run: `cd server && go test ./internal/domain/agent ./internal/infra/llm -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add server/internal/domain/agent/prompt.go server/internal/domain/agent/prompt_test.go server/internal/infra/llm/repo.go server/internal/infra/llm/service_test.go server/internal/infra/migration/mysql/013_agent_react_prompt.sql
git commit -m "feat: 动态加载 Agent system prompt"
```

### Task 2: 统一工具错误与稳定工具集合

**Files:**
- Modify: `server/internal/domain/agent/tools.go`
- Modify: `server/internal/domain/agent/tools_test.go`
- Modify: `server/internal/domain/agent/model.go`

- [ ] **Step 1: 写统一错误结构和 8 工具清单测试**

```go
func TestAdviceToolsExposeStableReActToolSet(t *testing.T) {
    tools := mustAdviceToolsWithAllDependencies(t)
    names := toolNames(t, tools)
    expected := []string{
        "inspect_user_image", "get_profile_context", "get_memory_context",
        "get_wardrobe_context", "get_current_advice_draft",
        "create_advice_draft", "update_advice_draft", "discard_advice_draft",
    }
    if diff := cmp.Diff(expected, names); diff != "" { t.Fatal(diff) }
}

func TestAdviceToolBusinessErrorIsSafe(t *testing.T) {
    output := adviceToolError("draft_not_found", "当前没有可修改的草稿", false)
    if output.OK || output.ErrorCode != "draft_not_found" || output.Retryable {
        t.Fatalf("unexpected error: %#v", output)
    }
}
```

- [ ] **Step 2: 运行测试确认 RED**

Run: `cd server && go test ./internal/domain/agent -run 'TestAdviceTool' -count=1`

Expected: FAIL，因为视觉工具和统一错误字段不存在。

- [ ] **Step 3: 删除旧自报协议模型并统一工具结果**

从 `model.go` 删除 `AdviceToolCall`、`CreateDraftInput/UpdateDraftInput` 的最终 JSON payload 映射用途和 `AdviceRunOutput.ToolCalls`。新增：

```go
type ToolResultEnvelope struct {
    OK        bool   `json:"ok"`
    ErrorCode string `json:"error_code,omitempty"`
    Message   string `json:"message,omitempty"`
    Retryable bool   `json:"retryable,omitempty"`
}
```

所有工具将可预期业务错误编码为 envelope，不返回数据库错误原文；基础设施错误仍返回 Go error，让 ADK 标记真实失败。

- [ ] **Step 4: 更新工具 description 和固定注册顺序**

每个 description 只包含用途、调用时机、返回内容和限制。`NewAdviceToolsWithDependencies` 缺任一必要依赖时返回装配错误，不静默减少工具集合。

- [ ] **Step 5: 运行测试确认 GREEN**

Run: `cd server && go test ./internal/domain/agent -run 'TestAdviceTool' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add server/internal/domain/agent/model.go server/internal/domain/agent/tools.go server/internal/domain/agent/tools_test.go
git commit -m "refactor: 统一 ReAct 工具契约"
```

### Task 3: 主动视觉工具

**Files:**
- Create: `server/internal/domain/agent/image_tool.go`
- Create: `server/internal/domain/agent/image_tool_test.go`
- Modify: `server/internal/domain/agent/tools.go`
- Modify: `server/internal/domain/agent/routes.go`
- Modify: `server/internal/domain/asset/service.go`
- Modify: `server/internal/domain/asset/repo.go`

- [ ] **Step 1: 写视觉工具权限和模型输入测试**

```go
func TestInspectUserImageUsesOwnedAssetAndVisionModel(t *testing.T) {
    assets := &stubImageAssetReader{items: []ImageAsset{{PublicID: "ast_photo", UserID: 12, MIMEType: "image/jpeg"}}}
    analyzer := &stubImageAnalyzer{result: ImageAnalysis{Images: []ImageObservation{{AssetPublicID: "ast_photo"}}}}
    tool := newInspectUserImageTool(assets, analyzer)
    raw, err := tool.InvokableRun(contextWithAdviceToolSession(context.Background(), 12, 101), `{"asset_public_ids":["ast_photo"],"analysis_focus":"穿搭比例"}`)
    if err != nil { t.Fatal(err) }
    if !strings.Contains(raw, `"asset_public_id":"ast_photo"`) { t.Fatal(raw) }
    if analyzer.requiredCaps[0] != "vision" { t.Fatalf("caps=%v", analyzer.requiredCaps) }
}
```

另写非本人素材、非图片 MIME、空 ID、超过数量限制、视觉模型不可用和非法结构化结果测试。

- [ ] **Step 2: 运行测试确认 RED**

Run: `cd server && go test ./internal/domain/agent -run 'TestInspectUserImage' -count=1`

Expected: FAIL，提示视觉工具类型未定义。

- [ ] **Step 3: 为素材域增加用户绑定的视觉读取接口**

```go
type ImageAssetReader interface {
    FindImagesForUser(ctx context.Context, userID int64, publicIDs []string) ([]ImageAsset, error)
    PrivateImageURLs(ctx context.Context, objectKeys []string) (map[string]string, error)
}
```

查询必须同时约束 `user_id`、`deleted_at IS NULL` 和支持的图片 MIME；结果顺序按请求 ID 重组。签名 URL 不写日志和审计。

- [ ] **Step 4: 实现独立视觉分析器**

```go
type ImageAnalyzer interface {
    Analyze(ctx context.Context, input ImageAnalysisInput) (ImageAnalysis, error)
}

type LLMImageAnalyzer struct { llm *llm.Service }
```

`LLMImageAnalyzer` 使用 `UsageKey: "image_analysis"`、`RequiredCaps: []string{"vision"}` 和 `ImageURLs` 调用现有 LLM Service；system 消息包含图片安全边界，返回 JSON 后严格解析 observation、confidence、limitations。

- [ ] **Step 5: 实现 `inspect_user_image` 并注册**

工具输入只接受 `asset_public_ids` 和 `analysis_focus`。成功结果只返回结构化观察；可预期错误返回统一 envelope。

- [ ] **Step 6: 运行测试确认 GREEN**

Run: `cd server && go test ./internal/domain/agent ./internal/domain/asset ./internal/infra/llm -count=1`

Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add server/internal/domain/agent/image_tool.go server/internal/domain/agent/image_tool_test.go server/internal/domain/agent/tools.go server/internal/domain/agent/routes.go server/internal/domain/asset/service.go server/internal/domain/asset/repo.go
git commit -m "feat: 新增 Agent 主动视觉工具"
```

### Task 4: 请求级 Agent Factory

**Files:**
- Create: `server/internal/domain/agent/factory.go`
- Create: `server/internal/domain/agent/factory_test.go`
- Modify: `server/internal/domain/agent/routes.go`
- Modify: `server/internal/domain/agent/service.go`

- [ ] **Step 1: 写每请求加载 prompt 和模型的测试**

```go
func TestAdviceAgentFactoryLoadsCurrentPromptForEveryRun(t *testing.T) {
    prompts := &sequencePromptLoader{values: []string{validPrompt("第一次"), validPrompt("第二次")}}
    factory := newTestAdviceAgentFactory(prompts)
    first, _ := factory.NewRunner(context.Background())
    second, _ := factory.NewRunner(context.Background())
    if first.Metadata().PromptDigest == second.Metadata().PromptDigest {
        t.Fatal("expected current prompt on every request")
    }
}
```

覆盖 prompt、模型和工具装配失败必须返回错误，不能得到 nil Runner。

- [ ] **Step 2: 运行测试确认 RED**

Run: `cd server && go test ./internal/domain/agent -run 'TestAdviceAgentFactory' -count=1`

Expected: FAIL，因为 Factory 未定义。

- [ ] **Step 3: 实现 Factory 接口**

```go
type AdviceRunnerFactory interface {
    NewRunner(ctx context.Context) (AdviceRunner, error)
}

func (f *EinoAdviceAgentFactory) NewRunner(ctx context.Context) (AdviceRunner, error) {
    instruction, err := f.prompts.Load(ctx)
    if err != nil { return nil, err }
    model, usage, err := f.llm.NewToolCallingChatModelWithUsage(ctx, llm.Request{UsageKey: "agent_chat", RequiredCaps: []string{"text"}})
    if err != nil { return nil, err }
    return newEinoReActRunner(ctx, instruction, model, f.tools, usage)
}
```

`routes.go` 只装配依赖并把 Factory 注入 Service，删除所有嵌套 `if err == nil` 和启动期固定 Agent。

- [ ] **Step 4: Service 改为本轮请求创建 Runner**

`Service` 将 `runner AdviceRunner` 替换为 `runnerFactory AdviceRunnerFactory`。Factory 失败时助手消息标记失败、写失败步骤并将错误返回 handler，不使用固定回复伪装成功。

- [ ] **Step 5: 运行测试确认 GREEN**

Run: `cd server && go test ./internal/domain/agent -run 'TestAdviceAgentFactory|TestServiceChat' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add server/internal/domain/agent/factory.go server/internal/domain/agent/factory_test.go server/internal/domain/agent/routes.go server/internal/domain/agent/service.go server/internal/domain/agent/service_test.go
git commit -m "refactor: 按请求创建 ReAct Agent"
```

### Task 5: ADK 原生事件 Runner

**Files:**
- Rewrite: `server/internal/domain/agent/adk_runner.go`
- Rewrite: `server/internal/domain/agent/adk_runner_test.go`
- Modify: `server/internal/domain/agent/model.go`
- Delete obsolete online code from: `server/internal/domain/agent/runner.go`

- [ ] **Step 1: 写原生事件映射测试**

使用可注入的事件迭代器依次返回：带 `ToolCalls` 的 assistant message、`Role=Tool` 的 tool message、普通 assistant final message。

```go
func TestEinoRunnerCollectsNativeToolEvents(t *testing.T) {
    output, err := runFromEvents(nativeToolCallEvent(), nativeToolResultEvent(), nativeFinalEvent("已完成建议"))
    if err != nil { t.Fatal(err) }
    if output.AssistantText != "已完成建议" { t.Fatalf("%#v", output) }
    if output.Events[0].StepType != AgentStepTypeToolCall || output.Events[1].StepType != AgentStepTypeToolResult {
        t.Fatalf("events=%#v", output.Events)
    }
    if output.Events[0].ToolCallID != output.Events[1].ToolCallID { t.Fatal("tool call id mismatch") }
}
```

另测流式消息合并、多个工具调用、工具错误、无最终回复和 ADK event error。

- [ ] **Step 2: 运行测试确认 RED**

Run: `cd server && go test ./internal/domain/agent -run 'TestEinoRunner' -count=1`

Expected: FAIL，因为现有 Runner 只保留最后文本并解析 JSON。

- [ ] **Step 3: 删除旧 JSON 协议和规则 Runner**

删除 `parseAdviceRunOutputJSON`、payload 类型、`adviceRunAuditStepsFromToolCalls`、最终 JSON Instruction 和线上 `RuleBasedAdviceRunner`。最终文本直接取最后一个无 ToolCalls 的 assistant message content。

- [ ] **Step 4: 实现原生事件 Collector**

```go
type AdviceRunEvent struct {
    StepType      string
    Status        string
    ToolName      string
    ToolCallID    string
    InputSummary  string
    OutputSummary string
    StartedAt     time.Time
    FinishedAt    time.Time
}
```

assistant message 的 `ToolCalls` 生成 `tool_call`；`schema.Tool` message 使用 `ToolCallID` 和 `ToolName` 生成 `tool_result`；最后一个普通 assistant message作为 final response。摘要必须裁剪且去除 URL、图片内容和完整 JSON。

- [ ] **Step 5: 运行测试确认 GREEN**

Run: `cd server && go test ./internal/domain/agent -run 'TestEinoRunner' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add server/internal/domain/agent/adk_runner.go server/internal/domain/agent/adk_runner_test.go server/internal/domain/agent/model.go server/internal/domain/agent/runner.go
git commit -m "refactor: 使用 ADK 原生事件驱动 Agent"
```

### Task 6: Service 审计与真实草稿投影

**Files:**
- Modify: `server/internal/domain/agent/service.go`
- Modify: `server/internal/domain/agent/service_test.go`
- Modify: `server/internal/domain/agent/repo.go`

- [ ] **Step 1: 写原生事件持久化测试**

```go
func TestServicePersistsNativeRunEvents(t *testing.T) {
    factory := &stubRunnerFactory{output: AdviceRunOutput{
        AssistantText: "已结合档案给出建议。",
        Events: []AdviceRunEvent{
            {StepType: AgentStepTypeToolCall, ToolName: "get_profile_context", ToolCallID: "call_1"},
            {StepType: AgentStepTypeToolResult, ToolName: "get_profile_context", ToolCallID: "call_1", Status: AgentStepStatusSucceeded},
        },
    }}
    repo := &spyAgentRepo{}
    _, err := NewServiceWithRunnerFactory(repo, nil, factory).Chat(context.Background(), 12, "结合我的情况建议")
    if err != nil { t.Fatal(err) }
    assertStoredNativeSteps(t, repo.steps, "call_1")
}
```

另测写工具成功后刷新当前草稿、读取工具不生成草稿卡片、工具失败步骤保留、Factory/Runner 失败将消息标记 failed。

- [ ] **Step 2: 运行测试确认 RED**

Run: `cd server && go test ./internal/domain/agent -run 'TestService(PersistsNative|RefreshesDraft|KeepsFailed)' -count=1`

Expected: FAIL，因为 Service 仍消费旧 `AuditSteps/ToolCalls`。

- [ ] **Step 3: 简化 Service 运行链路**

删除二次执行 `output.ToolCalls` 的逻辑。Runner 中工具已经真实执行，Service 只负责：创建消息、创建 Runner、运行、逐项保存 `output.Events`、检测成功写工具结果、刷新当前草稿、更新助手消息和最终步骤。

- [ ] **Step 4: 保留真实失败步骤**

Runner 返回错误时先持久化已经采集的 events，再标记助手消息失败。不得生成 fallback assistant success message。

- [ ] **Step 5: 运行测试确认 GREEN**

Run: `cd server && go test ./internal/domain/agent -count=1`

Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add server/internal/domain/agent/service.go server/internal/domain/agent/service_test.go server/internal/domain/agent/repo.go
git commit -m "refactor: 持久化 Agent 原生执行事件"
```

### Task 7: 脱敏过程文案和小程序自然回复

**Files:**
- Modify: `server/internal/domain/agent/service.go`
- Modify: `server/internal/domain/agent/service_test.go`
- Modify: `miniapp/pages/advisor/advisor.js`
- Modify: `miniapp/scripts/verify-advisor-page.js`

- [ ] **Step 1: 写 8 个工具的安全展示测试**

```go
func TestSafeProcessCopyMapsReActReadTools(t *testing.T) {
    cases := map[string]string{
        "inspect_user_image": "查看你上传的照片",
        "get_profile_context": "读取个人档案",
        "get_memory_context": "检索长期记忆",
        "get_wardrobe_context": "查看核心衣物",
        "get_current_advice_draft": "读取当前方案",
    }
    for toolName, want := range cases {
        got, _ := safeProcessCopy(AgentStepTypeToolCall, toolName, AgentStepStatusSucceeded)
        if got != want { t.Fatalf("tool=%s got=%s", toolName, got) }
    }
}
```

小程序验证改为断言 `extractAssistantText` 已删除，SSE `message.text` 直接渲染自然语言。

- [ ] **Step 2: 运行测试确认 RED**

Run: `cd server && go test ./internal/domain/agent -run 'TestSafeProcessCopyMapsReActReadTools' -count=1 && cd ../miniapp && npm run verify:advisor-page`

Expected: 至少一项 FAIL，因为读取工具仍显示通用文案且小程序仍解析旧 JSON。

- [ ] **Step 3: 实现安全文案并删除前端兼容解析**

`safeProcessCopy` 对 8 工具提供固定文案；失败状态使用“未能完成本次读取/调整”等中性说明。`advisor.js` 直接读取自然文本，删除 fenced JSON、`assistant_text` 和 `tool_calls` 兼容逻辑。

- [ ] **Step 4: 运行测试确认 GREEN**

Run: `cd server && go test ./internal/domain/agent -count=1 && cd ../miniapp && npm run verify:advisor-page`

Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add server/internal/domain/agent/service.go server/internal/domain/agent/service_test.go miniapp/pages/advisor/advisor.js miniapp/scripts/verify-advisor-page.js
git commit -m "feat: 展示 Agent 真实工具处理过程"
```

### Task 8: 删除旧路径并完成全量验证

**Files:**
- Modify: `server/internal/domain/agent/runner.go`
- Modify: `server/internal/domain/agent/routes.go`
- Modify: `server/internal/domain/agent/model.go`
- Modify: `server/internal/domain/agent/service_test.go`
- Modify: `docs/superpowers/plans/2026-07-16-agent-react-vertical-slice.md`

- [ ] **Step 1: 扫描旧协议和静默降级**

Run:

```bash
rg -n "RuleBasedAdviceRunner|parseAdviceRunOutputJSON|adviceRunOutputPayload|runnerFallbackText|tool_calls.*总结|if .*err == nil" server/internal/domain/agent miniapp/pages/advisor
```

Expected: 只允许测试说明或零结果；线上旧协议引用必须全部删除。

- [ ] **Step 2: 删除剩余旧代码并格式化**

Run: `cd server && gofmt -w internal/domain/agent internal/domain/asset internal/infra/llm`

Expected: 命令成功，只有本计划涉及文件产生格式变化。

- [ ] **Step 3: 运行服务端全量测试**

Run: `cd server && go test ./... -count=1`

Expected: PASS，零失败。

- [ ] **Step 4: 运行小程序验证**

Run: `cd miniapp && npm run verify:api-client && npm run verify:api-integration && npm run verify:advisor-page`

Expected: 三条命令全部 PASS。

- [ ] **Step 5: 运行静态检查**

Run: `git diff --check && git status --short`

Expected: `git diff --check` 无输出；状态只包含本计划相关文件。

- [ ] **Step 6: 更新计划勾选并提交**

```bash
git add server miniapp docs/superpowers/plans/2026-07-16-agent-react-vertical-slice.md
git commit -m "feat: 完成 Agent ReAct 端到端纵切"
```

提交前再次检查 `git status --short`，不得加入无关未跟踪文件。
