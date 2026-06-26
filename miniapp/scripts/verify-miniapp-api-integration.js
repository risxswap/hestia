const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const apiPath = path.join(root, "utils/api.js");

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function loadPage(relativePath, fakeApi) {
  const pagePath = path.join(root, relativePath);
  const originalPage = global.Page;
  let config;

  delete require.cache[require.resolve(pagePath)];
  require.cache[require.resolve(apiPath)] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: fakeApi || {}
  };

  global.Page = (pageConfig) => {
    config = pageConfig;
  };

  try {
    const mod = require(pagePath);
    return {
      config,
      mod
    };
  } finally {
    if (typeof originalPage === "undefined") {
      delete global.Page;
    } else {
      global.Page = originalPage;
    }
    delete require.cache[require.resolve(pagePath)];
    delete require.cache[require.resolve(apiPath)];
  }
}

async function verifyFirstEntryAppRouting() {
  const jumpCase = await runAppLaunchCase({
    storage: {},
    api: {
      ensureDevSession: async () => ({ token: "dev_token", onboarding_status: "not_started" }),
      getLatestReport: async () => null,
      getOnboardingDraft: async () => ({ status: "not_started", current_step: "", draft_data: {} })
    }
  });
  assert(jumpCase.appConfig && typeof jumpCase.appConfig.onLaunch === "function", "app should define onLaunch");
  assert(jumpCase.appInstance.globalData.apiBaseUrl === "http://127.0.0.1:8080", "app should keep apiBaseUrl");
  assert(jumpCase.appInstance.globalData.privacyVersion === "2026-06-03", "app should keep privacyVersion");
  assert(jumpCase.reLaunchUrl === "/pages/onboarding/onboarding", "first user without report should enter onboarding");

  const skippedCase = await runAppLaunchCase({
    storage: { onboarding_skip: true },
    api: {
      ensureDevSession: async () => {
        throw new Error("ensureDevSession should not run after skip");
      }
    }
  });
  assert(skippedCase.reLaunchUrl === "", "skip flag should prevent onboarding relaunch");

  const reportCase = await runAppLaunchCase({
    storage: {},
    api: {
      ensureDevSession: async () => ({ token: "dev_token" }),
      getLatestReport: async () => ({ public_id: "rpt_test" }),
      getOnboardingDraft: async () => {
        throw new Error("draft should not be loaded when report exists");
      }
    }
  });
  assert(reportCase.reLaunchUrl === "", "existing report should prevent onboarding relaunch");

  const completedSessionCase = await runAppLaunchCase({
    storage: {},
    api: {
      ensureDevSession: async () => ({ token: "dev_token", onboarding_status: "completed" }),
      getLatestReport: async () => {
        throw new Error("latest report should not be required after completed onboarding session");
      },
      getOnboardingDraft: async () => ({ status: "submitted", current_step: "wardrobe", draft_data: {} })
    }
  });
  assert(completedSessionCase.reLaunchUrl === "", "completed onboarding session should prevent onboarding relaunch");

  const errorCase = await runAppLaunchCase({
    storage: {},
    api: {
      ensureDevSession: async () => {
        throw new Error("network failed");
      }
    }
  });
  assert(errorCase.reLaunchUrl === "", "launch errors should be swallowed without relaunch");
}

