package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceChatPersistsMessagesStepsAndUpdatesCurrentDraft(t *testing.T) {
	repo := &spyAgentRepo{currentDraft: Draft{
		ID:                10,
		PublicID:          "drf_test",
		UserID:            12,
		Status:            DraftStatusDraft,
		CurrentRevisionNo: 1,
		Sections: []DraftSection{{
			ID:                   20,
			PublicID:             "ads_outfit",
			SectionType:          SectionTypeOutfit,
			ContentSchemaVersion: "v1",
			ContentJSON:          defaultAdviceContent("旧穿搭", "旧摘要"),
		}},
	}}
	service := NewServiceWithRunner(repo, nil, RuleBasedAdviceRunner{})

	result, err := service.Chat(context.Background(), 12, "鞋子换舒服点")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if len(repo.createdMessages) != 2 {
		t.Fatalf("expected user and assistant messages, got %#v", repo.createdMessages)
	}
	if repo.createdMessages[0].Role != ChatRoleUser || repo.createdMessages[1].Role != ChatRoleAssistant {
		t.Fatalf("unexpected message roles: %#v", repo.createdMessages)
	}
	if repo.createdMessages[1].SourceMsgID != repo.createdMessages[0].ID {
		t.Fatalf("assistant message should point to user message, got %#v", repo.createdMessages)
	}
	if len(repo.steps) != 4 {
		t.Fatalf("expected model, tool and final steps, got %#v", repo.steps)
	}
	if repo.steps[1].StepType != AgentStepTypeToolCall || repo.steps[2].StepType != AgentStepTypeToolResult {
		t.Fatalf("expected tool call/result steps, got %#v", repo.steps)
	}
	if !repo.updatedDraft {
		t.Fatalf("expected existing draft to be updated")
	}
	if repo.lastUpdate.SourceMsgID != repo.createdMessages[0].ID {
		t.Fatalf("expected draft update source msg id, got %#v", repo.lastUpdate)
	}
	if result.Draft == nil || result.Draft.RevisionNo != 2 {
		t.Fatalf("expected updated draft card, got %#v", result.Draft)
	}
}

