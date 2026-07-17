package agent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestADKRunnerQueryUsesLightDraftSummary(t *testing.T) {
	query := adkRunnerQuery(AdviceRunInput{
		Text: "鞋子换舒服点",
		AssetRefs: []ChatAssetRef{{
			AssetPublicID: "ast_photo",
			AssetType:     "chat_image",
			Note:          "用户穿搭照片",
		}},
		RecentMessages: []ChatMessage{{Role: ChatRoleUser, ContentText: "明天见客户"}},
		CurrentDraft: &Draft{
			PublicID:          "drf_test",
			CurrentRevisionNo: 3,
			SceneLabel:        "见客户",
			Sections: []DraftSection{{
				SectionType: SectionTypeOutfit,
				ContentJSON: map[string]any{
					"title":      "清爽通勤",
					"summary":    "米白衬衫搭直筒裤",
					"why_text":   "这段完整理由不应默认进入 prompt",
					"avoid_text": "避免",
				},
			}},
		},
	})
	if !containsAll(query, []string{"最近聊天", "明天见客户", "drf_test", "清爽通勤", "鞋子换舒服点"}) {
		t.Fatalf("expected query to include light context, got %s", query)
	}
	if !containsAll(query, []string{"本轮图片", "ast_photo", "用户穿搭照片"}) {
		t.Fatalf("expected query to include asset refs, got %s", query)
	}
	if strings.Contains(query, "这段完整理由不应默认进入 prompt") {
		t.Fatalf("expected query to omit full draft body, got %s", query)
	}
}

