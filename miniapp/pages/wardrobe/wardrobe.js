const api = require("../../utils/api");
const wardrobeUtils = require("../../utils/wardrobe");

const {
  categoryOptions,
  categoryLabels,
  recommendationLabels,
  cloneDraft,
  normalizeSceneTags,
  normalizeWardrobeItems,
  decorateWardrobeItem,
  filterItems,
  priorityItems,
  normalizeWardrobeGaps,
  buildPayload,
  imageFilesFromItem,
  imageFilesFromAsset,
  itemToDraft,
  categoryOptionsWithCounts,
  styleLogicForItem
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

function getUploadFile(event) {
  const detail = event && event.detail ? event.detail : {};
  if (detail.file) {
    return detail.file;
  }
  const files = detail.files || detail.fileList || detail.currentFiles;
  if (Array.isArray(files) && files.length) {
    return files[0];
  }
  return null;
}

function filePreviewUrl(file) {
  const source = file || {};
  return source.url || source.path || source.tempFilePath || "";
}

function pendingImageFiles(file) {
  const url = filePreviewUrl(file);
  if (!url) {
    return [];
  }
  return [
    {
      url,
      name: file.name || "衣服主图",
      type: "image",
      status: "loading"
    }
  ];
}

function failedImageFiles(file, message) {
  const pending = pendingImageFiles(file);
  if (!pending.length) {
    return [];
  }
  return pending.map((item) => Object.assign({}, item, {
    status: "failed",
    message
  }));
}

function confirmDelete() {
  if (typeof wx === "undefined" || !wx.showModal) {
    return Promise.resolve(true);
  }

  return new Promise((resolve) => {
    wx.showModal({
      title: "删除核心单品",
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

function nextWardrobeState(items, category) {
  return {
    items,
    visibleItems: filterItems(items, category),
    priorityItems: priorityItems(items),
    categoryOptions: categoryOptionsWithCounts(items)
  };
}

const wardrobePageConfig = {
  data: {
    loading: false,
    saving: false,
    errorMessage: "",
    items: [],
    visibleItems: [],
    priorityItems: [],
    gaps: [],
    categoryOptions: categoryOptionsWithCounts([]),
    activeCategory: "all",
    wardrobeDirty: false,
    editorVisible: false,
    draft: cloneDraft(),
    editingPublicID: "",
    imageFiles: [],
    imageUploading: false,
    imageUploadError: "",
    imageGridConfig: {
      column: 4,
      width: 160,
      height: 160
    },
    imageMediaType: ["image"],
    imageSizeLimit: {
      size: 8,
      unit: "MB",
      message: "图片大小不超过 8MB"
    }
  },

  onLoad() {
    return this.loadWardrobe();
  },

  async onShow() {
    const app = typeof getApp === "function" ? getApp() : null;
    const globalData = app && app.globalData ? app.globalData : {};
    if (globalData.wardrobeDirty) {
      this.setData({
        wardrobeDirty: true
      });
      await this.loadWardrobe();
      if (!this.data.errorMessage) {
        globalData.wardrobeDirty = false;
        this.setData({
          wardrobeDirty: false
        });
      }
    }
    return Promise.resolve();
  },

  async loadWardrobe() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const reportRequest = typeof api.getLatestReport === "function" ? api.getLatestReport() : Promise.resolve(null);
      const itemRequest = typeof api.getWardrobeItems === "function" ? api.getWardrobeItems() : Promise.resolve({ items: [] });
      const results = await Promise.all([itemRequest, reportRequest]);
      const items = normalizeWardrobeItems(results[0]);

      this.setData(Object.assign({
        loading: false,
        gaps: normalizeWardrobeGaps(results[1])
      }, nextWardrobeState(items, this.data.activeCategory)));
    } catch (error) {
      this.setData({
        loading: false,
        items: [],
        visibleItems: [],
        priorityItems: [],
        categoryOptions: categoryOptionsWithCounts([]),
        gaps: [],
        errorMessage: error && error.message ? error.message : "读取核心衣橱失败"
      });
    }
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
  },

  handleCategoryFilter(event) {
    const dataset = getDataset(event);
    const category = dataset.category || dataset.value || getDetailValue(event) || "all";
    this.setData({
      activeCategory: category,
      visibleItems: filterItems(this.data.items, category)
    });
  },

  handleOpenDetail(event) {
    const dataset = getDataset(event);
    const publicID = dataset.publicId || dataset.public_id || dataset.id || "";
    if (!publicID) {
      this.setData({
        errorMessage: "未找到要查看的单品"
      });
      return;
    }

    if (typeof wx !== "undefined" && wx.navigateTo) {
      wx.navigateTo({
        url: `/pages/wardrobe-detail/wardrobe-detail?public_id=${publicID}`
      });
    }
  },

  handleOpenCreate() {
    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    const category = this.data.activeCategory && this.data.activeCategory !== "all"
      ? this.data.activeCategory
      : "top";
    this.setData({
      editorVisible: true,
      editingPublicID: "",
      draft: cloneDraft({
        category
      }),
      imageFiles: [],
      imageUploading: false,
      imageUploadError: "",
      errorMessage: ""
    });
  },

  handleCloseEditor() {
    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    this.setData({
      editorVisible: false,
      editingPublicID: "",
      draft: cloneDraft(),
      imageFiles: [],
      imageUploading: false,
      imageUploadError: "",
      errorMessage: ""
    });
    this._imageUploadPromise = null;
  },

  handleEditItem(event) {
    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    const dataset = getDataset(event);
    const publicID = dataset.publicId || dataset.public_id || dataset.id || "";
    const item = this.data.items.find((entry) => entry.public_id === publicID);
    if (!item) {
      this.setData({
        errorMessage: "未找到要编辑的单品"
      });
      return;
    }

    this.setData({
      editorVisible: true,
      editingPublicID: publicID,
      draft: itemToDraft(item),
      imageFiles: imageFilesFromItem(item),
      imageUploading: false,
      imageUploadError: "",
      errorMessage: ""
    });
  },

  handleImageUpload(event) {
    if (this.data.imageUploading) {
      return this._imageUploadPromise || Promise.resolve([]);
    }

    const file = getUploadFile(event);
    if (!file) {
      return Promise.resolve([]);
    }

    this.setData({
      imageFiles: pendingImageFiles(file),
      imageUploading: true,
      imageUploadError: ""
    });

    const uploadRunID = (this._imageUploadRunID || 0) + 1;
    this._imageUploadRunID = uploadRunID;
    this._imageUploadPromise = api.uploadAssetToQiniu(file, {
      assetType: "wardrobe_item_photo"
    })
      .then((uploaded) => {
        if (this._imageUploadRunID !== uploadRunID) {
          return uploaded;
        }
        const draft = Object.assign({}, this.data.draft, {
          primary_asset_public_id: uploaded && uploaded.asset_public_id ? uploaded.asset_public_id : ""
        });
        this.setData({
          draft,
          imageFiles: imageFilesFromAsset(uploaded),
          imageUploading: false,
          imageUploadError: ""
        });
        this._imageUploadPromise = null;
        return uploaded;
      })
      .catch((error) => {
        if (this._imageUploadRunID !== uploadRunID) {
          return null;
        }
        const message = error && error.message ? error.message : "图片上传失败";
        this.setData({
          imageFiles: failedImageFiles(file, message),
          imageUploading: false,
          imageUploadError: message
        });
        this._imageUploadPromise = null;
        return null;
      });

    return this._imageUploadPromise;
  },

  handleImageRemove() {
    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    this._imageUploadPromise = null;
    this.setData({
      imageFiles: [],
      imageUploading: false,
      imageUploadError: "",
      draft: Object.assign({}, this.data.draft, {
        primary_asset_public_id: ""
      })
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
    if (this.data.imageUploading) {
      this.setData({
        errorMessage: "图片还在上传，请稍后再保存"
      });
      return;
    }

    const payload = buildPayload(this.data.draft);
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
      const saved = this.data.editingPublicID
        ? await api.updateWardrobeItem(this.data.editingPublicID, payload)
        : await api.createWardrobeItem(payload);
      const normalizedSaved = decorateWardrobeItem(saved);
      const currentItems = this.data.items.slice();
      const existingIndex = currentItems.findIndex((item) => item.public_id === normalizedSaved.public_id);
      const nextItems = existingIndex >= 0
        ? currentItems.map((item, index) => (index === existingIndex ? normalizedSaved : item))
        : currentItems.concat(normalizedSaved);

      this.setData(Object.assign({
        saving: false,
        editorVisible: false,
        editingPublicID: "",
        draft: cloneDraft(),
        imageFiles: [],
        imageUploading: false,
        imageUploadError: ""
      }, nextWardrobeState(nextItems, this.data.activeCategory)));
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
    const publicID = dataset.publicId || dataset.public_id || dataset.id || "";
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
      const nextItems = this.data.items.filter((item) => item.public_id !== publicID);
      this.setData(Object.assign({
        errorMessage: "",
        editorVisible: false,
        editingPublicID: "",
        draft: cloneDraft(),
        imageFiles: [],
        imageUploading: false,
        imageUploadError: ""
      }, nextWardrobeState(nextItems, this.data.activeCategory)));
      showToast("已删除", "success");
    } catch (error) {
      this.setData({
        errorMessage: error && error.message ? error.message : "删除单品失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(wardrobePageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    categoryOptions,
    categoryLabels,
    recommendationLabels,
    cloneDraft,
    normalizeSceneTags,
    normalizeWardrobeItems,
    decorateWardrobeItem,
    filterItems,
    priorityItems,
    normalizeWardrobeGaps,
    buildPayload,
    itemToDraft,
    categoryOptionsWithCounts,
    styleLogicForItem,
    nextWardrobeState,
    wardrobePageConfig
  };
}
