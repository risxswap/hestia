# Agent SSE 实时回复流实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Agent 最终自然文本从 Eino 原生 MessageStream 实时发送为 SSE `delta`，并在停止、断开或中途失败时持久化已经生成的部分回复。

**Architecture:** `AdviceRunner` 直接升级为流式接口，Eino Runner 以 `EnableStreaming` 运行并在消费唯一 MessageStream 时同步聚合正文、发送增量、采集真实工具事件。Service 负责消息生命周期和取消后的独立持久化，Handler 只把增量编码为 SSE；小程序按 50ms 合并 `delta` 更新正文。删除最终 `message` 事件、JSON 最终回复协议和旧 Runner 兼容分支。

**Tech Stack:** Go、Gin、Eino ADK v0.7、SSE、微信小程序 `wx.request(enableChunked)`、Node 验证脚本

---

## 文件结构

- 修改 `server/internal/domain/agent/runner.go`：将 Runner 接口改为必须接收正文增量回调。
- 修改 `server/internal/domain/agent/adk_runner.go`：启用 Eino 原生流、消费 MessageStream、聚合自然文本并从真实事件生成审计。
- 修改 `server/internal/domain/agent/adk_runner_test.go`：覆盖正文流、ToolCall 流隔离、Markdown/中文分块、取消和部分输出。
- 修改 `server/internal/domain/agent/model.go`：增加 `stopped` 状态、`StreamDelta` 和流式回调契约；删除最终 JSON 输出所需字段。
- 修改 `server/internal/domain/agent/service.go`：接入正文增量、区分停止与失败、使用独立 Context 保存部分回复。
- 修改 `server/internal/domain/agent/service_test.go`：覆盖正常流、停止、连接写失败、中途失败和历史状态。
- 修改 `server/internal/domain/agent/handler.go`：输出 `delta`，删除 `message` 事件。
- 修改 `server/internal/domain/agent/routes_test.go`：验证新 SSE 顺序和停止行为。
- 修改 `miniapp/utils/api.js`：保留通用 SSE 分发，删除正文 `message` 使用路径。
- 修改 `miniapp/scripts/verify-api-client.js`：验证 chunked `delta` 与 UTF-8 分块。
- 修改 `miniapp/pages/advisor/advisor.js`：增加 50ms 正文缓冲、停止保留和 `stopped` 历史映射。
- 修改 `miniapp/scripts/verify-advisor-page.js`：验证增量合并、缓冲清理和停止恢复。

### Task 1: 将 Eino Runner 改为原生正文流

**Files:**
- Modify: `server/internal/domain/agent/runner.go`
- Modify: `server/internal/domain/agent/model.go`
- Modify: `server/internal/domain/agent/adk_runner.go`
- Modify: `server/internal/domain/agent/adk_runner_test.go`

- [ ] **Step 1: 写 Runner 多 chunk 失败测试**

在 `adk_runner_test.go` 构造返回以下流事件的测试 Runner：

```go
schema.StreamReaderFromArray([]*schema.Message{
	schema.AssistantMessage("你好，", nil),
	schema.AssistantMessage("可以用 **米白衬衫**", nil),
	schema.AssistantMessage(" 搭直筒裤。", nil),
})
```

调用新接口并断言增量顺序和完整聚合文本：

```go
var chunks []string
output, err := runner.Run(ctx, input, func(text string) error {
	chunks = append(chunks, text)
	return nil
})
if err != nil { t.Fatalf("run stream: %v", err) }
if diff := cmp.Diff([]string{"你好，", "可以用 **米白衬衫**", " 搭直筒裤。"}, chunks); diff != "" {
	t.Fatalf("chunks mismatch (-want +got):\n%s", diff)
}
if output.AssistantText != "你好，可以用 **米白衬衫** 搭直筒裤。" {
	t.Fatalf("unexpected full text: %q", output.AssistantText)
}
```

- [ ] **Step 2: 运行测试确认红灯**

Run: `cd server && go test ./internal/domain/agent -run TestEinoADKAdviceRunnerStreamsFinalAssistantText -count=1`

Expected: FAIL，`AdviceRunner.Run` 尚未接收增量回调，ADK Runner 也未启用 streaming。

- [ ] **Step 3: 升级 Runner 接口并启用 Eino streaming**

在 `runner.go` 定义唯一接口：

```go
type AdviceTextDeltaEmitter func(text string) error

type AdviceRunner interface {
	Run(ctx context.Context, input AdviceRunInput, emit AdviceTextDeltaEmitter) (AdviceRunOutput, error)
}
```