func TestEinoADKAdviceRunnerStreamsAssistantTextInOrder(t *testing.T) {
	want := []string{
		"## 明日见客户\n\n",
		"建议选择**米白衬衫**搭配直筒裤。\n\n",
		"- 鞋子：低跟乐福鞋\n- 备选：干净的小白鞋\n",
	}
	stream := schema.StreamReaderFromArray([]*schema.Message{
		{Role: schema.Assistant, Extra: map[string]any{"request_id": "metadata-only"}},
		schema.AssistantMessage(want[0], nil),
		schema.AssistantMessage(want[1], nil),
		schema.AssistantMessage(want[2], nil),
	})
	runner := newTestEinoAdviceRunner(streamEvent(schema.Assistant, "", stream))
	var deltas []string

	output, err := runner.Run(context.Background(), AdviceRunInput{Text: "明天见客户"}, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !reflect.DeepEqual(deltas, want) {
		t.Fatalf("unexpected deltas: got %#v want %#v", deltas, want)
	}
	if output.AssistantText != strings.Join(want, "") || output.DecisionLabel != "final_response" {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestEinoADKAdviceRunnerAuditsToolEventsWithoutEmitting(t *testing.T) {
	toolCalls := []schema.ToolCall{{
		ID: "call_create_1",
		Function: schema.FunctionCall{
			Name:      AdviceToolCreateDraft,
			Arguments: `{"private":"must not be copied"}`,
		},
	}}
	runner := newTestEinoAdviceRunner(
		streamEvent(schema.Assistant, "", schema.StreamReaderFromArray([]*schema.Message{
			{Role: schema.Assistant, Content: "这段不能发出", ToolCalls: toolCalls},
			{Role: schema.Assistant, Extra: map[string]any{"usage": "metadata"}},
		})),
		&adk.AgentEvent{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{
			IsStreaming: false,
			Message:     schema.ToolMessage(`{"secret":"must not be copied"}`, "call_create_1"),
			Role:        schema.Tool,
			ToolName:    AdviceToolCreateDraft,
		}}},
		streamEvent(schema.Assistant, "", schema.StreamReaderFromArray([]*schema.Message{
			schema.AssistantMessage("草稿已经创建。", nil),
		})),
	)
	var deltas []string

	output, err := runner.Run(context.Background(), AdviceRunInput{}, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !reflect.DeepEqual(deltas, []string{"草稿已经创建。"}) {
		t.Fatalf("unexpected emitted deltas: %#v", deltas)
	}
	if len(output.AuditSteps) != 2 {
		t.Fatalf("unexpected audit steps: %#v", output.AuditSteps)
	}
	call, result := output.AuditSteps[0], output.AuditSteps[1]
	if call.StepType != AgentStepTypeToolCall || result.StepType != AgentStepTypeToolResult ||
		call.ToolName != AdviceToolCreateDraft || result.ToolName != AdviceToolCreateDraft ||
		call.ToolCallID != "call_create_1" || result.ToolCallID != "call_create_1" {
		t.Fatalf("unexpected audit order or identity: %#v", output.AuditSteps)
	}
	joined := call.InputSummary + call.OutputSummary + result.InputSummary + result.OutputSummary
	if strings.Contains(joined, "private") || strings.Contains(joined, "secret") {
		t.Fatalf("audit leaked tool payload: %#v", output.AuditSteps)
	}
}

func TestEinoADKAdviceRunnerReturnsConfirmedTextOnRecvError(t *testing.T) {
	wantErr := errors.New("stream interrupted")
	stream, writer := schema.Pipe[*schema.Message](2)
	writer.Send(schema.AssistantMessage("先穿轻薄外套", nil), nil)
	writer.Send(nil, wantErr)
	writer.Close()
	runner := newTestEinoAdviceRunner(streamEvent(schema.Assistant, "", stream))
	var deltas []string

	output, err := runner.Run(context.Background(), AdviceRunInput{}, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected recv error, got %v", err)
	}
	if output.AssistantText != "先穿轻薄外套" || !reflect.DeepEqual(deltas, []string{"先穿轻薄外套"}) {
		t.Fatalf("unexpected confirmed partial output: %#v, deltas %#v", output, deltas)
	}
}

func TestEinoADKAdviceRunnerStopsAtEmitterError(t *testing.T) {
	wantErr := errors.New("client disconnected")
	runner := newTestEinoAdviceRunner(streamEvent(schema.Assistant, "", schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("已确认", nil),
		schema.AssistantMessage("未确认", nil),
		schema.AssistantMessage("不应读取", nil),
	})))
	var deltas []string

	output, err := runner.Run(context.Background(), AdviceRunInput{}, func(delta string) error {
		if delta == "未确认" {
			return wantErr
		}
		deltas = append(deltas, delta)
		return nil
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected emitter error, got %v", err)
	}
	if output.AssistantText != "已确认" || !reflect.DeepEqual(deltas, []string{"已确认"}) {
		t.Fatalf("unexpected confirmed partial output: %#v, deltas %#v", output, deltas)
	}
}

func TestEinoADKAdviceRunnerRejectsToolCallAfterAssistantText(t *testing.T) {
	runner := newTestEinoAdviceRunner(streamEvent(schema.Assistant, "", schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("正文已发出", nil),
		{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "late", Function: schema.FunctionCall{Name: AdviceToolUpdateDraft}}}},
	})))

	output, err := runner.Run(context.Background(), AdviceRunInput{}, func(string) error { return nil })

	if !errors.Is(err, ErrMixedAssistantStream) {
		t.Fatalf("expected mixed stream error, got %v", err)
	}
	if output.AssistantText != "正文已发出" {
		t.Fatalf("expected confirmed partial text, got %#v", output)
	}
}

func TestEinoADKAdviceRunnerRejectsNonStreamingFinalAssistant(t *testing.T) {
	runner := newTestEinoAdviceRunner(&adk.AgentEvent{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{
		IsStreaming: false,
		Message:     schema.AssistantMessage("不能兼容", nil),
		Role:        schema.Assistant,
	}}})

	output, err := runner.Run(context.Background(), AdviceRunInput{}, func(string) error { return nil })

	if !errors.Is(err, ErrNonStreamingAssistantMessage) {
		t.Fatalf("expected protocol error, got %v", err)
	}
	if output.AssistantText != "" {
		t.Fatalf("unexpected assistant text: %#v", output)
	}
}

