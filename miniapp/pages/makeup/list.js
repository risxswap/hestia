const api = require("../../utils/api");
const privateItem = require("../../utils/private-item");

function getDataset(event) {
  return event && event.currentTarget && event.currentTarget.dataset ? event.currentTarget.dataset : {};
}

function valueOf(event) {
  return event && event.detail && Object.prototype.hasOwnProperty.call(event.detail, "value") ? event.detail.value : "";
}

function showToast(title, icon) {
  if (typeof wx !== "undefined" && wx.showToast) {
    wx.showToast({ title, icon: icon || "none" });
  }
}

const makeupPageConfig = {
  data: {
    loading: false,
    saving: false,
    errorMessage: "",
    modalVisible: false,
    items: [],
    draft: privateItem.baseDraft({ makeup_type: "", focus: "", color_palette: "", finish: "" })
  },

  onShow() {
    return this.loadMakeup();
  },

  async loadMakeup() {
    this.setData({ loading: true, errorMessage: "" });
    try {
      const result = typeof api.getMakeupItems === "function" ? await api.getMakeupItems() : { items: [] };
      this.setData({
        loading: false,
        items: privateItem.normalizeItems(result, privateItem.decorateMakeupItem)
      });
    } catch (error) {
      this.setData({
        loading: false,
        items: [],
        errorMessage: error && error.message ? error.message : "读取妆容失败"
      });
    }
  },

  handleOpenCreate() {
    this.setData({
      modalVisible: true,
      errorMessage: "",
      draft: privateItem.baseDraft({ makeup_type: "", focus: "", color_palette: "", finish: "" })
    });
  },

  handleCloseCreate() {
    this.setData({ modalVisible: false, saving: false });
  },

  handleInput(event) {
    const field = getDataset(event).field;
    if (!field) {
      return;
    }
    this.setData({
      draft: Object.assign({}, this.data.draft, {
        [field]: valueOf(event)
      })
    });
  },

  async handleSaveCreate() {
    const payload = privateItem.payloadFromDraft(this.data.draft);
    if (!payload.name) {
      this.setData({ errorMessage: "请填写妆容名称" });
      return;
    }
    this.setData({ saving: true, errorMessage: "" });
    try {
      const saved = await api.createMakeupItem(payload);
      const item = privateItem.decorateMakeupItem(saved);
      this.setData({
        saving: false,
        modalVisible: false,
        items: this.data.items.concat(item),
        draft: privateItem.baseDraft({ makeup_type: "", focus: "", color_palette: "", finish: "" })
      });
      showToast("已添加", "success");
    } catch (error) {
      this.setData({
        saving: false,
        errorMessage: error && error.message ? error.message : "保存妆容失败"
      });
    }
  },

  handleOpenDetail(event) {
    const publicID = getDataset(event).publicId || getDataset(event).public_id || "";
    if (publicID && typeof wx !== "undefined" && wx.navigateTo) {
      wx.navigateTo({ url: `/pages/makeup/detail?public_id=${publicID}` });
    }
  }
};

if (typeof Page === "function") {
  Page(makeupPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    makeupPageConfig
  };
}
