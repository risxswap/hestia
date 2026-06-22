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
	if !ok || draft.Status != onboarding.DraftStatusDraft {
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
	if !ok || draft.ID != draftID || draft.Status != onboarding.DraftStatusDraft {
		return onboarding.ErrDraftNotFound
	}
	draft.Status = onboarding.DraftStatusSubmitted
	r.drafts[userID] = draft
	return nil
}

func (r *memoryDraftRepo) ClaimDraft(_ context.Context, draftID int64, userID int64, contentHash string) error {
	draft, ok := r.drafts[userID]
	if !ok || draft.ID != draftID || draft.ContentHash != contentHash || draft.Status != onboarding.DraftStatusDraft {
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
	if env.transactor.commits != 1 || env.transactor.rollbacks != 0 {
		t.Fatalf("expected one committed transaction, got commits=%d rollbacks=%d", env.transactor.commits, env.transactor.rollbacks)
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

func TestSubmitOnboardingDoesNotCreateSecondReportAfterDraftSubmitted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newSubmitTestEnv(generator.NewRuleReportGenerator())

	putJSON(t, env.router, `{
		"step": "wardrobe",
		"data": {
			"basic": {"gender":"female","height_cm":168},
			"style_goal": {"goals": ["干净利落"]},
			"wardrobe": {"items": [{"name":"米白衬衫","category":"top"}]}
		}
	}`, http.StatusOK)

	submitOnboarding(t, env.router, http.StatusOK)
	submitOnboarding(t, env.router, http.StatusBadRequest)

	if len(env.reports.byID) != 1 {
		t.Fatalf("expected one report after duplicate submit, got %#v", env.reports.byID)
	}
	if len(env.jobs.byID) != 1 {
		t.Fatalf("expected one job after duplicate submit, got %#v", env.jobs.byID)
	}
}

func TestSubmitOnboardingRunsGeneratorOutsideTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	watcher := &transactionWatchingGenerator{}
	env := newSubmitTestEnv(watcher)
	watcher.transactor = env.transactor

	putJSON(t, env.router, `{
		"step": "wardrobe",
		"data": {
			"basic": {"gender":"female","height_cm":168},
			"style_goal": {"goals": ["干净利落"]},
			"wardrobe": {"items": [{"name":"米白衬衫","category":"top"}]}
		}
	}`, http.StatusOK)

	submitOnboarding(t, env.router, http.StatusOK)

	if watcher.calledInTx {
		t.Fatal("expected generator to run before transaction begins")
	}
}

func TestSubmitOnboardingRegistersPublicIDAndClientRefAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newSubmitTestEnv(generator.NewRuleReportGenerator())

	putJSON(t, env.router, `{
		"step": "wardrobe",
		"data": {
			"basic": {"gender":"female","height_cm":168},
			"style_goal": {
				"goals": ["干净利落"],
				"reference_styles": ["刘诗诗"]
			},
			"wardrobe": {"items": [{"name":"米白衬衫","category":"top"}]},
			"photos": [
				{"asset_public_id":"ast_existing_selfie","asset_type":"selfie","note":"自然光自拍"}
			],
			"reference": {
				"uploaded_refs": [
					{"client_ref":"tmp-ref-1","asset_type":"style_reference","note":"参考图"}
				]
			}
		}
	}`, http.StatusOK)

	submitOnboarding(t, env.router, http.StatusOK)

	if len(env.assets.created) != 2 {
		t.Fatalf("expected two registered assets, got %#v", env.assets.created)
	}
	if env.assets.created[0].PublicID != "ast_existing_selfie" {
		t.Fatalf("expected existing asset public id to be preserved, got %q", env.assets.created[0].PublicID)
	}
	if env.assets.created[0].AssetType != "selfie" || env.assets.created[0].Note != "自然光自拍" {
		t.Fatalf("expected selfie metadata to be preserved, got %#v", env.assets.created[0])
	}
	if env.assets.created[1].PublicID == "" || env.assets.created[1].PublicID == env.assets.created[1].ClientRef {
		t.Fatalf("expected formal public id for client ref asset, got %#v", env.assets.created[1])
	}
	if env.assets.created[1].ClientRef != "tmp-ref-1" || env.assets.created[1].Note != "参考图" {
		t.Fatalf("expected client ref metadata to be preserved, got %#v", env.assets.created[1])
	}
}

