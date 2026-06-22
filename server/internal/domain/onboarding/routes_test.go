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
	"hestia/server/internal/domain/asset"
	"hestia/server/internal/domain/generator"
	"hestia/server/internal/domain/imageroute"
	"hestia/server/internal/domain/job"
	"hestia/server/internal/domain/onboarding"
	"hestia/server/internal/domain/profile"
	"hestia/server/internal/domain/report"
	"hestia/server/internal/domain/wardrobe"

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

func (r *memoryDraftRepo) MarkSubmitted(_ context.Context, draftID int64, userID int64) error {
	draft, ok := r.drafts[userID]
	if !ok || draft.ID != draftID {
		return onboarding.ErrDraftNotFound
	}
	draft.Status = onboarding.DraftStatusSubmitted
	r.drafts[userID] = draft
	return nil
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

func TestSubmitOnboardingGeneratesInitialReportAndLatestReportCanBeRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newSubmitTestEnv(generator.NewRuleReportGenerator())

	putJSON(t, env.router, `{
		"step": "wardrobe",
		"data": {
			"basic": {"gender":"female","height_cm":168},
			"style_goal": {
				"goals": ["干净利落", "通勤有气质"],
				"avoidances": ["过度甜美"],
				"scenarios": ["工作日通勤"]
			},
			"wardrobe": {
				"items": [
					{"name":"米白衬衫","category":"top","color":"米白"},
					{"name":"直筒牛仔裤","category":"bottom","color":"蓝色"}
				]
			}
		}
	}`, http.StatusOK)

	submitted := submitOnboarding(t, env.router, http.StatusOK)
	if submitted.JobPublicID == "" {
		t.Fatalf("expected job_public_id")
	}
	if submitted.ReportPublicID == "" {
		t.Fatalf("expected report_public_id")
	}

	latest := getLatestReport(t, env.router)
	if latest.PublicID != submitted.ReportPublicID {
		t.Fatalf("expected latest report %q, got %q", submitted.ReportPublicID, latest.PublicID)
	}
	if latest.Summary == "" {
		t.Fatalf("expected report summary")
	}
	if len(latest.Routes) == 0 {
		t.Fatalf("expected report routes")
	}
}

func TestSubmitOnboardingMarksJobFailedWhenGeneratorFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newSubmitTestEnv(failingGenerator{})

	putJSON(t, env.router, `{
		"step": "wardrobe",
		"data": {
			"basic": {"gender":"female","height_cm":168},
			"style_goal": {"goals": ["干净利落"]},
			"wardrobe": {"items": [{"name":"米白衬衫","category":"top"}]}
		}
	}`, http.StatusOK)

	submitOnboarding(t, env.router, http.StatusInternalServerError)
	if env.jobs.lastJob.PublicID == "" {
		t.Fatalf("expected a job to be created")
	}
	got := getJob(t, env.router, env.jobs.lastJob.PublicID)
	if got.Status != "failed" {
		t.Fatalf("expected failed job, got %q", got.Status)
	}
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

type submitTestEnv struct {
	router *gin.Engine
	jobs   *memoryJobRepo
}

func newSubmitTestEnv(reportGenerator generator.ReportGenerator) submitTestEnv {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{
			UserID:       12,
			UserPublicID: "usr_test",
			Surface:      "user",
		})
		c.Next()
	})

	drafts := newMemoryDraftRepo()
	profiles := newMemoryProfileRepo()
	assets := newMemoryAssetRepo()
	wardrobes := newMemoryWardrobeRepo()
	jobs := newMemoryJobRepo()
	reports := newMemoryReportRepo()
	routes := newMemoryImageRouteRepo()

	profileService := profile.NewService(profiles)
	assetService := asset.NewService(assets)
	wardrobeService := wardrobe.NewService(wardrobes)
	jobService := job.NewService(jobs)
	reportService := report.NewService(reports)
	routeService := imageroute.NewService(routes)

	onboardingService := onboarding.NewSubmitService(drafts, onboarding.SubmitDependencies{
		Profiles:    profileService,
		Assets:      assetService,
		Wardrobe:    wardrobeService,
		Jobs:        jobService,
		Reports:     reportService,
		ImageRoutes: routeService,
		Generator:   reportGenerator,
	})
	onboarding.RegisterRoutes(router.Group("/api/user/onboarding"), onboarding.NewHandler(onboardingService, nil))
	report.RegisterUserRoutesWithService(router.Group("/api/user/reports"), reportService, nil)
	job.RegisterUserRoutesWithService(router.Group("/api/user/jobs"), jobService, nil)
	return submitTestEnv{router: router, jobs: jobs}
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

func submitOnboarding(t *testing.T, router *gin.Engine, expectedStatus int) submitResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/user/onboarding/submit", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d, body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string         `json:"code"`
		Data submitResponse `json:"data"`
	}
	if expectedStatus == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if body.Code != "ok" {
			t.Fatalf("expected code ok, got %q", body.Code)
		}
	}
	return body.Data
}

type submitResponse struct {
	JobPublicID    string `json:"job_public_id"`
	ReportPublicID string `json:"report_public_id"`
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

type reportResponse struct {
	PublicID    string         `json:"public_id"`
	Title       string         `json:"title"`
	Summary     string         `json:"summary"`
	ContentJSON map[string]any `json:"content_json"`
	Routes      []any          `json:"routes"`
}

func getLatestReport(t *testing.T, router *gin.Engine) reportResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/user/reports/latest", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string         `json:"code"`
		Data reportResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Data
}

