const api = require("../../utils/api");
const privateItem = require("../../utils/private-item");

function getPublicID(options) {
  return options && (options.public_id || options.publicId || options.id) ? options.public_id || options.publicId || options.id : "";
}

function confirmDelete() {
  if (typeof wx === "undefined" || !wx.showModal) {
    return Promise.resolve(true);
  }
  return new Promise((resolve) => {
    wx.showModal({
      title: "删除发型",
      content: "删除后不会再用于后续建议。",
      confirmText: "删除",
      success(result) {
        resolve(Boolean(result && result.confirm));
      },
      fail() {
        resolve(false);
      }
    });
  });
}

const hairDetailPageConfig = {
  data: {
    loading: false,
    errorMessage: "",
    itemPublicID: "",
    item: null
  },

  onLoad(options) {
    return this.loadItem(getPublicID(options));
  },

  async loadItem(publicID) {
    if (!publicID) {
      this.setData({ errorMessage: "未找到发型", item: null });
      return null;
    }
    this.setData({ loading: true, errorMessage: "", itemPublicID: publicID });
    try {
      const result = await api.getHairItem(publicID);
      const item = privateItem.decorateHairItem(result);
      this.setData({ loading: false, item });
      return item;
    } catch (error) {
      this.setData({
        loading: false,
        item: null,
        errorMessage: error && error.message ? error.message : "读取发型详情失败"
      });
      return null;
    }
  },

  handleOpenEdit() {
    if (this.data.item && typeof wx !== "undefined" && wx.navigateTo) {
      wx.navigateTo({ url: `/pages/hair/edit?public_id=${this.data.item.public_id}` });
    }
  },

  async handleDelete() {
    const publicID = this.data.itemPublicID;
    if (!publicID || !(await confirmDelete())) {
      return;
    }
    try {
      await api.deleteHairItem(publicID);
      if (typeof wx !== "undefined" && wx.navigateBack) {
        wx.navigateBack({ delta: 1 });
      }
    } catch (error) {
      this.setData({ errorMessage: error && error.message ? error.message : "删除发型失败" });
    }
  }
};

if (typeof Page === "function") {
  Page(hairDetailPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    hairDetailPageConfig
  };
}
