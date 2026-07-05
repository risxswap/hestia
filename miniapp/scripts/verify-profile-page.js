const path = require("path");

const root = path.resolve(__dirname, "..");
const apiPath = path.join(root, "utils/api.js");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function loadPage(relativePath, apiStub) {
  const pagePath = path.join(root, relativePath);
  const originalPage = global.Page;
  let pageConfig = null;

  delete require.cache[require.resolve(apiPath)];
  delete require.cache[require.resolve(pagePath)];
  require.cache[require.resolve(apiPath)] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: apiStub
  };

  global.Page = (config) => {
    pageConfig = config;
  };

  const exported = require(pagePath);

  if (typeof originalPage === "undefined") {
    delete global.Page;
  } else {
    global.Page = originalPage;
  }

  return { pageConfig, exported };
}

function createPageInstance(pageConfig) {
  return Object.assign({}, pageConfig, {
    data: clone(pageConfig.data || {}),
    setData(patch) {
      this.data = Object.assign({}, this.data, patch);
    }
  });
}

const initialSummary = {
  user: {
    user_public_id: "usr_test",
    nickname: "明明",
    onboarding_status: "completed"
  },
  profile: {
    gender: "female",
    height_cm: 165,
    body_notes: "想让通勤更利落",
    skin_notes: "偏好低饱和色",
    hair_notes: "希望好打理",
    lifestyle_scenarios: ["通勤", "周末见朋友"],
    style_goal_summary: "更利落"
  },
  preferences: {
    style_goals: ["更利落"],
    avoidances: ["过甜"],
    scenario_preferences: ["通勤更正式"]
  },
  memory_summary: {
    fact_count: 4,
    preference_count: 2,
    avoidance_count: 1,
    inference_count: 3,
    pending_confirmation_count: 1
  },
  quick_entries: [
    { key: "profile", title: "我的档案", summary: "基础信息与场景" },
    { key: "preferences", title: "偏好与禁忌", summary: "风格目标与不想要的方向" },
    { key: "report", title: "报告与路线", summary: "初版报告已生成" },
    { key: "privacy", title: "隐私与数据", summary: "本地登录与删除入口" }
  ]
};

