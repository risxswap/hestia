package onboarding_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/onboarding"

	"github.com/gin-gonic/gin"
)

type fakeSessionStore struct {
	session auth.Session
	err     error
}

func (s fakeSessionStore) Get(_ context.Context, _ string) (auth.Session, error) {
	if s.err != nil {
		return auth.Session{}, s.err
	}
	return s.session, nil
}

type memoryDraftRepo struct {
	drafts map[int64]onboarding.Draft
	nextID int64
}

func newMemoryDraftRepo() *memoryDraftRepo {
	return &memoryDraftRepo{drafts: map[int64]onboarding.Draft{}, nextID: 1}
}

func (r *memoryDraftRepo) FindActiveByUserID(_ context.Context, userID int64) (onboarding.Draft, error) {
	draft, ok := r.drafts[userID]
	if !ok {
		return onboarding.Draft{}, onboarding.ErrDraftNotFound
	}
	return draft, nil
}

func (r *memoryDraftRepo) Create(_ context.Context, draft onboarding.Draft) (onboarding.Draft, error) {
	draft.ID = r.nextID
	r.nextID++
	r.drafts[draft.UserID] = draft
	return draft, nil
}

func (r *memoryDraftRepo) Update(_ context.Context, draft onboarding.Draft) (onboarding.Draft, error) {
	if _, ok := r.drafts[draft.UserID]; !ok {
		return onboarding.Draft{}, onboarding.ErrDraftNotFound
	}
	r.drafts[draft.UserID] = draft
	return draft, nil
}

func TestOnboardingRequiresUserSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	repo := newMemoryDraftRepo()
	group := router.Group("/api/user/onboarding")
	group.Use(auth.RequireUserSession(fakeSessionStore{err: errors.New("missing session")}))
	onboarding.RegisterRoutes(group, onboarding.NewHandler(onboarding.NewService(repo), nil))
	request := httptest.NewRequest(http.MethodGet, "/api/user/onboarding", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestOnboardingDraftCanBeSavedReadAndMerged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newAuthenticatedRouter(newMemoryDraftRepo())

	putJSON(t, router, `{
		"step": "style_goal",
		"data": {
			"style_goal": {
				"goals": ["干净", "有气质"],
				"avoidances": ["甜美"],
				"note": "喜欢松弛感"
			}
		}
	}`, http.StatusOK)

	first := getDraft(t, router)
	if first.Status != "draft" {
		t.Fatalf("expected status draft, got %q", first.Status)
	}
	if first.CurrentStep != "style_goal" {
		t.Fatalf("expected current_step style_goal, got %q", first.CurrentStep)
	}
	if first.Version != 1 {
		t.Fatalf("expected version 1, got %d", first.Version)
	}
	styleGoal := first.DraftData["style_goal"].(map[string]any)
	goals := styleGoal["goals"].([]any)
	if goals[0] != "干净" || goals[1] != "有气质" {
		t.Fatalf("expected style goals to be preserved, got %#v", goals)
	}

	putJSON(t, router, `{
		"step": "basic",
		"data": {
			"basic": {
				"height_cm": 168
			}
		}
	}`, http.StatusOK)

	second := getDraft(t, router)
	if second.CurrentStep != "basic" {
		t.Fatalf("expected current_step basic, got %q", second.CurrentStep)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}
	if _, ok := second.DraftData["basic"]; !ok {
		t.Fatalf("expected basic data, got %#v", second.DraftData)
	}
	if _, ok := second.DraftData["style_goal"]; !ok {
		t.Fatalf("expected first step data to be kept, got %#v", second.DraftData)
	}
}

func TestGetOnboardingDraftReturnsNotStartedWhenEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newAuthenticatedRouter(newMemoryDraftRepo())

	got := getDraft(t, router)

	if got.Status != "not_started" {
		t.Fatalf("expected not_started, got %q", got.Status)
	}
	if got.CurrentStep != "" {
		t.Fatalf("expected empty current_step, got %q", got.CurrentStep)
	}
	if got.Version != 0 {
		t.Fatalf("expected version 0, got %d", got.Version)
	}
	if len(got.DraftData) != 0 {
		t.Fatalf("expected empty draft_data, got %#v", got.DraftData)
	}
}

func TestSaveOnboardingDraftRejectsEmptyStep(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newAuthenticatedRouter(newMemoryDraftRepo())

	putJSON(t, router, `{"step":"","data":{"style_goal":{}}}`, http.StatusBadRequest)
}

func newAuthenticatedRouter(repo onboarding.DraftRepository) *gin.Engine {
	router := gin.New()
	group := router.Group("/api/user/onboarding")
	group.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{
			UserID:       12,
			UserPublicID: "usr_test",
			Surface:      "user",
		})
		c.Next()
	})
	onboarding.RegisterRoutes(group, onboarding.NewHandler(onboarding.NewService(repo), nil))
	return router
}

func putJSON(t *testing.T, router *gin.Engine, body string, expectedStatus int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPut, "/api/user/onboarding", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d, body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}
}

type draftResponse struct {
	Status      string         `json:"status"`
	CurrentStep string         `json:"current_step"`
	Version     int            `json:"version"`
	DraftData   map[string]any `json:"draft_data"`
}

func getDraft(t *testing.T, router *gin.Engine) draftResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/user/onboarding", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string        `json:"code"`
		Data draftResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "ok" {
		t.Fatalf("expected code ok, got %q", body.Code)
	}
	return body.Data
}
