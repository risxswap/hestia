const api = require("../../utils/api");

const feedbackOptions = [
  { label: "照这个穿", action: "like" },
  { label: "不喜欢", action: "dislike" },
  { label: "想调整", action: "adjust" }
];

const emptyToday = {
  loading: false,
  errorMessage: "",
  hasReport: false,
  routePublicID: "",
  feedbackOptions,
  todayRecommendation: {
    context: "",
    label: "",
    title: "",
    summary: "",
    visualTitle: "",
    visualMeta: "",
    primaryAction: "照这个穿",
    secondaryAction: "聊聊调整"
  },
  todayPlanSections: [],
  memoryToast: ""
};

function normalizeTextList(value) {
  if (!Array.isArray(value)) {
    return value ? [String(value)] : [];
  }
  return value.filter(Boolean).map((item) => String(item));
}

function firstActionText(content) {
  const items = Array.isArray(content.action_items) ? content.action_items : [];
  if (!items.length) {
    return "";
  }
  const first = items[0];
  return typeof first === "string" ? first : first.title || first.name || first.text || "";
}

function strategyText(strategy) {
  if (!strategy || typeof strategy !== "object") {
    return "";
  }
  const steps = normalizeTextList(strategy.steps);
  const parts = [];
  if (strategy.direction) {
    parts.push(strategy.direction);
  }
  if (steps.length) {
    parts.push(steps.join("；"));
  }
  return parts.join("：");
}

function normalizeTodayFromReport(report) {
  if (!report) {
    return Object.assign({}, emptyToday, {
      hasReport: false,
      errorMessage: "完成 onboarding 后，这里会显示从服务端报告派生的今日建议。"
    });
  }

  const content = report && report.content_json ? report.content_json : {};
  const routes = Array.isArray(report && report.routes) ? report.routes : [];
  const route = routes[0] || {};
  const impressions = normalizeTextList(route.target_impression);
  const reasons = normalizeTextList(route.reason);
  const avoidPoints = normalizeTextList(route.avoid_points);
  const wardrobeGaps = normalizeTextList(content.wardrobe_gaps);
  const outfitText = strategyText(route.outfit_strategy || content.outfit_strategy);
  const hairText = strategyText(route.hair_strategy || content.hair_strategy);
  const actionText = firstActionText(content);

  const sections = [];
  if (reasons.length) {
    sections.push({
      title: "为什么适合你",
      body: reasons.join("；")
    });
  }
  if (outfitText || actionText) {
    sections.push({
      title: "今天先做",
      body: outfitText || actionText
    });
  }
  if (hairText) {
    sections.push({
      title: "发型方向",
      body: hairText
    });
  }
  if (avoidPoints.length) {
    sections.push({
      title: "今天不优先",
      body: avoidPoints.join("；")
    });
  }
  if (wardrobeGaps.length) {
    sections.push({
      title: "缺口单品",
      body: wardrobeGaps.join("；")
    });
  }

  return {
    loading: false,
    errorMessage: "",
    hasReport: true,
    routePublicID: route.public_id || "",
    feedbackOptions,
    todayRecommendation: {
      context: "来自最新初版报告",
      label: "今日建议",
      title: route.name || report.title || "今日形象建议",
      summary: actionText || report.summary || content.summary || "",
      visualTitle: impressions.length ? impressions.join(" / ") : "服务端报告建议",
      visualMeta: route.route_role === "primary" ? "主路线" : "形象路线",
      primaryAction: "照这个穿",
      secondaryAction: "聊聊调整"
    },
    todayPlanSections: sections,
    memoryToast: ""
  };
}

const homePageConfig = {
  data: emptyToday,

  onLoad() {
    return this.loadToday();
  },

  async loadToday() {
    this.setData({
      loading: true,
      errorMessage: "",
      memoryToast: ""
    });

    try {
      const report = await api.getLatestReport();
      this.setData(normalizeTodayFromReport(report));
    } catch (error) {
      this.setData(Object.assign({}, emptyToday, {
        hasReport: false,
        errorMessage: error && error.message ? error.message : "还没有可用建议"
      }));
    }
  },

  async handlePrimaryAction() {
    if (!this.data.routePublicID) {
      this.setData({
        memoryToast: "当前建议还没有可记录的路线，请先完成初版报告。"
      });
      return;
    }

    try {
      await api.sendImageRouteFeedback(this.data.routePublicID, "like", "小程序今日页采纳");
      this.setData({
        memoryToast: "已记录：你采纳了这条形象路线。"
      });
    } catch (error) {
      this.setData({
        memoryToast: error && error.message ? error.message : "反馈记录失败"
      });
    }
  },

  handleSceneChange() {
    if (typeof wx !== "undefined" && wx.switchTab) {
      wx.switchTab({
        url: "/pages/advisor/advisor"
      });
    }
  },

  handleStartOnboarding() {
    if (typeof wx !== "undefined" && wx.navigateTo) {
      wx.navigateTo({
        url: "/pages/onboarding/onboarding"
      });
    }
  },

  async handleFeedback(event) {
    const action = event.currentTarget.dataset.action;
    const label = event.currentTarget.dataset.value;
    if (!this.data.routePublicID) {
      this.setData({
        memoryToast: "当前建议还没有可记录的路线，请先完成初版报告。"
      });
      return;
    }

    try {
      await api.sendImageRouteFeedback(this.data.routePublicID, action, label);
      this.setData({
        memoryToast: `已记录：${label}`
      });
    } catch (error) {
      this.setData({
        memoryToast: error && error.message ? error.message : "反馈记录失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(homePageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    normalizeTodayFromReport,
    homePageConfig
  };
}
