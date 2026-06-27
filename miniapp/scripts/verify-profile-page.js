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

function createDeferred() {
  let resolve;
  let reject;
  const promise = new Promise((promiseResolve, promiseReject) => {
    resolve = promiseResolve;
    reject = promiseReject;
  });
  return {
    promise,
    resolve,
    reject
  };
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
  let failNextProfileUpdate = false;
  let failNextPreferencesUpdate = false;
  let deferredProfileUpdate = null;
  let deferredPreferencesUpdate = null;
  const apiStub = {
    getProfileSummary() {
      apiCalls.push({ name: "getProfileSummary" });
      return Promise.resolve(clone(initialSummary));
    },
    updateProfile(data) {
      apiCalls.push({ name: "updateProfile", data });
      if (failNextProfileUpdate) {
        failNextProfileUpdate = false;
        return Promise.reject(new Error("基础档案保存失败"));
      }
      if (deferredProfileUpdate) {
        const deferred = deferredProfileUpdate;
        deferredProfileUpdate = null;
        return deferred.promise;
      }
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
      if (failNextPreferencesUpdate) {
        failNextPreferencesUpdate = false;
        return Promise.reject(new Error("偏好保存失败"));
      }
      if (deferredPreferencesUpdate) {
        const deferred = deferredPreferencesUpdate;
        deferredPreferencesUpdate = null;
        return deferred.promise;
      }
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
  assert(page.data.loadErrorMessage === "", "loadProfile should clear loadErrorMessage");
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

  page.setData({
    profileDraft: Object.assign({}, page.data.profileDraft, {
      nickname: "失败后保留",
      body_notes: "保存失败时不要清掉"
    }),
    preferencesDraft: Object.assign({}, page.data.preferencesDraft, {
      avoidancesText: "失败时保留禁忌"
    })
  });
  failNextProfileUpdate = true;
  await page.handleSaveProfile.call(page);
  assert(page.data.loadErrorMessage === "", "profile save failure should not set full-page load error");
  assert(page.data.profileSaveMessage === "基础档案保存失败", "profile save failure should show panel message");
  assert(page.data.profileDraft.nickname === "失败后保留", "profile save failure should keep profile draft nickname");
  assert(page.data.profileDraft.body_notes === "保存失败时不要清掉", "profile save failure should keep profile draft notes");
  assert(page.data.preferencesDraft.avoidancesText === "失败时保留禁忌", "profile save failure should keep other panel draft");
  assert(page.data.user.user_public_id === "usr_test", "profile save failure should keep loaded dashboard visible");

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
    }),
    preferencesDraft: Object.assign({}, page.data.preferencesDraft, {
      styleGoalsText: "未保存目标",
      avoidancesText: "未保存禁忌",
      scenarioPreferencesText: "未保存场景"
    })
  });
  await page.handleSaveProfile.call(page);

  const profileSave = apiCalls.find((call) => call.name === "updateProfile" && call.data.nickname === "小明");
  assert(profileSave, "handleSaveProfile should call updateProfile");
  assert(profileSave.data.nickname === "小明", "handleSaveProfile should send nickname");
  assert(profileSave.data.body_notes === "保持自然利落", "handleSaveProfile should send body_notes");
  assert(profileSave.data.lifestyle_scenarios.length === 3, "handleSaveProfile should split scenario text");
  assert(page.data.savingProfile === false, "handleSaveProfile should clear savingProfile");
  assert(page.data.user.nickname === "小明", "handleSaveProfile should apply returned summary");
  assert(page.data.preferencesDraft.styleGoalsText === "未保存目标", "handleSaveProfile should keep unsaved preferences style goals");
  assert(page.data.preferencesDraft.avoidancesText === "未保存禁忌", "handleSaveProfile should keep unsaved preferences avoidances");
  assert(page.data.preferencesDraft.scenarioPreferencesText === "未保存场景", "handleSaveProfile should keep unsaved preferences scenarios");

  const pendingProfileDeferred = createDeferred();
  deferredProfileUpdate = pendingProfileDeferred;
  page.setData({
    profileDraft: Object.assign({}, page.data.profileDraft, {
      nickname: "pending 基础保存",
      body_notes: "pending 基础内容"
    }),
    preferencesDraft: {
      styleGoalsText: "请求前目标",
      avoidancesText: "请求前禁忌",
      scenarioPreferencesText: "请求前场景"
    }
  });
  const pendingProfileSave = page.handleSaveProfile.call(page);
  page.setData({
    preferencesDraft: {
      styleGoalsText: "请求中目标",
      avoidancesText: "请求中禁忌",
      scenarioPreferencesText: "请求中场景"
    }
  });
  pendingProfileDeferred.resolve(Object.assign({}, clone(initialSummary), {
    user: Object.assign({}, initialSummary.user, {
      nickname: "pending 基础保存"
    })
  }));
  await pendingProfileSave;
  assert(page.data.preferencesDraft.styleGoalsText === "请求中目标", "pending profile save should keep preferences edited during request");
  assert(page.data.preferencesDraft.avoidancesText === "请求中禁忌", "pending profile save should keep avoidances edited during request");
  assert(page.data.preferencesDraft.scenarioPreferencesText === "请求中场景", "pending profile save should keep scenarios edited during request");

  page.setData({
    profileDraft: Object.assign({}, page.data.profileDraft, {
      nickname: "未保存昵称",
      body_notes: "未保存基础档案"
    }),
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
  assert(page.data.profileDraft.nickname === "未保存昵称", "handleSavePreferences should keep unsaved profile nickname");
  assert(page.data.profileDraft.body_notes === "未保存基础档案", "handleSavePreferences should keep unsaved profile notes");

  const pendingPreferencesDeferred = createDeferred();
  deferredPreferencesUpdate = pendingPreferencesDeferred;
  page.setData({
    profileDraft: Object.assign({}, page.data.profileDraft, {
      nickname: "请求前昵称",
      body_notes: "请求前基础"
    }),
    preferencesDraft: {
      styleGoalsText: "pending 偏好目标",
      avoidancesText: "pending 偏好禁忌",
      scenarioPreferencesText: "pending 偏好场景"
    }
  });
  const pendingPreferencesSave = page.handleSavePreferences.call(page);
  page.setData({
    profileDraft: Object.assign({}, page.data.profileDraft, {
      nickname: "请求中昵称",
      body_notes: "请求中基础"
    })
  });
  pendingPreferencesDeferred.resolve(Object.assign({}, clone(initialSummary), {
    profile: Object.assign({}, initialSummary.profile, {
      style_goal_summary: "pending 偏好目标"
    })
  }));
  await pendingPreferencesSave;
  assert(page.data.profileDraft.nickname === "请求中昵称", "pending preferences save should keep profile nickname edited during request");
  assert(page.data.profileDraft.body_notes === "请求中基础", "pending preferences save should keep profile notes edited during request");

  page.setData({
    preferencesDraft: {
      styleGoalsText: "失败目标",
      avoidancesText: "失败禁忌",
      scenarioPreferencesText: "失败场景"
    }
  });
  failNextPreferencesUpdate = true;
  await page.handleSavePreferences.call(page);
  assert(page.data.loadErrorMessage === "", "preferences save failure should not set full-page load error");
  assert(page.data.preferencesSaveMessage === "偏好保存失败", "preferences save failure should show panel message");
  assert(page.data.preferencesDraft.styleGoalsText === "失败目标", "preferences save failure should keep style goals draft");
  assert(page.data.preferencesDraft.avoidancesText === "失败禁忌", "preferences save failure should keep avoidances draft");
  assert(page.data.profileDraft.nickname === "请求中昵称", "preferences save failure should keep other panel draft");

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
  assert(page.data.summary === null, "handleClearLocalSession should clear summary");
  assert(page.data.user.user_public_id === "", "handleClearLocalSession should clear user public id");
  assert(page.data.user.nickname === "", "handleClearLocalSession should clear nickname");
  assert(page.data.profile === null, "handleClearLocalSession should clear profile");
  assert(page.data.profileDraft.nickname === "", "handleClearLocalSession should clear profile draft");
  assert(page.data.preferencesDraft.styleGoalsText === "", "handleClearLocalSession should clear preferences draft");
  assert(page.data.memoryItems.every((item) => item.value === 0), "handleClearLocalSession should clear memory counters");
  assert(page.data.quickEntries.length === 0, "handleClearLocalSession should clear quick entries");
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
