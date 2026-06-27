# Profile Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the miniapp “我的” profile center as a profile dashboard backed by user-facing profile summary and update APIs.

**Architecture:** Add user-facing profile APIs inside `server/internal/domain/profile`, then route them under `/api/user/profile`. The miniapp calls these APIs through `miniapp/utils/api.js` and renders `pages/profile` as a dashboard with lightweight edit panels and safe privacy placeholders.

**Tech Stack:** Go 1.25 + Gin + sqlx for the server; native WeChat Mini Program JavaScript/WXML/WXSS for the client; existing Node verification scripts for miniapp behavior.

---

## File Structure

- Modify `server/internal/domain/profile/model.go`: add response/request models for summary, profile update and preferences update.
- Modify `server/internal/domain/profile/service.go`: add summary, update profile and update preferences use cases with validation.
- Modify `server/internal/domain/profile/repo.go`: add MySQL reads/writes for users, profiles, facts, prefs, inferences and latest report metadata.
- Create `server/internal/domain/profile/handler.go`: HTTP handlers for user-facing profile endpoints.
- Create `server/internal/domain/profile/routes.go`: route registration helpers.
- Create `server/internal/domain/profile/routes_test.go`: route-level tests with fake repository.
- Modify `server/internal/app/user/router.go`: mount `/api/user/profile`.
- Modify `miniapp/utils/api.js`: add `getProfileSummary`, `updateProfile`, `updateProfilePreferences`.
- Modify `miniapp/scripts/verify-api-client.js`: verify profile client paths and methods.
- Modify `miniapp/pages/profile/profile.js`: load summary, normalize dashboard data, handle edit/save and local logout.
- Modify `miniapp/pages/profile/profile.wxml`: render profile dashboard, quick entries, edit panels and privacy actions.
- Modify `miniapp/pages/profile/profile.wxss`: profile dashboard styles consistent with existing miniapp.
- Create `miniapp/scripts/verify-profile-page.js`: page behavior verification.
- Modify `miniapp/package.json`: add `verify:profile-page`.

---

### Task 1: Backend Profile Summary API

**Files:**
- Modify: `server/internal/domain/profile/model.go`
- Modify: `server/internal/domain/profile/service.go`
- Modify: `server/internal/domain/profile/repo.go`
- Create: `server/internal/domain/profile/handler.go`
- Create: `server/internal/domain/profile/routes.go`
- Create: `server/internal/domain/profile/routes_test.go`
- Modify: `server/internal/app/user/router.go`

- [ ] **Step 1: Write failing route tests**

Add `server/internal/domain/profile/routes_test.go` with tests equivalent to:

```go
package profile_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hestia/server/internal/common/auth"
	"hestia/server/internal/domain/profile"

	"github.com/gin-gonic/gin"
)

func TestSummaryReturnsDashboardData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generatedAt := time.Date(2026, 6, 27, 6, 0, 0, 0, time.UTC)
	repo := &routeProfileRepo{
		summary: profile.Summary{
			User: profile.UserSummary{UserPublicID: "usr_test", Nickname: "明明", OnboardingStatus: "completed"},
			Profile: &profile.ProfileSummary{ProfilePublicID: "prf_test", Gender: "female", HeightCM: intPtr(165), LifestyleScenarios: []string{"通勤", "周末见朋友"}, StyleGoalSummary: "更利落"},
			MemorySummary: profile.MemorySummary{FactCount: 4, PreferenceCount: 2, AvoidanceCount: 1, InferenceCount: 3, PendingConfirmationCount: 1},
			LatestReport: &profile.LatestReportSummary{PublicID: "rpt_test", Title: "初版个人形象报告", Status: "ready", GeneratedAt: &generatedAt},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodGet, "/api/user/profile/summary", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code string          `json:"code"`
		Data profile.Summary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.User.UserPublicID != "usr_test" {
		t.Fatalf("expected user summary, got %#v", body.Data.User)
	}
	if body.Data.Profile == nil || body.Data.Profile.ProfilePublicID != "prf_test" {
		t.Fatalf("expected profile summary, got %#v", body.Data.Profile)
	}
	if len(body.Data.QuickEntries) != 4 {
		t.Fatalf("expected four quick entries, got %#v", body.Data.QuickEntries)
	}
}

func TestSummaryReturnsEmptyStateWithoutProfileOrReport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{
		summary: profile.Summary{
			User:          profile.UserSummary{UserPublicID: "usr_test", Nickname: "明明", OnboardingStatus: "not_started"},
			MemorySummary: profile.MemorySummary{},
		},
	}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodGet, "/api/user/profile/summary", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data profile.Summary `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Profile != nil {
		t.Fatalf("expected nil profile, got %#v", body.Data.Profile)
	}
	if body.Data.LatestReport != nil {
		t.Fatalf("expected nil report, got %#v", body.Data.LatestReport)
	}
}

