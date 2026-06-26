const api = require("../../utils/api");

const emptyReportData = {
  loading: false,
  errorMessage: "",
  empty: false,
  updatedAt: "",
  title: "",
  summary: "",
  referenceStyleLogic: "",
  wardrobeGaps: [],
  routes: [],
  actionItems: []
};

function normalizeTextList(value) {
  if (!Array.isArray(value)) {
    return value ? [String(value)] : [];
  }

  return value.filter(Boolean).map((item) => String(item));
}

function normalizeActionItems(items, avoidances) {
  if (!Array.isArray(items)) {
    return [];
  }

  const avoidanceTexts = normalizeTextList(avoidances);

  return items.map((item, index) => {
    if (typeof item === "string") {
      return {
        title: item,
        reason: "",
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
      public_id: route.public_id || "",
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
    exploration: "探索路线"
  };

  return roleMap[role] || "";
}

function normalizeReportResponse(report) {
  const content = report && report.content_json ? report.content_json : {};

  return {
    loading: false,
    errorMessage: "",
    empty: false,
    updatedAt: report.updated_at || report.created_at || "",
    title: report.title || "初版个人形象报告",
    summary: report.summary || content.summary || "",
    referenceStyleLogic: content.reference_style_logic || "",
    wardrobeGaps: Array.isArray(content.wardrobe_gaps) ? content.wardrobe_gaps : [],
    routes: normalizeRoutes(report.routes),
    actionItems: normalizeActionItems(content.action_items, content.avoidances)
  };
}

const reportPageConfig = {
  data: emptyReportData,

  onLoad() {
    return this.loadLatestReport();
  },

  async loadLatestReport() {
    this.setData({
      loading: true,
      errorMessage: "",
      empty: false
    });

    try {
      const report = await api.getLatestReport();
      this.setData(normalizeReportResponse(report));
    } catch (error) {
      this.setData(Object.assign({}, emptyReportData, {
        empty: error && error.code === "report.not_found",
        errorMessage: error && error.message ? error.message : "读取报告失败"
      }));
    }
  }
};

if (typeof Page === "function") {
  Page(reportPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    normalizeReportResponse,
    reportPageConfig
  };
}
