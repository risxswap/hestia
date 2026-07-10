package agent

import (
	"context"
	"errors"
	"testing"
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
	service := NewServiceWithRepository(repo)

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

	if !repo.createdDraft {
		t.Fatalf("expected fallback runner to create draft")
	}
	if result.Draft == nil || result.Draft.DraftPublicID != "drf_test" {
		t.Fatalf("expected draft card from fallback, got %#v", result.Draft)
	}
	if repo.steps[0].DecisionLabel != "create_draft" {
		t.Fatalf("expected fallback decision step, got %#v", repo.steps)
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
		RelatedType:     input.RelatedType,
		RelatedID:       input.RelatedID,
		RelatedPublicID: input.RelatedPublicID,
		Status:          input.Status,
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
	input  AdviceRunInput
	output AdviceRunOutput
}

func (r *spyAdviceRunner) Run(_ context.Context, input AdviceRunInput) (AdviceRunOutput, error) {
	r.input = input
	return r.output, nil
}
