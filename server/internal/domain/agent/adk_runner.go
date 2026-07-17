package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var ErrAdviceRunnerUnavailable = errors.New("advice runner unavailable")
var ErrMixedAssistantStream = errors.New("assistant stream mixed content and tool calls")
var ErrNonStreamingAssistantMessage = errors.New("non-streaming assistant message is not allowed")
var ErrUnexpectedToolResult = errors.New("unexpected tool result")
var ErrUnexpectedMessageRole = errors.New("unexpected message role")
var ErrUnexpectedToolCall = errors.New("unexpected tool call")
var ErrUnmatchedToolCalls = errors.New("unmatched tool calls")

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

func (r *EinoADKAdviceRunner) Metadata() AdviceRunMetadata {
	if r == nil {
		return AdviceRunMetadata{}
	}
	return r.metadata
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
需要写入草稿时必须真实调用相应工具。调用工具时不要同时输出给用户的正文，等待工具结果后再回复。
最终回复使用自然语言或 Markdown，不输出 JSON、工具名称或内部执行步骤。
建议内容要中性、具体、可执行，避免医疗诊断、羞辱式表达和确定性变美承诺。
`),
		Model:         chatModel,
		ToolsConfig:   tools,
		MaxIterations: metadata.MaxIterations,
	})
	if err != nil {
		return nil, err
	}
	return &EinoADKAdviceRunner{runner: adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true}), metadata: metadata}, nil
}

func (r *EinoADKAdviceRunner) Run(ctx context.Context, input AdviceRunInput, emit AdviceTextDeltaEmitter) (AdviceRunOutput, error) {
	output := AdviceRunOutput{DecisionLabel: "final_response"}
	if r == nil || r.runner == nil {
		return output, ErrAdviceRunnerUnavailable
	}
	output.Metadata = r.metadata
	if emit == nil {
		emit = func(string) error { return nil }
	}
	ctx = contextWithAdviceToolSession(ctx, input.UserID, input.SourceMsgID)
	iterator := r.runner.Query(ctx, adkRunnerQuery(input), adk.WithSessionValues(map[string]any{
		adviceToolSessionUserID:      input.UserID,
		adviceToolSessionSourceMsgID: input.SourceMsgID,
	}))
	pendingToolCalls := make(map[string]string)
	seenToolCallIDs := make(map[string]struct{})
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			return output, event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		variant := event.Output.MessageOutput
		if variant.Role == schema.Assistant && !variant.IsStreaming {
			return output, ErrNonStreamingAssistantMessage
		}
		if variant.Role == schema.Tool {
			steps, err := auditToolResult(variant)
			if err != nil {
				return output, err
			}
			for _, step := range steps {
				pendingName, ok := pendingToolCalls[step.ToolCallID]
				if !ok || pendingName != step.ToolName {
					output.AuditSteps = withoutToolResultAudits(output.AuditSteps)
					return output, ErrUnexpectedToolResult
				}
				delete(pendingToolCalls, step.ToolCallID)
				output.AuditSteps = append(output.AuditSteps, step)
			}
			continue
		}
		if variant.Role != schema.Assistant {
			closeMessageVariantStream(variant)
			return output, ErrUnexpectedMessageRole
		}
		steps, err := consumeAssistantStream(variant.MessageStream, emit, &output.AssistantText)
		if err != nil {
			return output, err
		}
		for _, step := range steps {
			callID := strings.TrimSpace(step.ToolCallID)
			toolName := strings.TrimSpace(step.ToolName)
			if callID == "" || toolName == "" {
				return output, ErrUnexpectedToolCall
			}
			if _, exists := seenToolCallIDs[callID]; exists {
				return output, ErrUnexpectedToolCall
			}
			seenToolCallIDs[callID] = struct{}{}
			pendingToolCalls[callID] = toolName
			output.AuditSteps = append(output.AuditSteps, step)
		}
	}
	if len(pendingToolCalls) > 0 {
		return output, ErrUnmatchedToolCalls
	}
	return output, nil
}

func closeMessageVariantStream(variant *adk.MessageVariant) {
	if variant != nil && variant.MessageStream != nil {
		variant.MessageStream.Close()
	}
}

func withoutToolResultAudits(steps []AdviceRunAuditStep) []AdviceRunAuditStep {
	filtered := steps[:0]
	for _, step := range steps {
		if step.StepType != AgentStepTypeToolResult {
			filtered = append(filtered, step)
		}
	}
	return filtered
}

func validChunkRole(actual, expected schema.RoleType) bool {
	return actual == "" || actual == expected
}

func consumeAssistantStream(stream *schema.StreamReader[*schema.Message], emit AdviceTextDeltaEmitter, assistantText *string) ([]AdviceRunAuditStep, error) {
	if stream == nil {
		return nil, ErrAdviceRunnerUnavailable
	}
	defer stream.Close()
	var leading []*schema.Message
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if chunk == nil {
			continue
		}
		if !validChunkRole(chunk.Role, schema.Assistant) {
			return nil, ErrUnexpectedMessageRole
		}
		if len(chunk.ToolCalls) > 0 {
			leading = append(leading, chunk)
			return auditAssistantToolStream(stream, leading)
		}
		if chunk.Content == "" {
			leading = append(leading, chunk)
			continue
		}
		if err := emit(chunk.Content); err != nil {
			return nil, err
		}
		*assistantText += chunk.Content
		break
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if chunk == nil {
			continue
		}
		if !validChunkRole(chunk.Role, schema.Assistant) {
			return nil, ErrUnexpectedMessageRole
		}
		if len(chunk.ToolCalls) > 0 {
			return nil, ErrMixedAssistantStream
		}
		if chunk.Content == "" {
			continue
		}
		if err := emit(chunk.Content); err != nil {
			return nil, err
		}
		*assistantText += chunk.Content
	}
}

func auditAssistantToolStream(stream *schema.StreamReader[*schema.Message], chunks []*schema.Message) ([]AdviceRunAuditStep, error) {
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if chunk != nil {
			if !validChunkRole(chunk.Role, schema.Assistant) {
				return nil, ErrUnexpectedMessageRole
			}
			chunks = append(chunks, chunk)
		}
	}
	message, err := schema.ConcatMessages(chunks)
	if err != nil {
		return nil, err
	}
	steps := make([]AdviceRunAuditStep, 0, len(message.ToolCalls))
	for _, call := range message.ToolCalls {
		steps = append(steps, AdviceRunAuditStep{
			StepType:      AgentStepTypeToolCall,
			Status:        AgentStepStatusSucceeded,
			ToolName:      strings.TrimSpace(call.Function.Name),
			ToolCallID:    strings.TrimSpace(call.ID),
			DecisionLabel: strings.TrimSpace(call.Function.Name),
			InputSummary:  "工具调用参数已接收",
		})
	}
	return steps, nil
}

func auditToolResult(variant *adk.MessageVariant) ([]AdviceRunAuditStep, error) {
	var message *schema.Message
	if variant.IsStreaming {
		if variant.MessageStream == nil {
			return nil, ErrAdviceRunnerUnavailable
		}
		defer variant.MessageStream.Close()
		var chunks []*schema.Message
		for {
			chunk, err := variant.MessageStream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			if chunk != nil {
				if !validChunkRole(chunk.Role, schema.Tool) {
					return nil, ErrUnexpectedMessageRole
				}
				chunks = append(chunks, chunk)
			}
		}
		if len(chunks) == 0 {
			return nil, nil
		}
		var err error
		message, err = schema.ConcatMessages(chunks)
		if err != nil {
			return nil, err
		}
	} else {
		message = variant.Message
	}
	if message == nil {
		return nil, nil
	}
	if !validChunkRole(message.Role, schema.Tool) {
		return nil, ErrUnexpectedMessageRole
	}
	toolName := strings.TrimSpace(variant.ToolName)
	if toolName == "" {
		toolName = strings.TrimSpace(message.ToolName)
	}
	return []AdviceRunAuditStep{{
		StepType:      AgentStepTypeToolResult,
		Status:        AgentStepStatusSucceeded,
		ToolName:      toolName,
		ToolCallID:    strings.TrimSpace(message.ToolCallID),
		DecisionLabel: toolName,
		OutputSummary: "工具执行结果已接收",
	}}, nil
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

func normalizeAdviceRunMetadata(metadata AdviceRunMetadata) AdviceRunMetadata {
	if metadata.MaxIterations <= 0 {
		metadata.MaxIterations = einoADKMaxIterations
	}
	return metadata
}