async function main() {
  const apiCalls = [];
  const apiStub = {
    getProfileSummary() {
      apiCalls.push({ name: "getProfileSummary" });
      return Promise.resolve(clone(initialSummary));
    },
    updateProfile(data) {
      apiCalls.push({ name: "updateProfile", data });
      return Promise.resolve(clone(initialSummary));
    },
    updateProfilePreferences(data) {
      apiCalls.push({ name: "updateProfilePreferences", data });
      return Promise.resolve(clone(initialSummary));
    }
  };

  const profile = loadPage("pages/profile/index.js", apiStub);
  assert(profile.pageConfig, "profile/index.js should register Page config");
  assert(typeof profile.pageConfig.loadProfile === "function", "profile index should load profile summary");
  assert(typeof profile.pageConfig.handleQuickEntry === "function", "profile index should handle quick entries");
  assert(typeof profile.exported.normalizeProfileSummary === "function", "profile index should export normalizeProfileSummary");

  const normalizedEmpty = profile.exported.normalizeProfileSummary(null);
  assert(normalizedEmpty.quickEntries.map((entry) => entry.key).join(",") === "profile,preferences,memory,report,privacy", "profile fallback entries should include independent memory domain");

  const profilePage = createPageInstance(profile.pageConfig);
  await profilePage.loadProfile.call(profilePage);
  assert(profilePage.data.user.nickname === "明明", "profile index should hydrate user nickname");
  assert(profilePage.data.profileDraft.scenarioText === "通勤、周末见朋友", "profile index should hydrate scenario summary");
  assert(profilePage.data.preferencesDraft.avoidancesText === "过甜", "profile index should hydrate avoidance summary");
  assert(profilePage.data.quickEntries.map((entry) => entry.key).join(",") === "profile,preferences,memory,report,privacy", "profile entries should keep memory before privacy");

  const profileMarkup = require("fs").readFileSync(path.join(root, "pages/profile/index.wxml"), "utf8");
  const profileStyles = require("fs").readFileSync(path.join(root, "pages/profile/index.wxss"), "utf8");
  assert(!profileMarkup.includes("我的形象档案"), "profile index should not show top identity title");
  assert(!profileMarkup.includes("用户："), "profile index should not show user id copy");
  assert(!profileMarkup.includes("本地开发用户"), "profile index should not show local user fallback copy");
  assert(!profileMarkup.includes("onboarding"), "profile index should not show onboarding status copy");
  assert(!profileMarkup.includes("补充档案"), "profile index should not show supplemental profile action");
  assert(!profileMarkup.includes("handleStartOnboarding"), "profile index should not bind onboarding action");
  assert(!profileMarkup.includes("档案摘要"), "profile index should not duplicate profile summary");
  assert(!profileMarkup.includes("长期记忆摘要"), "profile index should not duplicate memory summary");
  assert(!Object.prototype.hasOwnProperty.call(profilePage.data, "memoryItems"), "profile index should not prepare removed memory summary data");
  assert(profileMarkup.includes('class="quick-list"'), "profile index should render quick entries as a single-column list");
  assert(!profileMarkup.includes('class="quick-grid"'), "profile index should no longer render quick entries as a grid");
  assert(profileStyles.includes(".quick-list"), "profile index styles should define single-column quick list");
  assert(!profileStyles.includes(".quick-grid"), "profile index styles should remove quick grid layout");
  assert(!profileStyles.includes(".memory-grid"), "profile index styles should remove memory summary grid");

  const navigations = [];
  const originalWx = global.wx;
  global.wx = {
    navigateTo(options) {
      navigations.push(options.url);
    }
  };
  profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "profile" } } });
  profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "preferences" } } });
  profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "memory" } } });
  profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "privacy" } } });
  global.wx = originalWx;
  assert(navigations.join(",") === "/pages/profile/edit,/pages/preferences/edit,/pages/memory/index,/pages/privacy/index", `profile quick entry routes mismatch: ${navigations.join(",")}`);

  const profileEdit = loadPage("pages/profile/edit.js", apiStub);
  assert(profileEdit.pageConfig, "profile/edit.js should register Page config");
  assert(typeof profileEdit.exported.payloadFromDraft === "function", "profile edit should export payloadFromDraft");
  const profileEditPage = createPageInstance(profileEdit.pageConfig);
  await profileEditPage.loadProfile.call(profileEditPage);
  profileEditPage.setData({
    draft: Object.assign({}, profileEditPage.data.draft, {
      nickname: "新的昵称",
      scenarioText: "通勤，周末"
    })
  });
  await profileEditPage.handleSave.call(profileEditPage);
  const profileSave = apiCalls.find((call) => call.name === "updateProfile");
  assert(profileSave, "profile edit should call updateProfile");
  assert(profileSave.data.nickname === "新的昵称", "profile edit should pass nickname");
  assert(profileSave.data.lifestyle_scenarios.length === 2, "profile edit should split scenarios");

  const preferencesEdit = loadPage("pages/preferences/edit.js", apiStub);
  assert(preferencesEdit.pageConfig, "preferences/edit.js should register Page config");
  assert(typeof preferencesEdit.exported.payloadFromDraft === "function", "preferences edit should export payloadFromDraft");
  const preferencesPage = createPageInstance(preferencesEdit.pageConfig);
  await preferencesPage.loadPreferences.call(preferencesPage);
  preferencesPage.setData({
    draft: {
      styleGoalsText: "更利落、轻松",
      avoidancesText: "过甜 / 太紧身",
      scenarioPreferencesText: "通勤，周末"
    }
  });
  await preferencesPage.handleSave.call(preferencesPage);
  const preferencesSave = apiCalls.find((call) => call.name === "updateProfilePreferences");
  assert(preferencesSave, "preferences edit should call updateProfilePreferences");
  assert(preferencesSave.data.style_goals.length === 2, "preferences edit should split style goals");
  assert(preferencesSave.data.avoidances.length === 2, "preferences edit should split avoidances");
  assert(preferencesSave.data.scenario_preferences.length === 2, "preferences edit should split scenario preferences");

  console.log("profile page verification passed");
}

main().catch((error) => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