func newProfileRouteTestRouter(repo *routeProfileRepo) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		auth.SetUserContext(c, auth.User{UserID: 12, UserPublicID: "usr_test", Surface: "user"})
		c.Next()
	})
	profile.RegisterUserRoutesWithService(router.Group("/api/user/profile"), profile.NewService(repo), nil)
	return router
}

type routeProfileRepo struct {
	summary profile.Summary
	err     error
}

func (r *routeProfileRepo) Summary(ctx context.Context, userID int64) (profile.Summary, error) {
	if r.err != nil {
		return profile.Summary{}, r.err
	}
	return r.summary, nil
}

func (r *routeProfileRepo) Upsert(ctx context.Context, item profile.Profile) (profile.Profile, error) { return profile.Profile{}, errors.New("unused") }
func (r *routeProfileRepo) ReplaceFacts(ctx context.Context, userID int64, profileID int64, facts []profile.Fact) error { return errors.New("unused") }
func (r *routeProfileRepo) ReplacePrefs(ctx context.Context, userID int64, profileID int64, prefs []profile.Pref) error { return errors.New("unused") }
func (r *routeProfileRepo) CreateInferences(ctx context.Context, inferences []profile.Inference) error { return errors.New("unused") }
func (r *routeProfileRepo) MarkUserOnboardingCompleted(ctx context.Context, userID int64) error { return errors.New("unused") }
func intPtr(value int) *int { return &value }
```

- [ ] **Step 2: Run route tests and confirm failure**

Run: `cd server && go test ./internal/domain/profile -run 'TestSummary' -count=1`

Expected: FAIL because `profile.Summary`, `RegisterUserRoutesWithService` and `Summary` repository method do not exist yet.

- [ ] **Step 3: Add summary models and service method**

Add to `server/internal/domain/profile/model.go`:

```go
type UserSummary struct {
	UserPublicID     string `json:"user_public_id"`
	Nickname         string `json:"nickname"`
	OnboardingStatus string `json:"onboarding_status"`
}

type ProfileSummary struct {
	ProfilePublicID    string   `json:"profile_public_id"`
	Gender             string   `json:"gender"`
	HeightCM           *int     `json:"height_cm"`
	BodyNotes          string   `json:"body_notes"`
	SkinNotes          string   `json:"skin_notes"`
	HairNotes          string   `json:"hair_notes"`
	LifestyleScenarios []string `json:"lifestyle_scenarios"`
	StyleGoalSummary   string   `json:"style_goal_summary"`
}

type MemorySummary struct {
	FactCount                int `json:"fact_count"`
	PreferenceCount          int `json:"preference_count"`
	AvoidanceCount            int `json:"avoidance_count"`
	InferenceCount            int `json:"inference_count"`
	PendingConfirmationCount  int `json:"pending_confirmation_count"`
}