func TestSubmitOnboardingDoesNotExposeReadyReportWhenAttachRoutesFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newSubmitTestEnv(generator.NewRuleReportGenerator())
	env.reports.failAddRoutes = true

	putJSON(t, env.router, `{
		"step": "wardrobe",
		"data": {
			"basic": {"gender":"female","height_cm":168},
			"style_goal": {"goals": ["干净利落"]},
			"wardrobe": {"items": [{"name":"米白衬衫","category":"top"}]}
		}
	}`, http.StatusOK)

	submitOnboarding(t, env.router, http.StatusInternalServerError)
	got := getJob(t, env.router, env.jobs.lastJob.PublicID)
	if got.Status != "failed" {
		t.Fatalf("expected failed job, got %q", got.Status)
	}
	assertLatestReportStatus(t, env.router, http.StatusNotFound)
}

func TestSubmitOnboardingRollsBackBusinessWritesWhenMarkSucceededFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newSubmitTestEnv(generator.NewRuleReportGenerator())
	env.jobs.failSucceeded = true

	putJSON(t, env.router, `{
		"step": "wardrobe",
		"data": {
			"basic": {"gender":"female","height_cm":168},
			"style_goal": {"goals": ["干净利落"]},
			"wardrobe": {"items": [{"name":"米白衬衫","category":"top"}]},
			"photos": [{"client_ref":"tmp-selfie","note":"自拍"}]
		}
	}`, http.StatusOK)

	submitOnboarding(t, env.router, http.StatusInternalServerError)

	assertLatestReportStatus(t, env.router, http.StatusNotFound)
	if len(env.assets.created) != 0 {
		t.Fatalf("expected asset writes to roll back, got %#v", env.assets.created)
	}
	if len(env.profiles.byUser) != 0 {
		t.Fatalf("expected profile writes to roll back, got %#v", env.profiles.byUser)
	}
	if len(env.wardrobes.created) != 0 {
		t.Fatalf("expected wardrobe writes to roll back, got %#v", env.wardrobes.created)
	}
	got := getJob(t, env.router, env.jobs.lastJob.PublicID)
	if got.Status != "failed" {
		t.Fatalf("expected transaction failure to leave failed job record, got %q", got.Status)
	}
	if env.transactor.rollbacks != 1 {
		t.Fatalf("expected one rolled back transaction, got %d", env.transactor.rollbacks)
	}
}

