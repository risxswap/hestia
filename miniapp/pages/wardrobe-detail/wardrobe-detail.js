const api = require("../../utils/api");
const wardrobeUtils = require("../../utils/wardrobe");

const {
  cloneDraft,
  normalizeSceneTags,
  normalizeWardrobeItems,
  decorateWardrobeItem,
  buildPayload,
  itemToDraft
} = wardrobeUtils;

function getDataset(event) {
  return event && event.currentTarget && event.currentTarget.dataset ? event.currentTarget.dataset : {};
}

function getDetailValue(event) {
  return event && event.detail && Object.prototype.hasOwnProperty.call(event.detail, "value")
    ? event.detail.value
    : "";
}

function showToast(title, icon) {
  if (typeof wx !== "undefined" && wx.showToast) {
    wx.showToast({
      title,
      icon: icon || "none"
    });
  }
}

function confirmDelete() {
  if (typeof wx === "undefined" || !wx.showModal) {
    return Promise.resolve(true);
  }

  return new Promise((resolve) => {
    wx.showModal({
      title: "删除单品",
      content: "删除后不会再用于后续穿搭建议。",
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

const wardrobeDetailPageConfig = {
  data: {
    loading: false,
    saving: false,
    errorMessage: "",
    itemPublicID: "",
    item: null,
    wardrobeDirty: false,
    editorVisible: false,
    draft: cloneDraft(),
    editingPublicID: ""
  },

  onLoad(options) {
    const publicID = options && (options.public_id || options.publicId || options.id) ? options.public_id || options.publicId || options.id : "";
    return this.loadWardrobeItem(publicID);
  },

  async loadWardrobeItem(publicID) {
    if (!publicID) {
      this.setData({
        loading: false,
        errorMessage: "未找到要查看的单品",
        itemPublicID: "",
        item: null
      });
      return null;
    }

    this.setData({
      loading: true,
      errorMessage: "",
      itemPublicID: publicID
    });

    try {
      const response = await api.getWardrobeItems();
      const items = normalizeWardrobeItems(response);
      const item = items.find((entry) => entry.public_id === publicID);
      if (!item) {
        this.setData({
          loading: false,
          item: null,
          errorMessage: "没有找到这件衣服"
        });
        return null;
      }

      this.setData({
        loading: false,
        item,
        errorMessage: ""
      });
      return item;
    } catch (error) {
      this.setData({
        loading: false,
        item: null,
        errorMessage: error && error.message ? error.message : "读取衣服详情失败"
      });
      return null;
    }
  },

  handleBack() {
    if (typeof wx !== "undefined" && wx.switchTab) {
      wx.switchTab({
        url: "/pages/wardrobe/wardrobe"
      });
    }
  },

  markWardrobeDirty() {
    const app = typeof getApp === "function" ? getApp() : null;
    if (app && app.globalData) {
      app.globalData.wardrobeDirty = true;
    }
    this.setData({
      wardrobeDirty: true
    });
  },

  handleOpenEdit() {
    if (!this.data.item) {
      this.setData({
        errorMessage: "未找到要编辑的单品"
      });
      return;
    }

    this.setData({
      editorVisible: true,
      editingPublicID: this.data.item.public_id,
      draft: itemToDraft(this.data.item),
      errorMessage: ""
    });
  },

  handleCloseEditor() {
    this.setData({
      editorVisible: false,
      editingPublicID: "",
      draft: cloneDraft(),
      errorMessage: ""
    });
  },

  handleDraftInput(event) {
    const field = getDataset(event).field;
    if (!field) {
      return;
    }

    const draft = Object.assign({}, this.data.draft);
    draft[field] = getDetailValue(event);
    if (field === "sceneText") {
      draft.scene_tags = normalizeSceneTags(draft.sceneText);
    }
    this.setData({
      draft
    });
  },

  handleSceneInput(event) {
    const sceneText = getDetailValue(event);
    const draft = Object.assign({}, this.data.draft, {
      sceneText,
      scene_tags: normalizeSceneTags(sceneText)
    });
    this.setData({
      draft
    });
  },

  handleRecommendationStatus(event) {
    const dataset = getDataset(event);
    const status = dataset.status || dataset.value || getDetailValue(event) || "normal";
    this.setData({
      draft: Object.assign({}, this.data.draft, {
        recommendation_status: status
      })
    });
  },

  handleCoreToggle(event) {
    const detail = event && event.detail ? event.detail : {};
    const dataset = getDataset(event);
    let value = detail.value;
    if (typeof value === "undefined") {
      value = detail.checked;
    }
    if (typeof value === "undefined" && Object.prototype.hasOwnProperty.call(dataset, "value")) {
      value = dataset.value;
    }
    if (typeof value === "undefined") {
      value = !this.data.draft.is_core;
    }

    this.setData({
      draft: Object.assign({}, this.data.draft, {
        is_core: value === true || value === "true" || value === 1 || value === "1"
      })
    });
  },

  async handleSaveItem() {
    const publicID = this.data.editingPublicID || this.data.itemPublicID;
    const payload = buildPayload(this.data.draft);
    if (!publicID) {
      this.setData({
        errorMessage: "未找到要保存的单品"
      });
      return;
    }
    if (!payload.name || !payload.category) {
      this.setData({
        errorMessage: "请填写单品名称和分类"
      });
      return;
    }

    this.setData({
      saving: true,
      errorMessage: ""
    });

    try {
      const saved = await api.updateWardrobeItem(publicID, payload);
      const item = decorateWardrobeItem(saved);
      this.setData({
        saving: false,
        editorVisible: false,
        editingPublicID: "",
        draft: cloneDraft(),
        item,
        itemPublicID: item.public_id
      });
      this.markWardrobeDirty();
      showToast("已保存", "success");
    } catch (error) {
      this.setData({
        saving: false,
        errorMessage: error && error.message ? error.message : "保存单品失败"
      });
    }
  },

  async handleDeleteItem(event) {
    const dataset = getDataset(event);
    const publicID = dataset.publicId || dataset.public_id || dataset.id || this.data.editingPublicID || this.data.itemPublicID;
    if (!publicID) {
      this.setData({
        errorMessage: "未找到要删除的单品"
      });
      return;
    }

    const confirmed = await confirmDelete();
    if (!confirmed) {
      return;
    }

    try {
      await api.deleteWardrobeItem(publicID);
      this.markWardrobeDirty();
      showToast("已删除", "success");
      if (typeof wx !== "undefined" && wx.switchTab) {
        wx.switchTab({
          url: "/pages/wardrobe/wardrobe"
        });
      }
    } catch (error) {
      this.setData({
        errorMessage: error && error.message ? error.message : "删除单品失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(wardrobeDetailPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    wardrobeDetailPageConfig
  };
}