删除 `RuleBasedAdviceRunner`。路由和服务测试统一使用显式的流式 spy Runner，不保留一次性或规则型旧链路。

创建 Eino Runner 时显式启用流：

```go
adk.NewRunner(ctx, adk.RunnerConfig{
	Agent:           agent,
	EnableStreaming: true,
})
```

把 system instruction 中的最终 JSON 要求替换为：

```text
最终回答直接输出面向用户的自然语言或 Markdown，不输出 JSON、工具名或内部步骤。
调用工具时不要同时输出用户可见正文；完成必要工具调用后再生成最终回复。
```

- [ ] **Step 4: 实现唯一 MessageStream 消费器**

在 `adk_runner.go` 增加集中式消费函数：

```go
func consumeAdviceMessageOutput(
	ctx context.Context,
	variant *adk.MessageVariant,
	emit AdviceTextDeltaEmitter,
) (message *schema.Message, finalText string, err error)
```

规则：

- `variant.Role == schema.Tool`：完整读取并合并消息，只生成工具结果审计，不 emit。
- Assistant 流的第一个语义 chunk 含 `ToolCalls`：完整读取并合并，只生成工具调用审计，不 emit。
- Assistant 流的第一个语义 chunk 含正文：立即 emit，并按顺序继续读取、聚合正文。
- 已开始 emit 正文后出现 ToolCall：返回稳定错误 `ErrMixedAssistantStream`，不能继续向用户暴露内容。
- `Recv()` 返回非 EOF 错误时，返回已聚合的部分正文和原错误。
- 始终 `defer stream.Close()`，不能再次调用 `GetMessage()` 消费同一流。

每个正文 chunk 在 emit 成功后才追加到已确认输出；这样 emit 写失败时返回的部分文本与客户端实际收到的文本一致。

- [ ] **Step 5: 从真实 ADK 事件生成审计步骤**

删除 `parseAdviceRunOutputJSON`、`adviceRunOutputPayload`、最终 JSON fence 解析，以及根据最终 `tool_calls` 生成审计的路径。同步删除仅为旧 JSON 协议服务的 payload/date/section 转换类型和测试。

从 `AdviceRunOutput` 删除 `ToolCalls`，从 `model.go` 删除 `AdviceToolCall`。真实工具执行只发生在 ADK ToolsNode 内。

对 Assistant ToolCall 消息生成 `AgentStepTypeToolCall`；对 Tool 消息生成 `AgentStepTypeToolResult`。审计只保存工具名、tool call id 和固定安全摘要，不复制原始 arguments/content：

```go
AdviceRunAuditStep{
	StepType:      AgentStepTypeToolCall,
	Status:        AgentStepStatusSucceeded,
	ToolName:      call.Function.Name,
	ToolCallID:    call.ID,
	DecisionLabel: call.Function.Name,
	InputSummary:  "ADK 工具调用",
}
```

最终输出固定 `DecisionLabel: "final_response"`。

- [ ] **Step 6: 写 ToolCall 隔离和部分输出失败测试**

新增测试：

1. ToolCall assistant stream 和 Tool result event 均不触发正文 emitter，审计顺序为 call/result。
2. 正文流第二个 chunk 返回错误时，output 保留第一个成功 emit 的 chunk。
3. emitter 返回错误时，Runner 立即停止并返回已确认部分文本。
4. 正文后混入 ToolCall 返回 `ErrMixedAssistantStream`。

- [ ] **Step 7: 运行 Runner 包测试确认绿灯**

Run: `cd server && go test ./internal/domain/agent -run 'TestEinoADKAdviceRunner' -count=1`

Expected: PASS。

- [ ] **Step 8: 提交 Runner 流改动**

```bash
git add server/internal/domain/agent/runner.go server/internal/domain/agent/model.go server/internal/domain/agent/adk_runner.go server/internal/domain/agent/adk_runner_test.go
git commit -m "feat: 使用 Eino 原生流生成 Agent 回复"
```

### Task 2: Service 保存完整或部分流式回复

**Files:**
- Modify: `server/internal/domain/agent/model.go`
- Modify: `server/internal/domain/agent/service.go`
- Modify: `server/internal/domain/agent/service_test.go`

- [ ] **Step 1: 写 Service 正常增量失败测试**

将 `spyAdviceRunner` 改为按顺序调用 emitter，并添加：

