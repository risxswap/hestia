package agent

import (
	"context"
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
	if len(repo.steps) != 2 {
		t.Fatalf("expected model and final steps, got %#v", repo.steps)
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

type spyAgentRepo struct {
	nextID          int64
	currentDraft    Draft
	createdMessages []ChatMessage
	steps           []AgentRunStepInput
	updatedDraft    bool
	lastUpdate      UpdateDraftInput
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

func (r *spyAgentRepo) CreateAgentRunStep(_ context.Context, input AgentRunStepInput) error {
	r.steps = append(r.steps, input)
	return nil
}

func (r *spyAgentRepo) CreateDraft(_ context.Context, input CreateDraftInput) (Draft, error) {
	draft := routeLikeDraft(input.UserID)
	draft.SourceMsgID = input.SourceMsgID
	return draft, nil
}

func (r *spyAgentRepo) GetCurrentDraft(context.Context, int64) (Draft, error) {
	return r.currentDraft, nil
}

func (r *spyAgentRepo) UpdateDraftSections(_ context.Context, input UpdateDraftInput) (Draft, error) {
	r.updatedDraft = true
	r.lastUpdate = input
	draft := r.currentDraft
	draft.CurrentRevisionNo++
	draft.Sections[0].ContentJSON = input.Sections[0].ContentJSON
	return draft, nil
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
