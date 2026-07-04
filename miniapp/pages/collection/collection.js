const api = require("../../utils/api");

const DEFAULT_TYPES = [
  {
    type: "wardrobe",
    label: "衣服",
    count: 0,
    hint: "常穿单品",
    enabled: true,
    entry_path: "/pages/wardrobe/wardrobe"
  },
  {
    type: "hair",
    label: "发型",
    count: 0,
    hint: "常用发型",
    enabled: true,
    entry_path: "/pages/hair/hair"
  },
  {
    type: "makeup",
    label: "妆容",
    count: 0,
    hint: "妆容方向",
    enabled: true,
    entry_path: "/pages/makeup/makeup"
  },
  {
    type: "references",
    label: "参考",
    count: 0,
    hint: "参考图",
    enabled: true,
    entry_path: "/pages/references/references"
  }
];

function getDataset(event) {
  return event && event.currentTarget && event.currentTarget.dataset ? event.currentTarget.dataset : {};
}

function normalizeType(rawType) {
  const source = rawType || {};
  const type = source.type || source.value || "";
  const fallback = DEFAULT_TYPES.find((item) => item.type === type) || {};
  const count = Number(source.count || 0);
  return Object.assign({}, fallback, {
    type: type || fallback.type || "wardrobe",
    label: source.label || fallback.label || "衣服",
    count: Number.isFinite(count) ? count : 0,
    hint: source.hint || fallback.hint || "",
    enabled: source.enabled !== false,
    entryPath: source.entry_path || source.entryPath || fallback.entry_path || "",
    countLabel: Number.isFinite(count) && count > 0 ? `${count} 项` : "待收录"
  });
}

function normalizeTypes(summary) {
  const byType = {};
  const rawTypes = summary && Array.isArray(summary.types) ? summary.types : [];
  rawTypes.forEach((item) => {
    const normalized = normalizeType(item);
    byType[normalized.type] = normalized;
  });
  return DEFAULT_TYPES.map((fallback) => normalizeType(Object.assign({}, fallback, byType[fallback.type] || {})));
}

function normalizeRecentItem(rawItem) {
  const source = rawItem || {};
  return {
    type: source.type || "wardrobe",
    public_id: source.public_id || source.publicID || "",
    title: source.title || source.name || "未命名",
    subtitle: source.subtitle || source.category || "",
    image: source.image || source.preview_url || source.primaryImageSrc || "",
    entryPath: source.entry_path || source.entryPath || ""
  };
}

function normalizeRecentItems(summary) {
  const items = summary && Array.isArray(summary.recent_items)
    ? summary.recent_items
    : summary && Array.isArray(summary.recentItems)
      ? summary.recentItems
      : [];
  return items.map(normalizeRecentItem).filter((item) => item.entryPath || item.public_id);
}

function navigateTo(url) {
  if (!url || typeof wx === "undefined" || !wx.navigateTo) {
    return;
  }
  wx.navigateTo({ url });
}

const collectionPageConfig = {
  data: {
    loading: false,
    errorMessage: "",
    types: normalizeTypes(null),
    recentItems: []
  },

  onLoad() {
    return this.loadCollection();
  },

  onShow() {
    return this.loadCollection();
  },

  async loadCollection() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const summary = typeof api.getCollectionSummary === "function"
        ? await api.getCollectionSummary()
        : null;
      this.setData({
        loading: false,
        types: normalizeTypes(summary),
        recentItems: normalizeRecentItems(summary)
      });
    } catch (error) {
      this.setData({
        loading: false,
        types: normalizeTypes(null),
        recentItems: [],
        errorMessage: error && error.message ? error.message : "读取私藏失败"
      });
    }
  },

  handleOpenType(event) {
    const dataset = getDataset(event);
    const entryPath = dataset.entryPath || dataset.entry_path || "";
    navigateTo(entryPath);
  },

  handleOpenRecent(event) {
    const dataset = getDataset(event);
    const entryPath = dataset.entryPath || dataset.entry_path || "";
    navigateTo(entryPath);
  }
};

if (typeof Page === "function") {
  Page(collectionPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    DEFAULT_TYPES,
    normalizeTypes,
    normalizeRecentItems,
    collectionPageConfig
  };
}
