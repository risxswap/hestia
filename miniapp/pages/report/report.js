const { actionItems } = require("../../utils/mock");

const fallbackReportData = {
  updatedAt: "",
  title: "你的下一步",
  summary: "当前展示本地预览报告。后端报告未准备好时，你仍然可以先按这些行动项试穿并记录反馈。",
  routes: [
    {
      name: "干净 + 有气质",
      route_role: "local_preview",
      roleLabel: "本地预览",
      reasonText: "用利落外套、清爽发型和稳定配色先建立可执行的第一版形象路线。"
    }
  ],
  actionItems
};

function normalizeTextList(value) {
  if (!Array.isArray(value)) {
    return value ? [String(value)] : [];
  }

  return value.filter(Boolean).map((item) => String(item));
}

function normalizeActionItems(items, avoidances) {
  if (!Array.isArray(items) || items.length === 0) {
    return fallbackReportData.actionItems;
  }

  const avoidanceTexts = normalizeTextList(avoidances);

  return items.map((item, index) => {
    if (typeof item === "string") {
      return {
        title: item,
        reason: "建议先在下一次真实场景中尝试，并记录采纳或修改反馈。",
        avoid: avoidanceTexts[index] || ""
      };
    }

    return {
      title: item.title || item.name || item.text || `行动项 ${index + 1}`,
      reason: item.reason || item.description || "",
      avoid: item.avoid || item.avoidance || avoidanceTexts[index] || ""
    };
  });
}

function normalizeRoutes(routes) {
  if (!Array.isArray(routes)) {
    return [];
  }

  return routes.map((route, index) => {
    const reasons = normalizeTextList(route.reason);
    const impressions = normalizeTextList(route.target_impression);

    return {
      name: route.name || `路线 ${index + 1}`,
      route_role: route.route_role || "",
      roleLabel: normalizeRouteRole(route.route_role),
      reason: reasons,
      reasonText: reasons.join("；"),
      target_impression: impressions,
      impressionText: impressions.join(" / ")
    };
  });
}

function normalizeRouteRole(role) {
  const roleMap = {
    primary: "主路线",
    scenario: "场景路线",
    explore: "探索路线",
    exploration: "探索路线",
    local_preview: "本地预览"
  };

  return roleMap[role] || "";
}

function normalizeReportResponse(report) {
  if (!report) {
    return fallbackReportData;
  }

  const content = report.content_json || {};

  return {
    updatedAt: report.updated_at || report.created_at || "",
    title: report.title || fallbackReportData.title,
    summary: report.summary || fallbackReportData.summary,
    referenceStyleLogic: content.reference_style_logic || "",
    wardrobeGaps: Array.isArray(content.wardrobe_gaps) ? content.wardrobe_gaps : [],
    routes: normalizeRoutes(report.routes),
    actionItems: normalizeActionItems(content.action_items, content.avoidances)
  };
}

const reportPageConfig = {
  data: fallbackReportData,

  onLoad() {
    this.loadLatestReport();
  },

  loadLatestReport() {
    const app = typeof getApp === "function" ? getApp() : null;
    const apiBaseUrl = app && app.globalData && app.globalData.apiBaseUrl;
    const token = wx.getStorageSync("user_token") || wx.getStorageSync("token");

    if (!apiBaseUrl || !token) {
      this.setData(fallbackReportData);
      return;
    }

    wx.request({
      url: `${apiBaseUrl.replace(/\/$/, "")}/api/user/reports/latest`,
      method: "GET",
      header: {
        Authorization: `Bearer ${token}`
      },
      success: (response) => {
        const body = response && response.data;

        if (body && body.code === "ok" && body.data) {
          this.setData(normalizeReportResponse(body.data));
          return;
        }

        this.setData(fallbackReportData);
      },
      fail: () => {
        this.setData(fallbackReportData);
      }
    });
  }
};

if (typeof Page === "function") {
  Page(reportPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    normalizeReportResponse,
    fallbackReportData,
    reportPageConfig
  };
}
