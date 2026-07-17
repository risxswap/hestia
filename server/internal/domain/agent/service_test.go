package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	serverlogger "hestia/server/internal/infra/logger"
)

func TestServiceLogsAdviceRunnerLifecycle(t *testing.T) {
	tests := []struct {
		name     string
		runner   *spyAdviceRunner
		expected []string
	}{
		{
			name: "completed",
			runner: &spyAdviceRunner{output: AdviceRunOutput{
				AssistantText: "建议已生成",
				DecisionLabel: "final_response",
				Metadata:      AdviceRunMetadata{UsageKey: "agent_chat", ProviderCode: "qwen", ModelCode: "qwen-plus", PromptVersion: "v2"},
			}},
			expected: []string{"agent llm call started", "agent llm call completed", "usage_key=agent_chat", "model_code=qwen-plus", "input_chars=", "output_chars=", "duration_ms="},
		},
		{
			name:     "failed",
			runner:   &spyAdviceRunner{err: errors.New("agent failed token=secret")},
			expected: []string{"agent llm call started", "agent llm call failed", "level=ERROR", "error_stage=agent_run", "error=\"agent failed token=[REDACTED]\""},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			service := NewServiceWithRunner(&spyAgentRepo{}, nil, test.runner)
			service.SetLogger(slog.New(slog.NewTextHandler(&logs, nil)))
			_, err := service.ChatStream(serverlogger.WithRequestID(context.Background(), "req_agent_test"), 12, "联系 test@example.com 后给建议", nil, nil)
			if err != nil && test.name != "failed" {
				t.Fatalf("chat: %v", err)
			}
			output := logs.String()
			for _, expected := range append(test.expected, "request_id=req_agent_test") {
				if !strings.Contains(output, expected) {
					t.Fatalf("expected %q in %s", expected, output)
				}
			}
			for _, leaked := range []string{"test@example.com", "token=secret"} {
				if strings.Contains(output, leaked) {
					t.Fatalf("leaked %q in %s", leaked, output)
				}
			}
		})
	}
}

