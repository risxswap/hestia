package agent_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	baseapp "hestia/server/internal/app"
	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/agent"
	"hestia/server/internal/domain/clothes"

	"github.com/gin-gonic/gin"
)

func TestStreamReturnsSSEEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	agent.RegisterUserRoutes(router.Group("/api/user/agent"), &baseapp.Deps{})
	request := httptest.NewRequest(http.MethodPost, "/api/user/agent/chat", strings.NewReader(`{"text":"今天怎么穿"}`))
	request.Header.Set("Accept", "text/event-stream")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: status\n") {
		t.Fatalf("expected status event, got %q", body)
	}
	if !strings.Contains(body, "event: done\n") {
		t.Fatalf("expected done event, got %q", body)
	}
}

func TestStreamStatusIncludesActiveClothesAdviceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	repo := &routeClothesRepo{items: []clothes.Item{
		{PublicID: "wdi_shirt", UserID: 12, Name: "米白衬衫", Category: "top", IsCore: true, RecommendationStatus: clothes.RecommendationStatusPreferred, Status: clothes.StatusActive},
		{PublicID: "wdi_paused", UserID: 12, Name: "黑色长裙", Category: "bottom", IsCore: true, RecommendationStatus: clothes.RecommendationStatusPaused, Status: clothes.StatusActive},
		{PublicID: "wdi_deleted", UserID: 12, Name: "灰色外套", Category: "outerwear", IsCore: true, RecommendationStatus: clothes.RecommendationStatusPreferred, Status: clothes.StatusDeleted},
		{PublicID: "wdi_other", UserID: 99, Name: "其他用户西装", Category: "outerwear", IsCore: true, RecommendationStatus: clothes.RecommendationStatusPreferred, Status: clothes.StatusActive},
	}}
	service := agent.NewServiceWithClothes(clothes.NewService(repo))
	agent.RegisterUserRoutesWithService(router.Group("/api/user/agent"), service)
	request := httptest.NewRequest(http.MethodPost, "/api/user/agent/chat", strings.NewReader(`{"text":"今天怎么穿"}`))
	request.Header.Set("Accept", "text/event-stream")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "米白衬衫") {
		t.Fatalf("expected status to include active clothes item name, got %q", body)
	}
	for _, excluded := range []string{"黑色长裙", "灰色外套", "其他用户西装"} {
		if strings.Contains(body, excluded) {
			t.Fatalf("expected status to exclude %s, got %q", excluded, body)
		}
	}
}

func TestChatRouteStreamsProcessEventsWithResponseTimes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	service := agent.NewServiceWithRunner(newRouteAgentRepo(), nil, routeAdviceRunner{})
	agent.RegisterUserRoutesWithService(router.Group("/api/user/agent"), service)
	request := httptest.NewRequest(http.MethodPost, "/api/user/agent/chat", strings.NewReader(`{"text":"明天见客户"}`))
	request.Header.Set("Accept", "text/event-stream")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	body := recorder.Body.String()
	events := sseEventNames(body)
	wantEvents := []string{"status", "process", "delta", "delta", "process", "process", "done"}
	if strings.Join(events, ",") != strings.Join(wantEvents, ",") {
		t.Fatalf("unexpected SSE event order: got %v want %v body=%q", events, wantEvents, body)
	}
	if strings.Contains(body, "event: message\n") {
		t.Fatalf("delta-only protocol must not send message event, got %q", body)
	}
	if !strings.Contains(body, `data: {"text":"已整理"}`) || !strings.Contains(body, `data: {"text":"好建议。"}`) {
		t.Fatalf("expected two delta payloads, got %q", body)
	}
	if !strings.Contains(body, "event: process\n") {
		t.Fatalf("expected process event, got %q", body)
	}
	if !strings.Contains(body, `"summary":"理解你的需求"`) || !strings.Contains(body, `"response_started_at"`) {
		t.Fatalf("expected safe process payload with start time, got %q", body)
	}
	firstProcess := strings.SplitN(strings.SplitN(body, "event: process\n", 2)[1], "\n\n", 2)[0]
	if strings.Contains(firstProcess, `"finished_at"`) {
		t.Fatalf("expected running process to omit finished_at, got %q", firstProcess)
	}
	if !strings.Contains(body, "event: done\n") || !strings.Contains(body, `"message_public_id":"msg_test"`) ||
		!strings.Contains(body, `"response_started_at"`) || !strings.Contains(body, `"finished_at"`) {
		t.Fatalf("expected done payload with finish time, got %q", body)
	}
}

