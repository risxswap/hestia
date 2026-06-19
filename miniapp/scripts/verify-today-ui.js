const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function assertIncludes(file, content, expected) {
  if (!content.includes(expected)) {
    throw new Error(`${file} should include ${expected}`);
  }
}

const appJson = JSON.parse(read("app.json"));
const mockSource = read("utils/mock.js");
const homeJs = read("pages/home/home.js");
const homeWxml = read("pages/home/home.wxml");
const homeWxss = read("pages/home/home.wxss");
const mock = require(path.join(root, "utils/mock.js"));

let homePageConfig;
const originalPage = global.Page;
global.Page = (config) => {
  homePageConfig = config;
};
require(path.join(root, "pages/home/home.js"));
global.Page = originalPage;

const firstTab = appJson.tabBar && appJson.tabBar.list && appJson.tabBar.list[0];

if (!firstTab) {
  throw new Error("app.json should define a first tab");
}

if (firstTab.text !== "今日") {
  throw new Error(`first tab should be 今日, got ${firstTab.text}`);
}

if (firstTab.pagePath !== "pages/home/home") {
  throw new Error(`first tab should point to pages/home/home, got ${firstTab.pagePath}`);
}

assertIncludes("utils/mock.js", mockSource, "todayRecommendation");
assertIncludes("utils/mock.js", mockSource, "todayPlanSections");
assertIncludes("utils/mock.js", mockSource, "feedbackOptions");
assertIncludes("utils/mock.js", mockSource, "今日推荐");
assertIncludes("utils/mock.js", mockSource, "照这个穿");
assertIncludes("utils/mock.js", mockSource, "换个场景");
assertIncludes("pages/home/home.js", homeJs, "todayRecommendation");
assertIncludes("pages/home/home.js", homeJs, "todayPlanSections");
assertIncludes("pages/home/home.js", homeJs, "feedbackOptions");
assertIncludes("pages/home/home.js", homeJs, "handlePrimaryAction");
assertIncludes("pages/home/home.js", homeJs, "handleSceneChange");
assertIncludes("pages/home/home.js", homeJs, "handleFeedback");
assertIncludes("pages/home/home.wxml", homeWxml, "{{todayRecommendation.label}}");
assertIncludes("pages/home/home.wxml", homeWxml, "{{todayRecommendation.primaryAction}}");
assertIncludes("pages/home/home.wxml", homeWxml, "{{todayRecommendation.secondaryAction}}");
assertIncludes("pages/home/home.wxml", homeWxml, "为什么适合今天");
assertIncludes("pages/home/home.wxml", homeWxml, "wx:for=\"{{todayPlanSections}}\"");
assertIncludes("pages/home/home.wxml", homeWxml, "wx:for=\"{{feedbackOptions}}\"");
assertIncludes("pages/home/home.wxml", homeWxml, "bind:tap=\"handleFeedback\"");
assertIncludes("pages/home/home.wxml", homeWxml, "bind:tap=\"handlePrimaryAction\"");
assertIncludes("pages/home/home.wxml", homeWxml, "bind:tap=\"handleSceneChange\"");
assertIncludes("pages/home/home.wxss", homeWxss, "#245d4f");
assertIncludes("pages/home/home.wxss", homeWxss, "today-visual");
assertIncludes("pages/home/home.wxss", homeWxss, "feedback-chip");

if (!mock.todayRecommendation || mock.todayRecommendation.label !== "今日推荐") {
  throw new Error("mock should export todayRecommendation with label 今日推荐");
}

if (!Array.isArray(mock.todayPlanSections) || mock.todayPlanSections.length < 4) {
  throw new Error("mock should export at least four todayPlanSections");
}

if (!Array.isArray(mock.feedbackOptions) || !mock.feedbackOptions.includes("我实际这样穿了")) {
  throw new Error("mock should export feedbackOptions with real-world feedback");
}

if (!homePageConfig) {
  throw new Error("home.js should register a Page config");
}

if (homePageConfig.data.todayRecommendation !== mock.todayRecommendation) {
  throw new Error("home page data should use exported todayRecommendation");
}

if (homePageConfig.data.todayPlanSections !== mock.todayPlanSections) {
  throw new Error("home page data should use exported todayPlanSections");
}

if (homePageConfig.data.feedbackOptions !== mock.feedbackOptions) {
  throw new Error("home page data should use exported feedbackOptions");
}

["handlePrimaryAction", "handleSceneChange", "handleFeedback"].forEach((handlerName) => {
  if (typeof homePageConfig[handlerName] !== "function") {
    throw new Error(`home page should define ${handlerName}`);
  }
});

console.log("today ui verification passed");