type LatestReportSummary struct {
	PublicID    string     `json:"public_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	GeneratedAt *time.Time `json:"generated_at"`
}

type QuickEntry struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type Summary struct {
	User          UserSummary          `json:"user"`
	Profile      *ProfileSummary      `json:"profile"`
	MemorySummary MemorySummary       `json:"memory_summary"`
	LatestReport *LatestReportSummary `json:"latest_report"`
	QuickEntries []QuickEntry         `json:"quick_entries"`
}
```

Update `server/internal/domain/profile/service.go` repository interface and add:

```go
func (s *Service) Summary(ctx context.Context, userID int64) (Summary, error) {
	if s == nil || s.repo == nil {
		return Summary{}, errors.New("profile service dependencies are nil")
	}
	summary, err := s.repo.Summary(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	summary.QuickEntries = buildQuickEntries(summary)
	return summary, nil
}
```

- [ ] **Step 4: Add handler, routes, and router mount**

Create `handler.go` with `Summary(c *gin.Context)` reading `auth.UserFromContext`, calling `service.Summary`, returning `response.OK`.

Create `routes.go` with:

```go
func RegisterUserRoutes(group *gin.RouterGroup, deps *baseapp.Deps) {
	var repo Repository
	var logger *slog.Logger
	if deps != nil {
		repo = NewMySQLRepository(deps.DB)
		logger = deps.Logger
	}
	RegisterUserRoutesWithService(group, NewService(repo), logger)
}

func RegisterUserRoutesWithService(group *gin.RouterGroup, service *Service, logger *slog.Logger) {
	handler := NewHandler(service, logger)
	group.GET("/summary", handler.Summary)
}
```

Modify `server/internal/app/user/router.go` to import `profile` and mount:

```go
profile.RegisterUserRoutes(protected.Group("/profile"), deps)
```

- [ ] **Step 5: Implement MySQL summary repository**

Add `Summary(ctx, userID)` to `server/internal/domain/profile/repo.go` using:

- `users` for public id, nickname and onboarding status.
- `profiles` for active profile, decoding `lifestyle_scenarios`.
- `profile_facts` count where `deleted_at IS NULL`.
- `profile_prefs` counts by polarity.
- `profile_inferences` count active and pending `confirmed_by_user = 0`.
- `reports` latest ready initial report.

If profile or report is missing, return `nil` pointers without error.

- [ ] **Step 6: Run tests**

Run: `cd server && go test ./internal/domain/profile ./internal/app/user -count=1`

Expected: PASS.

- [ ] **Step 7: Commit task**

Run:

```bash
git add server/internal/domain/profile server/internal/app/user/router.go
git commit -m "feat(server): 添加我的页面档案摘要接口"
```

---

### Task 2: Backend Profile Update APIs

**Files:**
- Modify: `server/internal/domain/profile/model.go`
- Modify: `server/internal/domain/profile/service.go`
- Modify: `server/internal/domain/profile/repo.go`
- Modify: `server/internal/domain/profile/handler.go`
- Modify: `server/internal/domain/profile/routes.go`
- Modify: `server/internal/domain/profile/routes_test.go`

- [ ] **Step 1: Write failing tests for profile and preferences update**

Extend `routes_test.go` with:

```go
func TestPatchProfileUpdatesExplicitProfileFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{summary: profile.Summary{User: profile.UserSummary{UserPublicID: "usr_test"}}}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile", strings.NewReader(`{"nickname":"明明","gender":"female","height_cm":165,"body_notes":"希望通勤更利落","lifestyle_scenarios":["通勤"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.lastProfileInput.Nickname != "明明" || repo.lastProfileInput.BodyNotes != "希望通勤更利落" {
		t.Fatalf("expected captured profile input, got %#v", repo.lastProfileInput)
	}
}

func TestPatchPreferencesUpdatesExplicitPrefs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &routeProfileRepo{summary: profile.Summary{User: profile.UserSummary{UserPublicID: "usr_test"}}}
	router := newProfileRouteTestRouter(repo)
	request := httptest.NewRequest(http.MethodPatch, "/api/user/profile/preferences", strings.NewReader(`{"style_goals":["更利落"],"avoidances":["过甜"],"scenario_preferences":["通勤"]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(repo.lastPreferencesInput.StyleGoals) != 1 || repo.lastPreferencesInput.StyleGoals[0] != "更利落" {
		t.Fatalf("expected captured preferences, got %#v", repo.lastPreferencesInput)
	}
}
```

- [ ] **Step 2: Run tests and confirm failure**

Run: `cd server && go test ./internal/domain/profile -run 'TestPatch' -count=1`

Expected: FAIL because update request models, service methods and routes do not exist.

- [ ] **Step 3: Add request models and validation**

Add request/input structs:

```go
type UpdateProfileRequest struct {
	Nickname           string   `json:"nickname"`
	Gender             string   `json:"gender"`
	HeightCM           *int     `json:"height_cm"`
	BodyNotes          string   `json:"body_notes"`
	SkinNotes          string   `json:"skin_notes"`
	HairNotes          string   `json:"hair_notes"`
	LifestyleScenarios []string `json:"lifestyle_scenarios"`
}

type UpdatePreferencesRequest struct {
	StyleGoals          []string `json:"style_goals"`
	Avoidances          []string `json:"avoidances"`
	ScenarioPreferences []string `json:"scenario_preferences"`
}
```

Add validation constants:

```go
const maxProfileTextLength = 220
const maxScenarioCount = 8
const maxScenarioLength = 40
const maxPreferenceCount = 12
const maxPreferenceLength = 60
```

Return `ValidationError` for too-long text, too many scenarios or too many preferences.

- [ ] **Step 4: Implement service and repository writes**

Add repository methods:

```go
UpdateExplicitProfile(ctx context.Context, userID int64, input UpdateProfileInput) (Summary, error)
UpdateExplicitPreferences(ctx context.Context, userID int64, input UpdatePreferencesInput) (Summary, error)
```

Implementation requirements:

- Update `users.nickname` when nickname is provided.
- Upsert `profiles` for the user with `id.NewPublicID("prf")` if absent.
- Replace `profile_facts` with explicit profile facts.
- Replace `profile_prefs` for style goals, avoidances and scenario preferences with source `user`.
- Return refreshed `Summary`.

- [ ] **Step 5: Add handlers and routes**

Add:

```go
group.PATCH("", handler.UpdateProfile)
group.PATCH("/preferences", handler.UpdatePreferences)
```

Handlers bind JSON, call service, return `400 profile.validation_failed` for validation errors, otherwise `response.OK`.

- [ ] **Step 6: Run tests**

Run: `cd server && go test ./internal/domain/profile ./internal/app/user -count=1`

Expected: PASS.

- [ ] **Step 7: Commit task**

Run:

```bash
git add server/internal/domain/profile
git commit -m "feat(server): 支持编辑形象档案和偏好"
```

---

### Task 3: Miniapp Profile API Client and Dashboard

**Files:**
- Modify: `miniapp/utils/api.js`
- Modify: `miniapp/scripts/verify-api-client.js`
- Modify: `miniapp/pages/profile/profile.js`
- Modify: `miniapp/pages/profile/profile.wxml`
- Modify: `miniapp/pages/profile/profile.wxss`
- Create: `miniapp/scripts/verify-profile-page.js`
- Modify: `miniapp/package.json`

- [ ] **Step 1: Write failing API client checks**

Extend `miniapp/scripts/verify-api-client.js` export list with:

```js
"getProfileSummary",
"updateProfile",
"updateProfilePreferences"
```

Add request assertions:

```js
const profileCalls = [];
await withGlobals({
  getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
  wx: {
    getStorageSync() {
      return "profile_token";
    },
    request(options) {
      profileCalls.push(options);
      options.success({ statusCode: 200, data: { code: "ok", data: { ok: true } } });
    }
  }
}, async () => {
  await api.getProfileSummary();
  await api.updateProfile({ nickname: "明明" });
  await api.updateProfilePreferences({ style_goals: ["更利落"] });
});

const profilePaths = profileCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
assert(profileCalls[0].method === "GET", "getProfileSummary should use GET");
assert(profilePaths[0] === "/api/user/profile/summary", `profile summary path mismatch: ${profilePaths[0]}`);
assert(profileCalls[1].method === "PATCH", "updateProfile should use PATCH");
assert(profilePaths[1] === "/api/user/profile", `update profile path mismatch: ${profilePaths[1]}`);
assert(profileCalls[2].method === "PATCH", "updateProfilePreferences should use PATCH");
assert(profilePaths[2] === "/api/user/profile/preferences", `update preferences path mismatch: ${profilePaths[2]}`);
```

- [ ] **Step 2: Write failing profile page verification**

Create `miniapp/scripts/verify-profile-page.js` that:

- Stubs `Page`.
- Stubs `../../utils/api` with `getProfileSummary`, `updateProfile`, `updateProfilePreferences`.
- Requires `pages/profile/profile.js`.
- Calls `loadProfile`.
- Asserts dashboard fields, four quick entries and edit save methods exist.

Use this core assertion:

```js
assert(pageConfig, "profile.js should register Page config");
assert(typeof pageConfig.loadProfile === "function", "profile page should define loadProfile");
assert(typeof pageConfig.handleSaveProfile === "function", "profile page should define handleSaveProfile");
assert(typeof pageConfig.handleSavePreferences === "function", "profile page should define handleSavePreferences");
```

- [ ] **Step 3: Run miniapp checks and confirm failure**

Run:

```bash
cd miniapp && node scripts/verify-api-client.js
cd miniapp && node scripts/verify-profile-page.js
```

Expected: FAIL because the profile API functions and page methods are missing.

- [ ] **Step 4: Implement API client methods**

Add to `miniapp/utils/api.js`:

```js
function getProfileSummary() {
  return authorizedRequest({ path: "/api/user/profile/summary" });
}

function updateProfile(data) {
  return authorizedRequest({ path: "/api/user/profile", method: "PATCH", data });
}

function updateProfilePreferences(data) {
  return authorizedRequest({ path: "/api/user/profile/preferences", method: "PATCH", data });
}
```

Export all three functions.

- [ ] **Step 5: Implement profile page state and handlers**

In `miniapp/pages/profile/profile.js`:

- Replace dev-session-only loading with `api.getProfileSummary`.
- Add `normalizeProfileSummary(summary)` export.
- Add editable `profileDraft` and `preferencesDraft`.
- Add `handleSaveProfile`, `handleSavePreferences`, `handleClearLocalSession`.
- Navigate report entry with `wx.navigateTo({ url: "/pages/report/report" })`.
- Navigate onboarding entry with `wx.navigateTo({ url: "/pages/onboarding/onboarding" })`.

- [ ] **Step 6: Implement WXML/WXSS dashboard**

In `profile.wxml`, render:

- top identity block.
- memory summary card.
- quick entries.
- profile edit panel.
- preferences edit panel.
- privacy actions with disabled/server-pending copy for destructive server deletes.

In `profile.wxss`, use existing colors `#245d4f`, `#fffdf8`, `#e6ded0`, `#71695d`, avoid nested cards.

- [ ] **Step 7: Run miniapp checks**

Run:

```bash
cd miniapp && npm run verify:api-client
cd miniapp && node scripts/verify-profile-page.js
```

Expected: PASS.

- [ ] **Step 8: Commit task**

Run:

```bash
git add miniapp/utils/api.js miniapp/scripts/verify-api-client.js miniapp/scripts/verify-profile-page.js miniapp/pages/profile miniapp/package.json
git commit -m "feat(miniapp): 完善我的页面档案仪表盘"
```

---

### Task 4: Integration Verification and Final Polish

**Files:**
- Modify only files touched by Tasks 1-3 if verification reveals issues.

- [ ] **Step 1: Run full focused verification**

Run:

```bash
cd server && go test ./...
cd miniapp && npm run verify:api-client
cd miniapp && npm run verify:report-page
cd miniapp && npm run verify:today-ui
cd miniapp && node scripts/verify-profile-page.js
```

Expected: all pass.

- [ ] **Step 2: Inspect git status**

Run: `git status --short`

Expected: only intentional files are modified, or clean if all task commits succeeded.

- [ ] **Step 3: Fix any verification failures with TDD**

For each failure:

1. Add or adjust the smallest failing test that captures the behavior.
2. Run the test and confirm the expected failure.
3. Implement the minimal fix.
4. Re-run the focused test.
5. Re-run the full focused verification command list.

- [ ] **Step 4: Commit verification fixes if needed**

Run:

```bash
git add -u
git commit -m "fix: 收口我的页面集成验证"
```

Skip this commit only if there are no changes after verification.
