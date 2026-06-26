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

async function main() {
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

  const onboardingInstance = createPageInstance(onboarding.config);
  onboardingInstance.data.basic.gender = "female";
  onboardingInstance.data.basic.height_cm = "168";
  onboardingInstance.data.styleGoal.goalsText = "干净利落, 通勤有气质";
  onboardingInstance.data.styleGoal.avoidancesText = "过度甜美";
  onboardingInstance.data.styleGoal.scenariosText = "工作日通勤";
  onboardingInstance.data.wardrobeText = "米白衬衫, top, 米白\n直筒牛仔裤, bottom, 蓝色";
  await onboarding.config.handleSubmit.call(onboardingInstance);
  assert(onboardingCalls[0][0] === "save", "onboarding submit should save draft first");
  assert(onboardingCalls[0][1] === "wardrobe", "onboarding should save final step as wardrobe");
  assert(onboardingCalls[0][2].style_goal.goals.length === 2, "onboarding should parse goals text");
  assert(onboardingCalls[0][2].wardrobe.items.length === 2, "onboarding should parse wardrobe text");
  assert(onboardingCalls[1][0] === "submit", "onboarding submit should call submit API");

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
