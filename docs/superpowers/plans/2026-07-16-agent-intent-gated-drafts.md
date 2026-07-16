# Agent Intent Gated Drafts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 仅在 LLM 调用草稿工具时创建或更新建议，普通聊天和 Agent 降级不再自动生成草稿。

**Architecture:** `agent.Service.Chat` 保留 LLM 的文本输出和已执行的工具调用。无工具调用时，助手消息正常完成但 `ChatResult.Draft` 为 `nil`；ADK 失败后写入失败步骤并返回文本引导，避免规则 Runner 代替 LLM 做创建决策。

**Tech Stack:** Go、Eino ADK、Go 标准测试。

---

### Task 1: 普通聊天不创建草稿

**Files:**
- Modify: `server/internal/domain/agent/service_test.go`
- Modify: `server/internal/domain/agent/service.go`

- [x] **Step 1: 写失败测试**

```go
func TestServiceChatKeepsTextResponseWhenRunnerDoesNotCallTools(t *testing.T) {
	service := NewServiceWithRunner(&spyAgentRepo{}, nil, &spyAdviceRunner{output: AdviceRunOutput{
		AssistantText: "可以，先告诉我明天的场景和想呈现的感觉。",
		DecisionLabel: "clarify_scene",
	}})

	result, err := service.Chat(context.Background(), 12, "你好")
	if err != nil { t.Fatalf("chat: %v", err) }
	if result.Draft != nil { t.Fatalf("expected no draft, got %#v", result.Draft) }
}
```

- [x] **Step 2: 运行失败测试**

Run: `cd server && go test ./internal/domain/agent -run '^TestServiceChatKeepsTextResponseWhenRunnerDoesNotCallTools$' -count=1`

Expected: FAIL，因为现有逻辑会调用规则 Runner 并创建草稿。

- [x] **Step 3: 最小实现**

删除 `currentDraftPtr == nil && len(output.ToolCalls) == 0` 时调用 `RuleBasedAdviceRunner` 的分支；在最终结果组装时，仅当当前草稿存在时才填充 `result.Draft`。

- [x] **Step 4: 运行通过测试**

Run: `cd server && go test ./internal/domain/agent -run '^TestServiceChatKeepsTextResponseWhenRunnerDoesNotCallTools$' -count=1`

Expected: PASS。

### Task 2: Agent 失败只返回文本引导

**Files:**
- Modify: `server/internal/domain/agent/service_test.go`
- Modify: `server/internal/domain/agent/service.go`

- [x] **Step 1: 写失败测试**

```go
func TestServiceChatReturnsTextGuidanceWhenAdviceRunnerFails(t *testing.T) {
	service := NewServiceWithRunner(&spyAgentRepo{}, nil, &spyAdviceRunner{err: errors.New("adk request failed")})

	result, err := service.Chat(context.Background(), 12, "你好")
	if err != nil { t.Fatalf("chat: %v", err) }
	if result.Draft != nil { t.Fatalf("expected no draft, got %#v", result.Draft) }
}
```

- [x] **Step 2: 运行失败测试**

Run: `cd server && go test ./internal/domain/agent -run '^TestServiceChatReturnsTextGuidanceWhenAdviceRunnerFails$' -count=1`

Expected: FAIL，因为现有降级会创建草稿。

- [x] **Step 3: 最小实现**

在记录 `runner_failed` 后构造 `AssistantText` 为“我暂时无法生成建议，请补充具体场景后再试。”、`DecisionLabel` 为 `runner_fallback_text` 的无工具输出；保留失败审计步骤。

- [x] **Step 4: 运行通过测试**

Run: `cd server && go test ./internal/domain/agent -run '^TestServiceChatReturnsTextGuidanceWhenAdviceRunnerFails$' -count=1`

Expected: PASS。

### Task 3: 全量验证与提交

**Files:**
- Modify: `docs/superpowers/plans/2026-07-16-agent-intent-gated-drafts.md`

- [x] **Step 1: 执行验证**

Run: `cd server && go test ./... && git diff --check`

Expected: PASS。

- [x] **Step 2: 提交**

```bash
git add server/internal/domain/agent/service.go server/internal/domain/agent/service_test.go docs/superpowers/plans/2026-07-16-agent-intent-gated-drafts.md
git commit -m "fix: 按智能体意图创建建议草稿"
```
