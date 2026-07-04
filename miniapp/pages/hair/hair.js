const api = require("../../utils/api");

function normalizeItems(result) {
  return result && Array.isArray(result.items) ? result.items : [];
}

const hairPageConfig = {
  data: {
    loading: false,
    errorMessage: "",
    items: []
  },

  onLoad() {
    return this.loadHair();
  },

  async loadHair() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const result = typeof api.getHairItems === "function" ? await api.getHairItems() : { items: [] };
      this.setData({
        loading: false,
        items: normalizeItems(result)
      });
    } catch (error) {
      this.setData({
        loading: false,
        items: [],
        errorMessage: error && error.message ? error.message : "读取发型失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(hairPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    hairPageConfig,
    normalizeItems
  };
}
