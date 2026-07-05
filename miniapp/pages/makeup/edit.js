const api = require("../../utils/api");
const privateItem = require("../../utils/private-item");

function getPublicID(options) {
  return options && (options.public_id || options.publicId || options.id) ? options.public_id || options.publicId || options.id : "";
}

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

const makeupEditPageConfig = {
  data: {
    loading: false,
    saving: false,
    errorMessage: "",
    itemPublicID: "",
    draft: privateItem.baseDraft({ makeup_type: "", focus: "", color_palette: "", finish: "" })
  },

  onLoad(options) {
    return this.loadItem(getPublicID(options));
  },

  async loadItem(publicID) {
    if (!publicID) {
      this.setData({ errorMessage: "未找到妆容" });
      return null;
    }
    this.setData({ loading: true, itemPublicID: publicID, errorMessage: "" });
    try {
      const result = await api.getMakeupItem(publicID);
      this.setData({
        loading: false,
        draft: privateItem.makeupDraft(result)
      });
      return result;
    } catch (error) {
      this.setData({
        loading: false,
        errorMessage: error && error.message ? error.message : "读取妆容失败"
      });
      return null;
    }
  },

  handleInput(event) {
    const field = getDataset(event).field;
    if (!field) {
      return;
    }
    const draft = Object.assign({}, this.data.draft, { [field]: valueOf(event) });
    if (field === "sceneText") {
      draft.scene_tags = privateItem.normalizeTags(draft.sceneText);
    }
    this.setData({ draft });
  },

  handleRecommendationStatus(event) {
    this.setData({
      draft: Object.assign({}, this.data.draft, {
        recommendation_status: getDataset(event).status || "normal"
      })
    });
  },

  async handleSave() {
    const payload = privateItem.payloadFromDraft(this.data.draft);
    if (!payload.name) {
      this.setData({ errorMessage: "请填写妆容名称" });
      return;
    }
    this.setData({ saving: true, errorMessage: "" });
    try {
      await api.updateMakeupItem(this.data.itemPublicID, payload);
      this.setData({ saving: false });
      showToast("已保存", "success");
      if (typeof wx !== "undefined" && wx.navigateBack) {
        wx.navigateBack({ delta: 1 });
      }
    } catch (error) {
      this.setData({ saving: false, errorMessage: error && error.message ? error.message : "保存妆容失败" });
    }
  }
};

if (typeof Page === "function") {
  Page(makeupEditPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    makeupEditPageConfig
  };
}
