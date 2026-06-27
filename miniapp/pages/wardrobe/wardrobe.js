const api = require("../../utils/api");

const categoryOptions = [
  { label: "全部", value: "all" },
  { label: "上装", value: "top" },
  { label: "下装", value: "bottom" },
  { label: "外套", value: "outerwear" },
  { label: "连衣裙", value: "dress" },
  { label: "鞋包配饰", value: "accessory" }
];

const categoryLabels = categoryOptions.reduce((result, option) => {
  result[option.value] = option.label;
  return result;
}, {});

const recommendationLabels = {
  preferred: "优先推荐",
  normal: "正常推荐",
  paused: "暂不推荐"
};

const emptyDraft = {
  name: "",
  category: "top",
  color: "",
  silhouette: "",
  material: "",
  season: "",
  scene_tags: [],
  sceneText: "",
  user_notes: "",
  is_core: true,
  recommendation_status: "normal",
  primary_asset_public_id: ""
};

function normalizeTextList(value) {
  if (!Array.isArray(value)) {
    return value ? [String(value)] : [];
  }
  return value.filter(Boolean).map((item) => String(item));
}

function normalizeSceneTags(value) {
  if (Array.isArray(value)) {
    return normalizeTextList(value);
  }

  return String(value || "")
    .split(/[,\n，、；;]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function cloneDraft(overrides) {
  return Object.assign({}, emptyDraft, overrides || {}, {
    scene_tags: normalizeSceneTags(overrides && overrides.scene_tags),
    sceneText: overrides && Object.prototype.hasOwnProperty.call(overrides, "sceneText")
      ? overrides.sceneText
      : normalizeSceneTags(overrides && overrides.scene_tags).join("，")
  });
}

function normalizeWardrobeItems(response) {
  const body = response && response.data ? response.data : response;
  const items = Array.isArray(body) ? body : body && Array.isArray(body.items) ? body.items : [];
  return items.map((item) => decorateWardrobeItem(item));
}

function decorateWardrobeItem(item) {
  const source = item || {};
  const sceneTags = normalizeTextList(source.scene_tags);
  const recommendationStatus = source.recommendation_status || "normal";

  return Object.assign({}, source, {
    public_id: source.public_id || source.publicID || "",
    name: source.name || "未命名单品",
    category: source.category || "",
    color: source.color || "",
    silhouette: source.silhouette || "",
    material: source.material || "",
    season: source.season || "",
    scene_tags: sceneTags,
    sceneText: sceneTags.join(" / "),
    user_notes: source.user_notes || source.notes || "",
    is_core: source.is_core !== false,
    recommendation_status: recommendationStatus,
    recommendationLabel: recommendationLabels[recommendationStatus] || "正常推荐",
    categoryLabel: categoryLabels[source.category] || source.category || "未分类",
    primary_image: source.primary_image || null,
    status: source.status || "active",
    isPreferred: recommendationStatus === "preferred",
    isPaused: recommendationStatus === "paused"
  });
}

function filterItems(items, category) {
  const activeCategory = category || "all";
  if (!activeCategory || activeCategory === "all") {
    return items.slice();
  }
  return items.filter((item) => item.category === activeCategory);
}

function priorityItems(items) {
  return items
    .filter((item) => item.recommendation_status === "preferred")
    .sort((left, right) => {
      if (left.is_core !== right.is_core) {
        return left.is_core ? -1 : 1;
      }
      return String(left.name || "").localeCompare(String(right.name || ""), "zh-Hans-CN");
    });
}

function normalizeWardrobeGaps(report) {
  const content = report && report.content_json ? report.content_json : {};
  if (!Array.isArray(content.wardrobe_gaps)) {
    return [];
  }
  return content.wardrobe_gaps
    .map((item) => {
      if (typeof item === "string") {
        return item;
      }
      return item.title || item.name || item.description || "";
    })
    .filter(Boolean);
}

function buildPayload(draft) {
  const source = draft || {};
  const sceneTags = normalizeSceneTags(source.scene_tags && source.scene_tags.length ? source.scene_tags : source.sceneText);
  const payload = {
    name: String(source.name || "").trim(),
    category: String(source.category || "").trim(),
    color: String(source.color || "").trim(),
    silhouette: String(source.silhouette || "").trim(),
    material: String(source.material || "").trim(),
    season: String(source.season || "").trim(),
    scene_tags: sceneTags,
    user_notes: String(source.user_notes || "").trim(),
    is_core: source.is_core !== false,
    recommendation_status: source.recommendation_status || "normal"
  };

  if (source.primary_asset_public_id) {
    payload.primary_asset_public_id = String(source.primary_asset_public_id).trim();
  }

  return payload;
}

function itemToDraft(item) {
  const decorated = decorateWardrobeItem(item);
  return cloneDraft({
    name: decorated.name,
    category: decorated.category || "top",
    color: decorated.color,
    silhouette: decorated.silhouette,
    material: decorated.material,
    season: decorated.season,
    scene_tags: decorated.scene_tags,
    user_notes: decorated.user_notes,
    is_core: decorated.is_core,
    recommendation_status: decorated.recommendation_status,
    primary_asset_public_id: decorated.primary_image && decorated.primary_image.asset_public_id
      ? decorated.primary_image.asset_public_id
      : ""
  });
}

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
    priorityItems: priorityItems(items)
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
    categoryOptions,
    activeCategory: "all",
    editorVisible: false,
    draft: cloneDraft(),
    editingPublicID: ""
  },

  onLoad() {
    return this.loadWardrobe();
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

  handleOpenCreate() {
    this.setData({
      editorVisible: true,
      editingPublicID: "",
      draft: cloneDraft()
    });
  },

  handleEditItem(event) {
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
        draft: cloneDraft()
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
        errorMessage: ""
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
    normalizeWardrobeItems,
    decorateWardrobeItem,
    filterItems,
    priorityItems,
    normalizeWardrobeGaps,
    buildPayload,
    wardrobePageConfig
  };
}