func TestServiceLogsStructuredRunnerFailureDiagnostics(t *testing.T) {
	var logs bytes.Buffer
	runner := &metadataSpyAdviceRunner{
		metadata: AdviceRunMetadata{
			UsageKey:       "agent_chat",
			ProviderCode:   "qwen",
			ModelCode:      "qwen-plus",
			ProviderHost:   "api.example.com",
			AgentTimeoutMS: 75000,
			LLMTimeoutMS:   60000,
		},
		err: errors.New("node path: [node_1, ChatModel] https://api.example.com/v1?q=secret Bearer secret-token API key: complete-secret sk-abcdefghijklmnopqrstuvwxyz eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature"),
	}
	service := NewServiceWithRunner(&spyAgentRepo{}, nil, runner)
	service.SetLogger(slog.New(slog.NewJSONHandler(&logs, nil)))

	if _, err := service.ChatStream(context.Background(), 12, "明天通勤怎么穿", nil, nil); !errors.Is(err, runner.err) {
		t.Fatalf("expected runner error, got %v", err)
	}
	output := logs.String()
	for _, expected := range []string{
		`"provider_code":"qwen"`, `"model_code":"qwen-plus"`, `"provider_host":"api.example.com"`,
		`"agent_timeout_ms":75000`, `"llm_timeout_ms":60000`, `"context_deadline":`,
		`"context_error":"none"`, `"error_type":`, `"error_chain":`,
		`"node_path":"node_1, ChatModel"`, `"timeout_source":"unknown"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in %s", expected, output)
		}
	}
	for _, leaked := range []string{
		"https://api.example.com/v1?q=secret",
		"secret-token",
		"complete-secret",
		"sk-abcdefghijklmnopqrstuvwxyz",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("leaked %q in %s", leaked, output)
		}
	}
}

func TestServiceChatFallsBackWhenAdviceRunnerFails(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{err: errors.New("adk request failed")}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.ChatStream(context.Background(), 12, "明天通勤怎么穿", nil, nil)
	if !errors.Is(err, runner.err) {
		t.Fatalf("expected ADK runner error, got %v", err)
	}
	if repo.createdDraft || result.Draft != nil {
		t.Fatalf("expected no automatic draft after ADK failure, got %#v", result)
	}
	if result.Message.Text != "我暂时无法生成建议，请补充具体场景后再试。" {
		t.Fatalf("unexpected fallback guidance: %#v", result.Message)
	}
	if len(repo.steps) != 1 {
		t.Fatalf("expected only failed ADK step, got %#v", repo.steps)
	}
	failed := repo.steps[0]
	if failed.StepType != AgentStepTypeModelDecision || failed.Status != AgentStepStatusFailed || failed.DecisionLabel != "runner_failed" || failed.ErrorMessage != "adk request failed" {
		t.Fatalf("expected recorded ADK failure, got %#v", failed)
	}
	if len(repo.updates) != 1 || repo.updates[0].Status != ChatStatusFailed {
		t.Fatalf("expected failed message update, got %#v", repo.updates)
	}
}

func TestServiceChatKeepsTextResponseWhenRunnerDoesNotCallTools(t *testing.T) {
	repo := &spyAgentRepo{currentDraft: routeLikeDraft(12)}
	runner := &spyAdviceRunner{output: AdviceRunOutput{
		AssistantText: "可以，先告诉我明天的场景和想呈现的感觉。",
		DecisionLabel: "clarify_scene",
	}}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.ChatStream(context.Background(), 12, "你好", nil, nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if result.Draft != nil || repo.createdDraft || repo.updatedDraft {
		t.Fatalf("expected text-only response without draft changes, got %#v", result)
	}
	if result.Message.Text != "可以，先告诉我明天的场景和想呈现的感觉。" {
		t.Fatalf("unexpected text response: %#v", result.Message)
	}
}

func TestServiceChatReturnsTextWhenAdviceRunnerIsUnavailable(t *testing.T) {
	service := NewServiceWithRepository(&spyAgentRepo{})

	result, err := service.ChatStream(context.Background(), 12, "你好", nil, nil)
	if !errors.Is(err, ErrAdviceRunnerUnavailable) {
		t.Fatalf("expected unavailable runner error, got %v", err)
	}
	if result.Draft != nil {
		t.Fatalf("expected no automatic draft without an advice runner, got %#v", result.Draft)
	}
	if result.Message.Text != "我暂时无法生成建议，请补充具体场景后再试。" {
		t.Fatalf("unexpected unavailable-runner guidance: %#v", result.Message)
	}
}

func TestServiceChatSetsDeadlineForAdviceRunner(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{requireDeadline: true}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.ChatStream(context.Background(), 12, "明天通勤怎么穿", nil, nil)
	if err != nil {
		t.Fatalf("chat should fall back after a runner deadline check: %v", err)
	}
	if !runner.hasDeadline {
		t.Fatal("expected advice runner context to have a deadline")
	}
	if result.Draft != nil {
		t.Fatalf("expected no draft when the runner only returns text, got %#v", result.Draft)
	}
}

func TestServiceChatUsesConfiguredRunnerTimeout(t *testing.T) {
	runner := &contextDeadlineRunner{}
	service := NewServiceWithRunnerTimeout(&spyAgentRepo{}, nil, runner, 20*time.Millisecond)
	startedAt := time.Now()

	if _, err := service.ChatStream(context.Background(), 12, "明天通勤怎么穿", nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected runner deadline error: %v", err)
	}
	elapsed := time.Since(startedAt)
	if runner.deadline.IsZero() {
		t.Fatal("expected runner context deadline")
	}
	deadlineDuration := runner.deadline.Sub(startedAt)
	if deadlineDuration < 10*time.Millisecond || deadlineDuration > 100*time.Millisecond {
		t.Fatalf("expected deadline near configured 20ms, got %s", deadlineDuration)
	}
	if elapsed < 10*time.Millisecond || elapsed > time.Second {
		t.Fatalf("expected chat to return after configured timeout, elapsed %s", elapsed)
	}
	repo := service.repo.(*spyAgentRepo)
	if len(repo.updates) != 1 || repo.updates[0].Status != ChatStatusFailed {
		t.Fatalf("runner deadline must be failed, got %#v", repo.updates)
	}
}

func TestServiceChatPassesAssetRefsToUserMessageAndRunner(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{
		output: AdviceRunOutput{
			AssistantText: "已根据照片生成建议。",
			DecisionLabel: "create_draft",
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	_, err := service.ChatStream(context.Background(), 12, "看看这张照片适合怎么搭", nil, nil, ChatAssetRef{
		AssetPublicID: "ast_photo",
		AssetType:     "chat_image",
		Note:          "用户聊天上传图",
	})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if len(repo.createdMessages) == 0 || len(repo.createdMessages[0].AssetRefs) != 1 {
		t.Fatalf("expected user message asset refs, got %#v", repo.createdMessages)
	}
	if repo.createdMessages[0].AssetRefs[0].AssetPublicID != "ast_photo" {
		t.Fatalf("unexpected message asset refs: %#v", repo.createdMessages[0].AssetRefs)
	}
	if len(runner.input.AssetRefs) != 1 || runner.input.AssetRefs[0].AssetPublicID != "ast_photo" {
		t.Fatalf("expected runner asset refs, got %#v", runner.input.AssetRefs)
	}
}

func TestServiceChatRecordsRunnerMetadataAndToolStepDetails(t *testing.T) {
	repo := &spyAgentRepo{currentDraft: routeLikeDraft(12)}
	runner := &spyAdviceRunner{
		output: AdviceRunOutput{
			AssistantText: "已创建一版适合通勤的草稿。",
			DecisionLabel: "create_draft",
			Metadata: AdviceRunMetadata{
				UsageKey:      "agent_chat",
				ProviderCode:  "qwen",
				ModelCode:     "qwen-plus",
				PromptVersion: "v1",
				MaxIterations: 8,
			},
			AuditSteps: []AdviceRunAuditStep{
				{StepType: AgentStepTypeToolCall, Status: AgentStepStatusSucceeded, ToolName: AdviceToolCreateDraft, ToolCallID: "call_create_1", InputSummary: "工具调用参数已接收"},
				{StepType: AgentStepTypeToolResult, Status: AgentStepStatusSucceeded, ToolName: AdviceToolCreateDraft, ToolCallID: "call_create_1", OutputSummary: "工具执行结果已接收"},
			},
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	_, err := service.ChatStream(context.Background(), 12, "明天通勤怎么穿", nil, nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if len(repo.steps) < 3 {
		t.Fatalf("expected recorded steps, got %#v", repo.steps)
	}
	modelStep := repo.steps[0]
	if modelStep.UsageKey != "agent_chat" || modelStep.ProviderCode != "qwen" || modelStep.ModelCode != "qwen-plus" || modelStep.PromptVersion != "v1" || modelStep.MaxIterations != 8 {
		t.Fatalf("expected runner metadata on model step, got %#v", modelStep)
	}
	if modelStep.DurationMS <= 0 {
		t.Fatalf("expected model duration, got %#v", modelStep)
	}
	toolCallStep := repo.steps[1]
	if toolCallStep.ToolName != AdviceToolCreateDraft || toolCallStep.ToolCallID != "call_create_1" {
		t.Fatalf("expected tool identity on call step, got %#v", toolCallStep)
	}
	toolResultStep := repo.steps[2]
	if toolResultStep.ToolName != AdviceToolCreateDraft || toolResultStep.ToolCallID != "call_create_1" || toolResultStep.DurationMS <= 0 {
		t.Fatalf("expected tool result details, got %#v", toolResultStep)
	}
}

func TestServiceChatRecordsRunnerAuditStepsWithoutExecutingTools(t *testing.T) {
	repo := &spyAgentRepo{currentDraft: routeLikeDraft(12)}
	runner := &spyAdviceRunner{
		output: AdviceRunOutput{
			AssistantText: "已按你的反馈更新草稿。",
			DecisionLabel: "update_draft",
			AuditSteps: []AdviceRunAuditStep{
				{
					StepType:      AgentStepTypeToolCall,
					Status:        AgentStepStatusSucceeded,
					ToolName:      AdviceToolUpdateDraft,
					ToolCallID:    "call_update_1",
					InputSummary:  "调用 update_advice_draft",
					DecisionLabel: AdviceToolUpdateDraft,
				},
				{
					StepType:        AgentStepTypeToolResult,
					Status:          AgentStepStatusSucceeded,
					ToolName:        AdviceToolUpdateDraft,
					ToolCallID:      "call_update_1",
					OutputSummary:   "草稿已由 ADK 工具更新",
					DecisionLabel:   AdviceToolUpdateDraft,
					RelatedType:     "advice_draft",
					RelatedID:       10,
					RelatedPublicID: "drf_test",
				},
			},
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.ChatStream(context.Background(), 12, "鞋子换舒服点", nil, nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if repo.updatedDraft || repo.createdDraft {
		t.Fatalf("expected runner audit steps not to execute service tools again")
	}
	if result.Draft == nil || result.Draft.DraftPublicID != "drf_test" {
		t.Fatalf("expected refreshed draft card, got %#v", result.Draft)
	}
	if len(repo.steps) != 4 {
		t.Fatalf("expected model, two runner audit steps and final response, got %#v", repo.steps)
	}
	if repo.steps[1].ToolName != AdviceToolUpdateDraft || repo.steps[1].ToolCallID != "call_update_1" || repo.steps[2].StepType != AgentStepTypeToolResult {
		t.Fatalf("expected runner audit tool steps, got %#v", repo.steps)
	}
}

func TestServiceChatDoesNotImplicitlyCreateDraftWithoutCompletedDraftAudit(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{
		output: AdviceRunOutput{
			AssistantText: "明天见客户可以穿米白衬衫搭配直筒裤。",
			DecisionLabel: "final_response",
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.ChatStream(context.Background(), 12, "明天见客户", nil, nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if repo.createdDraft || result.Draft != nil {
		t.Fatalf("expected no implicit draft without completed draft audit, got %#v", result)
	}
	for _, step := range repo.steps {
		if step.StepType == AgentStepTypeToolResult && step.RelatedType == "advice_draft" {
			t.Fatalf("expected no completed draft audit, got %#v", repo.steps)
		}
	}
	if result.Message.Text != "明天见客户可以穿米白衬衫搭配直筒裤。" || repo.steps[0].DecisionLabel != "final_response" {
		t.Fatalf("expected runner text decision to be retained, got %#v", result)
	}
}

func TestServiceChatHistoryUsesPersistedStartTimeForGeneratingAssistantMessage(t *testing.T) {
	startedAt := time.Date(2026, 7, 16, 9, 30, 0, 0, time.UTC)
	repo := &spyAgentRepo{history: []ChatMessage{
		{ID: 1, PublicID: "msg_user", UserID: 12, Role: ChatRoleUser, MsgType: ChatMsgTypeText, ContentText: "明天见客户", Status: ChatStatusSent, CreatedAt: startedAt.Add(-time.Minute)},
		{ID: 2, PublicID: "msg_assistant", UserID: 12, Role: ChatRoleAssistant, MsgType: ChatMsgTypeText, Status: ChatStatusGenerating, CreatedAt: startedAt},
	}}
	service := NewServiceWithRepository(repo)

	messages, err := service.ChatHistory(context.Background(), 12, 50)
	if err != nil {
		t.Fatalf("chat history: %v", err)
	}
	if len(messages) != 2 || messages[1].Process == nil {
		t.Fatalf("expected assistant process in history, got %#v", messages)
	}
	process := messages[1].Process
	if process.Summary != "理解你的需求" || process.Status != "running" || !process.ResponseStartedAt.Equal(startedAt) {
		t.Fatalf("expected persisted start time and safe running summary, got %#v", process)
	}
}

func TestServiceChatStreamPublishesSafeProgressAndResponseTimes(t *testing.T) {
	repo := &spyAgentRepo{}
	service := NewServiceWithRunner(repo, nil, &spyAdviceRunner{output: AdviceRunOutput{
		AssistantText: "明天见客户可以穿浅色衬衫配直筒裤。",
		DecisionLabel: "chat_response",
	}})
	var events []ChatProcessEvent

	result, err := service.ChatStream(context.Background(), 12, "明天见客户", func(event ChatProcessEvent) {
		events = append(events, event)
	}, nil)
	if err != nil {
		t.Fatalf("chat with process events: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected process events, got %#v", events)
	}
	if events[0].Summary != "理解你的需求" || events[0].Status != "running" || events[0].ResponseStartedAt.IsZero() {
		t.Fatalf("unexpected first process event: %#v", events[0])
	}
	last := events[len(events)-1]
	if last.Summary != "完成回复" || last.Status != AgentStepStatusSucceeded {
		t.Fatalf("unexpected completed process event: %#v", last)
	}
	if !result.ResponseStartedAt.Equal(events[0].ResponseStartedAt) || result.FinishedAt.IsZero() {
		t.Fatalf("expected result timestamps from process stream, got %#v", result)
	}
}

type spyAgentRepo struct {
	nextID          int64
	currentDraft    Draft
	createdMessages []ChatMessage
	steps           []AgentRunStepInput
	createdDraft    bool
	lastCreate      CreateDraftInput
	updatedDraft    bool
	lastUpdate      UpdateDraftInput
	updateErr       error
	history         []ChatMessage
	updates         []UpdateChatMessageInput
	updateCtxErrors []error
	getDraftCalls   int
	stepErr         error
	calls           []string
	stepHook        func(context.Context, AgentRunStepInput) error
	updateHook      func(context.Context, UpdateChatMessageInput) error
	getDraftHook    func(context.Context, int64) (Draft, error, bool)
	historyHook     func(context.Context, int64, int) ([]ChatMessage, error, bool)
}

func (r *spyAgentRepo) CreateChatMessage(_ context.Context, input CreateChatMessageInput) (ChatMessage, error) {
	if r.nextID == 0 {
		r.nextID = 1
	}
	item := ChatMessage{
		ID:              r.nextID,
		PublicID:        "msg_test",
		UserID:          input.UserID,
		SourceMsgID:     input.SourceMsgID,
		Role:            input.Role,
		MsgType:         input.MsgType,
		ContentText:     input.ContentText,
		AssetRefs:       input.AssetRefs,
		RelatedType:     input.RelatedType,
		RelatedID:       input.RelatedID,
		RelatedPublicID: input.RelatedPublicID,
		Status:          input.Status,
		CreatedAt:       input.CreatedAt,
	}
	r.nextID++
	r.createdMessages = append(r.createdMessages, item)
	return item, nil
}

func (r *spyAgentRepo) UpdateChatMessage(ctx context.Context, input UpdateChatMessageInput) (ChatMessage, error) {
	r.calls = append(r.calls, "message")
	r.updates = append(r.updates, input)
	r.updateCtxErrors = append(r.updateCtxErrors, ctx.Err())
	if r.updateHook != nil {
		if err := r.updateHook(ctx, input); err != nil {
			return ChatMessage{}, err
		}
	}
	return ChatMessage{ID: input.ID, Status: input.Status, MsgType: input.MsgType, ContentText: input.ContentText}, r.updateErr
}

func (r *spyAgentRepo) ListRecentChatMessages(ctx context.Context, userID int64, limit int) ([]ChatMessage, error) {
	if r.historyHook != nil {
		if messages, err, handled := r.historyHook(ctx, userID, limit); handled {
			return messages, err
		}
	}
	return nil, nil
}

func (r *spyAgentRepo) ListChatMessages(context.Context, int64, int) ([]ChatMessage, error) {
	return r.history, nil
}

func (r *spyAgentRepo) ListAgentRunSteps(context.Context, int64, []int64) ([]AgentRunStep, error) {
	return nil, nil
}

func (r *spyAgentRepo) CreateAgentRunStep(ctx context.Context, input AgentRunStepInput) error {
	r.calls = append(r.calls, "audit")
	if r.stepHook != nil {
		if err := r.stepHook(ctx, input); err != nil {
			return err
		}
	}
	if r.stepErr != nil {
		return r.stepErr
	}
	r.steps = append(r.steps, input)
	return nil
}

func (r *spyAgentRepo) CreateDraft(_ context.Context, input CreateDraftInput) (Draft, error) {
	r.createdDraft = true
	r.lastCreate = input
	draft := routeLikeDraft(input.UserID)
	draft.SourceMsgID = input.SourceMsgID
	return draft, nil
}

func (r *spyAgentRepo) GetCurrentDraft(ctx context.Context, userID int64) (Draft, error) {
	r.getDraftCalls++
	if r.getDraftHook != nil {
		if draft, err, handled := r.getDraftHook(ctx, userID); handled {
			return draft, err
		}
	}
	if r.currentDraft.ID == 0 {
		return Draft{}, ErrDraftNotFound
	}
	return r.currentDraft, nil
}

func (r *spyAgentRepo) UpdateDraftSections(_ context.Context, input UpdateDraftInput) (Draft, error) {
	if r.updateErr != nil {
		return Draft{}, r.updateErr
	}
	r.updatedDraft = true
	r.lastUpdate = input
	draft := r.currentDraft
	draft.CurrentRevisionNo++
	draft.Sections[0].ContentJSON = input.Sections[0].ContentJSON
	return draft, nil
}

func (r *spyAgentRepo) ListDraftVersions(context.Context, int64, string) ([]DraftRevision, error) {
	return nil, nil
}

func (r *spyAgentRepo) ConfirmDraft(context.Context, int64, string) (Advice, error) {
	return Advice{}, ErrDraftNotFound
}

func (r *spyAgentRepo) DiscardDraft(context.Context, int64, string) error {
	return ErrDraftNotFound
}

func routeLikeDraft(userID int64) Draft {
	return Draft{
		ID:                10,
		PublicID:          "drf_test",
		UserID:            userID,
		Status:            DraftStatusDraft,
		CurrentRevisionNo: 1,
		Sections: []DraftSection{{
			ID:                   20,
			PublicID:             "ads_outfit",
			SectionType:          SectionTypeOutfit,
			ContentSchemaVersion: "v1",
			ContentJSON:          defaultAdviceContent("穿搭建议", "摘要"),
		}},
	}
}

type spyAdviceRunner struct {
	input           AdviceRunInput
	output          AdviceRunOutput
	err             error
	requireDeadline bool
	hasDeadline     bool
	deltas          []string
	emitErr         error
}

type metadataSpyAdviceRunner struct {
	metadata AdviceRunMetadata
	err      error
}

func (r *metadataSpyAdviceRunner) Metadata() AdviceRunMetadata { return r.metadata }

func (r *metadataSpyAdviceRunner) Run(context.Context, AdviceRunInput, AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
	return AdviceRunOutput{}, r.err
}

type contextDeadlineRunner struct {
	deadline time.Time
}

func (r *contextDeadlineRunner) Run(ctx context.Context, _ AdviceRunInput, _ AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
	r.deadline, _ = ctx.Deadline()
	<-ctx.Done()
	return AdviceRunOutput{}, ctx.Err()
}

func (r *spyAdviceRunner) Run(ctx context.Context, input AdviceRunInput, emit AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
	r.input = input
	_, r.hasDeadline = ctx.Deadline()
	if r.requireDeadline && !r.hasDeadline {
		return AdviceRunOutput{}, errors.New("runner deadline missing")
	}
	deltas := r.deltas
	if len(deltas) == 0 && r.output.AssistantText != "" {
		deltas = []string{r.output.AssistantText}
	}
	for _, delta := range deltas {
		if emit != nil {
			if err := emit(delta); err != nil {
				return r.output, err
			}
		}
	}
	if r.emitErr != nil {
		return r.output, r.emitErr
	}
	return r.output, r.err
}

func TestServiceChatStreamPersistsDeltasOnce(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{deltas: []string{"浅色衬衫", "配直筒裤"}, output: AdviceRunOutput{AssistantText: "浅色衬衫配直筒裤", DecisionLabel: "chat_response"}}
	service := NewServiceWithRunner(repo, nil, runner)
	var deltas []StreamDelta

	result, err := service.ChatStream(context.Background(), 12, "明天见客户", nil, func(delta StreamDelta) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("chat stream: %v", err)
	}
	if len(deltas) != 2 || deltas[0].Text+deltas[1].Text != result.Message.Text {
		t.Fatalf("unexpected deltas/result: %#v %#v", deltas, result)
	}
	if len(repo.updates) != 1 || repo.updates[0].ContentText != result.Message.Text || repo.updates[0].Status != ChatStatusSent {
		t.Fatalf("expected one final update, got %#v", repo.updates)
	}
}

func TestServiceChatStreamRunnerFailures(t *testing.T) {
	runnerFailure := errors.New("runner failed")
	tests := []struct {
		name       string
		ctx        func() (context.Context, context.CancelFunc)
		runner     *spyAdviceRunner
		wantErr    error
		wantStatus string
		wantText   string
	}{
		{name: "stream closed after partial", ctx: backgroundContext, runner: &spyAdviceRunner{deltas: []string{"已生成"}, output: AdviceRunOutput{AssistantText: "已生成"}, emitErr: ErrStreamClosed}, wantErr: ErrChatStopped, wantStatus: ChatStatusStopped, wantText: "已生成"},
		{name: "ordinary error with partial", ctx: backgroundContext, runner: &spyAdviceRunner{deltas: []string{"部分建议"}, output: AdviceRunOutput{AssistantText: "部分建议"}, err: runnerFailure}, wantErr: runnerFailure, wantStatus: ChatStatusFailed, wantText: "部分建议"},
		{name: "ordinary error without partial", ctx: backgroundContext, runner: &spyAdviceRunner{err: runnerFailure}, wantErr: runnerFailure, wantStatus: ChatStatusFailed, wantText: runnerFallbackText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &spyAgentRepo{}
			service := NewServiceWithRunner(repo, nil, tt.runner)
			ctx, cancel := tt.ctx()
			defer cancel()
			_, err := service.ChatStream(ctx, 12, "明天见客户", nil, func(StreamDelta) error { return nil })
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
			if len(repo.updates) != 1 || repo.updates[0].Status != tt.wantStatus || repo.updates[0].ContentText != tt.wantText {
				t.Fatalf("unexpected update: %#v", repo.updates)
			}
			for _, step := range repo.steps {
				if step.StepType == AgentStepTypeFinalResponse && step.Status == AgentStepStatusSucceeded {
					t.Fatalf("failure persisted success final step: %#v", repo.steps)
				}
			}
		})
	}
}

func TestServiceChatStreamZeroDeltaStopsUseStoppedText(t *testing.T) {
	tests := []struct {
		name   string
		runner AdviceRunner
	}{
		{name: "stream closed", runner: &spyAdviceRunner{emitErr: ErrStreamClosed}},
		{name: "wrapped stream closed", runner: &spyAdviceRunner{emitErr: fmt.Errorf("client disconnected: %w", ErrStreamClosed)}},
		{name: "context canceled", runner: adviceRunnerFunc(func(ctx context.Context, _ AdviceRunInput, _ AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
			return AdviceRunOutput{}, context.Canceled
		})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &spyAgentRepo{}
			_, err := NewServiceWithRunner(repo, nil, tt.runner).ChatStream(context.Background(), 12, "继续", nil, nil)
			if !errors.Is(err, ErrChatStopped) {
				t.Fatalf("expected stopped, got %v", err)
			}
			if strings.Contains(tt.name, "stream closed") && !errors.Is(err, ErrStreamClosed) {
				t.Fatalf("expected original stream-close cause, got %v", err)
			}
			if len(repo.updates) != 1 || repo.updates[0].ContentText != "已停止生成。" {
				t.Fatalf("unexpected zero-delta stopped content: %#v", repo.updates)
			}
		})
	}
}

func TestServiceChatStreamMessageUpdateFailuresAreReturned(t *testing.T) {
	updateErr := errors.New("message update failed")
	runnerErr := errors.New("runner failed")
	tests := []struct {
		name        string
		runner      AdviceRunner
		wantPrimary error
		wantStatus  string
	}{
		{name: "failed", runner: &spyAdviceRunner{err: runnerErr}, wantPrimary: runnerErr, wantStatus: ChatStatusFailed},
		{name: "stopped", runner: &spyAdviceRunner{emitErr: ErrStreamClosed}, wantPrimary: ErrChatStopped, wantStatus: ChatStatusStopped},
		{name: "normal", runner: &spyAdviceRunner{output: AdviceRunOutput{AssistantText: "完整建议"}}, wantPrimary: updateErr, wantStatus: ChatStatusSent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &spyAgentRepo{updateErr: updateErr}
			_, err := NewServiceWithRunner(repo, nil, tt.runner).ChatStream(context.Background(), 12, "建议", nil, nil)
			if !errors.Is(err, tt.wantPrimary) || !errors.Is(err, updateErr) {
				t.Fatalf("expected primary %v and update error, got %v", tt.wantPrimary, err)
			}
			if len(repo.updates) != 1 || repo.updates[0].Status != tt.wantStatus {
				t.Fatalf("expected attempted %s update, got %#v", tt.wantStatus, repo.updates)
			}
		})
	}
}

func TestServiceChatStreamFailurePersistsAuditsBeforeMessage(t *testing.T) {
	auditErr := errors.New("audit unavailable")
	runnerErr := errors.New("runner failed")
	repo := &spyAgentRepo{stepErr: auditErr}
	runner := &spyAdviceRunner{output: AdviceRunOutput{AuditSteps: []AdviceRunAuditStep{
		{StepType: AgentStepTypeToolCall, Status: AgentStepStatusSucceeded, ToolName: AdviceToolUpdateDraft, ToolCallID: "call_1"},
		{StepType: AgentStepTypeToolResult, Status: AgentStepStatusSucceeded, ToolName: AdviceToolUpdateDraft, ToolCallID: "call_1"},
	}}, err: runnerErr}

	_, err := NewServiceWithRunner(repo, nil, runner).ChatStream(context.Background(), 12, "调整", nil, nil)
	if !errors.Is(err, runnerErr) || !errors.Is(err, auditErr) {
		t.Fatalf("expected runner root and audit error, got %v", err)
	}
	if len(repo.calls) < 3 || repo.calls[0] != "audit" || repo.calls[1] != "audit" || repo.calls[2] != "message" {
		t.Fatalf("expected all audits before message, got %#v", repo.calls)
	}
	if len(repo.updates) != 1 || repo.updates[0].Status != ChatStatusFailed {
		t.Fatalf("audit failure left message generating: %#v", repo.updates)
	}
}

func TestServiceChatStreamCanceledPersistsWithIndependentContext(t *testing.T) {
	repo := &spyAgentRepo{}
	ctx, cancel := context.WithCancel(context.Background())
	runner := adviceRunnerFunc(func(_ context.Context, _ AdviceRunInput, emit AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
		output := AdviceRunOutput{AssistantText: "部分"}
		if err := emit("部分"); err != nil {
			return output, err
		}
		cancel()
		return output, context.Canceled
	})
	service := NewServiceWithRunner(repo, nil, runner)

	_, err := service.ChatStream(ctx, 12, "继续", nil, func(StreamDelta) error { return nil })
	if !errors.Is(err, ErrChatStopped) {
		t.Fatalf("expected stopped, got %v", err)
	}
	if len(repo.updateCtxErrors) != 1 || repo.updateCtxErrors[0] != nil || repo.updates[0].Status != ChatStatusStopped {
		t.Fatalf("expected independent update context, got errors=%#v updates=%#v", repo.updateCtxErrors, repo.updates)
	}
}

func TestServiceChatStreamEmitterCloseSavesOnlySuccessfulDeltas(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := adviceRunnerFunc(func(_ context.Context, _ AdviceRunInput, emit AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
		output := AdviceRunOutput{}
		for _, text := range []string{"已生成", "不应保存"} {
			if err := emit(text); err != nil {
				return output, err
			}
			output.AssistantText += text
		}
		return output, nil
	})
	service := NewServiceWithRunner(repo, nil, runner)
	emitted := 0

	_, err := service.ChatStream(context.Background(), 12, "继续", nil, func(delta StreamDelta) error {
		emitted++
		if emitted == 2 {
			return ErrStreamClosed
		}
		return nil
	})
	if !errors.Is(err, ErrChatStopped) {
		t.Fatalf("expected stopped, got %v", err)
	}
	if len(repo.updates) != 1 || repo.updates[0].ContentText != "已生成" || repo.updates[0].Status != ChatStatusStopped {
		t.Fatalf("unexpected stopped update: %#v", repo.updates)
	}
}

func TestServiceChatStreamAuditFailureStillFinalizesMessage(t *testing.T) {
	auditErr := errors.New("audit unavailable")
	repo := &spyAgentRepo{stepErr: auditErr}
	runnerErr := errors.New("runner failed")
	runner := &spyAdviceRunner{output: AdviceRunOutput{AssistantText: "部分", AuditSteps: []AdviceRunAuditStep{{StepType: AgentStepTypeToolCall, Status: AgentStepStatusSucceeded, ToolName: AdviceToolUpdateDraft}}}, err: runnerErr}

	_, err := NewServiceWithRunner(repo, nil, runner).ChatStream(context.Background(), 12, "调整", nil, nil)
	if !errors.Is(err, runnerErr) || !errors.Is(err, auditErr) {
		t.Fatalf("expected runner and audit errors, got %v", err)
	}
	if len(repo.updates) != 1 || repo.updates[0].Status != ChatStatusFailed {
		t.Fatalf("assistant left generating: %#v", repo.updates)
	}
}

func TestServiceChatStreamSuccessfulRunnerAuditFailureDoesNotLeaveGenerating(t *testing.T) {
	auditErr := errors.New("audit unavailable")
	repo := &spyAgentRepo{stepErr: auditErr}
	runner := &spyAdviceRunner{output: AdviceRunOutput{AssistantText: "完整建议"}}

	_, err := NewServiceWithRunner(repo, nil, runner).ChatStream(context.Background(), 12, "建议", nil, nil)
	if !errors.Is(err, auditErr) {
		t.Fatalf("expected audit error, got %v", err)
	}
	if len(repo.updates) != 1 || repo.updates[0].Status != ChatStatusFailed || repo.updates[0].ContentText != "完整建议" {
		t.Fatalf("assistant left generating: %#v", repo.updates)
	}
}

func TestServiceChatStreamFailurePersistsAuditWithoutRefreshingDraft(t *testing.T) {
	repo := &spyAgentRepo{currentDraft: routeLikeDraft(12)}
	runnerErr := errors.New("after tool")
	runner := &spyAdviceRunner{output: AdviceRunOutput{AssistantText: "部分", AuditSteps: []AdviceRunAuditStep{{StepType: AgentStepTypeToolResult, Status: AgentStepStatusSucceeded, ToolName: AdviceToolUpdateDraft, ToolCallID: "call_1"}}}, err: runnerErr}
	service := NewServiceWithRunner(repo, nil, runner)

	_, err := service.ChatStream(context.Background(), 12, "调整", nil, nil)
	if !errors.Is(err, runnerErr) {
		t.Fatalf("expected runner error, got %v", err)
	}
	if len(repo.steps) != 2 || repo.steps[0].ToolCallID != "call_1" || repo.getDraftCalls != 1 {
		t.Fatalf("expected audit persisted without post-run draft refresh, calls=%d steps=%#v", repo.getDraftCalls, repo.steps)
	}
}

func TestServiceChatStreamPostRunnerCancellationPersistsStopped(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*spyAgentRepo, context.CancelFunc)
		output    AdviceRunOutput
		wantText  string
		check     func(*testing.T, *spyAgentRepo)
	}{
		{
			name: "decision step",
			configure: func(repo *spyAgentRepo, cancel context.CancelFunc) {
				fired := false
				repo.stepHook = func(_ context.Context, input AgentRunStepInput) error {
					if !fired && input.StepType == AgentStepTypeModelDecision {
						fired = true
						cancel()
						return context.Canceled
					}
					return nil
				}
			},
			output:   AdviceRunOutput{},
			wantText: "已停止生成。",
		},
		{
			name: "draft refresh",
			configure: func(repo *spyAgentRepo, cancel context.CancelFunc) {
				repo.currentDraft = routeLikeDraft(12)
				repo.getDraftHook = func(_ context.Context, _ int64) (Draft, error, bool) {
					if repo.getDraftCalls == 2 {
						cancel()
						return Draft{}, context.Canceled, true
					}
					return Draft{}, nil, false
				}
			},
			output: AdviceRunOutput{AssistantText: "草稿正文", AuditSteps: []AdviceRunAuditStep{{StepType: AgentStepTypeToolResult, Status: AgentStepStatusSucceeded, ToolName: AdviceToolUpdateDraft}}},
			check: func(t *testing.T, repo *spyAgentRepo) {
				if len(repo.steps) != 1 {
					t.Fatalf("expected cancellation before audit persistence, got %#v", repo.steps)
				}
			},
		},
		{
			name: "audit step",
			configure: func(repo *spyAgentRepo, cancel context.CancelFunc) {
				fired := false
				repo.stepHook = func(_ context.Context, input AgentRunStepInput) error {
					if !fired && input.StepType == AgentStepTypeToolCall {
						fired = true
						cancel()
						return context.Canceled
					}
					return nil
				}
			},
			output: AdviceRunOutput{AssistantText: "审计正文", AuditSteps: []AdviceRunAuditStep{
				{StepType: AgentStepTypeToolCall, Status: AgentStepStatusSucceeded, ToolName: "get_profile_context"},
				{StepType: AgentStepTypeToolResult, Status: AgentStepStatusSucceeded, ToolName: "get_profile_context"},
			}},
			check: func(t *testing.T, repo *spyAgentRepo) {
				if len(repo.steps) != 1 {
					t.Fatalf("expected no audit after canceled audit write, got %#v", repo.steps)
				}
			},
		},
		{
			name: "final message update",
			configure: func(repo *spyAgentRepo, cancel context.CancelFunc) {
				fired := false
				repo.updateHook = func(_ context.Context, input UpdateChatMessageInput) error {
					if !fired && input.Status == ChatStatusSent {
						fired = true
						cancel()
						return context.Canceled
					}
					return nil
				}
			},
			output: AdviceRunOutput{AssistantText: "最终正文"},
		},
		{
			name: "final step after sent message",
			configure: func(repo *spyAgentRepo, cancel context.CancelFunc) {
				fired := false
				repo.stepHook = func(_ context.Context, input AgentRunStepInput) error {
					if !fired && input.StepType == AgentStepTypeFinalResponse {
						fired = true
						cancel()
						return nil
					}
					return nil
				}
			},
			output: AdviceRunOutput{AssistantText: "已发送正文"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &spyAgentRepo{}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tt.configure(repo, cancel)
			wantText := tt.wantText
			if wantText == "" {
				wantText = tt.output.AssistantText
			}

			_, err := NewServiceWithRunner(repo, nil, &spyAdviceRunner{output: tt.output}).ChatStream(ctx, 12, "继续", nil, nil)

			if !errors.Is(err, ErrChatStopped) || !errors.Is(err, context.Canceled) {
				t.Fatalf("expected stopped and canceled errors, got %v", err)
			}
			if len(repo.updates) == 0 || repo.updates[len(repo.updates)-1].Status != ChatStatusStopped || repo.updates[len(repo.updates)-1].ContentText != wantText {
				t.Fatalf("expected stopped update preserving output, got %#v", repo.updates)
			}
			if repo.updateCtxErrors[len(repo.updateCtxErrors)-1] != nil {
				t.Fatalf("expected detached stopped update context, got %#v", repo.updateCtxErrors)
			}
			if tt.check != nil {
				tt.check(t, repo)
			}
		})
	}
}

func TestServiceChatStreamPreRunnerCancellationPersistsStopped(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*spyAgentRepo)
	}{
		{
			name: "current draft",
			configure: func(repo *spyAgentRepo) {
				repo.getDraftHook = func(ctx context.Context, _ int64) (Draft, error, bool) {
					return Draft{}, ctx.Err(), true
				}
			},
		},
		{
			name: "recent history",
			configure: func(repo *spyAgentRepo) {
				repo.getDraftHook = func(_ context.Context, _ int64) (Draft, error, bool) {
					return Draft{}, ErrDraftNotFound, true
				}
				repo.historyHook = func(ctx context.Context, _ int64, _ int) ([]ChatMessage, error, bool) {
					return nil, ctx.Err(), true
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &spyAgentRepo{}
			tt.configure(repo)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runnerCalls := 0
			runner := adviceRunnerFunc(func(context.Context, AdviceRunInput, AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
				runnerCalls++
				return AdviceRunOutput{AssistantText: "不应执行"}, nil
			})

			_, err := NewServiceWithRunner(repo, nil, runner).ChatStream(ctx, 12, "继续", func(ChatProcessEvent) {
				cancel()
			}, nil)

			if !errors.Is(err, ErrChatStopped) || !errors.Is(err, context.Canceled) {
				t.Fatalf("expected stopped and canceled errors, got %v", err)
			}
			if runnerCalls != 0 {
				t.Fatalf("expected runner not to execute, got %d calls", runnerCalls)
			}
			if len(repo.updates) == 0 || repo.updates[len(repo.updates)-1].Status != ChatStatusStopped || repo.updates[len(repo.updates)-1].ContentText != "已停止生成。" {
				t.Fatalf("expected detached stopped update, got %#v", repo.updates)
			}
			if repo.updateCtxErrors[len(repo.updateCtxErrors)-1] != nil {
				t.Fatalf("expected uncanceled update context, got %#v", repo.updateCtxErrors)
			}
		})
	}
}

func TestServiceChatHistoryStopped(t *testing.T) {
	repo := &spyAgentRepo{history: []ChatMessage{{ID: 2, Role: ChatRoleAssistant, Status: ChatStatusStopped, CreatedAt: time.Now()}}}
	messages, err := NewServiceWithRepository(repo).ChatHistory(context.Background(), 12, 50)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if messages[0].Process.Status != ChatStatusStopped || messages[0].Process.Summary != "已停止" {
		t.Fatalf("unexpected stopped process: %#v", messages[0].Process)
	}
}

type adviceRunnerFunc func(context.Context, AdviceRunInput, AdviceTextDeltaEmitter) (AdviceRunOutput, error)

func (f adviceRunnerFunc) Run(ctx context.Context, input AdviceRunInput, emit AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
	return f(ctx, input, emit)
}

func backgroundContext() (context.Context, context.CancelFunc) {
	return context.Background(), func() {}
}