func TestChatRouteSendsErrorAfterPartialDeltaWithoutDone(t *testing.T) {
	runnerErr := errors.New("provider included sensitive details")
	repo := newRouteAgentRepo()
	router := newAgentChatRouter(agent.NewServiceWithRunner(repo, nil, routeAdviceRunner{
		deltas: []string{"部分建议"}, output: agent.AdviceRunOutput{AssistantText: "部分建议"}, err: runnerErr,
	}))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, newAgentChatRequest())

	body := recorder.Body.String()
	events := sseEventNames(body)
	if !containsOrdered(events, "delta", "error") || strings.Contains(body, "event: done\n") {
		t.Fatalf("expected delta then error without done, events=%v body=%q", events, body)
	}
	if !strings.Contains(body, `data: {"message":"智能体请求失败"}`) || strings.Contains(body, runnerErr.Error()) {
		t.Fatalf("expected safe error payload, got %q", body)
	}
}

func TestChatRouteSendsErrorWithoutDeltaOrDone(t *testing.T) {
	runnerErr := errors.New("provider included sensitive details")
	router := newAgentChatRouter(agent.NewServiceWithRunner(newRouteAgentRepo(), nil, routeAdviceRunner{err: runnerErr}))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, newAgentChatRequest())

	body := recorder.Body.String()
	events := sseEventNames(body)
	if !containsOrdered(events, "status", "process", "error") || strings.Contains(body, "event: delta\n") || strings.Contains(body, "event: done\n") {
		t.Fatalf("expected error without delta or done, events=%v body=%q", events, body)
	}
	if !strings.Contains(body, `data: {"message":"智能体请求失败"}`) || strings.Contains(body, runnerErr.Error()) {
		t.Fatalf("expected safe error payload, got %q", body)
	}
}

func TestChatRouteStoppedDoesNotWriteErrorOrDone(t *testing.T) {
	router := newAgentChatRouter(agent.NewServiceWithRunner(newRouteAgentRepo(), nil, routeAdviceRunner{
		deltas: []string{"部分建议"}, output: agent.AdviceRunOutput{AssistantText: "部分建议"}, err: agent.ErrStreamClosed,
	}))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, newAgentChatRequest())

	body := recorder.Body.String()
	if strings.Contains(body, "event: error\n") || strings.Contains(body, "event: done\n") {
		t.Fatalf("stopped stream must end silently, got %q", body)
	}
}

func TestChatRouteStreamsDraftAfterDeltaBeforeDone(t *testing.T) {
	repo := newRouteAgentRepo()
	runner := routeAdviceRunner{
		deltas: []string{"先给你建议"},
		output: agent.AdviceRunOutput{AssistantText: "先给你建议", AuditSteps: []agent.AdviceRunAuditStep{{
			StepType: agent.AgentStepTypeToolResult, Status: agent.AgentStepStatusSucceeded, ToolName: agent.AdviceToolUpdateDraft,
		}}},
	}
	router := newAgentChatRouter(agent.NewServiceWithRunner(repo, nil, runner))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, newAgentChatRequest())

	events := sseEventNames(recorder.Body.String())
	if !containsOrdered(events, "delta", "draft", "done") {
		t.Fatalf("expected delta, draft, done order, got %v body=%q", events, recorder.Body.String())
	}
}

