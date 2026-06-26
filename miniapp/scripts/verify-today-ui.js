const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const homePath = path.join(root, "pages/home/home.js");
const apiPath = path.join(root, "utils/api.js");

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

const homeSource = read("pages/home/home.js");
const homeWxml = read("pages/home/home.wxml");
const homeWxss = read("pages/home/home.wxss");
const appJson = JSON.parse(read("app.json"));

assert(homeSource.includes("../../utils/api"), "home page should import shared api client");
assert(!homeSource.includes("utils/mock"), "home page should not import mock data");
assert(homeWxml.includes("{{!hasReport}}"), "home page should render empty report state");
assert(homeWxml.includes("data-action=\"{{item.action}}\""), "home page should send feedback action");
assert(homeWxss.includes("today-visual"), "home page should keep today visual layout");

const firstTab = appJson.tabBar && appJson.tabBar.list && appJson.tabBar.list[0];
assert(firstTab && firstTab.text === "今日", "first tab should be 今日");
assert(firstTab.pagePath === "pages/home/home", "first tab should point to pages/home/home");

let pageConfig;
const originalPage = global.Page;
global.Page = (config) => {
  pageConfig = config;
};
require.cache[require.resolve(apiPath)] = {
  id: apiPath,
  filename: apiPath,
  loaded: true,
  exports: {}
};
delete require.cache[require.resolve(homePath)];
const homeModule = require(homePath);
global.Page = originalPage;
delete require.cache[require.resolve(apiPath)];

assert(pageConfig, "home.js should register a Page config");
assert(typeof pageConfig.loadToday === "function", "home page should define loadToday");
assert(typeof pageConfig.handlePrimaryAction === "function", "home page should define handlePrimaryAction");
assert(typeof homeModule.normalizeTodayFromReport === "function", "home.js should export normalizeTodayFromReport");

const normalized = homeModule.normalizeTodayFromReport({
  title: "初版个人形象报告",
  summary: "稳定第一印象。",
  content_json: {
    action_items: ["固定一套通勤模板"],
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
});

assert(normalized.hasReport, "normalized today data should mark report as available");
assert(normalized.routePublicID === "irt_test", "normalized today data should keep route public id");
assert(normalized.todayRecommendation.title === "干净 + 有气质", "today title should come from server route");
assert(normalized.todayPlanSections.length >= 2, "today plan should include server-derived sections");

console.log("today ui verification passed");
