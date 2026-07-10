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

const einoADKMaxIterations = 8

type EinoADKAdviceRunner struct {
	runner   *adk.Runner
	metadata AdviceRunMetadata
}

func NewEinoADKAdviceRunner(runner *adk.Runner) *EinoADKAdviceRunner {
	return &EinoADKAdviceRunner{runner: runner}
}

func NewEinoADKAdviceRunnerWithMetadata(runner *adk.Runner, metadata AdviceRunMetadata) *EinoADKAdviceRunner {
	return &EinoADKAdviceRunner{runner: runner, metadata: normalizeAdviceRunMetadata(metadata)}
}

func NewEinoADKChatModelAdviceRunner(ctx context.Context, chatModel model.ToolCallingChatModel, tools adk.ToolsConfig) (*EinoADKAdviceRunner, error) {
	return NewEinoADKChatModelAdviceRunnerWithMetadata(ctx, chatModel, tools, AdviceRunMetadata{})
}

func NewEinoADKChatModelAdviceRunnerWithMetadata(ctx context.Context, chatModel model.ToolCallingChatModel, tools adk.ToolsConfig, metadata AdviceRunMetadata) (*EinoADKAdviceRunner, error) {
	if chatModel == nil {
		return nil, ErrAdviceRunnerUnavailable
	}
	metadata = normalizeAdviceRunMetadata(metadata)
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "style_advice_agent",
		Description: "通过对话创建和精修穿搭、发型、妆容建议草稿",
		Instruction: strings.TrimSpace(`
	你是 Hestia 的个人 AI 形象顾问智能体。你需要判断用户意图，必要时通过 ADK 工具调用创建、读取、更新或废弃建议草稿。
	最终回答必须是 JSON，字段为 assistant_text、decision_label、tool_calls。tool_calls 只用于总结本轮已经完成的工具动作，不会再次执行。
	需要写入草稿时必须先真实调用 create_advice_draft、update_advice_draft 或 discard_advice_draft 工具，不得只在最终 JSON 里声明。
建议内容要中性、具体、可执行，避免医疗诊断、羞辱式表达和确定性变美承诺。
`),
		Model:         chatModel,
		ToolsConfig:   tools,
		MaxIterations: metadata.MaxIterations,
	})
	if err != nil {
		return nil, err
	}
	return &EinoADKAdviceRunner{runner: adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent}), metadata: metadata}, nil
}

func (r *EinoADKAdviceRunner) Run(ctx context.Context, input AdviceRunInput) (AdviceRunOutput, error) {
	if r == nil || r.runner == nil {
		return AdviceRunOutput{}, ErrAdviceRunnerUnavailable
	}
	ctx = contextWithAdviceToolSession(ctx, input.UserID, input.SourceMsgID)
	iterator := r.runner.Query(ctx, adkRunnerQuery(input), adk.WithSessionValues(map[string]any{
		adviceToolSessionUserID:      input.UserID,
		adviceToolSessionSourceMsgID: input.SourceMsgID,
	}))
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
		return AdviceRunOutput{AssistantText: finalText, DecisionLabel: "final_response", Metadata: r.metadata}, nil
	}
	output.Metadata = r.metadata
	output.AuditSteps = adviceRunAuditStepsFromToolCalls(output.ToolCalls)
	output.ToolCalls = nil
	return output, nil
}

func adkRunnerQuery(input AdviceRunInput) string {
	var builder strings.Builder
	builder.WriteString("最近聊天：\n")
	if len(input.RecentMessages) == 0 {
		builder.WriteString("无\n")
	} else {
		for _, message := range input.RecentMessages {
			text := strings.TrimSpace(message.ContentText)
			if text == "" {
				continue
			}
			builder.WriteString("- ")
			builder.WriteString(message.Role)
			builder.WriteString(": ")
			builder.WriteString(text)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("当前草稿摘要：")
	if input.CurrentDraft != nil {
		builder.WriteString(draftPromptSummary(*input.CurrentDraft))
	} else {
		builder.WriteString("无")
	}
	builder.WriteString("\n用户输入：")
	builder.WriteString(strings.TrimSpace(input.Text))
	if len(input.AssetRefs) > 0 {
		builder.WriteString("\n本轮图片：")
		builder.WriteString(assetRefsPromptSummary(input.AssetRefs))
	}
	return builder.String()
}

func assetRefsPromptSummary(refs []ChatAssetRef) string {
	normalized := normalizeChatAssetRefs(refs)
	if len(normalized) == 0 {
		return "无"
	}
	payload := make([]map[string]string, 0, len(normalized))
	for _, ref := range normalized {
		payload = append(payload, map[string]string{
			"asset_public_id": ref.AssetPublicID,
			"asset_type":      ref.AssetType,
			"note":            ref.Note,
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "无"
	}
	return string(raw)
}

func draftPromptSummary(draft Draft) string {
	summary := map[string]any{
		"draft_public_id":     draft.PublicID,
		"current_revision_no": draft.CurrentRevisionNo,
		"scene_label":         draft.SceneLabel,
		"target_date":         draft.TargetDate,
		"sections":            make([]map[string]string, 0, len(draft.Sections)),
	}
	sections := summary["sections"].([]map[string]string)
	for _, section := range draft.Sections {
		content := section.ContentJSON
		sections = append(sections, map[string]string{
			"section_type": section.SectionType,
			"title":        jsonString(content["title"]),
			"summary":      jsonString(content["summary"]),
		})
	}
	summary["sections"] = sections
	raw, err := json.Marshal(summary)
	if err != nil {
		return draft.PublicID
	}
	return string(raw)
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
			ToolCallID:       strings.TrimSpace(call.ToolCallID),
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
	ToolCallID       string                   `json:"tool_call_id"`
	InputSummary     string                   `json:"input_summary"`
	CreateDraftInput *createDraftInputPayload `json:"create_draft_input"`
	UpdateDraftInput *updateDraftInputPayload `json:"update_draft_input"`
}

func normalizeAdviceRunMetadata(metadata AdviceRunMetadata) AdviceRunMetadata {
	if metadata.MaxIterations <= 0 {
		metadata.MaxIterations = einoADKMaxIterations
	}
	return metadata
}

func adviceRunAuditStepsFromToolCalls(calls []AdviceToolCall) []AdviceRunAuditStep {
	steps := make([]AdviceRunAuditStep, 0, len(calls)*2)
	for _, call := range calls {
		if strings.TrimSpace(call.Name) == "" {
			continue
		}
		steps = append(steps, AdviceRunAuditStep{
			StepType:      AgentStepTypeToolCall,
			Status:        AgentStepStatusSucceeded,
			ToolName:      call.Name,
			ToolCallID:    call.ToolCallID,
			DecisionLabel: call.Name,
			InputSummary:  call.InputSummary,
		})
		steps = append(steps, AdviceRunAuditStep{
			StepType:      AgentStepTypeToolResult,
			Status:        AgentStepStatusSucceeded,
			ToolName:      call.Name,
			ToolCallID:    call.ToolCallID,
			DecisionLabel: call.Name,
			OutputSummary: "ADK 工具已执行",
		})
	}
	return steps
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
