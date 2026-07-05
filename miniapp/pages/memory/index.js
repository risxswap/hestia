const api = require("../../utils/api");

function getDataset(event) {
  return event && event.currentTarget && event.currentTarget.dataset ? event.currentTarget.dataset : {};
}

function valueOf(event) {
  return event && event.detail && Object.prototype.hasOwnProperty.call(event.detail, "value") ? event.detail.value : "";
}

function normalizeMemoryItems(result) {
  const source = result && Array.isArray(result.items) ? result.items : [];
  return source.map(normalizeMemoryItem);
}

function normalizeMemoryItem(item) {
  const source = item || {};
  const confidence = Number(source.confidence);
  const hasConfidence = Number.isFinite(confidence);
  return {
    public_id: source.public_id || "",
    memory_type: source.memory_type || "",
    type_label: source.type_label || fallbackTypeLabel(source.memory_type, source.polarity),
    memory_key: source.memory_key || "",
    memory_value: stringValue(source.memory_value || source.display_text),
    display_text: stringValue(source.display_text || source.memory_value),
    polarity: source.polarity || "neutral",
    confidence: hasConfidence ? confidence : null,
    confidenceText: hasConfidence ? `置信度 ${Math.round(confidence * 100)}%` : "",
    source_label: source.source_label || "长期记忆",
    correction_note: source.correction_note || "",
    updated_at: source.updated_at || "",
    updatedText: formatDate(source.updated_at)
  };
}

function fallbackTypeLabel(type, polarity) {
  if (type === "fact") {
    return "明确事实";
  }
  if (type === "preference") {
    return polarity === "negative" ? "禁忌" : "偏好";
  }
  if (type === "avoidance") {
    return "禁忌";
  }
  if (type === "inference") {
    return "AI 推断";
  }
  if (type === "pending") {
    return "待确认";
  }
  return "其他";
}

function stringValue(value) {
  if (value === undefined || value === null) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  return JSON.stringify(value);
}

function formatDate(value) {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${month}-${day}`;
}

function confirmDelete() {
  if (typeof wx === "undefined" || !wx.showModal) {
    return Promise.resolve(true);
  }
  return new Promise((resolve) => {
    wx.showModal({
      title: "删除记忆",
      content: "删除后，这条记忆不会继续用于后续建议。",
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

function showToast(title, icon) {
  if (typeof wx !== "undefined" && wx.showToast) {
    wx.showToast({ title, icon: icon || "none" });
  }
}

const memoryPageConfig = {
  data: {
    loading: false,
    saving: false,
    errorMessage: "",
    editVisible: false,
    editingPublicID: "",
    items: [],
    draft: {
      memory_value: ""
    }
  },

  onShow() {
    return this.loadMemory();
  },

  async loadMemory() {
    this.setData({ loading: true, errorMessage: "" });
    try {
      const result = await api.getMemoryItems();
      this.setData({
        loading: false,
        items: normalizeMemoryItems(result)
      });
    } catch (error) {
      this.setData({
        loading: false,
        items: [],
        errorMessage: error && error.message ? error.message : "读取记忆失败"
      });
    }
  },

  handleOpenEdit(event) {
    const publicID = getDataset(event).publicId || getDataset(event).public_id || "";
    const item = this.data.items.find((memory) => memory.public_id === publicID);
    if (!item) {
      return;
    }
    this.setData({
      editVisible: true,
      editingPublicID: publicID,
      errorMessage: "",
      draft: {
        memory_value: item.memory_value || item.display_text || ""
      }
    });
  },

  handleCloseEdit() {
    this.setData({
      editVisible: false,
      saving: false,
      editingPublicID: "",
      draft: {
        memory_value: ""
      }
    });
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

  async handleSaveEdit() {
    const publicID = this.data.editingPublicID;
    const memoryValue = String(this.data.draft.memory_value || "").trim();
    if (!publicID || !memoryValue) {
      this.setData({ errorMessage: "记忆内容不能为空" });
      return;
    }
    this.setData({ saving: true, errorMessage: "" });
    try {
      const saved = await api.updateMemoryItem(publicID, {
        memory_value: memoryValue
      });
      const nextItem = normalizeMemoryItem(saved);
      this.setData({
        saving: false,
        editVisible: false,
        editingPublicID: "",
        items: this.data.items.map((item) => (item.public_id === publicID ? nextItem : item)),
        draft: {
          memory_value: ""
        }
      });
      showToast("已保存", "success");
    } catch (error) {
      this.setData({
        saving: false,
        errorMessage: error && error.message ? error.message : "保存记忆失败"
      });
    }
  },

  async handleDelete(event) {
    const publicID = getDataset(event).publicId || getDataset(event).public_id || "";
    if (!publicID || !(await confirmDelete())) {
      return;
    }
    try {
      await api.deleteMemoryItem(publicID);
      this.setData({
        items: this.data.items.filter((item) => item.public_id !== publicID)
      });
      showToast("已删除", "success");
    } catch (error) {
      this.setData({ errorMessage: error && error.message ? error.message : "删除记忆失败" });
    }
  }
};

if (typeof Page === "function") {
  Page(memoryPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    normalizeMemoryItems,
    normalizeMemoryItem,
    memoryPageConfig
  };
}
