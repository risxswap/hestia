const api = require("../../utils/api");
const wardrobeUtils = require("../../utils/wardrobe");

const {
  cloneDraft,
  normalizeSceneTags,
  normalizeWardrobeItems,
  normalizeWardrobeOptions,
  decorateWardrobeItem,
  buildPayload,
  imageFilesFromItem,
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

function optionsState(rawOptions) {
  const wardrobeOptions = normalizeWardrobeOptions(rawOptions);
  return {
    wardrobeOptions,
    editorCategoryOptions: wardrobeOptions.categories,
    colorOptions: wardrobeOptions.colors,
    materialOptions: wardrobeOptions.materials,
    seasonOptions: wardrobeOptions.seasons,
    silhouetteOptions: wardrobeOptions.silhouettes
  };
}

function suggestionTitle(field) {
  const titles = {
    category: "选择分类",
    color: "选择颜色",
    silhouette: "选择廓形",
    material: "选择材质",
    season: "选择季节"
  };
  return titles[field] || "选择";
}

function suggestionOptionsForField(state, field) {
  const source = state || {};
  const optionsMap = {
    category: source.editorCategoryOptions,
    color: source.colorOptions,
    silhouette: source.silhouetteOptions,
    material: source.materialOptions,
    season: source.seasonOptions
  };
  return optionsMap[field] || [];
}

function activeSuggestionState(state, field) {
  return {
    activeSuggestionField: field,
    activeSuggestionTitle: suggestionTitle(field),
    activeSuggestionOptions: field ? suggestionOptionsForField(state, field) : []
  };
}

function decorateDraftForOptions(draft, state) {
  return Object.assign({}, draft || {});
}

function draftState(draft, state) {
  return {
    draft: decorateDraftForOptions(draft, state)
  };
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
      name: file.name || "衣服图片",
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

function confirmedLocalImageFiles(file, uploaded) {
  const pending = pendingImageFiles(file);
  if (!pending.length) {
    return [];
  }
  return pending.map((item) => Object.assign({}, item, {
    status: "done",
    asset_public_id: uploaded && uploaded.asset_public_id ? uploaded.asset_public_id : "",
    object_key: uploaded && uploaded.object_key ? uploaded.object_key : ""
  }));
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

function confirmRecognizeOverwrite() {
  if (typeof wx === "undefined" || !wx.showModal) {
    return Promise.resolve(true);
  }

  return new Promise((resolve) => {
    wx.showModal({
      title: "重新识别图片",
      content: "重新识别会覆盖当前名称、分类、颜色、廓形、材质、季节、场景和备注，是否继续？",
      confirmText: "覆盖",
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
    editingPublicID: "",
    wardrobeOptions: normalizeWardrobeOptions(),
    editorCategoryOptions: normalizeWardrobeOptions().categories,
    colorOptions: normalizeWardrobeOptions().colors,
    materialOptions: normalizeWardrobeOptions().materials,
    seasonOptions: normalizeWardrobeOptions().seasons,
    silhouetteOptions: normalizeWardrobeOptions().silhouettes,
    activeSuggestionField: "",
    activeSuggestionTitle: "",
    activeSuggestionOptions: [],
    imageFiles: [],
    imageUploading: false,
    imageUploadError: "",
    imageRecognizing: false,
    imageRecognizeError: "",
    canRecognizeImage: false,
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

  onLoad(options) {
    const publicID = options && (options.public_id || options.publicId || options.id) ? options.public_id || options.publicId || options.id : "";
    return this.loadWardrobeItem(publicID);
  },

  async loadWardrobeItem(publicID) {
    if (!publicID) {
      this.setData({
        loading: false,
        errorMessage: "未找到要查看的私藏",
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
      const optionsRequest = typeof api.getWardrobeOptions === "function" ? api.getWardrobeOptions() : Promise.resolve(null);
      const itemRequest = api.getWardrobeItems();
      const results = await Promise.all([itemRequest, optionsRequest]);
      const items = normalizeWardrobeItems(results[0]);
      const optionState = optionsState(results[1]);
      const item = items.find((entry) => entry.public_id === publicID);
      if (!item) {
        this.setData(Object.assign({}, optionState, {
          loading: false,
          item: null,
          errorMessage: "没有找到这件衣服"
        }));
        return null;
      }

      this.setData(Object.assign({}, optionState, {
        loading: false,
        item,
        errorMessage: ""
      }));
      return item;
    } catch (error) {
      this.setData({
        loading: false,
        item: null,
        errorMessage: error && error.message ? error.message : "读取私藏详情失败"
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
    if (this.data.item.isRecognizing) {
      this.setData({
        errorMessage: "图片识别完成前不能编辑"
      });
      return;
    }

    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    this.setData(Object.assign({
      editorVisible: true,
      editingPublicID: this.data.item.public_id,
      draft: decorateDraftForOptions(itemToDraft(this.data.item), this.data),
      imageFiles: imageFilesFromItem(this.data.item),
      imageUploading: false,
      imageUploadError: "",
      imageRecognizing: false,
      imageRecognizeError: "",
      canRecognizeImage: Boolean(this.data.item.primary_image && this.data.item.primary_image.asset_public_id),
      errorMessage: ""
    }, activeSuggestionState(this.data, "")));
    if (typeof wx !== "undefined" && wx.navigateTo) {
      wx.navigateTo({
        url: `/pages/wardrobe-edit/wardrobe-edit?public_id=${this.data.item.public_id}`
      });
    }
  },

  handleCloseEditor() {
    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    this.setData(Object.assign({
      editorVisible: false,
      editingPublicID: "",
      draft: cloneDraft(),
      imageFiles: [],
      imageUploading: false,
      imageUploadError: "",
      imageRecognizing: false,
      imageRecognizeError: "",
      canRecognizeImage: false,
      errorMessage: ""
    }, activeSuggestionState(this.data, "")));
    this._imageUploadPromise = null;
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
    this._imageUploadPromise = api.uploadFileToQiniu(file, {
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
          imageFiles: confirmedLocalImageFiles(file, uploaded),
          imageUploading: false,
          imageUploadError: "",
          imageRecognizeError: "",
          canRecognizeImage: Boolean(uploaded && uploaded.asset_public_id)
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
          imageUploadError: message,
          canRecognizeImage: Boolean(this.data.draft.primary_asset_public_id)
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
      imageRecognizing: false,
      imageRecognizeError: "",
      canRecognizeImage: false,
      draft: Object.assign({}, this.data.draft, {
        primary_asset_public_id: ""
      })
    });
  },

  async handleRecognizeImageOverwrite() {
    if (this.data.imageUploading) {
      this.setData({
        imageRecognizeError: "图片还在上传，请稍后再识别"
      });
      return null;
    }

    const assetPublicID = this.data.draft && this.data.draft.primary_asset_public_id
      ? this.data.draft.primary_asset_public_id
      : "";
    if (!assetPublicID) {
      this.setData({
        imageRecognizeError: "请先上传图片"
      });
      return null;
    }

    const confirmed = await confirmRecognizeOverwrite();
    if (!confirmed) {
      return null;
    }

    const recognizeRunID = this._imageUploadRunID || 0;
    this.setData({
      imageRecognizing: true,
      imageRecognizeError: ""
    });

    try {
      const recognized = await api.recognizeWardrobeItemImage(assetPublicID, {
        itemPublicID: this.data.editingPublicID || this.data.itemPublicID,
        overwrite: true
      });
      if (this._imageUploadRunID !== recognizeRunID) {
        return recognized;
      }
      this.setData({
        imageRecognizing: false,
        imageRecognizeError: ""
      });
      showToast("已提交识别", "success");
      return recognized;
    } catch (error) {
      if (this._imageUploadRunID !== recognizeRunID) {
        return null;
      }
      this.setData({
        imageRecognizing: false,
        imageRecognizeError: error && error.message ? error.message : "图片识别失败，请稍后重试"
      });
      return null;
    }
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
    this.setData(draftState(draft, this.data));
  },

  handleSuggestionOpen(event) {
    const dataset = getDataset(event);
    const field = dataset.field || "";
    if (!field) {
      return;
    }
    this.setData(activeSuggestionState(this.data, field));
  },

  handleSuggestionSelect(event) {
    const dataset = getDataset(event);
    const field = dataset.field || "";
    const value = dataset.value || "";
    if (!field) {
      return;
    }
    this.setData(Object.assign({}, draftState(Object.assign({}, this.data.draft, {
      [field]: value
    }), this.data), activeSuggestionState(this.data, "")));
  },

  handleSuggestionClose() {
    this.setData(activeSuggestionState(this.data, ""));
  },

  handleSceneInput(event) {
    const sceneText = getDetailValue(event);
    const draft = Object.assign({}, this.data.draft, {
      sceneText,
      scene_tags: normalizeSceneTags(sceneText)
    });
    this.setData(draftState(draft, this.data));
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
      this.setData(Object.assign({
        saving: false,
        editorVisible: false,
        editingPublicID: "",
        draft: cloneDraft(),
        imageFiles: [],
        imageUploading: false,
        imageUploadError: "",
        imageRecognizing: false,
        imageRecognizeError: "",
        canRecognizeImage: false,
        item,
        itemPublicID: item.public_id
      }, activeSuggestionState(this.data, "")));
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
