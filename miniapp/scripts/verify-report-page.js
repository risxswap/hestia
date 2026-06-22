const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");

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

const reportJs = read("pages/report/report.js");
const reportWxml = read("pages/report/report.wxml");

let pageConfig;
const originalPage = global.Page;
const originalWx = global.wx;
const originalGetApp = global.getApp;

global.Page = (config) => {
  pageConfig = config;
};
global.wx = {
  getStorageSync() {
    return "";
  },
  request() {}
};
global.getApp = () => ({
  globalData: {
    apiBaseUrl: ""
  }
});

const reportModule = require(path.join(root, "pages/report/report.js"));

global.Page = originalPage;
global.wx = originalWx;
global.getApp = originalGetApp;

assert(
  reportModule && typeof reportModule.normalizeReportResponse === "function",
  "report.js should export normalizeReportResponse"
);
assert(
  reportModule && reportModule.fallbackReportData,
  "report.js should export fallbackReportData"
);
assert(pageConfig, "report.js should register a Page config");
assert(typeof pageConfig.onLoad === "function", "report page should define onLoad");
assert(typeof pageConfig.loadLatestReport === "function", "report page should define loadLatestReport");

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
assert(normalized.routes[0].route_role === "primary", "routes should keep route_role");
assert(
  normalized.routes[0].reasonText && normalized.routes[0].reasonText.includes("适合通勤和见客户"),
  "routes should expose reason text"
);

const fallback = reportModule.normalizeReportResponse(null);
assert(
  fallback && fallback.actionItems === reportModule.fallbackReportData.actionItems,
  "normalizeReportResponse should use mock fallback when report data is missing"
);

assertIncludes("pages/report/report.wxml", reportWxml, "{{summary}}");
assertIncludes("pages/report/report.wxml", reportWxml, "wx:for=\"{{routes}}\"");
assertIncludes("pages/report/report.wxml", reportWxml, "wx:for=\"{{actionItems}}\"");

assertIncludes("pages/report/report.js", reportJs, "wx.request");
assert(
  reportJs.includes('wx.getStorageSync("user_token")') ||
    reportJs.includes("wx.getStorageSync('user_token')"),
  "report.js should read user_token from storage"
);
assert(
  reportJs.includes('wx.getStorageSync("token")') ||
    reportJs.includes("wx.getStorageSync('token')"),
  "report.js should read token from storage"
);
assertIncludes("pages/report/report.js", reportJs, "Authorization");
assertIncludes("pages/report/report.js", reportJs, "Bearer");

console.log("report page verification passed");
