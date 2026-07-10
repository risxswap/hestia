package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
)

var ErrAdviceRunnerUnavailable = errors.New("advice runner unavailable")

type EinoADKAdviceRunner struct {
	runner *adk.Runner
}

func NewEinoADKAdviceRunner(runner *adk.Runner) *EinoADKAdviceRunner {
	return &EinoADKAdviceRunner{runner: runner}
}

func NewEinoADKChatModelAdviceRunner(ctx context.Context, chatModel model.ToolCallingChatModel, tools adk.ToolsConfig) (*EinoADKAdviceRunner, error) {
	if chatModel == nil {
		return nil, ErrAdviceRunnerUnavailable
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "style_advice_agent",
		Description: "通过对话创建和精修穿搭、发型、妆容建议草稿",
		Instruction: strings.TrimSpace(`
你是 Hestia 的个人 AI 形象顾问智能体。你需要判断用户意图，必要时调用草稿工具创建或更新建议草稿。
最终回答必须是 JSON，字段为 assistant_text、decision_label、tool_calls。
tool_calls[].name 只能是 create_advice_draft 或 update_advice_draft。
建议内容要中性、具体、可执行，避免医疗诊断、羞辱式表达和确定性变美承诺。
`),
		Model:         chatModel,
		ToolsConfig:   tools,
		MaxIterations: 8,
	})
	if err != nil {
		return nil, err
	}
	return &EinoADKAdviceRunner{runner: adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})}, nil
}

func (r *EinoADKAdviceRunner) Run(ctx context.Context, input AdviceRunInput) (AdviceRunOutput, error) {
	if r == nil || r.runner == nil {
		return AdviceRunOutput{}, ErrAdviceRunnerUnavailable
	}
	iterator := r.runner.Query(ctx, adkRunnerQuery(input))
	var finalText string
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return AdviceRunOutput{}, event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		message, err := event.Output.MessageOutput.GetMessage()
		if err != nil {
			return AdviceRunOutput{}, err
		}
		if message != nil && strings.TrimSpace(message.Content) != "" {
			finalText = strings.TrimSpace(message.Content)
		}
	}
	if finalText == "" {
		return AdviceRunOutput{}, ErrAdviceRunnerUnavailable
	}
	output, err := parseAdviceRunOutputJSON(finalText)
	if err != nil {
		return AdviceRunOutput{AssistantText: finalText, DecisionLabel: "final_response"}, nil
	}
	return output, nil
}

func adkRunnerQuery(input AdviceRunInput) string {
	currentDraft := "无"
	if input.CurrentDraft != nil {
		if raw, err := json.Marshal(input.CurrentDraft); err == nil {
			currentDraft = string(raw)
		}
	}
	return "用户输入：" + strings.TrimSpace(input.Text) + "\n当前草稿：" + currentDraft
}

func parseAdviceRunOutputJSON(raw string) (AdviceRunOutput, error) {
	var payload adviceRunOutputPayload
	if err := json.Unmarshal([]byte(stripJSONFence(raw)), &payload); err != nil {
		return AdviceRunOutput{}, err
	}
	output := AdviceRunOutput{
		AssistantText: payload.AssistantText,
		DecisionLabel: payload.DecisionLabel,
		ToolCalls:     make([]AdviceToolCall, 0, len(payload.ToolCalls)),
	}
	for _, call := range payload.ToolCalls {
		output.ToolCalls = append(output.ToolCalls, AdviceToolCall{
			Name:             strings.TrimSpace(call.Name),
			InputSummary:     call.InputSummary,
			CreateDraftInput: call.CreateDraftInput.domainInput(),
			UpdateDraftInput: call.UpdateDraftInput.domainInput(),
		})
	}
	return output, nil
}

func stripJSONFence(raw string) string {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	return strings.TrimSpace(text)
}

type adviceRunOutputPayload struct {
	AssistantText string                  `json:"assistant_text"`
	DecisionLabel string                  `json:"decision_label"`
	ToolCalls     []adviceToolCallPayload `json:"tool_calls"`
}

type adviceToolCallPayload struct {
	Name             string                   `json:"name"`
	InputSummary     string                   `json:"input_summary"`
	CreateDraftInput *createDraftInputPayload `json:"create_draft_input"`
	UpdateDraftInput *updateDraftInputPayload `json:"update_draft_input"`
}

type createDraftInputPayload struct {
	TargetDate      string                     `json:"target_date"`
	SceneKey        string                     `json:"scene_key"`
	SceneLabel      string                     `json:"scene_label"`
	Occasion        string                     `json:"occasion"`
	WeatherText     string                     `json:"weather_text"`
	MoodText        string                     `json:"mood_text"`
	StyleGoal       string                     `json:"style_goal"`
	AvoidGoal       string                     `json:"avoid_goal"`
	Sections        []draftSectionInputPayload `json:"sections"`
	UserIntent      string                     `json:"user_intent"`
	RevisionSummary string                     `json:"revision_summary"`
}

func (p *createDraftInputPayload) domainInput() *CreateDraftInput {
	if p == nil {
		return nil
	}
	targetDate := parsePayloadDate(p.TargetDate)
	return &CreateDraftInput{
		TargetDate:      targetDate,
		SceneKey:        p.SceneKey,
		SceneLabel:      p.SceneLabel,
		Occasion:        p.Occasion,
		WeatherText:     p.WeatherText,
		MoodText:        p.MoodText,
		StyleGoal:       p.StyleGoal,
		AvoidGoal:       p.AvoidGoal,
		Sections:        draftSectionPayloads(p.Sections).domainInputs(),
		UserIntent:      p.UserIntent,
		RevisionSummary: p.RevisionSummary,
	}
}

func parsePayloadDate(value string) *time.Time {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", text)
	if err != nil {
		return nil
	}
	return &parsed
}

type updateDraftInputPayload struct {
	PublicID        string                     `json:"public_id"`
	Sections        []draftSectionInputPayload `json:"sections"`
	UserIntent      string                     `json:"user_intent"`
	RevisionSummary string                     `json:"revision_summary"`
}

func (p *updateDraftInputPayload) domainInput() *UpdateDraftInput {
	if p == nil {
		return nil
	}
	return &UpdateDraftInput{
		PublicID:        p.PublicID,
		Sections:        draftSectionPayloads(p.Sections).domainInputs(),
		UserIntent:      p.UserIntent,
		RevisionSummary: p.RevisionSummary,
	}
}

type draftSectionInputPayload struct {
	SectionType          string         `json:"section_type"`
	ContentSchemaVersion string         `json:"content_schema_version"`
	ContentJSON          map[string]any `json:"content_json"`
	RevisionSummary      string         `json:"revision_summary"`
}

type draftSectionPayloads []draftSectionInputPayload

func (items draftSectionPayloads) domainInputs() []DraftSectionInput {
	result := make([]DraftSectionInput, 0, len(items))
	for _, item := range items {
		result = append(result, DraftSectionInput{
			SectionType:          item.SectionType,
			ContentSchemaVersion: item.ContentSchemaVersion,
			ContentJSON:          item.ContentJSON,
			RevisionSummary:      item.RevisionSummary,
		})
	}
	return result
}