```go
func TestServiceChatStreamEmitsDeltasAndPersistsFullText(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{chunks: []string{"你好，", "今天可以穿米白衬衫。"}}
	service := NewServiceWithRunner(repo, nil, runner)
	var deltas []StreamDelta
	result, err := service.ChatStream(context.Background(), 12, "你好", nil, func(delta StreamDelta) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil { t.Fatalf("chat stream: %v", err) }
	if got := repo.lastMessageUpdate.ContentText; got != "你好，今天可以穿米白衬衫。" {
		t.Fatalf("unexpected persisted text: %q", got)
	}
}
```

`StreamDelta` 定义为：

```go
type StreamDelta struct {
	Text string `json:"text"`
}
```

- [ ] **Step 2: 运行正常增量测试确认红灯**

Run: `cd server && go test ./internal/domain/agent -run TestServiceChatStreamEmitsDeltasAndPersistsFullText -count=1`

Expected: FAIL，`ChatStream` 和 `StreamDelta` 尚不存在。

- [ ] **Step 3: 用 ChatStream 替换旧聊天入口**

删除 `Chat` 和 `ChatWithProcessEvents`，定义唯一入口：

```go
func (s *Service) ChatStream(
	ctx context.Context,
	userID int64,
	text string,
	emitProcess func(ChatProcessEvent),
	emitDelta func(StreamDelta) error,
	assetRefs ...ChatAssetRef,
) (ChatResult, error)
```

Service 传给 Runner 的 emitter 必须先调用 `emitDelta(StreamDelta{Text: text})`；只有成功后 Runner 才把该 chunk 计入 `AdviceRunOutput.AssistantText`。

正常完成时沿用现有一次性消息更新、草稿关联和最终步骤落库，不按 chunk 更新数据库。

删除 Service 中遍历 `output.ToolCalls`、再次执行 `executeAdviceToolCall` 的整条旧路径，并删除仅被该路径使用的 `executeAdviceToolCall`、`defaultCreateDraftInput`、`defaultUpdateDraftInput`。草稿是否更新只依据 ADK 原生 `AuditSteps`，完成后按现有 `hasCompletedAdviceDraftAudit` 刷新当前草稿。

- [ ] **Step 4: 写取消和中途失败测试**

增加三类 Runner：

- 发出 `"已经生成的部分"` 后等待 `ctx.Done()` 并返回部分 output + `ctx.Err()`。
- 发出部分文本后返回普通模型错误。
- 在任何文本前返回普通模型错误。

断言：

- Context canceled 保存部分文本，状态为 `stopped`。
- emitter 返回 `ErrStreamClosed` 保存成功 emit 的部分文本，状态为 `stopped`。
- 普通错误有部分文本时保存部分文本，状态为 `failed`。
- 普通错误无部分文本时保存现有安全兜底，状态为 `failed`。
- 停止和失败均不继续处理后续审计、草稿刷新或最终成功步骤。

- [ ] **Step 5: 实现停止识别和独立持久化**

在 `model.go` 增加：

```go
const ChatStatusStopped = "stopped"
var ErrChatStopped = errors.New("agent chat stopped")
var ErrStreamClosed = errors.New("agent stream closed")
```

在 `service.go` 增加：

```go
const partialReplyPersistTimeout = 3 * time.Second

func (s *Service) persistPartialReply(
	parent context.Context,
	assistantMessageID int64,
	text string,
	status string,
) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), partialReplyPersistTimeout)
	defer cancel()
	if strings.TrimSpace(text) == "" && status == ChatStatusStopped {
		text = "已停止生成。"
	}
	_, err := s.repo.UpdateChatMessage(ctx, UpdateChatMessageInput{
		ID: assistantMessageID, Status: status, MsgType: ChatMsgTypeText, ContentText: text,
	})
	return err
}
```

停止判定为 `ctx.Err() != nil`、`errors.Is(runnerErr, context.Canceled)` 或 `errors.Is(runnerErr, ErrStreamClosed)`。Agent 自身 75 秒 deadline 仍属于失败，不标记为用户停止。

先用适用 Context 持久化已经生成的 audit steps，再保存消息。独立 Context 只能执行消息和审计落库，不能继续 Runner、工具或草稿操作。

- [ ] **Step 6: 更新历史过程状态映射**

`chatProcessForMessage` 对 `ChatStatusStopped` 返回：

```go
process.Status = ChatStatusStopped
process.Summary = "已停止"
```