async function runAppLaunchCase(options) {
  const appPath = path.join(root, "app.js");
  const appCacheKey = require.resolve(appPath);
  const apiCacheKey = require.resolve(apiPath);
  const originalApp = global.App;
  const originalWx = global.wx;
  const originalGetApp = global.getApp;
  const originalAppCache = require.cache[appCacheKey];
  const originalApiCache = require.cache[apiCacheKey];
  let appConfig;
  let appInstance;
  let reLaunchUrl = "";

  delete require.cache[appCacheKey];
  require.cache[apiCacheKey] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: options.api || {}
  };

  global.wx = {
    getStorageSync: (key) => (options.storage || {})[key] || "",
    reLaunch(options) {
      reLaunchUrl = options && options.url ? options.url : "";
    }
  };
  global.App = (config) => {
    appConfig = config;
    appInstance = {
      globalData: config.globalData
    };
  };
  global.getApp = () => appInstance;

  try {
    require(appPath);
    if (appConfig && typeof appConfig.onLaunch === "function") {
      await appConfig.onLaunch.call(appInstance);
    }
    return {
      appConfig,
      appInstance,
      reLaunchUrl
    };
  } finally {
    if (typeof originalApp === "undefined") {
      delete global.App;
    } else {
      global.App = originalApp;
    }
    if (typeof originalWx === "undefined") {
      delete global.wx;
    } else {
      global.wx = originalWx;
    }
    if (typeof originalGetApp === "undefined") {
      delete global.getApp;
    } else {
      global.getApp = originalGetApp;
    }
    if (originalAppCache) {
      require.cache[appCacheKey] = originalAppCache;
    } else {
      delete require.cache[appCacheKey];
    }
    if (originalApiCache) {
      require.cache[apiCacheKey] = originalApiCache;
    } else {
      delete require.cache[apiCacheKey];
    }
  }
}

function createPageInstance(config) {
  return Object.assign({}, config, {
    data: JSON.parse(JSON.stringify(config.data || {})),
    setData(payload, callback) {
      this.data = Object.assign({}, this.data, payload);
      if (callback) {
        callback();
      }
    },
    selectComponent() {
      return {
        scrollToBottom() {}
      };
    }
  });
}

function runNavigateCase(action) {
  const originalWx = global.wx;
  let navigateUrl = "";
  global.wx = {
    navigateTo(options) {
      navigateUrl = options && options.url ? options.url : "";
    }
  };

  try {
    action();
    return navigateUrl;
  } finally {
    if (typeof originalWx === "undefined") {
      delete global.wx;
    } else {
      global.wx = originalWx;
    }
  }
}

