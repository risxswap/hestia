const api = require("../../utils/api");

function normalizeItems(result) {
  return result && Array.isArray(result.items) ? result.items : [];
}

const referencesPageConfig = {
  data: {
    loading: false,
    errorMessage: "",
    items: []
  },

  onLoad() {
    return this.loadReferences();
  },

  async loadReferences() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const result = typeof api.getReferenceItems === "function" ? await api.getReferenceItems() : { items: [] };
      this.setData({
        loading: false,
        items: normalizeItems(result)
      });
    } catch (error) {
      this.setData({
        loading: false,
        items: [],
        errorMessage: error && error.message ? error.message : "读取参考失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(referencesPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    referencesPageConfig,
    normalizeItems
  };
}