func TestServiceChatUsesAdviceRunnerToolCalls(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{
		output: AdviceRunOutput{
			AssistantText: "已按你的反馈改成更稳的鞋子。",
			DecisionLabel: "update_outfit",
			ToolCalls: []AdviceToolCall{{
				Name:         AdviceToolCreateDraft,
				InputSummary: "创建建议",
				CreateDraftInput: &CreateDraftInput{
					Sections: []DraftSectionInput{{
						SectionType:          SectionTypeOutfit,
						ContentSchemaVersion: "v1",
						ContentJSON:          defaultAdviceContent("穿搭", "更稳的鞋子"),
					}},
				},
			}},
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.Chat(context.Background(), 12, "鞋子换稳一点")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if runner.input.UserID != 12 || runner.input.Text != "鞋子换稳一点" || runner.input.SourceMsgID == 0 {
		t.Fatalf("unexpected runner input: %#v", runner.input)
	}
	if runner.input.CurrentDraft != nil {
		t.Fatalf("expected no current draft, got %#v", runner.input.CurrentDraft)
	}
	if !repo.createdDraft {
		t.Fatalf("expected create draft tool to run")
	}
	if result.Message.Text != "已按你的反馈改成更稳的鞋子。" {
		t.Fatalf("unexpected assistant text: %#v", result.Message)
	}
	if repo.steps[0].DecisionLabel != "update_outfit" {
		t.Fatalf("expected runner decision label to be recorded, got %#v", repo.steps)
	}
	if repo.steps[1].DecisionLabel != AdviceToolCreateDraft || repo.steps[2].RelatedPublicID != "drf_test" {
		t.Fatalf("expected tool execution steps, got %#v", repo.steps)
	}
}

func TestServiceChatFallsBackWhenAdviceRunnerFails(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{err: errors.New("adk request failed")}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.Chat(context.Background(), 12, "明天通勤怎么穿")
	if err != nil {
		t.Fatalf("chat should return text guidance when ADK runner fails: %v", err)
	}
	if repo.createdDraft || result.Draft != nil {
		t.Fatalf("expected no automatic draft after ADK failure, got %#v", result)
	}
	if result.Message.Text != "我暂时无法生成建议，请补充具体场景后再试。" {
		t.Fatalf("unexpected fallback guidance: %#v", result.Message)
	}
	if len(repo.steps) < 2 {
		t.Fatalf("expected failed ADK and text fallback steps, got %#v", repo.steps)
	}
	failed := repo.steps[0]
	if failed.StepType != AgentStepTypeModelDecision || failed.Status != AgentStepStatusFailed || failed.DecisionLabel != "runner_failed" || failed.ErrorMessage != "adk request failed" {
		t.Fatalf("expected recorded ADK failure, got %#v", failed)
	}
	if repo.steps[1].StepNo != 2 || repo.steps[1].Status != AgentStepStatusSucceeded {
		t.Fatalf("expected fallback model decision after failed ADK step, got %#v", repo.steps[1])
	}
}

func TestServiceChatKeepsTextResponseWhenRunnerDoesNotCallTools(t *testing.T) {
	repo := &spyAgentRepo{currentDraft: routeLikeDraft(12)}
	runner := &spyAdviceRunner{output: AdviceRunOutput{
		AssistantText: "可以，先告诉我明天的场景和想呈现的感觉。",
		DecisionLabel: "clarify_scene",
	}}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.Chat(context.Background(), 12, "你好")
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

	result, err := service.Chat(context.Background(), 12, "你好")
	if err != nil {
		t.Fatalf("chat: %v", err)
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

	result, err := service.Chat(context.Background(), 12, "明天通勤怎么穿")
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

func TestServiceChatPassesAssetRefsToUserMessageAndRunner(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{
		output: AdviceRunOutput{
			AssistantText: "已根据照片生成建议。",
			DecisionLabel: "create_draft",
			ToolCalls: []AdviceToolCall{{
				Name:         AdviceToolCreateDraft,
				InputSummary: "根据照片创建建议",
				CreateDraftInput: &CreateDraftInput{
					Sections: []DraftSectionInput{{
						SectionType:          SectionTypeOutfit,
						ContentSchemaVersion: "v1",
						ContentJSON:          defaultAdviceContent("照片穿搭", "根据照片调整搭配"),
					}},
				},
			}},
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	_, err := service.Chat(context.Background(), 12, "看看这张照片适合怎么搭", ChatAssetRef{
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
	repo := &spyAgentRepo{}
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
			ToolCalls: []AdviceToolCall{{
				Name:         AdviceToolCreateDraft,
				ToolCallID:   "call_create_1",
				InputSummary: "创建通勤建议",
				CreateDraftInput: &CreateDraftInput{
					Sections: []DraftSectionInput{{
						SectionType:          SectionTypeOutfit,
						ContentSchemaVersion: "v1",
						ContentJSON:          defaultAdviceContent("通勤穿搭", "清爽利落"),
					}},
				},
			}},
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	_, err := service.Chat(context.Background(), 12, "明天通勤怎么穿")
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

	result, err := service.Chat(context.Background(), 12, "鞋子换舒服点")
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

func TestServiceChatFallsBackWhenRunnerReturnsNoToolCallsForFirstDraft(t *testing.T) {
	repo := &spyAgentRepo{}
	runner := &spyAdviceRunner{
		output: AdviceRunOutput{
			AssistantText: "自然语言回复，没有工具调用。",
			DecisionLabel: "final_response",
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	result, err := service.Chat(context.Background(), 12, "明天见客户")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if repo.createdDraft || result.Draft != nil {
		t.Fatalf("expected no draft without an explicit tool call, got %#v", result)
	}
	if result.Message.Text != "自然语言回复，没有工具调用。" || repo.steps[0].DecisionLabel != "final_response" {
		t.Fatalf("expected runner text decision to be retained, got %#v", result)
	}
}

func TestServiceChatRecordsFailedToolResult(t *testing.T) {
	expected := errors.New("draft update failed")
	repo := &spyAgentRepo{
		currentDraft: routeLikeDraft(12),
		updateErr:    expected,
	}
	runner := &spyAdviceRunner{
		output: AdviceRunOutput{
			AssistantText: "准备更新草稿。",
			DecisionLabel: "update_draft",
			ToolCalls: []AdviceToolCall{{
				Name:         AdviceToolUpdateDraft,
				InputSummary: "更新草稿",
				UpdateDraftInput: &UpdateDraftInput{
					PublicID: "drf_test",
					Sections: []DraftSectionInput{{
						SectionType:          SectionTypeOutfit,
						ContentSchemaVersion: "v1",
						ContentJSON:          defaultAdviceContent("穿搭", "更新"),
					}},
				},
			}},
		},
	}
	service := NewServiceWithRunner(repo, nil, runner)

	_, err := service.Chat(context.Background(), 12, "鞋子换舒服点")
	if !errors.Is(err, expected) {
		t.Fatalf("expected update error, got %v", err)
	}
	if len(repo.steps) != 3 {
		t.Fatalf("expected model, tool call and failed tool result steps, got %#v", repo.steps)
	}
	failed := repo.steps[2]
	if failed.StepType != AgentStepTypeToolResult || failed.Status != AgentStepStatusFailed || failed.ErrorMessage != expected.Error() {
		t.Fatalf("unexpected failed step: %#v", failed)
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

func TestServiceChatWithProcessEventsPublishesSafeProgressAndResponseTimes(t *testing.T) {
	repo := &spyAgentRepo{}
	service := NewServiceWithRunner(repo, nil, &spyAdviceRunner{output: AdviceRunOutput{
		AssistantText: "明天见客户可以穿浅色衬衫配直筒裤。",
		DecisionLabel: "chat_response",
	}})
	var events []ChatProcessEvent

	result, err := service.ChatWithProcessEvents(context.Background(), 12, "明天见客户", func(event ChatProcessEvent) {
		events = append(events, event)
	})
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

func (r *spyAgentRepo) UpdateChatMessage(_ context.Context, input UpdateChatMessageInput) (ChatMessage, error) {
	return ChatMessage{ID: input.ID, Status: input.Status, MsgType: input.MsgType, ContentText: input.ContentText}, nil
}

func (r *spyAgentRepo) ListRecentChatMessages(context.Context, int64, int) ([]ChatMessage, error) {
	return nil, nil
}

func (r *spyAgentRepo) ListChatMessages(context.Context, int64, int) ([]ChatMessage, error) {
	return r.history, nil
}

func (r *spyAgentRepo) ListAgentRunSteps(context.Context, int64, []int64) ([]AgentRunStep, error) {
	return nil, nil
}

func (r *spyAgentRepo) CreateAgentRunStep(_ context.Context, input AgentRunStepInput) error {
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

func (r *spyAgentRepo) GetCurrentDraft(context.Context, int64) (Draft, error) {
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
}

func (r *spyAdviceRunner) Run(ctx context.Context, input AdviceRunInput) (AdviceRunOutput, error) {
	r.input = input
	_, r.hasDeadline = ctx.Deadline()
	if r.requireDeadline && !r.hasDeadline {
		return AdviceRunOutput{}, errors.New("runner deadline missing")
	}
	return r.output, r.err
}