`failed` 且有部分正文仍返回“本轮处理未完成”。

- [ ] **Step 7: 运行 Service 测试确认绿灯**

Run: `cd server && go test ./internal/domain/agent -run 'TestServiceChatStream|TestServiceChatHistory' -count=1`

Expected: PASS。

- [ ] **Step 8: 运行 Agent 包全量测试**

Run: `cd server && go test ./internal/domain/agent -count=1`

Expected: PASS。

- [ ] **Step 9: 提交 Service 生命周期改动**

```bash
git add server/internal/domain/agent/model.go server/internal/domain/agent/service.go server/internal/domain/agent/service_test.go
git commit -m "feat: 保存 Agent 流式部分回复"
```

### Task 3: 将正文增量接入 SSE 协议

**Files:**
- Modify: `server/internal/domain/agent/handler.go`
- Modify: `server/internal/domain/agent/routes_test.go`

- [ ] **Step 1: 写 delta-only SSE 失败测试**

使用按两段输出的测试 Runner，断言响应：

```go
body := recorder.Body.String()
first := strings.Index(body, "event: delta\n")
done := strings.Index(body, "event: done\n")
if first < 0 || done < 0 || first > done { t.Fatalf("unexpected SSE order: %s", body) }
if strings.Contains(body, "event: message\n") { t.Fatalf("message event must be removed: %s", body) }
if !strings.Contains(body, `data: {"text":"你好，"}`) { t.Fatalf("missing first delta: %s", body) }
```

- [ ] **Step 2: 运行路由测试确认红灯**

Run: `cd server && go test ./internal/domain/agent -run TestChatRouteStreamsDeltaWithoutMessageEvent -count=1`

Expected: FAIL，Handler 仍只发送最终 `message`。

- [ ] **Step 3: 修改 Handler SSE 写入**

调用 `ChatStream`，delta emitter 每次写入并 Flush：

```go
func(delta StreamDelta) error {
	if err := response.WriteSSE(c.Writer, "delta", delta); err != nil {
		return fmt.Errorf("%w: %v", ErrStreamClosed, err)
	}
	c.Writer.Flush()
	return nil
}
```

删除：

```go
response.WriteSSE(c.Writer, "message", result.Message)
```

如果 `errors.Is(err, ErrChatStopped)` 或请求 Context 已取消，直接结束 Handler，不尝试向已关闭连接写 `error`。其他错误继续发送用户安全的 `error`。

- [ ] **Step 4: 增加失败事件和 done 顺序测试**

覆盖：

- 正常：`status/process/delta/draft?/done`，无 `message`。
- 部分正文后失败：有 `delta` 和 `error`，无 `done`。
- 停止：不把内部 `ErrChatStopped` 写给客户端。
- `done.message_public_id`、开始和完成时间保持存在。

- [ ] **Step 5: 运行路由测试确认绿灯**

Run: `cd server && go test ./internal/domain/agent -run 'TestChatRoute|TestStream' -count=1`

Expected: PASS。

- [ ] **Step 6: 提交 SSE 协议改动**

```bash
git add server/internal/domain/agent/handler.go server/internal/domain/agent/routes_test.go
git commit -m "feat: 通过 SSE 实时发送 Agent 正文"
```

### Task 4: 小程序合并 delta 并保留停止内容

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/scripts/verify-api-client.js`
- Modify: `miniapp/pages/advisor/advisor.js`
- Modify: `miniapp/scripts/verify-advisor-page.js`

- [ ] **Step 1: 更新 API 客户端 delta 失败测试**

把模拟 SSE 改为：

```text
event: delta
data: {"text":"准备"}

event: delta
data: {"text":"好了"}

