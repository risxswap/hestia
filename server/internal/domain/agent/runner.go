package agent

import (
	"context"
	"strings"
)

const (
	AdviceToolCreateDraft = "create_advice_draft"
	AdviceToolUpdateDraft = "update_advice_draft"
)

type AdviceRunner interface {
	Run(ctx context.Context, input AdviceRunInput) (AdviceRunOutput, error)
}

type RuleBasedAdviceRunner struct{}

func (RuleBasedAdviceRunner) Run(_ context.Context, input AdviceRunInput) (AdviceRunOutput, error) {
	text := strings.TrimSpace(input.Text)
	if text == "" {
		text = "我会先根据你的场景生成一个可调整的形象建议草稿。"
	}
	if input.CurrentDraft == nil {
		createInput := defaultCreateDraftInput(input.UserID, text)
		createInput.SourceMsgID = input.SourceMsgID
		return AdviceRunOutput{
			AssistantText: "我先给你一版可继续调整的形象建议草稿。",
			DecisionLabel: "create_draft",
			ToolCalls: []AdviceToolCall{{
				Name:             AdviceToolCreateDraft,
				InputSummary:     text,
				CreateDraftInput: &createInput,
			}},
		}, nil
	}
	updateInput := defaultUpdateDraftInput(input.UserID, input.CurrentDraft.PublicID, input.SourceMsgID, text)
	return AdviceRunOutput{
		AssistantText: "我已按你的反馈更新草稿，可以继续细调其中任意一项。",
		DecisionLabel: "update_draft",
		ToolCalls: []AdviceToolCall{{
			Name:             AdviceToolUpdateDraft,
			InputSummary:     text,
			UpdateDraftInput: &updateInput,
		}},
	}, nil
}
