# Miniapp First Entry Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the approved semi-forced first-entry onboarding flow for the miniapp using multi-step cards and quick options.

**Architecture:** Keep the existing backend onboarding draft APIs and reshape the miniapp onboarding page into a guided stepper. Add a small launch decision helper so first-entry users land on onboarding unless they explicitly skip, and add consistent “继续建档” entry points from empty report/home/wardrobe/profile states.

**Tech Stack:** Native WeChat Mini Program JavaScript/WXML/WXSS, existing `miniapp/utils/api.js`, Node verification scripts, Go backend tests where existing behavior is touched.

---

## File Structure

- Modify `miniapp/app.js`: run first-entry routing after launch by checking token/session, latest report, onboarding draft, and skip flag.
- Modify `miniapp/pages/onboarding/onboarding.js`: replace large-form state with step-based card flow, quick option toggles, draft save/restore, skip, and submit.
- Modify `miniapp/pages/onboarding/onboarding.wxml`: render welcome, goals, scenarios/avoidances, basic, wardrobe, and submitting cards.
- Modify `miniapp/pages/onboarding/onboarding.wxss`: style stepper, chips, form fields, item list, and actions.
- Modify `miniapp/pages/home/home.wxml`: add continue onboarding button in empty state.
- Modify `miniapp/pages/report/report.wxml`: add continue onboarding button in empty state.
- Modify `miniapp/pages/wardrobe/wardrobe.wxml`: add continue onboarding button in empty state.
- Modify `miniapp/pages/profile/profile.wxml` and `miniapp/pages/profile/profile.js`: show onboarding state and continue onboarding.
- Modify `miniapp/scripts/verify-miniapp-api-integration.js`: add behavior checks for first-entry routing and multi-step onboarding.
- Optionally modify `miniapp/scripts/verify-today-ui.js` and `miniapp/scripts/verify-report-page.js`: static assertions for continue onboarding buttons if existing checks fail.

## Task 1: First-Entry Routing Helper

**Files:**
- Modify: `miniapp/app.js`
- Test: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: Write failing verification for app first-entry routing**

Add a test block to `verify-miniapp-api-integration.js` that loads `app.js` with fake `App`, fake `wx`, and fake API module behavior. Assert:

```js
assert(appConfig.onLaunch, "app should define onLaunch");
await appConfig.onLaunch.call(appInstance);
assert(reLaunchUrl === "/pages/onboarding/onboarding", "first user without report should enter onboarding");
```

Use fake API responses:

```js
ensureDevSession: async () => ({ token: "dev_token", onboarding_status: "not_started" }),
getLatestReport: async () => null,
getOnboardingDraft: async () => ({ status: "not_started", current_step: "", draft_data: {} })
```

- [ ] **Step 2: Run red test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: FAIL because `app.js` does not export/testably register first-entry routing.

- [ ] **Step 3: Implement app launch routing**

In `miniapp/app.js`, require `./utils/api`, set `globalData.apiBaseUrl`, and implement:

```js
async function routeFirstEntry() {
  if (wx.getStorageSync("onboarding_skip")) return;
  await api.ensureDevSession();
  const report = await api.getLatestReport();
  if (report) return;
  const draft = await api.getOnboardingDraft();
  if (!draft || draft.status !== "submitted") {
    wx.reLaunch({ url: "/pages/onboarding/onboarding" });
  }
}
```

Call from `onLaunch`, but swallow errors so app startup is not blocked by network failure.

- [ ] **Step 4: Run green test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: PASS for new first-entry routing assertion.

## Task 2: Onboarding Step State and Payload Builders

**Files:**
- Modify: `miniapp/pages/onboarding/onboarding.js`
- Test: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: Write failing verification for step flow helpers**

Add assertions that onboarding exports or Page config supports:

```js
assert(typeof onboarding.mod.buildDraftPayload === "function");
assert(typeof onboarding.mod.toggleListValue === "function");
assert(typeof onboarding.config.handleNext === "function");
assert(typeof onboarding.config.handleSkip === "function");
```

Assert payload mapping:

```js
const payload = onboarding.mod.buildDraftPayload({
  selectedGoals: ["通勤更有气质"],
  customGoal: "减少穿搭纠结",
  selectedScenarios: ["工作日通勤"],
  selectedAvoidances: ["不要太甜美"],
  basic: { height_cm: "168", hair_notes: "及肩发" },
  wardrobeItems: [{ name: "米白衬衫", category: "top", color: "米白" }]
});
assert(payload.style_goal.goals.length === 2);
assert(payload.wardrobe.items[0].name === "米白衬衫");
```

- [ ] **Step 2: Run red test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: FAIL because current onboarding page does not have the new step helpers.

- [ ] **Step 3: Implement step state and payload builders**

Replace `initialData` with:

```js
currentStep: "welcome",
stepOrder: ["welcome", "goals", "scenarios", "basic", "wardrobe", "submit"],
selectedGoals: [],
customGoal: "",
selectedScenarios: [],
selectedAvoidances: [],
basic: { height_cm: "", body_notes: "", skin_notes: "", hair_notes: "", gender: "" },
wardrobeItems: [],
wardrobeDraft: { name: "", category: "top", color: "" }
```

Implement `toggleListValue(list, value)`, `buildDraftPayload(data)`, `draftToPageData(draftData)`, and `stepFromDraft(draft)`.

- [ ] **Step 4: Run green test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: PASS for helper assertions.

