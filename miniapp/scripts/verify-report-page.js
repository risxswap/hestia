const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const apiPath = path.join(root, "utils/api.js");
const reportPath = path.join(root, "pages/report/report.js");

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

async function main() {
  const source = read("pages/report/report.js");
  const wxml = read("pages/report/report.wxml");
  assert(source.includes("../../utils/api"), "report page should import shared api client");
  assert(!source.includes("utils/mock"), "report page should not import mock data");
  assert(wxml.includes("loadLatestReport"), "report page should expose retry through loadLatestReport");

  let pageConfig;
  const originalPage = global.Page;
  global.Page = (config) => {
    pageConfig = config;
  };
  require.cache[require.resolve(apiPath)] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: {
      getLatestReport: async () => ({
        title: "初版个人形象报告",
        summary: "先用清爽线条建立稳定的第一印象。",
        content_json: {
          action_items: ["固定一套通勤模板"],
          avoidances: ["避免上下都宽松"]
        },
        routes: [
          {
            name: "干净 + 有气质",
            route_role: "primary",
            target_impression: ["干净"],
            reason: ["适合通勤和见客户"]
          }
        ]
      })
    }
  };
  delete require.cache[require.resolve(reportPath)];
  const reportModule = require(reportPath);
  global.Page = originalPage;
  delete require.cache[require.resolve(apiPath)];

  assert(pageConfig, "report.js should register a Page config");
  assert(typeof pageConfig.loadLatestReport === "function", "report page should define loadLatestReport");
  assert(typeof reportModule.normalizeReportResponse === "function", "report page should export normalizeReportResponse");
  assert(!reportModule.fallbackReportData, "report page should not export mock fallback data");

  const instance = {
    data: JSON.parse(JSON.stringify(pageConfig.data)),
    setData(payload) {
      this.data = Object.assign({}, this.data, payload);
    }
  };
  await pageConfig.loadLatestReport.call(instance);
  assert(instance.data.title === "初版个人形象报告", "report page should set server title");
  assert(instance.data.actionItems[0].title === "固定一套通勤模板", "report page should normalize server action items");
  assert(instance.data.routes[0].roleLabel === "主路线", "report page should normalize route role label");
}

main()
  .then(() => {
    console.log("report page verification passed");
  })
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
