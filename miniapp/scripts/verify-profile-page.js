const path = require("path");

const root = path.resolve(__dirname, "..");
const apiPath = path.join(root, "utils/api.js");
const pagePath = path.join(root, "pages/profile/profile.js");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function requireProfilePageWithApi(apiStub) {
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

  return {
    pageConfig,
    exported
  };
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
    profile_public_id: "prf_test",
    gender: "female",
    height_cm: 165,
    body_notes: "想让通勤更利落",
    skin_notes: "偏好低饱和色",
    hair_notes: "希望好打理",
    lifestyle_scenarios: ["通勤", "周末见朋友"],
    style_goal_summary: "更利落"
  },
  memory_summary: {
    fact_count: 4,
    preference_count: 2,
    avoidance_count: 1,
    inference_count: 3,
    pending_confirmation_count: 1
  },
  latest_report: {
    public_id: "rpt_test",
    title: "初版个人形象报告",
    status: "ready",
    generated_at: "2026-06-27T06:00:00Z"
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
      return Promise.resolve(Object.assign({}, clone(initialSummary), {
        user: Object.assign({}, initialSummary.user, {
          nickname: data.nickname
        }),
        profile: Object.assign({}, initialSummary.profile, {
          body_notes: data.body_notes,
          lifestyle_scenarios: data.lifestyle_scenarios
        })
      }));
    },
    updateProfilePreferences(data) {
      apiCalls.push({ name: "updateProfilePreferences", data });
      return Promise.resolve(Object.assign({}, clone(initialSummary), {
        profile: Object.assign({}, initialSummary.profile, {
          style_goal_summary: data.style_goals.join("、")
        })
      }));
    }
  };

  const { pageConfig, exported } = requireProfilePageWithApi(apiStub);

  assert(pageConfig, "profile.js should register Page config");
  assert(typeof pageConfig.loadProfile === "function", "profile page should define loadProfile");
  assert(typeof pageConfig.handleSaveProfile === "function", "profile page should define handleSaveProfile");
  assert(typeof pageConfig.handleSavePreferences === "function", "profile page should define handleSavePreferences");
  assert(typeof pageConfig.handleClearLocalSession === "function", "profile page should define handleClearLocalSession");
  assert(typeof exported.normalizeProfileSummary === "function", "profile.js should export normalizeProfileSummary");

  const normalizedEmpty = exported.normalizeProfileSummary(null);
  assert(normalizedEmpty.empty === true, "normalizeProfileSummary should mark missing summary as empty");
  assert(normalizedEmpty.quickEntries.length === 4, "normalizeProfileSummary should provide four fallback quick entries");
  assert(
    normalizedEmpty.quickEntries.map((entry) => entry.key).join(",") === "profile,preferences,report,privacy",
    "normalizeProfileSummary fallback quick entries should match profile contract"
  );

  const page = createPageInstance(pageConfig);
  await page.loadProfile.call(page);

  assert(apiCalls[0].name === "getProfileSummary", "loadProfile should call getProfileSummary");
  assert(page.data.loading === false, "loadProfile should clear loading");
  assert(page.data.errorMessage === "", "loadProfile should clear errorMessage");
  assert(page.data.empty === false, "loadProfile should mark summary as non-empty");
  assert(page.data.user.nickname === "明明", "loadProfile should expose user nickname");
  assert(page.data.profileDraft.nickname === "明明", "loadProfile should prepare profile draft nickname");
  assert(page.data.profileDraft.scenarioText === "通勤、周末见朋友", "loadProfile should normalize scenario draft text");
  assert(page.data.preferencesDraft.styleGoalsText === "更利落", "loadProfile should prepare style goals draft");
  assert(page.data.quickEntries.length === 4, "loadProfile should expose four quick entries");
  assert(
    page.data.quickEntries.map((entry) => entry.key).join(",") === "profile,preferences,report,privacy",
    "loadProfile should keep profile quick entry contract"
  );
  assert(page.data.memoryItems.length === 5, "loadProfile should normalize memory summary items");

  const navigations = [];
  const scrolls = [];
  const originalWxForQuickEntries = global.wx;
  global.wx = {
    navigateTo(options) {
      navigations.push(options);
    },
    pageScrollTo(options) {
      scrolls.push(options);
    }
  };
  page.handleQuickEntry.call(page, { currentTarget: { dataset: { key: "profile" } } });
  assert(page.data.activeSection === "profile", "profile quick entry should activate profile section");
  assert(scrolls[0].selector === "#profile-section", "profile quick entry should scroll to profile section");
  page.handleQuickEntry.call(page, { currentTarget: { dataset: { key: "preferences" } } });
  assert(page.data.activeSection === "preferences", "preferences quick entry should activate preferences section");
  assert(scrolls[1].selector === "#preferences-section", "preferences quick entry should scroll to preferences section");
  page.handleQuickEntry.call(page, { currentTarget: { dataset: { key: "privacy" } } });
  assert(page.data.activeSection === "privacy", "privacy quick entry should activate privacy section");
  assert(scrolls[2].selector === "#privacy-section", "privacy quick entry should scroll to privacy section");
  page.handleQuickEntry.call(page, { currentTarget: { dataset: { key: "report" } } });
  assert(navigations[0].url === "/pages/report/report", "report quick entry should navigate to report page");
  global.wx = originalWxForQuickEntries;

  page.setData({
    profileDraft: Object.assign({}, page.data.profileDraft, {
      nickname: "小明",
      body_notes: "保持自然利落",
      scenarioText: "通勤，约会 / 周末"
    })
  });
  await page.handleSaveProfile.call(page);

  const profileSave = apiCalls.find((call) => call.name === "updateProfile");
  assert(profileSave, "handleSaveProfile should call updateProfile");
  assert(profileSave.data.nickname === "小明", "handleSaveProfile should send nickname");
  assert(profileSave.data.body_notes === "保持自然利落", "handleSaveProfile should send body_notes");
  assert(profileSave.data.lifestyle_scenarios.length === 3, "handleSaveProfile should split scenario text");
  assert(page.data.savingProfile === false, "handleSaveProfile should clear savingProfile");
  assert(page.data.user.nickname === "小明", "handleSaveProfile should apply returned summary");

  page.setData({
    preferencesDraft: {
      styleGoalsText: "更利落、轻松",
      avoidancesText: "过甜 / 太紧身",
      scenarioPreferencesText: "通勤，周末"
    }
  });
  await page.handleSavePreferences.call(page);

  const preferencesSave = apiCalls.find((call) => call.name === "updateProfilePreferences");
  assert(preferencesSave, "handleSavePreferences should call updateProfilePreferences");
  assert(preferencesSave.data.style_goals.length === 2, "handleSavePreferences should split style goals");
  assert(preferencesSave.data.avoidances.length === 2, "handleSavePreferences should split avoidances");
  assert(preferencesSave.data.scenario_preferences.length === 2, "handleSavePreferences should split scenario preferences");
  assert(page.data.savingPreferences === false, "handleSavePreferences should clear savingPreferences");
  assert(page.data.preferencesDraft.styleGoalsText === "更利落、轻松", "handleSavePreferences should keep submitted style goals draft");
  assert(page.data.preferencesDraft.avoidancesText === "过甜 / 太紧身", "handleSavePreferences should keep submitted avoidances draft");
  assert(page.data.preferencesDraft.scenarioPreferencesText === "通勤，周末", "handleSavePreferences should keep submitted scenario preferences draft");

  const removedKeys = [];
  const toastCalls = [];
  const originalWx = global.wx;
  global.wx = {
    removeStorageSync(key) {
      removedKeys.push(key);
    },
    showToast(options) {
      toastCalls.push(options);
    }
  };
  page.handleClearLocalSession.call(page);
  global.wx = originalWx;

  assert(removedKeys.includes("user_token"), "handleClearLocalSession should remove user_token");
  assert(removedKeys.includes("token"), "handleClearLocalSession should remove token");
  assert(page.data.tokenReady === false, "handleClearLocalSession should update tokenReady");
  assert(toastCalls[0].title === "已清除本地登录", "handleClearLocalSession should show neutral toast");
}

main()
  .then(() => {
    console.log("profile page verification passed");
  })
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
