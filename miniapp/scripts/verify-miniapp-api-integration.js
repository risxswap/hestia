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

  const appJson = JSON.parse(read("app.json"));
  assert(appJson.pages.includes("pages/wardrobe-detail/wardrobe-detail"), "app should register wardrobe detail page");
  assert(
    !appJson.tabBar.list.some((item) => item.pagePath === "pages/wardrobe-detail/wardrobe-detail"),
    "wardrobe detail page should not be a tabBar page"
  );

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
    getWardrobeItems: async () => ({
      items: [
        {
          public_id: "wdi_shirt",
          name: "米白衬衫",
          category: "top",
          is_core: true,
          recommendation_status: "preferred",
          status: "active"
        },
        {
          public_id: "wdi_paused",
          name: "黑色长裙",
          category: "bottom",
          is_core: true,
          recommendation_status: "paused",
          status: "active"
        }
      ]
    }),
    sendImageRouteFeedback: async () => ({ public_id: "irt_test", status: "active" })
  });
  assert(home.config, "home.js should register a Page config");
  assert(typeof home.config.loadToday === "function", "home page should define loadToday");
  assert(typeof home.mod.normalizeTodayFromReport === "function", "home.js should export normalizeTodayFromReport");

  const homeInstance = createPageInstance(home.config);
  await home.config.loadToday.call(homeInstance);
  assert(homeInstance.data.todayRecommendation.title === "干净 + 有气质", "home should derive title from latest report route");
  assert(homeInstance.data.routePublicID === "irt_test", "home should keep route public id for feedback");
  const coreWardrobeSection = homeInstance.data.todayPlanSections.find((section) => section.title === "核心衣橱");
  assert(coreWardrobeSection, "home should append core wardrobe section after loading latest report");
  assert(coreWardrobeSection.body.includes("米白衬衫"), "home core wardrobe section should include active preferred/core item");
  assert(!coreWardrobeSection.body.includes("黑色长裙"), "home core wardrobe section should exclude paused items");
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

  const wardrobeApiCalls = [];
  const wardrobeUploadCalls = [];
  const wardrobeUploads = [];
  const wardrobe = loadPage("pages/wardrobe/wardrobe.js", {
    getWardrobeItems: async () => ({
      items: [
        {
          public_id: "wdi_shirt",
          name: "米白衬衫",
          category: "top",
          color: "米白",
          is_core: true,
          recommendation_status: "preferred",
          scene_tags: ["通勤"],
          primary_image: {
            object_key: "wardrobe/wdi_shirt/main.jpg"
          }
        },
        {
          public_id: "wdi_jeans",
          name: "直筒牛仔裤",
          category: "bottom",
          color: "蓝色",
          is_core: true,
          recommendation_status: "normal",
          scene_tags: ["日常"]
        }
      ]
    }),
    getLatestReport: async () => sampleReport,
    createWardrobeItem: async (payload) => {
      wardrobeApiCalls.push(["create", payload]);
      return Object.assign({ public_id: "wdi_new", status: "active" }, payload);
    },
    updateWardrobeItem: async (publicID, payload) => {
      wardrobeApiCalls.push(["update", publicID, payload]);
      return Object.assign({ public_id: publicID, status: "active" }, payload);
    },
    deleteWardrobeItem: async (publicID) => {
      wardrobeApiCalls.push(["delete", publicID]);
      return { public_id: publicID };
    },
    uploadAssetToQiniu: async (file, options) => {
      wardrobeUploadCalls.push(["upload", file, options]);
      const upload = createDeferred();
      wardrobeUploads.push(upload);
      return upload.promise;
    }
  });
  assert(wardrobe.config, "wardrobe.js should register a Page config");
  assert(typeof wardrobe.config.loadWardrobe === "function", "wardrobe should load core wardrobe items");
  assert(typeof wardrobe.config.onShow === "function", "wardrobe should refresh when returning from detail changes");
  assert(typeof wardrobe.config.handleCategoryFilter === "function", "wardrobe should filter by category");
  assert(typeof wardrobe.config.handleOpenCreate === "function", "wardrobe should open create form");
  assert(typeof wardrobe.config.handleCloseEditor === "function", "wardrobe should close item editor modal");
  assert(typeof wardrobe.config.handleOpenDetail === "function", "wardrobe should open item detail page");
  assert(typeof wardrobe.config.handleSaveItem === "function", "wardrobe should save item");
  assert(typeof wardrobe.config.handleDeleteItem === "function", "wardrobe should delete item");
  assert(typeof wardrobe.config.handleImageUpload === "function", "wardrobe should handle public image upload");
  assert(typeof wardrobe.config.handleImageRemove === "function", "wardrobe should remove uploaded public image");
  assert(typeof wardrobe.config.loadWardrobeGaps === "function", "wardrobe should load gaps from latest report");
  assert(typeof wardrobe.mod.priorityItems === "function", "wardrobe should export priorityItems helper");
  assert(
    wardrobe.config.data.categoryOptions.map((item) => item.value).join(",") === "all,top,bottom,outerwear,shoes,bag,accessory,sport,home,other",
    "wardrobe category options should support gallery categories"
  );
  assert(
    !wardrobe.config.data.categoryOptions.some((item) => item.label === "鞋包配饰"),
    "wardrobe category options should not merge shoes, bag and accessory"
  );
  const helperPriorityItems = wardrobe.mod.priorityItems([
    { public_id: "wdi_preferred", name: "米白衬衫", status: "active", recommendation_status: "preferred", is_core: false },
    { public_id: "wdi_paused", name: "黑色长裙", status: "active", recommendation_status: "paused", is_core: true },
    { public_id: "wdi_deleted", name: "灰色外套", status: "deleted", recommendation_status: "preferred", is_core: true },
    { public_id: "wdi_core", name: "直筒牛仔裤", status: "active", recommendation_status: "normal", is_core: true },
    { public_id: "wdi_normal", name: "帆布鞋", status: "active", recommendation_status: "normal", is_core: false }
  ]);
  assert(helperPriorityItems.length === 2, "priorityItems should exclude paused, deleted and non-core normal items");
  assert(helperPriorityItems[0].public_id === "wdi_preferred", "priorityItems should sort preferred first");
  assert(helperPriorityItems[1].public_id === "wdi_core", "priorityItems should include active normal core items");
  const fallbackPriorityItems = wardrobe.mod.priorityItems([
    { public_id: "wdi_core_only", name: "直筒牛仔裤", status: "active", recommendation_status: "normal", is_core: true },
    { public_id: "wdi_normal_only", name: "帆布鞋", status: "active", recommendation_status: "normal", is_core: false },
    { public_id: "wdi_paused_preferred", name: "黑色长裙", status: "active", recommendation_status: "paused", is_core: true }
  ]);
  assert(fallbackPriorityItems.length === 1, "priorityItems should keep normal core item when no preferred item exists");
  assert(fallbackPriorityItems[0].public_id === "wdi_core_only", "priorityItems fallback should use normal core item");
  const wardrobeInstance = createPageInstance(wardrobe.config);
  await wardrobe.config.loadWardrobe.call(wardrobeInstance);
  assert(wardrobeInstance.data.activeCategory === "all", "wardrobe should default to all category");
  assert(wardrobeInstance.data.items.length === 2, "wardrobe should load two core wardrobe items");
  assert(wardrobeInstance.data.visibleItems.length === 2, "wardrobe all category should show all loaded items");
  assert(
    wardrobeInstance.data.items[0].primaryImageSrc === "",
    "wardrobe should not expose primary image object_key as display src when signed url is unavailable"
  );
  assert(wardrobeInstance.data.gaps[0] === "浅色短外套", "wardrobe should display report wardrobe gaps");
  assert(
    wardrobeInstance.data.priorityItems.some((item) => item.public_id === "wdi_shirt"),
    "wardrobe should put preferred items into priorityItems"
  );
  assert(
    wardrobeInstance.data.categoryOptions[0].countLabel === "全部 2",
    "wardrobe should show all count in category chip"
  );
  const originalWardrobeWx = global.wx;
  let wardrobeDetailUrl = "";
  global.wx = {
    navigateTo(options) {
      wardrobeDetailUrl = options && options.url ? options.url : "";
    }
  };
  wardrobe.config.handleOpenDetail.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        publicId: "wdi_shirt"
      }
    }
  });
  if (typeof originalWardrobeWx === "undefined") {
    delete global.wx;
  } else {
    global.wx = originalWardrobeWx;
  }
  assert(
    wardrobeDetailUrl === "/pages/wardrobe-detail/wardrobe-detail?public_id=wdi_shirt",
    "wardrobe item tap should navigate to detail page"
  );

  wardrobe.config.handleCategoryFilter.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        category: "top"
      }
    }
  });
  assert(wardrobeInstance.data.visibleItems.length === 1, "wardrobe category=top should show one item");
  assert(wardrobeInstance.data.visibleItems[0].public_id === "wdi_shirt", "wardrobe category=top should keep top item");
  wardrobe.config.handleCategoryFilter.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        category: "bag"
      }
    }
  });
  wardrobe.config.handleOpenCreate.call(wardrobeInstance);
  assert(wardrobeInstance.data.draft.category === "bag", "wardrobe create from an active category should default to that category");
  wardrobe.config.handleCloseEditor.call(wardrobeInstance);

  wardrobe.config.handleOpenCreate.call(wardrobeInstance);
  assert(wardrobeInstance.data.editorVisible === true, "wardrobe create should open editor modal");
  assert(Array.isArray(wardrobeInstance.data.imageFiles) && wardrobeInstance.data.imageFiles.length === 0, "wardrobe create should initialize empty imageFiles");
  wardrobe.config.handleCloseEditor.call(wardrobeInstance);
  assert(wardrobeInstance.data.editorVisible === false, "wardrobe close should hide editor modal");
  assert(wardrobeInstance.data.editingPublicID === "", "wardrobe close should clear editingPublicID");
  assert(wardrobeInstance.data.draft.name === "", "wardrobe close should reset draft");
  assert(Array.isArray(wardrobeInstance.data.imageFiles) && wardrobeInstance.data.imageFiles.length === 0, "wardrobe close should clear imageFiles");
  wardrobe.config.handleOpenCreate.call(wardrobeInstance);
  const pendingWardrobeUpload = wardrobe.config.handleImageUpload.call(wardrobeInstance, {
    detail: {
      files: [{ url: "wxfile://wardrobe-create.jpg", size: 2048, type: "image/jpeg" }]
    }
  });
  const duplicateWardrobeUpload = wardrobe.config.handleImageUpload.call(wardrobeInstance, {
    detail: {
      files: [{ url: "wxfile://wardrobe-create.jpg", size: 2048, type: "image/jpeg" }]
    }
  });
  assert(wardrobeUploadCalls.length === 1, "wardrobe image upload should avoid duplicate add/success uploads while uploading");
  assert(wardrobeUploadCalls[0][2].assetType === "wardrobe_item_photo", "wardrobe image upload should use wardrobe_item_photo asset type");
  await wardrobe.config.handleSaveItem.call(wardrobeInstance);
  assert(wardrobeInstance.data.errorMessage === "图片还在上传，请稍后再保存", "wardrobe save should be blocked while image is uploading");
  wardrobeUploads[0].resolve({
    asset_public_id: "ast_create",
    url: "https://cdn.example.com/wardrobe-create.jpg",
    object_key: "users/u1/assets/ast_create.jpg"
  });
  await pendingWardrobeUpload;
  await duplicateWardrobeUpload;
  assert(wardrobeInstance.data.draft.primary_asset_public_id === "ast_create", "wardrobe upload success should set draft primary asset id");
  assert(wardrobeInstance.data.imageFiles[0].url === "https://cdn.example.com/wardrobe-create.jpg", "wardrobe upload success should show uploaded image url");
  assert(wardrobeInstance.data.imageUploadError === "", "wardrobe upload success should clear image error");
  wardrobe.config.handleDraftInput.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        field: "name"
      }
    },
    detail: {
      value: "黑色西装"
    }
  });
  wardrobe.config.handleDraftInput.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        field: "category"
      }
    },
    detail: {
      value: "outerwear"
    }
  });
  await wardrobe.config.handleSaveItem.call(wardrobeInstance);
  assert(wardrobeApiCalls[0][0] === "create", "wardrobe save should create a new item");
  assert(wardrobeApiCalls[0][1].primary_asset_public_id === "ast_create", "wardrobe create payload should include uploaded primary asset id");
  assert(
    wardrobeApiCalls[0][1].recommendation_status === "normal",
    "wardrobe create payload should default recommendation_status to normal"
  );
  wardrobe.config.handleOpenCreate.call(wardrobeInstance);
  const removedWardrobeUpload = wardrobe.config.handleImageUpload.call(wardrobeInstance, {
    detail: {
      files: [{ url: "wxfile://wardrobe-removed.jpg", size: 2048, type: "image/jpeg" }]
    }
  });
  assert(wardrobeInstance.data.imageUploading === true, "wardrobe remove guard should start from uploading state");
  wardrobe.config.handleImageRemove.call(wardrobeInstance);
  assert(wardrobeInstance.data.imageUploading === false, "wardrobe image remove should clear uploading state");
  wardrobeUploads[1].resolve({
    asset_public_id: "ast_removed",
    url: "https://cdn.example.com/removed.jpg",
    object_key: "users/u1/assets/ast_removed.jpg"
  });
  await removedWardrobeUpload;
  assert(wardrobeInstance.data.draft.primary_asset_public_id === "", "wardrobe removed stale upload should not restore primary asset id");
  assert(wardrobeInstance.data.imageFiles.length === 0, "wardrobe removed stale upload should not restore imageFiles");

  wardrobe.config.handleEditItem.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        publicId: "wdi_jeans"
      }
    }
  });
  assert(Array.isArray(wardrobeInstance.data.imageFiles) && wardrobeInstance.data.imageFiles.length === 0, "wardrobe edit should initialize empty imageFiles when item has no primary image");
  wardrobe.config.handleImageRemove.call(wardrobeInstance);
  assert(wardrobeInstance.data.draft.primary_asset_public_id === "", "wardrobe image remove should clear draft primary asset id");
  wardrobe.config.handleDraftInput.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        field: "color"
      }
    },
    detail: {
      value: "深蓝"
    }
  });
  wardrobe.config.handleSceneInput.call(wardrobeInstance, {
    detail: {
      value: "周末，旅行"
    }
  });
  wardrobe.config.handleRecommendationStatus.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        status: "paused"
      }
    }
  });
  wardrobe.config.handleCoreToggle.call(wardrobeInstance, {
    detail: {
      value: false
    }
  });
  await wardrobe.config.handleSaveItem.call(wardrobeInstance);
  const updateCall = wardrobeApiCalls.find((call) => call[0] === "update" && call[1] === "wdi_jeans");
  assert(updateCall, "wardrobe edit save should call updateWardrobeItem");
  assert(updateCall[2].color === "深蓝", "wardrobe edit should include changed color");
  assert(updateCall[2].recommendation_status === "paused", "wardrobe edit should include changed recommendation status");
  assert(updateCall[2].is_core === false, "wardrobe edit should include changed core flag");
  assert(updateCall[2].scene_tags.join(",") === "周末,旅行", "wardrobe edit should parse scene tags");
  assert(
    Object.prototype.hasOwnProperty.call(updateCall[2], "primary_asset_public_id") && updateCall[2].primary_asset_public_id === "",
    "wardrobe edit save should keep empty primary_asset_public_id so backend can clear primary image"
  );

  await wardrobe.config.handleDeleteItem.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        publicId: "wdi_shirt"
      }
    }
  });
  assert(wardrobeApiCalls.some((call) => call[0] === "delete" && call[1] === "wdi_shirt"), "wardrobe delete should call API");
  assert(
    wardrobeInstance.data.items.every((item) => item.public_id !== "wdi_shirt"),
    "wardrobe delete should remove deleted item locally"
  );

  wardrobe.config.handleEditItem.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        publicId: "wdi_jeans"
      }
    }
  });
  await wardrobe.config.handleDeleteItem.call(wardrobeInstance, {
    currentTarget: {
      dataset: {
        publicId: "wdi_jeans"
      }
    }
  });
  assert(wardrobeInstance.data.editorVisible === false, "wardrobe delete should close editor when deleting edited item");
  assert(wardrobeInstance.data.editingPublicID === "", "wardrobe delete should clear editingPublicID");
  assert(wardrobeInstance.data.draft.name === "", "wardrobe delete should reset draft");

  await wardrobe.config.loadWardrobeGaps.call(wardrobeInstance);
  assert(wardrobeInstance.data.gaps[0] === "浅色短外套", "wardrobe loadWardrobeGaps should stay compatible");

  const wardrobeMarkup = read("pages/wardrobe/wardrobe.wxml");
  const wardrobeStyles = read("pages/wardrobe/wardrobe.wxss");
  assert(wardrobeMarkup.includes("我的衣服"), "wardrobe gallery should use gallery title");
  assert(wardrobeMarkup.includes("category-scroll"), "wardrobe category row should be horizontally scrollable");
  assert(wardrobeMarkup.includes("gallery-grid"), "wardrobe page should render gallery grid");
  assert(
    wardrobeStyles.includes(".gallery-grid") && wardrobeStyles.includes("grid-template-columns: repeat(2, minmax(0, 1fr))"),
    "wardrobe gallery grid should use two columns"
  );
  assert(wardrobeMarkup.includes("gallery-add-card"), "wardrobe page should include grid add card");
  assert(wardrobeMarkup.includes("handleOpenDetail"), "wardrobe cards should bind detail navigation");
  assert(wardrobeMarkup.includes("visibleItems"), "wardrobe page should render filtered wardrobe cards");
  assert(wardrobeMarkup.includes("建议补齐"), "wardrobe page should keep wardrobe gaps section");
  assert(wardrobeMarkup.includes("handleSaveItem"), "wardrobe page should bind save item action");
  assert(wardrobeMarkup.includes("handleCategoryFilter"), "wardrobe page should bind category filter action");
  assert(!wardrobeMarkup.includes("bind:tap=\"handleEditItem\""), "wardrobe gallery cards should not open editor directly");
  assert(wardrobeMarkup.includes("modal-layer"), "wardrobe editor should render as modal layer");
  assert(wardrobeMarkup.includes("modal-backdrop"), "wardrobe editor should include a backdrop");
  assert(wardrobeMarkup.includes("modal-sheet"), "wardrobe editor should use a bottom sheet");
  assert(wardrobeMarkup.includes("handleCloseEditor"), "wardrobe page should bind close editor action");
  assert(!wardrobeMarkup.includes("class=\"card editor-panel\""), "wardrobe editor should not render as inline card");
  assert(wardrobeMarkup.includes("handleRecommendationStatus"), "wardrobe page should bind recommendation status action");
  assert(wardrobeMarkup.includes("handleCoreToggle"), "wardrobe page should bind core item toggle action");
  assert(wardrobeMarkup.includes("primaryImageSrc"), "wardrobe page should render normalized primary image source");
  assert(wardrobeMarkup.includes("<t-upload"), "wardrobe editor should render TDesign upload block");
  assert(wardrobeMarkup.includes("mediaType=\"{{imageMediaType}}\""), "wardrobe upload should restrict media type to image");
  assert(wardrobeMarkup.includes("gridConfig=\"{{imageGridConfig}}\""), "wardrobe upload should pass TDesign gridConfig prop");
  assert(wardrobeMarkup.includes("sizeLimit=\"{{imageSizeLimit}}\""), "wardrobe upload should pass TDesign sizeLimit prop");
  assert(wardrobeMarkup.includes("handleImageUpload"), "wardrobe editor should bind image upload handler");
  assert(wardrobeMarkup.includes("handleImageRemove"), "wardrobe editor should bind image remove handler");
  assert(!wardrobeMarkup.includes("主图资产 ID"), "wardrobe editor should not ask users to type primary asset id");
  assert(
    !wardrobeMarkup.includes("这里展示服务端报告识别出的关键缺口"),
    "wardrobe page should remove old report-gap-only copy"
  );
  const wardrobePageJson = JSON.parse(read("pages/wardrobe/wardrobe.json"));
  assert(
    wardrobePageJson.usingComponents && wardrobePageJson.usingComponents["t-upload"] === "/miniprogram_npm/tdesign-miniprogram/upload/upload",
    "wardrobe page should register TDesign upload component"
  );

  const detailApiCalls = [];
  const detailUploadCalls = [];
  let detailRemoveUpload = null;
  const wardrobeDetail = loadPage("pages/wardrobe-detail/wardrobe-detail.js", {
    getWardrobeItems: async () => ({
      items: [
        {
          public_id: "wdi_shirt",
          name: "米白衬衫",
          category: "top",
          color: "米白",
          material: "棉",
          season: "春秋",
          silhouette: "微宽松",
          user_notes: "下摆处理要干净",
          is_core: true,
          recommendation_status: "preferred",
          scene_tags: ["通勤", "见客户"],
          primary_image: {
            asset_public_id: "ast_old",
            url: "https://cdn.example.com/wardrobe/wdi_shirt/main.jpg",
            object_key: "wardrobe/wdi_shirt/main.jpg"
          }
        }
      ]
    }),
    updateWardrobeItem: async (publicID, payload) => {
      detailApiCalls.push(["update", publicID, payload]);
      return Object.assign({ public_id: publicID, status: "active" }, payload);
    },
    deleteWardrobeItem: async (publicID) => {
      detailApiCalls.push(["delete", publicID]);
      return { public_id: publicID };
    },
    uploadAssetToQiniu: async (file, options) => {
      detailUploadCalls.push(["upload", file, options]);
      if (file && file.url === "wxfile://detail-fail.jpg") {
        throw new Error("七牛上传失败");
      }
      if (file && file.url === "wxfile://detail-pending-remove.jpg") {
        detailRemoveUpload = createDeferred();
        return detailRemoveUpload.promise;
      }
      return {
        asset_public_id: "ast_detail",
        url: "https://cdn.example.com/detail.jpg",
        object_key: "users/u1/assets/ast_detail.jpg"
      };
    }
  });
  assert(wardrobeDetail.config, "wardrobe detail should register a Page config");
  assert(typeof wardrobeDetail.config.loadWardrobeItem === "function", "wardrobe detail should load item");
  assert(typeof wardrobeDetail.config.handleOpenEdit === "function", "wardrobe detail should open edit modal");
  assert(typeof wardrobeDetail.config.handleSaveItem === "function", "wardrobe detail should save edits");
  assert(typeof wardrobeDetail.config.handleImageUpload === "function", "wardrobe detail should handle public image upload");
  assert(typeof wardrobeDetail.config.handleImageRemove === "function", "wardrobe detail should remove uploaded public image");

  const detailInstance = createPageInstance(wardrobeDetail.config);
  const originalGetAppForWardrobeDirty = global.getApp;
  const appGlobalData = {
    wardrobeDirty: false
  };
  global.getApp = () => ({
    globalData: appGlobalData
  });
  await wardrobeDetail.config.onLoad.call(detailInstance, { public_id: "wdi_shirt" });
  assert(detailInstance.data.item.public_id === "wdi_shirt", "wardrobe detail should find item by public_id");
  assert(detailInstance.data.item.styleLogic, "wardrobe detail should expose style logic text");
  wardrobeDetail.config.handleOpenEdit.call(detailInstance);
  assert(detailInstance.data.editorVisible === true, "wardrobe detail edit should open editor modal");
  assert(detailInstance.data.imageFiles[0].url === "https://cdn.example.com/wardrobe/wdi_shirt/main.jpg", "wardrobe detail edit should echo existing primary image");
  await wardrobeDetail.config.handleImageUpload.call(detailInstance, {
    detail: {
      files: [{ url: "wxfile://detail.jpg", size: 4096, type: "image/jpeg" }]
    }
  });
  assert(detailUploadCalls[0][2].assetType === "wardrobe_item_photo", "wardrobe detail image upload should use wardrobe_item_photo asset type");
  assert(detailInstance.data.draft.primary_asset_public_id === "ast_detail", "wardrobe detail upload success should set draft primary asset id");
  await wardrobeDetail.config.handleImageUpload.call(detailInstance, {
    detail: {
      files: [{ url: "wxfile://detail-fail.jpg", size: 4096, type: "image/jpeg" }]
    }
  });
  assert(detailInstance.data.imageUploadError === "七牛上传失败", "wardrobe detail upload failure should show error");
  assert(detailInstance.data.imageFiles[0].status === "failed", "wardrobe detail upload failure should mark imageFiles failed");
  assert(detailInstance.data.draft.primary_asset_public_id === "ast_detail", "wardrobe detail upload failure should not overwrite existing uploaded asset id");
  wardrobeDetail.config.handleImageRemove.call(detailInstance);
  assert(detailInstance.data.imageFiles.length === 0, "wardrobe detail remove should clear imageFiles");
  assert(detailInstance.data.draft.primary_asset_public_id === "", "wardrobe detail remove should clear draft primary asset id");
  const removedDetailUpload = wardrobeDetail.config.handleImageUpload.call(detailInstance, {
    detail: {
      files: [{ url: "wxfile://detail-pending-remove.jpg", size: 4096, type: "image/jpeg" }]
    }
  });
  wardrobeDetail.config.handleImageRemove.call(detailInstance);
  detailRemoveUpload.resolve({
    asset_public_id: "ast_detail_removed",
    url: "https://cdn.example.com/detail-removed.jpg",
    object_key: "users/u1/assets/ast_detail_removed.jpg"
  });
  await removedDetailUpload;
  assert(detailInstance.data.draft.primary_asset_public_id === "", "wardrobe detail stale upload should not restore primary asset id after remove");
  assert(detailInstance.data.imageFiles.length === 0, "wardrobe detail stale upload should not restore imageFiles after remove");
  wardrobeDetail.config.handleDraftInput.call(detailInstance, {
    currentTarget: {
      dataset: {
        field: "color"
      }
    },
    detail: {
      value: "暖白"
    }
  });
  await wardrobeDetail.config.handleSaveItem.call(detailInstance);
  assert(detailApiCalls[0][0] === "update", "wardrobe detail save should call updateWardrobeItem");
  assert(detailApiCalls[0][1] === "wdi_shirt", "wardrobe detail save should update current item");
  assert(
    Object.prototype.hasOwnProperty.call(detailApiCalls[0][2], "primary_asset_public_id"),
    "wardrobe detail save should include primary asset field when user clears it"
  );
  assert(detailApiCalls[0][2].primary_asset_public_id === "", "wardrobe detail save should allow clearing primary asset");
  assert(detailInstance.data.item.color === "暖白", "wardrobe detail save should refresh current detail");
  assert(detailInstance.data.editorVisible === false, "wardrobe detail save should close editor modal");
  assert(detailInstance.data.wardrobeDirty === true, "wardrobe detail save should mark wardrobe list dirty");
  assert(appGlobalData.wardrobeDirty === true, "wardrobe detail save should mark app wardrobe dirty");

  const originalDetailWx = global.wx;
  let detailSwitchUrl = "";
  global.wx = {
    switchTab(options) {
      detailSwitchUrl = options && options.url ? options.url : "";
    }
  };
  await wardrobeDetail.config.handleDeleteItem.call(detailInstance, {
    currentTarget: {
      dataset: {
        publicId: "wdi_shirt"
      }
    }
  });
  if (typeof originalDetailWx === "undefined") {
    delete global.wx;
  } else {
    global.wx = originalDetailWx;
  }
  assert(detailApiCalls.some((call) => call[0] === "delete" && call[1] === "wdi_shirt"), "wardrobe detail delete should call API");
  assert(detailSwitchUrl === "/pages/wardrobe/wardrobe", "wardrobe detail delete should return to wardrobe tab");
  assert(detailInstance.data.wardrobeDirty === true, "wardrobe detail delete should mark wardrobe list dirty");
  assert(appGlobalData.wardrobeDirty === true, "wardrobe detail delete should keep app wardrobe dirty");

  await wardrobe.config.onShow.call(wardrobeInstance);
  assert(wardrobeInstance.data.items.length === 2, "wardrobe onShow should refresh dirty list");
  assert(wardrobeInstance.data.wardrobeDirty === false, "wardrobe onShow should clear dirty flag after refresh");
  assert(appGlobalData.wardrobeDirty === false, "wardrobe onShow should clear app dirty flag");

  const failingWardrobe = loadPage("pages/wardrobe/wardrobe.js", {
    getWardrobeItems: async () => {
      throw new Error("wardrobe refresh failed");
    },
    getLatestReport: async () => sampleReport
  });
  const failingWardrobeInstance = createPageInstance(failingWardrobe.config);
  appGlobalData.wardrobeDirty = true;
  await failingWardrobe.config.onShow.call(failingWardrobeInstance);
  assert(appGlobalData.wardrobeDirty === true, "wardrobe onShow should keep dirty flag when refresh fails");
  assert(failingWardrobeInstance.data.wardrobeDirty === true, "wardrobe failed refresh should keep page dirty state");

  if (typeof originalGetAppForWardrobeDirty === "undefined") {
    delete global.getApp;
  } else {
    global.getApp = originalGetAppForWardrobeDirty;
  }

  const detailMarkup = read("pages/wardrobe-detail/wardrobe-detail.wxml");
  assert(detailMarkup.includes("搭配逻辑"), "wardrobe detail should render style logic section");
  assert(detailMarkup.includes("结构化档案"), "wardrobe detail should render structured profile section");
  assert(detailMarkup.includes("最近反馈"), "wardrobe detail should render recent feedback section");
  assert(detailMarkup.includes("<t-upload"), "wardrobe detail editor should render TDesign upload block");
  assert(detailMarkup.includes("mediaType=\"{{imageMediaType}}\""), "wardrobe detail upload should restrict media type to image");
  assert(detailMarkup.includes("gridConfig=\"{{imageGridConfig}}\""), "wardrobe detail upload should pass TDesign gridConfig prop");
  assert(detailMarkup.includes("sizeLimit=\"{{imageSizeLimit}}\""), "wardrobe detail upload should pass TDesign sizeLimit prop");
  assert(detailMarkup.includes("handleImageUpload"), "wardrobe detail editor should bind image upload handler");
  assert(detailMarkup.includes("handleImageRemove"), "wardrobe detail editor should bind image remove handler");
  assert(!detailMarkup.includes("主图资产 ID"), "wardrobe detail editor should not ask users to type primary asset id");
  const wardrobeDetailPageJson = JSON.parse(read("pages/wardrobe-detail/wardrobe-detail.json"));
  assert(
    wardrobeDetailPageJson.usingComponents && wardrobeDetailPageJson.usingComponents["t-upload"] === "/miniprogram_npm/tdesign-miniprogram/upload/upload",
    "wardrobe detail page should register TDesign upload component"
  );

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