event: done
data: {"message_public_id":"msg_test"}
```

测试 `onDelta` 按顺序收到两个增量，`events` 中不存在 `message`；UTF-8 字节拆分仍能还原中文。

- [ ] **Step 2: 运行 API 验证确认红灯**

Run: `cd miniapp && npm run verify:api-client`

Expected: FAIL，现有 fixture 和断言仍使用 `message`。

- [ ] **Step 3: 收敛 API 正文协议**

`createSSEParser` 已按事件名动态调用 `onDelta`，保留这一通用机制；删除测试和调用方中的 `onMessage`。`sendAgentMessage` 仍可返回事件数组，但正文只来自 `delta`。

- [ ] **Step 4: 为页面增量缓冲写失败测试**

从 `advisor.js` 导出并测试一个无页面依赖的缓冲器：

```js
const buffer = advisor.createTextDeltaBuffer({
  delay: 50,
  apply(text) { applied.push(text); },
  schedule(fn) { scheduled = fn; return 1; },
  cancel() {}
});
buffer.push("你好，");
buffer.push("今天怎么穿");
scheduled();
assert(applied.join("") === "你好，今天怎么穿", "delta buffer should preserve order");
```

另测 `flush()`、`clear()` 及停止前 flush 不丢正文。

- [ ] **Step 5: 实现 50ms 页面缓冲和停止状态**

`sendToAgent` 创建当前请求专用 buffer；`onDelta` 调用 `buffer.push(data.text)`。buffer 的 apply 回调把文本追加到对应 assistant message，不使用 `trim()`，避免丢失 chunk 边界空格。

以下时机调用 `flush()`：

- `onDone`
- `onError`
- `handleStop`
- Promise resolve/reject 的 finally 前
- `onUnload`

`handleStop` 在 abort 前 flush，保留正文并设置 `status: "stopped"`。删除 `onMessage` 和 `extractAssistantText` 的 JSON 解析逻辑；历史正文直接使用服务端自然文本。

`normalizeChatMessage` 映射：

```js
status: message.status === "failed"
  ? "error"
  : (message.status === "stopped" ? "stopped" : "")
```

增加 `stopChatProcess`，摘要固定为“已停止”。

- [ ] **Step 6: 更新 Advisor 验证脚本**

模拟 `streamAgentChat(payload, handlers)` 保存 handlers，依次调用：

```js
handlers.onDelta({ text: "米白" });
handlers.onDelta({ text: "衬衫" });
handlers.onDone({ finished_at: "2026-07-17T10:00:02Z" });
```

断言正文为“米白衬衫”、没有覆盖或重复；停止测试断言已有正文保留且状态为 `stopped`；历史 `stopped` 映射一致。

- [ ] **Step 7: 运行小程序验证确认绿灯**

Run: `cd miniapp && npm run verify:api-client && npm run verify:advisor-page && npm run verify:api-integration`

Expected: 三个脚本全部 PASS。

- [ ] **Step 8: 提交小程序改动**

```bash
git add miniapp/utils/api.js miniapp/scripts/verify-api-client.js miniapp/pages/advisor/advisor.js miniapp/scripts/verify-advisor-page.js
git commit -m "feat: 实时显示 Agent SSE 回复"
```

### Task 5: 全量验证和真实 SSE 检查

**Files:**
- Modify only if verification exposes a defect in files listed above.

- [ ] **Step 1: 格式化并运行服务端全量测试**

Run: `cd server && gofmt -w internal/domain/agent/runner.go internal/domain/agent/model.go internal/domain/agent/adk_runner.go internal/domain/agent/adk_runner_test.go internal/domain/agent/service.go internal/domain/agent/service_test.go internal/domain/agent/handler.go internal/domain/agent/routes_test.go`

Run: `cd server && go test ./... -count=1 && go test -race ./internal/domain/agent -count=1 && go vet ./...`

Expected: 全部 PASS。

- [ ] **Step 2: 运行小程序相关验证**

Run: `cd miniapp && npm run verify:api-client && npm run verify:advisor-page && npm run verify:api-integration`

Expected: 全部 PASS。

- [ ] **Step 3: 启动服务并检查真实事件顺序**

重启本地服务后，用当前开发会话调用聊天 SSE，保存事件时间和正文：

```bash
curl -N -H 'Accept: text/event-stream' \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"text":"你好，请给我一个简短的通勤建议"}' \
  http://127.0.0.1:8080/api/user/agent/chat
```

Expected:

- 至少一个 `event: delta`。
- 多个 delta 按顺序组成自然回复。
- 不存在 `event: message`。
- 最终是 `event: done`。
- 数据库助手消息正文等于 delta 拼接文本。

- [ ] **Step 4: 检查停止和历史恢复**

发起较长回复，在收到至少一个 delta 后终止 curl；查询 `GET /api/user/agent/messages`。

Expected: 对应助手消息状态为 `stopped`，正文包含终止前已收到的部分文本，且终止后没有新的工具步骤。

- [ ] **Step 5: 检查工作区**

Run: `git diff --check && git status --short`

Expected: 无空白错误；只包含本计划列出的文件。

- [ ] **Step 6: 提交验证修复（如有）**

仅当步骤 1-4 暴露并修复缺陷时提交对应文件；没有修复则不创建空提交。