func TestChatRouteDeltaWriteFailureStopsPersistedMessage(t *testing.T) {
	repo := newRouteAgentRepo()
	router := newAgentChatRouter(agent.NewServiceWithRunner(repo, nil, routeAdviceRunner{
		deltas: []string{"第一段", "第二段"}, output: agent.AdviceRunOutput{AssistantText: "第一段第二段"},
	}))
	writer := &failDeltaResponseWriter{header: make(http.Header), failEvent: "delta"}

	router.ServeHTTP(writer, newAgentChatRequest())

	if len(repo.updates) != 1 || repo.updates[0].Status != agent.ChatStatusStopped {
		t.Fatalf("expected stopped assistant after write failure, got %#v", repo.updates)
	}
	if strings.Contains(writer.body.String(), "event: error\n") || strings.Contains(writer.body.String(), "event: done\n") {
		t.Fatalf("write failure must not continue stream, got %q", writer.body.String())
	}
}

func TestChatRouteDeltaShortWriteStopsPersistedMessage(t *testing.T) {
	repo := newRouteAgentRepo()
	router := newAgentChatRouter(agent.NewServiceWithRunner(repo, nil, routeAdviceRunner{
		deltas: []string{"第一段", "第二段"}, output: agent.AdviceRunOutput{AssistantText: "第一段第二段"},
	}))
	writer := &failSSEEventResponseWriter{header: make(http.Header), failEvent: "delta", short: true}

	router.ServeHTTP(writer, newAgentChatRequest())

	if len(repo.updates) != 1 || repo.updates[0].Status != agent.ChatStatusStopped {
		t.Fatalf("expected stopped assistant after short write, got %#v", repo.updates)
	}
	if strings.Contains(writer.body.String(), "event: error\n") || strings.Contains(writer.body.String(), "event: done\n") {
		t.Fatalf("short write must not continue stream, got %q", writer.body.String())
	}
	if writer.writesAfterFailure != 0 || writer.body.Len() != writer.bodyLenAtFailure {
		t.Fatalf("expected no writes after first failure, writes_after=%d body_len=%d failed_len=%d", writer.writesAfterFailure, writer.body.Len(), writer.bodyLenAtFailure)
	}
}

func TestChatRouteProcessWriteFailureStopsBeforeDelta(t *testing.T) {
	repo := newRouteAgentRepo()
	router := newAgentChatRouter(agent.NewServiceWithRunner(repo, nil, routeAdviceRunner{
		deltas: []string{"不应发送"}, output: agent.AdviceRunOutput{AssistantText: "不应发送"},
	}))
	writer := &failSSEEventResponseWriter{header: make(http.Header), failEvent: "process"}

	router.ServeHTTP(writer, newAgentChatRequest())

	if len(repo.updates) != 1 || repo.updates[0].Status != agent.ChatStatusStopped {
		t.Fatalf("expected process failure to stop assistant, got %#v", repo.updates)
	}
	if strings.Contains(writer.body.String(), "event: delta\n") || writer.writesAfterFailure != 0 {
		t.Fatalf("expected no event writes after process failure, writes_after=%d body=%q", writer.writesAfterFailure, writer.body.String())
	}
}

func TestChatRouteProcessWriteFailureCancelsRunnerBeforeToolExecution(t *testing.T) {
	repo := newRouteAgentRepo()
	runner := &processCancelRouteRunner{}
	router := newAgentChatRouter(agent.NewServiceWithRunner(repo, nil, runner))
	writer := &failSSEEventResponseWriter{header: make(http.Header), failEvent: "process"}

	router.ServeHTTP(writer, newAgentChatRequest())

	if runner.toolExecuted {
		t.Fatal("expected failed process write to cancel runner before tool execution")
	}
	if len(repo.updates) != 1 || repo.updates[0].Status != agent.ChatStatusStopped {
		t.Fatalf("expected stopped assistant after process write failure, got %#v", repo.updates)
	}
	if writer.writesAfterFailure != 0 {
		t.Fatalf("expected no writes after process failure, got %d", writer.writesAfterFailure)
	}
}