func TestSubmitOnboardingRejectsInvalidDraftShapes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
	}{
		{
			name: "basic is a string",
			body: `{
				"step": "wardrobe",
				"data": {
					"basic": "not-an-object",
					"style_goal": {"goals": ["干净利落"]},
					"wardrobe": {"items": [{"name":"米白衬衫","category":"top"}]}
				}
			}`,
		},
		{
			name: "style goals is a string",
			body: `{
				"step": "wardrobe",
				"data": {
					"basic": {"gender":"female"},
					"style_goal": {"goals": "干净利落"},
					"wardrobe": {"items": [{"name":"米白衬衫","category":"top"}]}
				}
			}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newSubmitTestEnv(generator.NewRuleReportGenerator())
			putJSON(t, env.router, tt.body, http.StatusOK)

			submitOnboarding(t, env.router, http.StatusBadRequest)
		})
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
	router     *gin.Engine
	jobs       *memoryJobRepo
	assets     *memoryAssetRepo
	profiles   *memoryProfileRepo
	wardrobes  *memoryWardrobeRepo
	reports    *memoryReportRepo
	transactor *memoryTransactor
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
	transactor := &memoryTransactor{
		drafts:    drafts,
		profiles:  profiles,
		assets:    assets,
		wardrobes: wardrobes,
		jobs:      jobs,
		reports:   reports,
		routes:    routes,
		deps: onboarding.SubmitDependencies{
			Drafts:      drafts,
			Profiles:    profileService,
			Assets:      assetService,
			Wardrobe:    wardrobeService,
			Jobs:        jobService,
			Reports:     reportService,
			ImageRoutes: routeService,
			Generator:   reportGenerator,
		},
	}

	onboardingService := onboarding.NewSubmitService(drafts, onboarding.SubmitDependencies{
		Profiles:    profileService,
		Assets:      assetService,
		Wardrobe:    wardrobeService,
		Jobs:        jobService,
		Reports:     reportService,
		ImageRoutes: routeService,
		Generator:   reportGenerator,
		Transactor:  transactor,
	})
	onboarding.RegisterRoutes(router.Group("/api/user/onboarding"), onboarding.NewHandler(onboardingService, nil))
	report.RegisterUserRoutesWithService(router.Group("/api/user/reports"), reportService, nil)
	job.RegisterUserRoutesWithService(router.Group("/api/user/jobs"), jobService, nil)
	return submitTestEnv{router: router, jobs: jobs, assets: assets, profiles: profiles, wardrobes: wardrobes, reports: reports, transactor: transactor}
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

func assertLatestReportStatus(t *testing.T, router *gin.Engine, expectedStatus int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/user/reports/latest", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d, body=%s", expectedStatus, recorder.Code, recorder.Body.String())
	}
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

type transactionWatchingGenerator struct {
	transactor *memoryTransactor
	calledInTx bool
}

func (g *transactionWatchingGenerator) GenerateInitialReport(ctx context.Context, input generator.InitialReportInput) (*generator.InitialReportResult, error) {
	if g.transactor != nil && g.transactor.active {
		g.calledInTx = true
	}
	return generator.NewRuleReportGenerator().GenerateInitialReport(ctx, input)
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
	nextID  int64
	created []asset.Asset
}

func newMemoryAssetRepo() *memoryAssetRepo {
	return &memoryAssetRepo{nextID: 1}
}

func (r *memoryAssetRepo) CreateMany(_ context.Context, items []asset.Asset) ([]asset.Asset, error) {
	for i := range items {
		items[i].ID = r.nextID
		r.nextID++
	}
	r.created = append(r.created, items...)
	return items, nil
}

type memoryWardrobeRepo struct {
	nextID  int64
	created []wardrobe.Item
}

func newMemoryWardrobeRepo() *memoryWardrobeRepo {
	return &memoryWardrobeRepo{nextID: 1}
}

func (r *memoryWardrobeRepo) CreateCoreItems(_ context.Context, items []wardrobe.Item) ([]wardrobe.Item, error) {
	for i := range items {
		items[i].ID = r.nextID
		r.nextID++
	}
	r.created = append(r.created, items...)
	return items, nil
}

type memoryJobRepo struct {
	nextID        int64
	lastJob       job.Job
	byID          map[int64]job.Job
	failSucceeded bool
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
	if status == job.StatusSucceeded && r.failSucceeded {
		return errors.New("mark succeeded failed")
	}
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
	nextID        int64
	byID          map[int64]report.Report
	routes        map[int64][]report.ReportRoute
	failAddRoutes bool
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
	if r.failAddRoutes {
		return errors.New("attach routes failed")
	}
	r.routes[reportID] = append(r.routes[reportID], items...)
	return nil
}

func (r *memoryReportRepo) MarkReady(_ context.Context, reportID int64) error {
	item, ok := r.byID[reportID]
	if !ok {
		return report.ErrReportNotFound
	}
	item.Status = report.StatusReady
	r.byID[reportID] = item
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

type memoryTransactor struct {
	drafts    *memoryDraftRepo
	profiles  *memoryProfileRepo
	assets    *memoryAssetRepo
	wardrobes *memoryWardrobeRepo
	jobs      *memoryJobRepo
	reports   *memoryReportRepo
	routes    *memoryImageRouteRepo
	deps      onboarding.SubmitDependencies
	commits   int
	rollbacks int
	active    bool
}

func (t *memoryTransactor) WithinTx(ctx context.Context, fn func(ctx context.Context, deps onboarding.SubmitDependencies) error) error {
	snapshot := t.snapshot()
	t.active = true
	defer func() {
		t.active = false
	}()
	if err := fn(ctx, t.deps); err != nil {
		t.restore(snapshot)
		t.rollbacks++
		return err
	}
	t.commits++
	return nil
}

type memorySnapshot struct {
	drafts    memoryDraftSnapshot
	profiles  memoryProfileSnapshot
	assets    memoryAssetSnapshot
	wardrobes memoryWardrobeSnapshot
	jobs      memoryJobSnapshot
	reports   memoryReportSnapshot
	routes    memoryImageRouteSnapshot
}

func (t *memoryTransactor) snapshot() memorySnapshot {
	return memorySnapshot{
		drafts:    t.drafts.snapshot(),
		profiles:  t.profiles.snapshot(),
		assets:    t.assets.snapshot(),
		wardrobes: t.wardrobes.snapshot(),
		jobs:      t.jobs.snapshot(),
		reports:   t.reports.snapshot(),
		routes:    t.routes.snapshot(),
	}
}

func (t *memoryTransactor) restore(snapshot memorySnapshot) {
	t.drafts.restore(snapshot.drafts)
	t.profiles.restore(snapshot.profiles)
	t.assets.restore(snapshot.assets)
	t.wardrobes.restore(snapshot.wardrobes)
	t.jobs.restore(snapshot.jobs)
	t.reports.restore(snapshot.reports)
	t.routes.restore(snapshot.routes)
}

type memoryDraftSnapshot struct {
	drafts map[int64]onboarding.Draft
	nextID int64
}

func (r *memoryDraftRepo) snapshot() memoryDraftSnapshot {
	drafts := make(map[int64]onboarding.Draft, len(r.drafts))
	for key, value := range r.drafts {
		drafts[key] = value
	}
	return memoryDraftSnapshot{drafts: drafts, nextID: r.nextID}
}

func (r *memoryDraftRepo) restore(snapshot memoryDraftSnapshot) {
	r.drafts = snapshot.drafts
	r.nextID = snapshot.nextID
}

type memoryProfileSnapshot struct {
	nextID int64
	byUser map[int64]profile.Profile
}

func (r *memoryProfileRepo) snapshot() memoryProfileSnapshot {
	byUser := make(map[int64]profile.Profile, len(r.byUser))
	for key, value := range r.byUser {
		byUser[key] = value
	}
	return memoryProfileSnapshot{nextID: r.nextID, byUser: byUser}
}

func (r *memoryProfileRepo) restore(snapshot memoryProfileSnapshot) {
	r.nextID = snapshot.nextID
	r.byUser = snapshot.byUser
}

type memoryAssetSnapshot struct {
	nextID  int64
	created []asset.Asset
}

func (r *memoryAssetRepo) snapshot() memoryAssetSnapshot {
	return memoryAssetSnapshot{nextID: r.nextID, created: append([]asset.Asset(nil), r.created...)}
}

func (r *memoryAssetRepo) restore(snapshot memoryAssetSnapshot) {
	r.nextID = snapshot.nextID
	r.created = snapshot.created
}

type memoryWardrobeSnapshot struct {
	nextID  int64
	created []wardrobe.Item
}

func (r *memoryWardrobeRepo) snapshot() memoryWardrobeSnapshot {
	return memoryWardrobeSnapshot{nextID: r.nextID, created: append([]wardrobe.Item(nil), r.created...)}
}

func (r *memoryWardrobeRepo) restore(snapshot memoryWardrobeSnapshot) {
	r.nextID = snapshot.nextID
	r.created = snapshot.created
}

type memoryJobSnapshot struct {
	nextID        int64
	lastJob       job.Job
	byID          map[int64]job.Job
	failSucceeded bool
}

func (r *memoryJobRepo) snapshot() memoryJobSnapshot {
	byID := make(map[int64]job.Job, len(r.byID))
	for key, value := range r.byID {
		byID[key] = value
	}
	return memoryJobSnapshot{nextID: r.nextID, lastJob: r.lastJob, byID: byID, failSucceeded: r.failSucceeded}
}

func (r *memoryJobRepo) restore(snapshot memoryJobSnapshot) {
	r.nextID = snapshot.nextID
	r.lastJob = snapshot.lastJob
	r.byID = snapshot.byID
	r.failSucceeded = snapshot.failSucceeded
}

type memoryReportSnapshot struct {
	nextID        int64
	byID          map[int64]report.Report
	routes        map[int64][]report.ReportRoute
	failAddRoutes bool
}

func (r *memoryReportRepo) snapshot() memoryReportSnapshot {
	byID := make(map[int64]report.Report, len(r.byID))
	for key, value := range r.byID {
		byID[key] = value
	}
	routes := make(map[int64][]report.ReportRoute, len(r.routes))
	for key, value := range r.routes {
		routes[key] = append([]report.ReportRoute(nil), value...)
	}
	return memoryReportSnapshot{nextID: r.nextID, byID: byID, routes: routes, failAddRoutes: r.failAddRoutes}
}

func (r *memoryReportRepo) restore(snapshot memoryReportSnapshot) {
	r.nextID = snapshot.nextID
	r.byID = snapshot.byID
	r.routes = snapshot.routes
	r.failAddRoutes = snapshot.failAddRoutes
}

type memoryImageRouteSnapshot struct {
	nextID int64
}

func (r *memoryImageRouteRepo) snapshot() memoryImageRouteSnapshot {
	return memoryImageRouteSnapshot{nextID: r.nextID}
}

func (r *memoryImageRouteRepo) restore(snapshot memoryImageRouteSnapshot) {
	r.nextID = snapshot.nextID
}
