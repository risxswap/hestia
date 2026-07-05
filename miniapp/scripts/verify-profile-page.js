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
  assert(profilePage.data.memoryItems.length === 5, "profile index should show memory summary items");

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