func TestEinoADKAdviceRunnerRejectsUnmatchedToolResults(t *testing.T) {
	tests := []struct {
		name   string
		events []*adk.AgentEvent
	}{
		{
			name: "result only",
			events: []*adk.AgentEvent{
				nonStreamingToolResultEvent(AdviceToolCreateDraft, "call_create_1"),
			},
		},
		{
			name: "wrong call id",
			events: []*adk.AgentEvent{
				assistantToolCallEvent(AdviceToolCreateDraft, "call_create_1"),
				nonStreamingToolResultEvent(AdviceToolCreateDraft, "call_other"),
			},
		},
		{
			name: "wrong tool name",
			events: []*adk.AgentEvent{
				assistantToolCallEvent(AdviceToolCreateDraft, "call_create_1"),
				nonStreamingToolResultEvent(AdviceToolUpdateDraft, "call_create_1"),
			},
		},
		{
			name: "duplicate result",
			events: []*adk.AgentEvent{
				assistantToolCallEvent(AdviceToolCreateDraft, "call_create_1"),
				nonStreamingToolResultEvent(AdviceToolCreateDraft, "call_create_1"),
				nonStreamingToolResultEvent(AdviceToolCreateDraft, "call_create_1"),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newTestEinoAdviceRunner(test.events...)

			output, err := runner.Run(context.Background(), AdviceRunInput{}, func(string) error { return nil })

			if !errors.Is(err, ErrUnexpectedToolResult) {
				t.Fatalf("expected unmatched tool result protocol error, got %v", err)
			}
			for _, step := range output.AuditSteps {
				if step.StepType == AgentStepTypeToolResult {
					t.Fatalf("unexpected completed tool audit: %#v", output.AuditSteps)
				}
			}
		})
	}
}

func TestEinoADKAdviceRunnerRejectsCallOnlyAtIteratorEOF(t *testing.T) {
	runner := newTestEinoAdviceRunner(assistantToolCallEvent(AdviceToolCreateDraft, "call_create_1"))

	output, err := runner.Run(context.Background(), AdviceRunInput{}, func(string) error { return nil })

	if !errors.Is(err, ErrUnmatchedToolCalls) {
		t.Fatalf("expected unmatched tool calls error, got %v", err)
	}
	assertNoToolResultAudit(t, output)
}

func TestEinoADKAdviceRunnerRejectsDuplicatePendingToolCallID(t *testing.T) {
	tests := []struct {
		name       string
		secondName string
	}{
		{name: "same name", secondName: AdviceToolCreateDraft},
		{name: "different name", secondName: AdviceToolUpdateDraft},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newTestEinoAdviceRunner(
				assistantToolCallEvent(AdviceToolCreateDraft, "call_create_1"),
				assistantToolCallEvent(test.secondName, "call_create_1"),
			)

			output, err := runner.Run(context.Background(), AdviceRunInput{}, func(string) error { return nil })

			if !errors.Is(err, ErrUnexpectedToolCall) {
				t.Fatalf("expected duplicate tool call error, got %v", err)
			}
			assertNoToolResultAudit(t, output)
		})
	}
}

func TestEinoADKAdviceRunnerRejectsCompletedToolCallIDReuse(t *testing.T) {
	runner := newTestEinoAdviceRunner(
		assistantToolCallEvent(AdviceToolCreateDraft, "call_create_1"),
		nonStreamingToolResultEvent(AdviceToolCreateDraft, "call_create_1"),
		assistantToolCallEvent(AdviceToolUpdateDraft, "call_create_1"),
	)

	_, err := runner.Run(context.Background(), AdviceRunInput{}, func(string) error { return nil })

	if !errors.Is(err, ErrUnexpectedToolCall) {
		t.Fatalf("expected reused tool call id error, got %v", err)
	}
}

func TestEinoADKAdviceRunnerRejectsEmptyToolCallIdentity(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		callID   string
	}{
		{name: "empty call id", toolName: AdviceToolCreateDraft},
		{name: "empty tool name", callID: "call_create_1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := newTestEinoAdviceRunner(assistantToolCallEvent(test.toolName, test.callID))

			output, err := runner.Run(context.Background(), AdviceRunInput{}, func(string) error { return nil })

			if !errors.Is(err, ErrUnexpectedToolCall) {
				t.Fatalf("expected invalid tool call error, got %v", err)
			}
			assertNoToolResultAudit(t, output)
		})
	}
}