type jobResponse struct {
	PublicID string `json:"public_id"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

func getJob(t *testing.T, router *gin.Engine, publicID string) jobResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/user/jobs/"+publicID, nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string      `json:"code"`
		Data jobResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Data
}

type failingGenerator struct{}

func (failingGenerator) GenerateInitialReport(context.Context, generator.InitialReportInput) (*generator.InitialReportResult, error) {
	return nil, errors.New("generator unavailable")
}

type memoryProfileRepo struct {
	nextID int64
	byUser map[int64]profile.Profile
}

func newMemoryProfileRepo() *memoryProfileRepo {
	return &memoryProfileRepo{nextID: 1, byUser: map[int64]profile.Profile{}}
}

func (r *memoryProfileRepo) Upsert(ctx context.Context, item profile.Profile) (profile.Profile, error) {
	if existing, ok := r.byUser[item.UserID]; ok {
		item.ID = existing.ID
		item.PublicID = existing.PublicID
	} else {
		item.ID = r.nextID
		r.nextID++
	}
	r.byUser[item.UserID] = item
	return item, nil
}

func (r *memoryProfileRepo) ReplaceFacts(context.Context, int64, int64, []profile.Fact) error {
	return nil
}

func (r *memoryProfileRepo) ReplacePrefs(context.Context, int64, int64, []profile.Pref) error {
	return nil
}

func (r *memoryProfileRepo) CreateInferences(context.Context, []profile.Inference) error {
	return nil
}

func (r *memoryProfileRepo) MarkUserOnboardingCompleted(context.Context, int64) error {
	return nil
}

type memoryAssetRepo struct {
	nextID int64
}

func newMemoryAssetRepo() *memoryAssetRepo {
	return &memoryAssetRepo{nextID: 1}
}

func (r *memoryAssetRepo) CreateMany(_ context.Context, items []asset.Asset) ([]asset.Asset, error) {
	for i := range items {
		items[i].ID = r.nextID
		r.nextID++
	}
	return items, nil
}

type memoryWardrobeRepo struct {
	nextID int64
}

func newMemoryWardrobeRepo() *memoryWardrobeRepo {
	return &memoryWardrobeRepo{nextID: 1}
}

func (r *memoryWardrobeRepo) CreateCoreItems(_ context.Context, items []wardrobe.Item) ([]wardrobe.Item, error) {
	for i := range items {
		items[i].ID = r.nextID
		r.nextID++
	}
	return items, nil
}

type memoryJobRepo struct {
	nextID  int64
	lastJob job.Job
	byID    map[int64]job.Job
}

func newMemoryJobRepo() *memoryJobRepo {
	return &memoryJobRepo{nextID: 1, byID: map[int64]job.Job{}}
}

func (r *memoryJobRepo) Create(_ context.Context, item job.Job) (job.Job, error) {
	item.ID = r.nextID
	r.nextID++
	r.byID[item.ID] = item
	r.lastJob = item
	return item, nil
}

func (r *memoryJobRepo) UpdateStatus(_ context.Context, id int64, status string, output map[string]any, errorMessage string) error {
	item := r.byID[id]
	item.Status = status
	item.OutputSummary = output
	item.ErrorMessage = errorMessage
	r.byID[id] = item
	r.lastJob = item
	return nil
}

func (r *memoryJobRepo) FindByPublicIDForUser(_ context.Context, userID int64, publicID string) (job.Job, error) {
	for _, item := range r.byID {
		if item.UserID == userID && item.PublicID == publicID {
			return item, nil
		}
	}
	return job.Job{}, job.ErrJobNotFound
}

type memoryReportRepo struct {
	nextID int64
	byID   map[int64]report.Report
	routes map[int64][]report.ReportRoute
}

func newMemoryReportRepo() *memoryReportRepo {
	return &memoryReportRepo{nextID: 1, byID: map[int64]report.Report{}, routes: map[int64][]report.ReportRoute{}}
}

func (r *memoryReportRepo) Create(_ context.Context, item report.Report) (report.Report, error) {
	item.ID = r.nextID
	r.nextID++
	r.byID[item.ID] = item
	return item, nil
}

func (r *memoryReportRepo) AddRoutes(_ context.Context, reportID int64, items []report.ReportRoute) error {
	r.routes[reportID] = append(r.routes[reportID], items...)
	return nil
}

func (r *memoryReportRepo) LatestInitialForUser(_ context.Context, userID int64) (report.Report, error) {
	var latest report.Report
	for _, item := range r.byID {
		if item.UserID == userID && item.ReportType == report.TypeInitial && item.Status == report.StatusReady {
			if latest.ID == 0 || item.ID > latest.ID {
				latest = item
			}
		}
	}
	if latest.ID == 0 {
		return report.Report{}, report.ErrReportNotFound
	}
	return latest, nil
}

func (r *memoryReportRepo) FindByPublicIDForUser(_ context.Context, userID int64, publicID string) (report.Report, error) {
	for _, item := range r.byID {
		if item.UserID == userID && item.PublicID == publicID {
			return item, nil
		}
	}
	return report.Report{}, report.ErrReportNotFound
}

func (r *memoryReportRepo) RoutesByReportID(_ context.Context, reportID int64) ([]report.ReportRoute, error) {
	return append([]report.ReportRoute(nil), r.routes[reportID]...), nil
}

type memoryImageRouteRepo struct {
	nextID int64
}

func newMemoryImageRouteRepo() *memoryImageRouteRepo {
	return &memoryImageRouteRepo{nextID: 1}
}

func (r *memoryImageRouteRepo) CreateMany(_ context.Context, items []imageroute.Route) ([]imageroute.Route, error) {
	for i := range items {
		items[i].ID = r.nextID
		r.nextID++
	}
	return items, nil
}