async function main() {
  await verifyFirstEntryAppRouting();

  [
    "pages/home/home.js",
    "pages/report/report.js",
    "pages/advisor/advisor.js",
    "pages/onboarding/onboarding.js",
    "pages/wardrobe/wardrobe.js",
    "pages/profile/profile.js"
  ].forEach((file) => {
    const source = read(file);
    assert(!source.includes("utils/mock") && !source.includes("../../utils/mock"), `${file} should not import mock data`);
    assert(source.includes("../../utils/api"), `${file} should import the shared api client`);
  });

  const sampleReport = {
    public_id: "rpt_test",
    title: "初版个人形象报告",
    summary: "先用清爽线条建立稳定的第一印象。",
    content_json: {
      action_items: [
        {
          title: "固定一套通勤模板",
          reason: "减少早晨选择成本。",
          avoid: "避免上下都宽松。"
        }
      ],
      wardrobe_gaps: ["浅色短外套"]
    },
    routes: [
      {
        public_id: "irt_test",
        name: "干净 + 有气质",
        route_role: "primary",
        target_impression: ["干净"],
        reason: ["适合通勤和见客户"]
      }
    ]
  };

  const home = loadPage("pages/home/home.js", {
    getLatestReport: async () => sampleReport,
    sendImageRouteFeedback: async () => ({ public_id: "irt_test", status: "active" })
  });
  assert(home.config, "home.js should register a Page config");
  assert(typeof home.config.loadToday === "function", "home page should define loadToday");
  assert(typeof home.mod.normalizeTodayFromReport === "function", "home.js should export normalizeTodayFromReport");

  const homeInstance = createPageInstance(home.config);
  await home.config.loadToday.call(homeInstance);
  assert(homeInstance.data.todayRecommendation.title === "干净 + 有气质", "home should derive title from latest report route");
  assert(homeInstance.data.routePublicID === "irt_test", "home should keep route public id for feedback");
  await home.config.handlePrimaryAction.call(homeInstance);
  assert(homeInstance.data.memoryToast.includes("已记录"), "home primary feedback should call route feedback");

  const emptyHome = loadPage("pages/home/home.js", {
    getLatestReport: async () => null,
    sendImageRouteFeedback: async () => ({})
  });
  const emptyHomeInstance = createPageInstance(emptyHome.config);
  await emptyHome.config.loadToday.call(emptyHomeInstance);
  assert(!emptyHomeInstance.data.hasReport, "home should show empty state when latest report is null");
  assert(typeof emptyHome.config.handleStartOnboarding === "function", "home empty state should support onboarding entry");
  assert(runNavigateCase(() => emptyHome.config.handleStartOnboarding.call(emptyHomeInstance)) === "/pages/onboarding/onboarding", "home onboarding entry should navigate to onboarding");

  const report = loadPage("pages/report/report.js", {
    getLatestReport: async () => sampleReport
  });
  assert(report.config, "report.js should register a Page config");
  assert(typeof report.config.loadLatestReport === "function", "report page should define loadLatestReport");
  assert(typeof report.mod.normalizeReportResponse === "function", "report.js should export normalizeReportResponse");

  const reportInstance = createPageInstance(report.config);
  await report.config.loadLatestReport.call(reportInstance);
  assert(reportInstance.data.title === "初版个人形象报告", "report should load title from server report");
  assert(reportInstance.data.actionItems[0].title === "固定一套通勤模板", "report should load action items from server report");
  assert(!report.mod.fallbackReportData, "report.js should not export fallback mock report data");

  const emptyReport = loadPage("pages/report/report.js", {
    getLatestReport: async () => null
  });
  const emptyReportInstance = createPageInstance(emptyReport.config);
  await emptyReport.config.loadLatestReport.call(emptyReportInstance);
  assert(emptyReportInstance.data.empty, "report should show empty state when latest report is null");
  assert(typeof emptyReport.config.handleStartOnboarding === "function", "report empty state should support onboarding entry");
  assert(runNavigateCase(() => emptyReport.config.handleStartOnboarding.call(emptyReportInstance)) === "/pages/onboarding/onboarding", "report onboarding entry should navigate to onboarding");

  const onboardingCalls = [];
  const onboarding = loadPage("pages/onboarding/onboarding.js", {
    getOnboardingDraft: async () => ({
      status: "not_started",
      current_step: "",
      version: 0,
      draft_data: {}
    }),
    saveOnboardingDraft: async (step, data) => {
      onboardingCalls.push(["save", step, data]);
      return { status: "draft", current_step: step, version: 1, draft_data: data };
    },
    submitOnboarding: async () => {
      onboardingCalls.push(["submit"]);
      return { report_public_id: "rpt_test", job_public_id: "job_test" };
    }
  });
  assert(onboarding.config, "onboarding.js should register a Page config");
  assert(typeof onboarding.config.loadDraft === "function", "onboarding should load existing draft");
  assert(typeof onboarding.config.handleSubmit === "function", "onboarding should submit through API");
  assert(typeof onboarding.mod.buildDraftPayload === "function", "onboarding should export buildDraftPayload");
  assert(typeof onboarding.mod.toggleListValue === "function", "onboarding should export toggleListValue");
  assert(typeof onboarding.config.handleNext === "function", "onboarding should support step next action");
  assert(typeof onboarding.config.handleSkip === "function", "onboarding should support skip action");

  assert(
    onboarding.mod.stepFromDraft({
      status: "draft",
      current_step: "welcome",
      draft_data: {
        basic: { height_cm: 168 },
        style_goal: {
          goals: ["通勤更有气质"],
          scenarios: ["工作日通勤"],
          avoidances: ["避免过度甜美"]
        },
        wardrobe: {
          items: [{ name: "米白衬衫", category: "top", color: "米白" }]
        }
      }
    }) === "wardrobe",
    "onboarding should infer a useful step when a legacy draft was saved at welcome"
  );

  const onboardingPayload = onboarding.mod.buildDraftPayload({
    selectedGoals: ["通勤更有气质"],
    customGoal: "减少穿搭纠结",
    selectedScenarios: ["工作日通勤"],
    selectedAvoidances: ["不要太甜美"],
    basic: { height_cm: "168", hair_notes: "及肩发" },
    wardrobeItems: [{ name: "米白衬衫", category: "top", color: "米白" }]
  });
  assert(onboardingPayload.style_goal.goals.length === 2, "onboarding should merge selected and custom goals");
  assert(onboardingPayload.style_goal.goals[1] === "减少穿搭纠结", "onboarding should keep custom goal text");
  assert(onboardingPayload.style_goal.scenarios[0] === "工作日通勤", "onboarding should map selected scenarios");
  assert(onboardingPayload.style_goal.avoidances[0] === "不要太甜美", "onboarding should map selected avoidances");
  assert(onboardingPayload.basic.height_cm === 168, "onboarding should convert height string to number");
  assert(onboardingPayload.wardrobe.items[0].name === "米白衬衫", "onboarding should map wardrobe items");
  assert(onboarding.mod.withViewState({}, { currentStep: "submit" }).topbarMeta === "5/5", "onboarding submit step should display final progress");

  const onboardingTemplate = read("pages/onboarding/onboarding.wxml");
  assert(!onboardingTemplate.includes(".indexOf("), "onboarding template should not call indexOf in WXML");
  assert(!onboardingTemplate.includes(".join("), "onboarding template should not call join in WXML");
  assert(!onboardingTemplate.includes(" + "), "onboarding template should not concatenate strings in WXML");
  assert(!onboardingTemplate.includes('wx:key="index"'), "onboarding template should not use index as wx:key");

  const nextCalls = [];
  const nextOnboarding = loadPage("pages/onboarding/onboarding.js", {
    saveOnboardingDraft: async (step, data) => {
      nextCalls.push(["save", step, data]);
      return { status: "draft", current_step: step, version: 1, draft_data: data };
    }
  });
  const nextInstance = createPageInstance(nextOnboarding.config);
  nextInstance.data.currentStep = "goals";
  nextInstance.data.selectedGoals = ["通勤更有气质"];
  await nextOnboarding.config.handleNext.call(nextInstance);
  assert(nextCalls[0][0] === "save", "onboarding next should save the current draft");
  assert(nextCalls[0][1] === "scenarios", "onboarding next should persist the step the user will resume on");
  assert(nextCalls[0][2].style_goal.goals[0] === "通勤更有气质", "onboarding next should save current step payload");
  assert(nextInstance.data.currentStep === "scenarios", "onboarding next should advance to the following step");

  const oldFormCalls = [];
  const oldFormOnboarding = loadPage("pages/onboarding/onboarding.js", {
    saveOnboardingDraft: async (step, data) => {
      oldFormCalls.push(["save", step, data]);
      return { status: "draft", current_step: step, version: 1, draft_data: data };
    }
  });
  const oldFormInstance = createPageInstance(oldFormOnboarding.config);
  oldFormInstance.data.currentStep = "welcome";
  oldFormInstance.data.basic.height_cm = "168";
  oldFormInstance.data.styleGoal.goalsText = "干净利落, 通勤有气质";
  oldFormInstance.data.styleGoal.scenariosText = "工作日通勤";
  oldFormInstance.data.wardrobeText = "米白衬衫, top, 米白";
  await oldFormOnboarding.config.handleSave.call(oldFormInstance);
  assert(oldFormCalls[0][1] === "wardrobe", "legacy full form save should persist the inferred draft step");

  const originalWx = global.wx;
  const skipStorage = {};
  let skipUrl = "";
  global.wx = {
    setStorageSync(key, value) {
      skipStorage[key] = value;
    },
    switchTab(options) {
      skipUrl = options && options.url ? options.url : "";
    }
  };
  try {
    onboarding.config.handleSkip.call(createPageInstance(onboarding.config));
  } finally {
    if (typeof originalWx === "undefined") {
      delete global.wx;
    } else {
      global.wx = originalWx;
    }
  }
  assert(skipStorage.onboarding_skip === true, "onboarding skip should persist the skip flag");
  assert(skipUrl === "/pages/home/home", "onboarding skip should switch to the home tab");

  const onboardingInstance = createPageInstance(onboarding.config);
  onboardingInstance.data.basic.gender = "female";
  onboardingInstance.data.basic.height_cm = "168";
  onboardingInstance.data.styleGoal.goalsText = "干净利落, 通勤有气质";
  onboardingInstance.data.styleGoal.avoidancesText = "过度甜美";
  onboardingInstance.data.styleGoal.scenariosText = "工作日通勤";
  onboardingInstance.data.wardrobeText = "米白衬衫, top, 米白\n直筒牛仔裤, bottom, 蓝色";
  const submitStorage = {};
  const originalSubmitWx = global.wx;
  let reportNavigateUrl = "";
  global.wx = {
    setStorageSync(key, value) {
      submitStorage[key] = value;
    },
    removeStorageSync(key) {
      submitStorage[key] = "";
    },
    navigateTo(options) {
      reportNavigateUrl = options && options.url ? options.url : "";
    }
  };
  try {
    await onboarding.config.handleSubmit.call(onboardingInstance);
  } finally {
    if (typeof originalSubmitWx === "undefined") {
      delete global.wx;
    } else {
      global.wx = originalSubmitWx;
    }
  }
  assert(onboardingCalls[0][0] === "save", "onboarding submit should save draft first");
  assert(onboardingCalls[0][1] === "wardrobe", "onboarding should save final step as wardrobe");
  assert(onboardingCalls[0][2].style_goal.goals.length === 2, "onboarding should parse goals text");
  assert(onboardingCalls[0][2].wardrobe.items.length === 2, "onboarding should parse wardrobe text");
  assert(onboardingCalls[1][0] === "submit", "onboarding submit should call submit API");
  assert(submitStorage.onboarding_status === "completed", "onboarding submit should persist completed status locally");
  assert(reportNavigateUrl === "/pages/report/report?public_id=rpt_test", "onboarding submit should navigate to the generated report");

  const advisor = loadPage("pages/advisor/advisor.js", {
    sendAgentMessage: async () => [
      { event: "status", data: { text: "agent stream ready" } },
      { event: "done", data: {} }
    ]
  });
  assert(advisor.config, "advisor.js should register a Page config");
  assert(typeof advisor.config.sendToAgent === "function", "advisor should send messages through agent API");
  const advisorInstance = createPageInstance(advisor.config);
  await advisor.config.sendToAgent.call(advisorInstance, "今天怎么穿");
  assert(
    advisorInstance.data.messages.some((item) => item.role === "assistant" && item.content.includes("agent stream ready")),
    "advisor should append assistant message from server SSE status"
  );

  const wardrobe = loadPage("pages/wardrobe/wardrobe.js", {
    getLatestReport: async () => sampleReport
  });
  assert(wardrobe.config, "wardrobe.js should register a Page config");
  assert(typeof wardrobe.config.loadWardrobeGaps === "function", "wardrobe should load gaps from latest report");
  const wardrobeInstance = createPageInstance(wardrobe.config);
  await wardrobe.config.loadWardrobeGaps.call(wardrobeInstance);
  assert(wardrobeInstance.data.gaps[0] === "浅色短外套", "wardrobe should display report wardrobe gaps");

  const profile = loadPage("pages/profile/profile.js", {
    ensureDevSession: async () => ({
      token: "dev_token",
      user_public_id: "usr_dev",
      onboarding_status: "not_started"
    })
  });
  assert(profile.config, "profile.js should register a Page config");
  assert(typeof profile.config.loadProfile === "function", "profile should load profile state through API client");
  const profileInstance = createPageInstance(profile.config);
  await profile.config.loadProfile.call(profileInstance);
  assert(profileInstance.data.userPublicID === "usr_dev", "profile should display logged-in user public id");
}

main()
  .then(() => {
    console.log("miniapp api integration verification passed");
  })
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
