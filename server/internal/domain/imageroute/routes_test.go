package imageroute_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/imageroute"

	"github.com/gin-gonic/gin"
)

func TestApplyFeedbackLikeActivatesRouteAndWritesEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_like", UserID: 12, Status: imageroute.StatusCandidate})
	router := newAuthenticatedRouter(repo)

	got := postFeedback(t, router, "irt_like", `{"action":"like"}`, http.StatusOK)

	if got.PublicID != "irt_like" || got.Status != imageroute.StatusActive {
		t.Fatalf("expected active route response, got %#v", got)
	}
	saved := repo.routes["irt_like"]
	if saved.Status != imageroute.StatusActive {
		t.Fatalf("expected route status active, got %q", saved.Status)
	}
	if saved.ActivatedAt == nil {
		t.Fatal("expected activated_at to be set")
	}
	event := repo.lastEvent(t)
	if event.EventType != imageroute.EventTypeRouteFeedback || event.Source != imageroute.EventSourceOnboardingReport || event.CreatedBy != imageroute.EventCreatedByUser {
		t.Fatalf("expected route feedback event metadata, got %#v", event)
	}
	if event.EventValue["action"] != imageroute.FeedbackActionLike || event.EventValue["status"] != imageroute.StatusActive {
		t.Fatalf("expected event action/status, got %#v", event.EventValue)
	}
}

func TestApplyFeedbackLikeAlreadyActiveRouteWritesEventAgain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	activatedAt := time.Now().UTC().Add(-time.Hour)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_active", UserID: 12, Status: imageroute.StatusActive, ActivatedAt: &activatedAt})
	router := newAuthenticatedRouter(repo)

	first := postFeedback(t, router, "irt_active", `{"action":"like"}`, http.StatusOK)
	second := postFeedback(t, router, "irt_active", `{"action":"like"}`, http.StatusOK)

	if first.Status != imageroute.StatusActive || second.Status != imageroute.StatusActive {
		t.Fatalf("expected repeated like to keep active status, got first=%#v second=%#v", first, second)
	}
	if len(repo.events) != 2 {
		t.Fatalf("expected two feedback events, got %#v", repo.events)
	}
	if repo.routes["irt_active"].ActivatedAt == nil || !repo.routes["irt_active"].ActivatedAt.Equal(activatedAt) {
		t.Fatalf("expected existing activated_at to be preserved, got %#v", repo.routes["irt_active"].ActivatedAt)
	}
}

func TestApplyFeedbackDislikeArchivesRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_dislike", UserID: 12, Status: imageroute.StatusCandidate})
	router := newAuthenticatedRouter(repo)

	got := postFeedback(t, router, "irt_dislike", `{"action":"dislike"}`, http.StatusOK)

	if got.Status != imageroute.StatusArchived {
		t.Fatalf("expected archived status, got %#v", got)
	}
	if repo.routes["irt_dislike"].Status != imageroute.StatusArchived {
		t.Fatalf("expected saved route archived, got %#v", repo.routes["irt_dislike"])
	}
}

func TestApplyFeedbackAdjustMovesRouteToRefinementAndKeepsReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_adjust", UserID: 12, Status: imageroute.StatusCandidate})
	router := newAuthenticatedRouter(repo)

	got := postFeedback(t, router, "irt_adjust", `{"action":"adjust","reason":"想更轻松一点"}`, http.StatusOK)

	if got.Status != imageroute.StatusRefinement {
		t.Fatalf("expected refinement status, got %#v", got)
	}
	event := repo.lastEvent(t)
	if event.EventValue["reason"] != "想更轻松一点" {
		t.Fatalf("expected reason in event value, got %#v", event.EventValue)
	}
	if event.EventValue["action"] != imageroute.FeedbackActionAdjust || event.EventValue["status"] != imageroute.StatusRefinement {
		t.Fatalf("expected adjust event value, got %#v", event.EventValue)
	}
}

func TestApplyFeedbackTrimsActionAndReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_trim", UserID: 12, Status: imageroute.StatusCandidate})
	router := newAuthenticatedRouter(repo)

	got := postFeedback(t, router, "irt_trim", `{"action":" like ","reason":"  想更利落一点  "}`, http.StatusOK)

	if got.Status != imageroute.StatusActive {
		t.Fatalf("expected trimmed like action to activate route, got %#v", got)
	}
	event := repo.lastEvent(t)
	if event.EventValue["action"] != imageroute.FeedbackActionLike || event.EventValue["reason"] != "想更利落一点" {
		t.Fatalf("expected trimmed event value, got %#v", event.EventValue)
	}
}

func TestApplyFeedbackRejectsTooLongReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_long_reason", UserID: 12, Status: imageroute.StatusCandidate})
	router := newAuthenticatedRouter(repo)

	request := httptest.NewRequest(http.MethodPost, "/api/user/image-routes/irt_long_reason/feedback", strings.NewReader(`{"action":"adjust","reason":"`+strings.Repeat("很", 501)+`"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "image_route.invalid_feedback_reason" {
		t.Fatalf("expected invalid feedback reason code, got %q", body.Code)
	}
	if repo.routes["irt_long_reason"].Status != imageroute.StatusCandidate {
		t.Fatalf("expected route status unchanged, got %#v", repo.routes["irt_long_reason"])
	}
	if len(repo.events) != 0 {
		t.Fatalf("expected no event for invalid reason, got %#v", repo.events)
	}
}

func TestApplyFeedbackRollsBackStatusWhenEventWriteFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRouteRepo()
	repo.failEventInsert = true
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_event_fail", UserID: 12, Status: imageroute.StatusCandidate})
	router := newAuthenticatedRouter(repo)

	request := httptest.NewRequest(http.MethodPost, "/api/user/image-routes/irt_event_fail/feedback", strings.NewReader(`{"action":"like"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.routes["irt_event_fail"].Status != imageroute.StatusCandidate {
		t.Fatalf("expected route status to roll back, got %#v", repo.routes["irt_event_fail"])
	}
	if len(repo.events) != 0 {
		t.Fatalf("expected no event after failed insert, got %#v", repo.events)
	}
}

func TestApplyFeedbackRejectsUnknownAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_unknown", UserID: 12, Status: imageroute.StatusCandidate})
	router := newAuthenticatedRouter(repo)

	request := httptest.NewRequest(http.MethodPost, "/api/user/image-routes/irt_unknown/feedback", strings.NewReader(`{"action":"skip"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "image_route.invalid_feedback_action" {
		t.Fatalf("expected invalid feedback action code, got %q", body.Code)
	}
	if len(repo.events) != 0 {
		t.Fatalf("expected no event for invalid action, got %#v", repo.events)
	}
}

func TestApplyFeedbackReturnsNotFoundForOtherUserRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMemoryRouteRepo()
	repo.add(imageroute.Route{ID: 1, PublicID: "irt_other", UserID: 99, Status: imageroute.StatusCandidate})
	router := newAuthenticatedRouter(repo)

	request := httptest.NewRequest(http.MethodPost, "/api/user/image-routes/irt_other/feedback", strings.NewReader(`{"action":"like"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if len(repo.events) != 0 {
		t.Fatalf("expected no event for missing route, got %#v", repo.events)
	}
}

type feedbackResponse struct {
	PublicID string `json:"public_id"`
	Status   string `json:"status"`
}

func newAuthenticatedRouter(repo *memoryRouteRepo) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{
			UserID:       12,
			UserPublicID: "usr_test",
			Surface:      "user",
		})
		c.Next()
	})
	imageroute.RegisterUserRoutesWithService(router.Group("/api/user/image-routes"), imageroute.NewFeedbackService(repo), nil)
	return router
}

func postFeedback(t *testing.T, router *gin.Engine, routePublicID string, body string, expectedStatus int) feedbackResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/user/image-routes/"+routePublicID+"/feedback", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d, body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}
	var responseBody struct {
		Code string           `json:"code"`
		Data feedbackResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &responseBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if responseBody.Code != "ok" {
		t.Fatalf("expected code ok, got %q", responseBody.Code)
	}
	return responseBody.Data
}

type memoryRouteRepo struct {
	routes          map[string]imageroute.Route
	events          []imageroute.Event
	failEventInsert bool
}

func newMemoryRouteRepo() *memoryRouteRepo {
	return &memoryRouteRepo{routes: map[string]imageroute.Route{}}
}

func (r *memoryRouteRepo) add(route imageroute.Route) {
	r.routes[route.PublicID] = route
}

func (r *memoryRouteRepo) CreateMany(_ context.Context, items []imageroute.Route) ([]imageroute.Route, error) {
	for i := range items {
		if items[i].ID == 0 {
			items[i].ID = int64(len(r.routes) + 1)
		}
		r.routes[items[i].PublicID] = items[i]
	}
	return items, nil
}

func (r *memoryRouteRepo) FindByPublicIDForUser(_ context.Context, userID int64, publicID string) (imageroute.Route, error) {
	route, ok := r.routes[publicID]
	if !ok || route.UserID != userID {
		return imageroute.Route{}, imageroute.ErrRouteNotFound
	}
	return route, nil
}

func (r *memoryRouteRepo) UpdateFeedback(_ context.Context, route imageroute.Route, event imageroute.Event) (imageroute.Route, error) {
	previous, ok := r.routes[route.PublicID]
	if !ok {
		return imageroute.Route{}, imageroute.ErrRouteNotFound
	}
	if route.Status == imageroute.StatusActive && route.ActivatedAt == nil {
		now := time.Now().UTC()
		route.ActivatedAt = &now
	}
	previousEvents := append([]imageroute.Event(nil), r.events...)
	r.routes[route.PublicID] = route
	if r.failEventInsert {
		r.routes[route.PublicID] = previous
		r.events = previousEvents
		return imageroute.Route{}, errors.New("insert route feedback event failed")
	}
	r.events = append(r.events, event)
	return route, nil
}

func (r *memoryRouteRepo) lastEvent(t *testing.T) imageroute.Event {
	t.Helper()
	if len(r.events) == 0 {
		t.Fatal("expected event to be written")
	}
	return r.events[len(r.events)-1]
}
