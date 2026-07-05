const api = require("../../utils/api");

function normalizeMemoryItems(summary) {
  const memory = summary && summary.memory_summary ? summary.memory_summary : {};
  return [
    { key: "facts", title: "明确事实", count: Number(memory.fact_count || 0), hint: "来自你明确填写或确认的信息" },
    { key: "preferences", title: "偏好", count: Number(memory.preference_count || 0), hint: "会影响建议排序和表达方向" },
    { key: "avoidances", title: "禁忌", count: Number(memory.avoidance_count || 0), hint: "建议会优先避开的方向" },
    { key: "inferences", title: "AI 推断", count: Number(memory.inference_count || 0), hint: "需要允许你后续修正和确认" },
    { key: "pending", title: "待确认", count: Number(memory.pending_confirmation_count || 0), hint: "不确定或需要复核的信息" }
  ];
}

const memoryPageConfig = {
  data: {
    loading: false,
    errorMessage: "",
    items: normalizeMemoryItems(null)
  },

  onShow() {
    return this.loadMemory();
  },

  async loadMemory() {
    this.setData({ loading: true, errorMessage: "" });
    try {
      const summary = await api.getProfileSummary();
      this.setData({
        loading: false,
        items: normalizeMemoryItems(summary)
      });
    } catch (error) {
      this.setData({
        loading: false,
        errorMessage: error && error.message ? error.message : "读取记忆失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(memoryPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    normalizeMemoryItems,
    memoryPageConfig
  };
}
