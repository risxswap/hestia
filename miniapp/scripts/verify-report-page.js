const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const reportPath = path.join(root, "pages/report/report.js");

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function assertIncludes(file, content, expected) {
  assert(content.includes(expected), `${file} should include ${expected}`);
}

function withGlobals(globals, fn) {
  const originalPage = global.Page;
  const originalWx = global.wx;
  const originalGetApp = global.getApp;

  try {
    if (Object.prototype.hasOwnProperty.call(globals, "Page")) {
      global.Page = globals.Page;
    } else {
      delete global.Page;
    }

    if (Object.prototype.hasOwnProperty.call(globals, "wx")) {
      global.wx = globals.wx;
    } else {
      delete global.wx;
    }

    if (Object.prototype.hasOwnProperty.call(globals, "getApp")) {
      global.getApp = globals.getApp;
    } else {
      delete global.getApp;
    }

    return fn();
  } finally {
    if (typeof originalPage === "undefined") {
      delete global.Page;
    } else {
      global.Page = originalPage;
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
  }
}

function requireFreshReportModule(globals) {
  return withGlobals(globals || {}, () => {
    delete require.cache[require.resolve(reportPath)];
    return require(reportPath);
  });
}

function callLoadLatestReport(reportModule, globals) {
  let setDataPayload;
  const pageInstance = {
    setData(payload) {
      setDataPayload = payload;
    }
  };

  withGlobals(globals, () => {
    reportModule.reportPageConfig.loadLatestReport.call(pageInstance);
  });

  return setDataPayload;
}

const reportWxml = read("pages/report/report.wxml");
const packageJson = JSON.parse(read("package.json"));

const reportModule = requireFreshReportModule();

assert(
  reportModule && typeof reportModule.normalizeReportResponse === "function",
  "report.js should export normalizeReportResponse"
);
assert(
  reportModule && reportModule.fallbackReportData,
  "report.js should export fallbackReportData"
);
assert(
  reportModule && reportModule.reportPageConfig,
  "report.js should export reportPageConfig"
);
assert(typeof reportModule.reportPageConfig.onLoad === "function", "report page should define onLoad");
assert(
  typeof reportModule.reportPageConfig.loadLatestReport === "function",
  "report page should define loadLatestReport"
);

const sampleReport = {
  public_id: "rpt_test",
  title: "初版个人形象报告",
  summary: "先用清爽线条建立稳定的第一印象。",
  content_json: {
    action_items: ["本周先固定一套通勤模板", "发型保留额头呼吸感"],
    avoidances: ["避免上下都宽松"],
    wardrobe_gaps: [{ title: "浅色短外套", reason: "补足轻正式场景" }],
    reference_style_logic: "参考干净比例、浅色层次和利落发型。"
  },
  routes: [
    {
      name: "干净 + 有气质",
      route_role: "primary",
      target_impression: ["干净"],
      reason: ["适合通勤和见客户", "不需要强电商导购"]
    }
  ]
};

const normalized = reportModule.normalizeReportResponse(sampleReport);
assert(normalized.title === sampleReport.title, "normalizeReportResponse should keep title");
assert(normalized.summary === sampleReport.summary, "normalizeReportResponse should keep summary");
assert(normalized.updatedAt === "", "real report without date should not use fallback updatedAt");
assert(
  Array.isArray(normalized.actionItems) && normalized.actionItems.length === 2,
  "normalizeReportResponse should convert content_json.action_items to actionItems"
);
assert(
  normalized.actionItems[0].title === sampleReport.content_json.action_items[0],
  "actionItems should expose action item text as title"
);
assert(
  Array.isArray(normalized.routes) && normalized.routes.length === 1,
  "normalizeReportResponse should convert routes to page routes"
);
assert(normalized.routes[0].name === "干净 + 有气质", "routes should keep name");
assert(normalized.routes[0].roleLabel === "主路线", "primary route role should show as 主路线");
assert(
  normalized.routes[0].reasonText && normalized.routes[0].reasonText.includes("适合通勤和见客户"),
  "routes should expose reason text"
);
assert(
  reportModule.normalizeReportResponse({
    content_json: {},
    routes: [{ name: "未知路线", route_role: "internal_new_role" }]
  }).routes[0].roleLabel === "",
  "unknown route role should be hidden"
);

const fallback = reportModule.normalizeReportResponse(null);
assert(
  fallback && fallback.actionItems === reportModule.fallbackReportData.actionItems,
  "normalizeReportResponse should use mock fallback when report data is missing"
);

let registeredPageConfig;
requireFreshReportModule({
  Page(config) {
    registeredPageConfig = config;
  }
});
assert(registeredPageConfig, "report.js should register a Page config inside miniapp runtime");

let requestOptions;
const successPayload = callLoadLatestReport(reportModule, {
  getApp: () => ({
    globalData: {
      apiBaseUrl: "https://api.example.test/"
    }
  }),
  wx: {
    getStorageSync(key) {
      return key === "user_token" ? "token" : "";
    },
    request(options) {
      requestOptions = options;
      options.success({
        data: {
          code: "ok",
          message: "",
          data: sampleReport
        }
      });
    }
  }
});

assert(requestOptions, "loadLatestReport should call wx.request");
assert(
  requestOptions.url === "https://api.example.test/api/user/reports/latest",
  `loadLatestReport should request latest report url, got ${requestOptions.url}`
);
assert(
  requestOptions.header && requestOptions.header.Authorization === "Bearer token",
  "loadLatestReport should send Authorization bearer token"
);
assert(successPayload.summary === sampleReport.summary, "success response should set normalized report");
assert(successPayload.routes[0].roleLabel === "主路线", "success response should set normalized route role label");

const failPayload = callLoadLatestReport(reportModule, {
  getApp: () => ({
    globalData: {
      apiBaseUrl: "https://api.example.test"
    }
  }),
  wx: {
    getStorageSync() {
      return "token";
    },
    request(options) {
      options.fail();
    }
  }
});
assert(failPayload === reportModule.fallbackReportData, "request failure should use fallback");

const noTokenPayload = callLoadLatestReport(reportModule, {
  getApp: () => ({
    globalData: {
      apiBaseUrl: "https://api.example.test"
    }
  }),
  wx: {
    getStorageSync() {
      return "";
    },
    request() {
      throw new Error("wx.request should not run without token");
    }
  }
});
assert(noTokenPayload === reportModule.fallbackReportData, "missing token should use fallback");

const noBaseUrlPayload = callLoadLatestReport(reportModule, {
  getApp: () => ({
    globalData: {
      apiBaseUrl: ""
    }
  }),
  wx: {
    getStorageSync() {
      return "token";
    },
    request() {
      throw new Error("wx.request should not run without apiBaseUrl");
    }
  }
});
assert(noBaseUrlPayload === reportModule.fallbackReportData, "missing apiBaseUrl should use fallback");

assertIncludes("pages/report/report.wxml", reportWxml, "{{summary}}");
assertIncludes("pages/report/report.wxml", reportWxml, "wx:if=\"{{updatedAt}}\"");
assertIncludes("pages/report/report.wxml", reportWxml, "wx:for=\"{{routes}}\"");
assertIncludes("pages/report/report.wxml", reportWxml, "wx:for=\"{{actionItems}}\"");
assert(
  reportWxml.includes("{{item.roleLabel}}") && !reportWxml.includes("{{item.route_role}}"),
  "report.wxml should display roleLabel instead of route_role"
);

assert(
  packageJson.scripts && packageJson.scripts["verify:report-page"] === "node scripts/verify-report-page.js",
  "package.json should define verify:report-page script"
);

console.log("report page verification passed");
