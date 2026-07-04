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
  styleLogicForItem,
  mergeRecognizedFieldsIntoDraft,
  normalizeWardrobeOptions,
  overwriteDraftWithRecognizedFields,
  withEmptyOption,
  optionLabel,
  optionIndex
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
      title: "删除衣服",
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

function decorateDraftForOptions(draft, state) {
  const source = draft || {};
  const optionsState = state || {};
  return Object.assign({}, source, {
    categoryLabel: optionLabel(optionsState.editorCategoryOptions, source.category, "请选择"),
    categoryIndex: optionIndex(optionsState.editorCategoryOptions, source.category),
    materialLabel: optionLabel(optionsState.materialOptions, source.material, "不选择"),
    materialIndex: optionIndex(optionsState.materialOptions, source.material),
    seasonLabel: optionLabel(optionsState.seasonOptions, source.season, "不选择"),
    seasonIndex: optionIndex(optionsState.seasonOptions, source.season),
    silhouetteLabel: optionLabel(optionsState.silhouetteOptions, source.silhouette, "不选择"),
    silhouetteIndex: optionIndex(optionsState.silhouetteOptions, source.silhouette)
  });
}

function draftState(draft, state) {
  return {
    draft: decorateDraftForOptions(draft, state)
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
    wardrobeOptions: normalizeWardrobeOptions(),
    editorCategoryOptions: normalizeWardrobeOptions().categories,
    materialOptions: withEmptyOption(normalizeWardrobeOptions().materials),
    seasonOptions: withEmptyOption(normalizeWardrobeOptions().seasons),
    silhouetteOptions: withEmptyOption(normalizeWardrobeOptions().silhouettes),
    activeCategory: "all",
    wardrobeDirty: false,
    editorVisible: false,
    draft: decorateDraftForOptions(cloneDraft(), {
      editorCategoryOptions: normalizeWardrobeOptions().categories,
      materialOptions: withEmptyOption(normalizeWardrobeOptions().materials),
      seasonOptions: withEmptyOption(normalizeWardrobeOptions().seasons),
      silhouetteOptions: withEmptyOption(normalizeWardrobeOptions().silhouettes)
    }),
    editingPublicID: "",
    imageFiles: [],
    imageUploading: false,
    imageUploadError: "",
    imageRecognizing: false,
    imageRecognizeError: "",
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
    const wasDirty = Boolean(globalData.wardrobeDirty || this.data.wardrobeDirty);
    if (wasDirty) {
      this.setData({
        wardrobeDirty: true
      });
    }
    await this.loadWardrobe();
    if (wasDirty && !this.data.errorMessage) {
      globalData.wardrobeDirty = false;
      this.setData({
        wardrobeDirty: false
      });
    }
    return Promise.resolve();
  },

  async loadWardrobe() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const optionsRequest = typeof api.getWardrobeOptions === "function" ? api.getWardrobeOptions() : Promise.resolve(null);
      const reportRequest = typeof api.getLatestReport === "function" ? api.getLatestReport() : Promise.resolve(null);
      const itemRequest = typeof api.getWardrobeItems === "function" ? api.getWardrobeItems() : Promise.resolve({ items: [] });
      const results = await Promise.all([itemRequest, reportRequest, optionsRequest]);
      const items = normalizeWardrobeItems(results[0]);
      const optionsState = this.optionsState(results[2]);

      this.setData(Object.assign({}, optionsState, draftState(this.data.draft, optionsState), {
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
        errorMessage: error && error.message ? error.message : "读取衣服失败"
      });
    }
  },

  optionsState(rawOptions) {
    const wardrobeOptions = normalizeWardrobeOptions(rawOptions);
    return {
      wardrobeOptions,
      editorCategoryOptions: wardrobeOptions.categories,
      materialOptions: withEmptyOption(wardrobeOptions.materials),
      seasonOptions: withEmptyOption(wardrobeOptions.seasons),
      silhouetteOptions: withEmptyOption(wardrobeOptions.silhouettes)
    };
  },

  async loadWardrobeOptions() {
    try {
      const options = typeof api.getWardrobeOptions === "function" ? await api.getWardrobeOptions() : null;
      this.setData(this.optionsState(options));
      return options;
    } catch (error) {
      this.setData(this.optionsState(null));
      return null;
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
        errorMessage: error && error.message ? error.message : "读取补齐建议失败"
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
    this._imageUploadPromisesByURL = {};
    const category = this.data.activeCategory && this.data.activeCategory !== "all"
      ? this.data.activeCategory
      : "上装";
    this.setData({
      editorVisible: true,
      editingPublicID: "",
      draft: decorateDraftForOptions(cloneDraft({
        category
      }), this.data),
      imageFiles: [],
      imageUploading: false,
      imageUploadError: "",
      imageRecognizing: false,
      imageRecognizeError: "",
      errorMessage: ""
    });
  },

  handleCloseEditor() {
    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    this._imageUploadPromisesByURL = {};
    this.setData({
      editorVisible: false,
      editingPublicID: "",
      draft: decorateDraftForOptions(cloneDraft(), this.data),
      imageFiles: [],
      imageUploading: false,
      imageUploadError: "",
      imageRecognizing: false,
      imageRecognizeError: "",
      errorMessage: ""
    });
    this._imageUploadPromise = null;
  },

  handleEditItem(event) {
    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    this._imageUploadPromisesByURL = {};
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
      draft: decorateDraftForOptions(itemToDraft(item), this.data),
      imageFiles: imageFilesFromItem(item),
      imageUploading: false,
      imageUploadError: "",
      imageRecognizing: false,
      imageRecognizeError: "",
      errorMessage: ""
    });
  },

  handleImageUpload(event) {
    const file = getUploadFile(event);
    if (!file) {
      return Promise.resolve([]);
    }

    const previewURL = filePreviewUrl(file);
    if (previewURL) {
      this._imageUploadPromisesByURL = this._imageUploadPromisesByURL || {};
      if (this._imageUploadPromisesByURL[previewURL]) {
        return this._imageUploadPromisesByURL[previewURL];
      }
    }

    const pendingFiles = this.data.imageFiles.concat(pendingImageFiles(file));
    this.setData({
      imageFiles: pendingFiles,
      imageUploading: true,
      imageUploadError: ""
    });

    const uploadRunID = this._imageUploadRunID || 0;
    this._activeImageUploads = (this._activeImageUploads || 0) + 1;
    const uploadPromise = api.uploadFileToQiniu(file, {
      assetType: "wardrobe_item_photo"
    })
      .then((uploaded) => {
        if (this._imageUploadRunID !== uploadRunID) {
          return uploaded;
        }
        const assetPublicID = uploaded && uploaded.asset_public_id ? uploaded.asset_public_id : "";
        const assetPublicIDs = (this.data.draft.asset_public_ids || []).slice();
        if (assetPublicID && !assetPublicIDs.includes(assetPublicID)) {
          assetPublicIDs.push(assetPublicID);
        }
        const draft = decorateDraftForOptions(Object.assign({}, this.data.draft, {
          primary_asset_public_id: this.data.draft.primary_asset_public_id || assetPublicID,
          asset_public_ids: assetPublicIDs
        }), this.data);
        const nextFiles = this.data.imageFiles.map((item) => (
          item.url === previewURL ? Object.assign({}, item, confirmedLocalImageFiles(file, uploaded)[0]) : item
        ));
        this._activeImageUploads = Math.max(0, (this._activeImageUploads || 1) - 1);
        this.setData({
          draft,
          imageFiles: nextFiles,
          imageUploading: this._activeImageUploads > 0,
          imageUploadError: "",
          imageRecognizeError: ""
        });
        return uploaded;
      })
      .catch((error) => {
        if (this._imageUploadRunID !== uploadRunID) {
          return null;
        }
        const message = error && error.message ? error.message : "图片上传失败";
        this._activeImageUploads = Math.max(0, (this._activeImageUploads || 1) - 1);
        const previewURL = filePreviewUrl(file);
        this.setData({
          imageFiles: this.data.imageFiles.map((item) => (
            item.url === previewURL ? Object.assign({}, item, failedImageFiles(file, message)[0]) : item
          )),
          imageUploading: this._activeImageUploads > 0,
          imageUploadError: message,
          imageRecognizing: false
        });
        return null;
      });

    if (previewURL) {
      this._imageUploadPromisesByURL[previewURL] = uploadPromise;
    }
    return uploadPromise;
  },

  handleImageRemove() {
    this._imageUploadRunID = (this._imageUploadRunID || 0) + 1;
    this._imageUploadPromise = null;
    this._imageUploadPromisesByURL = {};
    this._activeImageUploads = 0;
    this.setData({
      imageFiles: [],
      imageUploading: false,
      imageUploadError: "",
      imageRecognizing: false,
      imageRecognizeError: "",
      draft: decorateDraftForOptions(Object.assign({}, this.data.draft, {
        primary_asset_public_id: "",
        asset_public_ids: []
      }), this.data)
    });
  },

  async recognizeUploadedWardrobeImage(uploaded, uploadRunID) {
    return this.applyUploadedWardrobeRecognition(uploaded, uploadRunID);
  },

  async applyUploadedWardrobeRecognition(uploaded, uploadRunID) {
    const assetPublicID = uploaded && uploaded.asset_public_id ? uploaded.asset_public_id : "";
    const confirmedFields = uploaded && uploaded.recognized_fields ? uploaded.recognized_fields : null;
    if (!assetPublicID && !confirmedFields) {
      return null;
    }

    const recognizeRunID = uploadRunID || this._imageUploadRunID || 0;
    this.setData({
      imageRecognizing: true,
      imageRecognizeError: ""
    });

    try {
      const recognized = confirmedFields || (typeof api.recognizeWardrobeItemImage === "function"
        ? await api.recognizeWardrobeItemImage(assetPublicID)
        : null);
      if (this._imageUploadRunID !== recognizeRunID) {
        return recognized;
      }
      if (!recognized) {
        this.setData({
          imageRecognizing: false,
          imageRecognizeError: ""
        });
        return null;
      }
      this.setData({
        draft: decorateDraftForOptions(mergeRecognizedFieldsIntoDraft(this.data.draft, recognized, this.data.wardrobeOptions), this.data),
        imageRecognizing: false,
        imageRecognizeError: ""
      });
      return recognized;
    } catch (error) {
      if (this._imageUploadRunID !== recognizeRunID) {
        return null;
      }
      this.setData({
        imageRecognizing: false,
        imageRecognizeError: error && error.message ? error.message : "图片已上传，识别失败，可手动填写"
      });
      return null;
    }
  },

  async createItemFromUploadedImage(uploaded, uploadRunID) {
    const assetPublicID = uploaded && uploaded.asset_public_id ? uploaded.asset_public_id : "";
    if (!assetPublicID) {
      this.setData({
        imageRecognizing: false,
        imageRecognizeError: "图片上传失败，请重新上传"
      });
      return null;
    }

    const recognizeRunID = uploadRunID || this._imageUploadRunID || 0;
    this.setData({
      imageRecognizing: true,
      imageRecognizeError: "",
      saving: true,
      errorMessage: ""
    });

    try {
      const recognized = uploaded && uploaded.recognized_fields
        ? uploaded.recognized_fields
        : typeof api.recognizeWardrobeItemImage === "function"
          ? await api.recognizeWardrobeItemImage(assetPublicID)
          : null;
      if (this._imageUploadRunID !== recognizeRunID) {
        return recognized;
      }
      const draft = overwriteDraftWithRecognizedFields(Object.assign({}, cloneDraft({
        category: "",
        primary_asset_public_id: assetPublicID
      })), recognized || {}, this.data.wardrobeOptions);
      const payload = buildPayload(draft);
      if (!payload.name || !payload.category) {
        this.setData({
          saving: false,
          imageRecognizing: false,
          imageRecognizeError: "图片已上传，但没有识别出可保存的信息，请重新上传更清晰的图片"
        });
        return null;
      }

      const saved = await api.createWardrobeItem(payload);
      if (this._imageUploadRunID !== recognizeRunID) {
        return saved;
      }
      const normalizedSaved = decorateWardrobeItem(saved);
      const nextItems = this.data.items.concat(normalizedSaved);
      this.setData(Object.assign({
        saving: false,
        editorVisible: false,
        editingPublicID: "",
        draft: decorateDraftForOptions(cloneDraft(), this.data),
        imageFiles: [],
        imageUploading: false,
        imageUploadError: "",
        imageRecognizing: false,
        imageRecognizeError: "",
        errorMessage: ""
      }, nextWardrobeState(nextItems, this.data.activeCategory)));
      showToast("已添加", "success");
      return saved;
    } catch (error) {
      if (this._imageUploadRunID !== recognizeRunID) {
        return null;
      }
      this.setData({
        saving: false,
        imageRecognizing: false,
        imageRecognizeError: error && error.message ? error.message : "图片识别失败，请重新上传"
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

  handleOptionChange(event) {
    const dataset = getDataset(event);
    const field = dataset.field;
    const optionKey = dataset.optionKey;
    if (!field || !optionKey) {
      return;
    }
    const options = this.data[optionKey] || [];
    const index = Number(getDetailValue(event));
    const selected = options[index] || options[0] || { value: "" };
    this.setData(draftState(Object.assign({}, this.data.draft, {
      [field]: selected.value || ""
    }), this.data));
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
      draft: decorateDraftForOptions(Object.assign({}, this.data.draft, {
        recommendation_status: status
      }), this.data)
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
      draft: decorateDraftForOptions(Object.assign({}, this.data.draft, {
        is_core: value === true || value === "true" || value === 1 || value === "1"
      }), this.data)
    });
  },

  async handleSaveItem() {
    if (this.data.imageUploading) {
      this.setData({
        errorMessage: "图片还在上传，请稍后再保存"
      });
      return;
    }

    const draft = this.data.editingPublicID
      ? Object.assign({}, this.data.draft)
      : Object.assign({}, this.data.draft, {
        name: "识别中",
        category: "其他",
        recognition_status: "pending"
      });
    const payload = buildPayload(draft);
    if (!payload.name || !payload.category) {
      this.setData({
        errorMessage: "请填写单品名称和分类"
      });
      return;
    }
    if (!this.data.editingPublicID && (!payload.asset_public_ids || !payload.asset_public_ids.length)) {
      this.setData({
        errorMessage: "请先上传图片"
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
        draft: decorateDraftForOptions(cloneDraft(), this.data),
        imageFiles: [],
        imageUploading: false,
        imageUploadError: "",
        imageRecognizing: false,
        imageRecognizeError: ""
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
        draft: decorateDraftForOptions(cloneDraft(), this.data),
        imageFiles: [],
        imageUploading: false,
        imageUploadError: "",
        imageRecognizing: false,
        imageRecognizeError: ""
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
    normalizeWardrobeOptions,
    mergeRecognizedFieldsIntoDraft,
    overwriteDraftWithRecognizedFields,
    nextWardrobeState,
    wardrobePageConfig
  };
}