func assertNoToolResultAudit(t *testing.T, output AdviceRunOutput) {
	t.Helper()
	for _, step := range output.AuditSteps {
		if step.StepType == AgentStepTypeToolResult {
			t.Fatalf("unexpected completed tool audit: %#v", output.AuditSteps)
		}
	}
}

func TestEinoADKAdviceRunnerClosesUnknownRoleStream(t *testing.T) {
	runner := newTestEinoAdviceRunner(streamEvent("unknown", "", schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("不能忽略", nil),
	})))

	_, err := runner.Run(context.Background(), AdviceRunInput{}, func(string) error { return nil })

	if !errors.Is(err, ErrUnexpectedMessageRole) {
		t.Fatalf("expected role protocol error, got %v", err)
	}

	stream, writer := schema.Pipe[*schema.Message](1)
	writer.Send(schema.AssistantMessage("不能忽略", nil), nil)
	closeMessageVariantStream(&adk.MessageVariant{IsStreaming: true, MessageStream: stream})
	if closed := writer.Send(schema.AssistantMessage("reader 应已关闭", nil), nil); !closed {
		t.Fatal("expected unknown-role ownership helper to close the stream")
	}
	writer.Close()
}

func TestEinoADKAdviceRunnerRejectsToolChunkInsideAssistantBody(t *testing.T) {
	runner := newTestEinoAdviceRunner(streamEvent(schema.Assistant, "", schema.StreamReaderFromArray([]*schema.Message{
		schema.AssistantMessage("已确认正文", nil),
		{Role: schema.Tool, Content: "不能发给用户"},
	})))
	var deltas []string

	output, err := runner.Run(context.Background(), AdviceRunInput{}, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})

	if !errors.Is(err, ErrUnexpectedMessageRole) {
		t.Fatalf("expected role protocol error, got %v", err)
	}
	if output.AssistantText != "已确认正文" || !reflect.DeepEqual(deltas, []string{"已确认正文"}) {
		t.Fatalf("unexpected confirmed output: %#v, deltas %#v", output, deltas)
	}
}

func assistantToolCallEvent(toolName, callID string) *adk.AgentEvent {
	return streamEvent(schema.Assistant, "", schema.StreamReaderFromArray([]*schema.Message{{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:       callID,
			Function: schema.FunctionCall{Name: toolName, Arguments: `{}`},
		}},
	}}))
}

func nonStreamingToolResultEvent(toolName, callID string) *adk.AgentEvent {
	return &adk.AgentEvent{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{
		Message:  schema.ToolMessage(`{"ok":true}`, callID),
		Role:     schema.Tool,
		ToolName: toolName,
	}}}
}

func streamEvent(role schema.RoleType, toolName string, stream *schema.StreamReader[*schema.Message]) *adk.AgentEvent {
	return &adk.AgentEvent{Output: &adk.AgentOutput{MessageOutput: &adk.MessageVariant{
		IsStreaming:   true,
		MessageStream: stream,
		Role:          role,
		ToolName:      toolName,
	}}}
}

func newTestEinoAdviceRunner(events ...*adk.AgentEvent) *EinoADKAdviceRunner {
	agent := &adviceRunnerTestAgent{events: events}
	return NewEinoADKAdviceRunner(adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	}))
}

type adviceRunnerTestAgent struct {
	events []*adk.AgentEvent
}

func (*adviceRunnerTestAgent) Name(context.Context) string        { return "test_advice_agent" }
func (*adviceRunnerTestAgent) Description(context.Context) string { return "test advice agent" }

func (a *adviceRunnerTestAgent) Run(context.Context, *adk.AgentInput, ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer generator.Close()
		for _, event := range a.events {
			generator.Send(event)
		}
	}()
	return iterator
}

func containsAll(text string, values []string) bool {
	for _, value := range values {
		if !strings.Contains(text, value) {
			return false
		}
	}
	return true
}
