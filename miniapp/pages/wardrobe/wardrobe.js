const api = require("../../utils/api");

function normalizeWardrobeGaps(report) {
  const content = report && report.content_json ? report.content_json : {};
  if (!Array.isArray(content.wardrobe_gaps)) {
    return [];
  }
  return content.wardrobe_gaps
    .map((item) => {
      if (typeof item === "string") {
        return item;
      }
      return item.title || item.name || item.description || "";
    })
    .filter(Boolean);
}

const wardrobePageConfig = {
  data: {
    loading: false,
    errorMessage: "",
    gaps: []
  },

  onLoad() {
    return this.loadWardrobeGaps();
  },

  async loadWardrobeGaps() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const report = await api.getLatestReport();
      this.setData({
        loading: false,
        gaps: normalizeWardrobeGaps(report)
      });
    } catch (error) {
      this.setData({
        loading: false,
        gaps: [],
        errorMessage: error && error.message ? error.message : "读取衣橱缺口失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(wardrobePageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    normalizeWardrobeGaps,
    wardrobePageConfig
  };
}