func TestChatRouteFinalProcessWriteFailureOverridesSentMessageAsStopped(t *testing.T) {
	repo := newRouteAgentRepo()
	router := newAgentChatRouter(agent.NewServiceWithRunner(repo, nil, routeAdviceRunner{
		output: agent.AdviceRunOutput{AssistantText: "完整建议"},
	}))
	writer := &failSSEEventResponseWriter{header: make(http.Header), failEvent: "process", failOccurrence: 3}

	router.ServeHTTP(writer, newAgentChatRequest())

	if len(repo.updates) != 2 || repo.updates[0].Status != agent.ChatStatusSent || repo.updates[1].Status != agent.ChatStatusStopped {
		t.Fatalf("expected sent message overridden as stopped, got %#v", repo.updates)
	}
	if strings.Contains(writer.body.String(), "event: done\n") || writer.writesAfterFailure != 0 {
		t.Fatalf("expected no writes after final process failure, writes_after=%d body=%q", writer.writesAfterFailure, writer.body.String())
	}
}

func TestChatRouteStatusWriteFailureDoesNotStartRunner(t *testing.T) {
	runner := &countingRouteRunner{}
	router := newAgentChatRouter(agent.NewServiceWithRunner(newRouteAgentRepo(), nil, runner))
	writer := &failSSEEventResponseWriter{header: make(http.Header), failEvent: "status"}

	router.ServeHTTP(writer, newAgentChatRequest())

	if runner.calls != 0 {
		t.Fatalf("expected status failure to stop before runner, got %d calls", runner.calls)
	}
	if writer.writesAfterFailure != 0 {
		t.Fatalf("expected no writes after status failure, got %d", writer.writesAfterFailure)
	}
}

func TestChatMessagesRouteReturnsSafeProcessHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	startedAt := time.Date(2026, 7, 16, 9, 30, 0, 0, time.UTC)
	repo := newRouteAgentRepo()
	repo.history = []agent.ChatMessage{{
		ID: 2, PublicID: "msg_assistant", UserID: 12, Role: agent.ChatRoleAssistant,
		MsgType: agent.ChatMsgTypeText, ContentText: "已整理好建议。", Status: agent.ChatStatusSent, CreatedAt: startedAt,
	}}
	repo.historySteps = []agent.AgentRunStep{{
		AssistantMsgID: 2, StepNo: 1, StepType: agent.AgentStepTypeModelDecision, Status: agent.AgentStepStatusSucceeded,
		StartedAt: startedAt, FinishedAt: startedAt.Add(time.Second),
	}}
	agent.RegisterUserRoutesWithService(router.Group("/api/user/agent"), agent.NewServiceWithRepository(repo))
	request := httptest.NewRequest(http.MethodGet, "/api/user/agent/messages", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"public_id":"msg_assistant"`) || !strings.Contains(body, `"summary":"查看处理过程"`) {
		t.Fatalf("expected visible message and process summary, got %q", body)
	}
	if strings.Contains(body, "input_summary") || strings.Contains(body, "raw model prompt") {
		t.Fatalf("expected no raw audit payload, got %q", body)
	}
}

func TestChatRouteAcceptsAssetRefs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	repo := newRouteAgentRepo()
	service := agent.NewServiceWithRunner(repo, nil, routeAdviceRunner{})
	agent.RegisterUserRoutesWithService(router.Group("/api/user/agent"), service)
	request := httptest.NewRequest(http.MethodPost, "/api/user/agent/chat", strings.NewReader(`{
		"text":"看看这张照片怎么搭",
		"asset_refs":[{"asset_public_id":"ast_photo","asset_type":"chat_image","note":"聊天上传图"}]
	}`))
	request.Header.Set("Accept", "text/event-stream")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(repo.createdMessages) == 0 || len(repo.createdMessages[0].AssetRefs) != 1 {
		t.Fatalf("expected request asset refs to be persisted, got %#v", repo.createdMessages)
	}
	if repo.createdMessages[0].AssetRefs[0].AssetPublicID != "ast_photo" {
		t.Fatalf("unexpected asset refs: %#v", repo.createdMessages[0].AssetRefs)
	}
}

func TestConfirmDraftRouteReturnsAdvicePublicID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newAgentDraftRouter(agent.NewServiceWithRepository(newRouteAgentRepo()))
	request := httptest.NewRequest(http.MethodPost, "/api/user/advice-drafts/drf_test/confirm", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"advice_public_id":"adv_test"`) {
		t.Fatalf("expected advice public id response, got %s", recorder.Body.String())
	}
}

