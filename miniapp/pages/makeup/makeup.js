const api = require("../../utils/api");

function normalizeItems(result) {
  return result && Array.isArray(result.items) ? result.items : [];
}

const makeupPageConfig = {
  data: {
    loading: false,
    errorMessage: "",
    items: []
  },

  onLoad() {
    return this.loadMakeup();
  },

  async loadMakeup() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const result = typeof api.getMakeupItems === "function" ? await api.getMakeupItems() : { items: [] };
      this.setData({
        loading: false,
        items: normalizeItems(result)
      });
    } catch (error) {
      this.setData({
        loading: false,
        items: [],
        errorMessage: error && error.message ? error.message : "读取妆容失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(makeupPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    makeupPageConfig,
    normalizeItems
  };
}
