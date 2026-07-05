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

const hairPageConfig = {
  data: {
    loading: false,
    saving: false,
    errorMessage: "",
    modalVisible: false,
    items: [],
    draft: privateItem.baseDraft({ length: "", shape: "", bangs: "", color: "", care_time: "" })
  },

  onShow() {
    return this.loadHair();
  },

  async loadHair() {
    this.setData({ loading: true, errorMessage: "" });
    try {
      const result = typeof api.getHairItems === "function" ? await api.getHairItems() : { items: [] };
      this.setData({
        loading: false,
        items: privateItem.normalizeItems(result, privateItem.decorateHairItem)
      });
    } catch (error) {
      this.setData({
        loading: false,
        items: [],
        errorMessage: error && error.message ? error.message : "读取发型失败"
      });
    }
  },

  handleOpenCreate() {
    this.setData({
      modalVisible: true,
      errorMessage: "",
      draft: privateItem.baseDraft({ length: "", shape: "", bangs: "", color: "", care_time: "" })
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
      this.setData({ errorMessage: "请填写发型名称" });
      return;
    }
    this.setData({ saving: true, errorMessage: "" });
    try {
      const saved = await api.createHairItem(payload);
      const item = privateItem.decorateHairItem(saved);
      this.setData({
        saving: false,
        modalVisible: false,
        items: this.data.items.concat(item),
        draft: privateItem.baseDraft({ length: "", shape: "", bangs: "", color: "", care_time: "" })
      });
      showToast("已添加", "success");
    } catch (error) {
      this.setData({
        saving: false,
        errorMessage: error && error.message ? error.message : "保存发型失败"
      });
    }
  },

  handleOpenDetail(event) {
    const publicID = getDataset(event).publicId || getDataset(event).public_id || "";
    if (publicID && typeof wx !== "undefined" && wx.navigateTo) {
      wx.navigateTo({ url: `/pages/hair/detail?public_id=${publicID}` });
    }
  }
};

if (typeof Page === "function") {
  Page(hairPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    hairPageConfig
  };
}