func TestDiscardDraftRouteMarksDraftDiscarded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteAgentRepo()
	router := newAgentDraftRouter(agent.NewServiceWithRepository(repo))
	request := httptest.NewRequest(http.MethodPost, "/api/user/advice-drafts/drf_test/discard", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !repo.discarded {
		t.Fatalf("expected draft to be discarded")
	}
}

func TestCurrentDraftRouteReturnsDraftCard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newAgentDraftRouter(agent.NewServiceWithRepository(newRouteAgentRepo()))
	request := httptest.NewRequest(http.MethodGet, "/api/user/advice-drafts/current", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"draft_public_id":"drf_test"`) || !strings.Contains(body, `"revision_no":2`) {
		t.Fatalf("expected current draft card, got %s", body)
	}
}

func TestCurrentDraftRouteReturnsEmptyResultWhenNoDraftExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newRouteAgentRepo()
	repo.noCurrentDraft = true
	router := newAgentDraftRouter(agent.NewServiceWithRepository(repo))
	request := httptest.NewRequest(http.MethodGet, "/api/user/advice-drafts/current", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200 when no draft exists, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"ok"`) || !strings.Contains(recorder.Body.String(), `"data":null`) {
		t.Fatalf("expected empty success result, got %s", recorder.Body.String())
	}
}

func TestDraftVersionsRouteReturnsRevisionGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newAgentDraftRouter(agent.NewServiceWithRepository(newRouteAgentRepo()))
	request := httptest.NewRequest(http.MethodGet, "/api/user/advice-drafts/drf_test/versions", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"versions"`) || !strings.Contains(body, `"draft_revision_no":1`) {
		t.Fatalf("expected versions response, got %s", body)
	}
}

func newAgentDraftRouter(service *agent.Service) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	agent.RegisterAdviceDraftRoutesWithService(router.Group("/api/user/advice-drafts"), service)
	return router
}

type routeAgentRepo struct {
	discarded       bool
	noCurrentDraft  bool
	nextID          int64
	createdMessages []agent.ChatMessage
	history         []agent.ChatMessage
	historySteps    []agent.AgentRunStep
	updates         []agent.UpdateChatMessageInput
}

func newRouteAgentRepo() *routeAgentRepo {
	return &routeAgentRepo{nextID: 1}
}

func (r *routeAgentRepo) CreateChatMessage(_ context.Context, input agent.CreateChatMessageInput) (agent.ChatMessage, error) {
	item := agent.ChatMessage{
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

func (r *routeAgentRepo) UpdateChatMessage(_ context.Context, input agent.UpdateChatMessageInput) (agent.ChatMessage, error) {
	r.updates = append(r.updates, input)
	return agent.ChatMessage{ID: input.ID, PublicID: "msg_assistant", Status: input.Status, MsgType: input.MsgType, ContentText: input.ContentText}, nil
}

func (r *routeAgentRepo) ListRecentChatMessages(context.Context, int64, int) ([]agent.ChatMessage, error) {
	return nil, nil
}

func (r *routeAgentRepo) ListChatMessages(context.Context, int64, int) ([]agent.ChatMessage, error) {
	return r.history, nil
}

func (r *routeAgentRepo) ListAgentRunSteps(context.Context, int64, []int64) ([]agent.AgentRunStep, error) {
	return r.historySteps, nil
}

func (r *routeAgentRepo) CreateAgentRunStep(context.Context, agent.AgentRunStepInput) error {
	return nil
}

func (r *routeAgentRepo) CreateDraft(_ context.Context, input agent.CreateDraftInput) (agent.Draft, error) {
	return routeDraft(input.UserID), nil
}

func (r *routeAgentRepo) GetCurrentDraft(_ context.Context, userID int64) (agent.Draft, error) {
	if r.noCurrentDraft {
		return agent.Draft{}, agent.ErrDraftNotFound
	}
	return routeDraft(userID), nil
}

func (r *routeAgentRepo) UpdateDraftSections(_ context.Context, input agent.UpdateDraftInput) (agent.Draft, error) {
	draft := routeDraft(input.UserID)
	draft.CurrentRevisionNo++
	return draft, nil
}

func (r *routeAgentRepo) ListDraftVersions(_ context.Context, userID int64, publicID string) ([]agent.DraftRevision, error) {
	if userID != 12 || publicID != "drf_test" {
		return nil, agent.ErrDraftNotFound
	}
	return []agent.DraftRevision{{
		DraftRevisionNo: 1,
		UserIntent:      "创建建议",
		Sections: []agent.DraftVersionSection{{
			PublicID:             "adsv_test",
			SectionType:          agent.SectionTypeOutfit,
			SectionVersionNo:     1,
			ContentSchemaVersion: "v1",
			ContentJSON: map[string]any{
				"title": "清爽通勤",
			},
		}},
	}}, nil
}

func (r *routeAgentRepo) ConfirmDraft(_ context.Context, userID int64, publicID string) (agent.Advice, error) {
	if userID != 12 || publicID != "drf_test" {
		return agent.Advice{}, agent.ErrDraftNotFound
	}
	return agent.Advice{PublicID: "adv_test", UserID: userID, Status: agent.AdviceStatusReady}, nil
}

func (r *routeAgentRepo) DiscardDraft(_ context.Context, userID int64, publicID string) error {
	if userID != 12 || publicID != "drf_test" {
		return agent.ErrDraftNotFound
	}
	r.discarded = true
	return nil
}

func routeDraft(userID int64) agent.Draft {
	return agent.Draft{
		PublicID:          "drf_test",
		UserID:            userID,
		Status:            agent.DraftStatusDraft,
		SceneLabel:        "明天见客户",
		CurrentRevisionNo: 2,
		Sections: []agent.DraftSection{{
			PublicID:             "ads_outfit",
			SectionType:          agent.SectionTypeOutfit,
			ContentSchemaVersion: "v1",
			ContentJSON: map[string]any{
				"title":            "清爽通勤",
				"summary":          "米白衬衫配直筒裤",
				"why_text":         "利落但不紧绷",
				"avoid_text":       "避免太厚重",
				"alternative_text": "可换针织衫",
			},
		}},
	}
}

type routeClothesRepo struct {
	items []clothes.Item
}

type routeAdviceRunner struct {
	deltas []string
	output agent.AdviceRunOutput
	err    error
}

func (r routeAdviceRunner) Run(_ context.Context, _ agent.AdviceRunInput, emit agent.AdviceTextDeltaEmitter) (agent.AdviceRunOutput, error) {
	if len(r.deltas) == 0 && r.output.AssistantText == "" && r.err == nil {
		r.deltas = []string{"已整理", "好建议。"}
		r.output = agent.AdviceRunOutput{AssistantText: "已整理好建议。", DecisionLabel: "chat_response"}
	}
	output := r.output
	for _, delta := range r.deltas {
		if err := emit(delta); err != nil {
			return output, err
		}
	}
	return output, r.err
}

func newAgentChatRouter(service *agent.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	agent.RegisterUserRoutesWithService(router.Group("/api/user/agent"), service)
	return router
}

func newAgentChatRequest() *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/user/agent/chat", strings.NewReader(`{"text":"明天见客户"}`))
	request.Header.Set("Accept", "text/event-stream")
	return request
}

func sseEventNames(body string) []string {
	var events []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "event: ") {
			events = append(events, strings.TrimPrefix(line, "event: "))
		}
	}
	return events
}

func containsOrdered(events []string, want ...string) bool {
	next := 0
	for _, event := range events {
		if next < len(want) && event == want[next] {
			next++
		}
	}
	return next == len(want)
}

type failDeltaResponseWriter = failSSEEventResponseWriter

type failSSEEventResponseWriter struct {
	header             http.Header
	body               strings.Builder
	status             int
	failEvent          string
	short              bool
	failed             bool
	writesAfterFailure int
	bodyLenAtFailure   int
	failOccurrence     int
	matchingWrites     int
}

func (w *failSSEEventResponseWriter) Header() http.Header    { return w.header }
func (w *failSSEEventResponseWriter) WriteHeader(status int) { w.status = status }
func (w *failSSEEventResponseWriter) Flush()                 {}
func (w *failSSEEventResponseWriter) Write(p []byte) (int, error) {
	if w.failed {
		w.writesAfterFailure++
	}
	if strings.Contains(string(p), "event: "+w.failEvent+"\n") {
		w.matchingWrites++
	}
	failOccurrence := w.failOccurrence
	if failOccurrence == 0 {
		failOccurrence = 1
	}
	if !w.failed && w.matchingWrites == failOccurrence && strings.Contains(string(p), "event: "+w.failEvent+"\n") {
		w.failed = true
		if w.short {
			n := len(p) / 2
			_, _ = w.body.Write(p[:n])
			w.bodyLenAtFailure = w.body.Len()
			return n, nil
		}
		w.bodyLenAtFailure = w.body.Len()
		return 0, errors.New("client disconnected")
	}
	return w.body.Write(p)
}

type countingRouteRunner struct {
	calls int
}

type processCancelRouteRunner struct {
	toolExecuted bool
}

func (r *processCancelRouteRunner) Run(ctx context.Context, _ agent.AdviceRunInput, _ agent.AdviceTextDeltaEmitter) (agent.AdviceRunOutput, error) {
	select {
	case <-ctx.Done():
		return agent.AdviceRunOutput{}, ctx.Err()
	default:
		r.toolExecuted = true
		return agent.AdviceRunOutput{AssistantText: "不应执行工具"}, nil
	}
}

func (r *countingRouteRunner) Run(_ context.Context, _ agent.AdviceRunInput, _ agent.AdviceTextDeltaEmitter) (agent.AdviceRunOutput, error) {
	r.calls++
	return agent.AdviceRunOutput{AssistantText: "不应运行"}, nil
}

func (r *routeClothesRepo) CreateCoreItems(_ context.Context, items []clothes.Item) ([]clothes.Item, error) {
	return items, nil
}

func (r *routeClothesRepo) ListItems(_ context.Context, userID int64, _ clothes.ListFilter) ([]clothes.Item, error) {
	result := make([]clothes.Item, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID == userID && item.Status != clothes.StatusDeleted {
			result = append(result, item)
		}
	}
	return result, nil
}

func (r *routeClothesRepo) FindItemForUser(_ context.Context, userID int64, publicID string) (clothes.Item, error) {
	for _, item := range r.items {
		if item.UserID == userID && item.PublicID == publicID && item.Status != clothes.StatusDeleted {
			return item, nil
		}
	}
	return clothes.Item{}, clothes.ErrItemNotFound
}

func (r *routeClothesRepo) CreateItem(_ context.Context, item clothes.Item, _ string) (clothes.Item, error) {
	return item, nil
}

func (r *routeClothesRepo) UpdateItem(_ context.Context, _ int64, _ string, _ clothes.UpdateInput) (clothes.Item, error) {
	return clothes.Item{}, clothes.ErrItemNotFound
}

func (r *routeClothesRepo) SoftDeleteItem(_ context.Context, _ int64, _ string) error {
	return clothes.ErrItemNotFound
}