## Task 3: Onboarding Interactions and Submit

**Files:**
- Modify: `miniapp/pages/onboarding/onboarding.js`
- Test: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: Write failing interaction verification**

Extend the onboarding page instance test:

```js
await onboarding.config.handleGoalToggle.call(instance, { currentTarget: { dataset: { value: "通勤更有气质" } } });
await onboarding.config.handleNext.call(instance);
assert(onboardingCalls[0][0] === "save");
assert(instance.data.currentStep === "scenarios");
```

Also verify skip:

```js
onboarding.config.handleSkip.call(instance);
assert(stored.onboarding_skip === true);
assert(switchedTab === "/pages/home/home");
```

- [ ] **Step 2: Run red test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: FAIL because handlers are missing.

- [ ] **Step 3: Implement handlers**

Add handlers:

- `handleStart`
- `handleSkip`
- `handleGoalToggle`
- `handleScenarioToggle`
- `handleAvoidanceToggle`
- `handleCustomInput`
- `handleBasicInput`
- `handleWardrobeDraftInput`
- `handleWardrobeCategory`
- `handleAddWardrobeItem`
- `handleRemoveWardrobeItem`
- `handleBack`
- `handleNext`
- `handleSubmit`

Make `handleNext` save draft for the current semantic step before moving forward. Make `handleSubmit` validate at least one goal and one wardrobe item, save final draft, submit, clear `onboarding_skip`, and navigate to report.

- [ ] **Step 4: Run green test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: PASS.

## Task 4: Onboarding WXML/WXSS Cards

**Files:**
- Modify: `miniapp/pages/onboarding/onboarding.wxml`
- Modify: `miniapp/pages/onboarding/onboarding.wxss`
- Test: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: Write failing static verification**

Add static assertions:

```js
const onboardingWxml = read("pages/onboarding/onboarding.wxml");
assert(onboardingWxml.includes("currentStep === 'welcome'"));
assert(onboardingWxml.includes("handleGoalToggle"));
assert(onboardingWxml.includes("handleScenarioToggle"));
assert(onboardingWxml.includes("handleAddWardrobeItem"));
assert(onboardingWxml.includes("handleSubmit"));
```

- [ ] **Step 2: Run red test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: FAIL because WXML still renders old large form.

- [ ] **Step 3: Implement card UI**

Render one card per `currentStep`:

- `welcome`: intro, “开始建档”, “先逛逛”
- `goals`: goal chips and custom input
- `scenarios`: scenario chips and avoidance chips
- `basic`: optional inputs
- `wardrobe`: item draft inputs, category chips, added item list
- `submit`: generating/report submission state

Use existing palette and stable button sizes.

- [ ] **Step 4: Run green test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: PASS.

## Task 5: Continue-Onboarding Entrypoints

**Files:**
- Modify: `miniapp/pages/home/home.wxml`
- Modify: `miniapp/pages/home/home.js`
- Modify: `miniapp/pages/report/report.wxml`
- Modify: `miniapp/pages/report/report.js`
- Modify: `miniapp/pages/wardrobe/wardrobe.wxml`
- Modify: `miniapp/pages/wardrobe/wardrobe.js`
- Modify: `miniapp/pages/profile/profile.wxml`
- Modify: `miniapp/pages/profile/profile.js`
- Test: `miniapp/scripts/verify-miniapp-api-integration.js`

- [ ] **Step 1: Write failing verification**

Assert all empty-state pages include `handleContinueOnboarding` and route to `/pages/onboarding/onboarding`:

```js
assert(read("pages/home/home.wxml").includes("handleContinueOnboarding"));
assert(read("pages/report/report.wxml").includes("handleContinueOnboarding"));
assert(read("pages/wardrobe/wardrobe.wxml").includes("handleContinueOnboarding"));
assert(read("pages/profile/profile.wxml").includes("handleContinueOnboarding"));
```

- [ ] **Step 2: Run red test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: FAIL.

- [ ] **Step 3: Implement handlers and buttons**

Add `handleContinueOnboarding` to each page:

```js
if (wx.setStorageSync) wx.setStorageSync("onboarding_skip", false);
wx.navigateTo({ url: "/pages/onboarding/onboarding" });
```

Use `wx.switchTab` only if onboarding becomes a tab; currently it is not a tab, so use `navigateTo`.

- [ ] **Step 4: Run green test**

Run: `cd miniapp && npm run verify:api-integration`

Expected: PASS.

## Task 6: Full Verification

**Files:**
- No production changes expected.

- [ ] **Step 1: Run miniapp verification**

Run:

```bash
cd miniapp
npm run verify:api-client
npm run verify:api-integration
npm run verify:report-page
npm run verify:today-ui
npm run verify:tdesign-icon-font
```

Expected: all commands exit 0.

- [ ] **Step 2: Run server verification**

Run:

```bash
cd server
go test ./...
```

Expected: all packages pass.

- [ ] **Step 3: Run web build**

Run:

```bash
cd web
npm run build
```

Expected: build exits 0.

## Self-Review

- Spec coverage: first-entry semi-forced routing, skip behavior, multi-step card flow, minimum completion, empty-state entry points, latest-report null behavior, and verification are covered.
- Placeholder scan: no TODO/TBD placeholders remain.
- Type consistency: plan uses existing API methods and existing draft shape: `basic`, `style_goal`, `wardrobe`.
