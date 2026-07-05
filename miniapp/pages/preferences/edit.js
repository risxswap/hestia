const api = require("../../utils/api");

function normalizeText(value) {
  return String(value || "").trim();
}

function splitTextList(value) {
  return String(value || "")
    .split(/[，,、/；;\n]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function joinList(value) {
  return Array.isArray(value) ? value.map(normalizeText).filter(Boolean).join("、") : normalizeText(value);
}

function buildDraft(summary) {
  const profile = summary && summary.profile ? summary.profile : {};
  const preferences = summary && summary.preferences ? summary.preferences : {};
  return {
    styleGoalsText: joinList(preferences.style_goals || profile.style_goal_summary),
    avoidancesText: joinList(preferences.avoidances),
    scenarioPreferencesText: joinList(preferences.scenario_preferences || profile.lifestyle_scenarios)
  };
}

function payloadFromDraft(draft) {
  const source = draft || {};
  return {
    style_goals: splitTextList(source.styleGoalsText),
    avoidances: splitTextList(source.avoidancesText),
    scenario_preferences: splitTextList(source.scenarioPreferencesText)
  };
}

function showToast(title, icon) {
  if (typeof wx !== "undefined" && wx.showToast) {
    wx.showToast({ title, icon: icon || "none" });
  }
}

const preferencesEditPageConfig = {
  data: {
    loading: false,
    saving: false,
    errorMessage: "",
    draft: buildDraft(null)
  },

  onLoad() {
    return this.loadPreferences();
  },

  async loadPreferences() {
    this.setData({ loading: true, errorMessage: "" });
    try {
      const summary = await api.getProfileSummary();
      this.setData({
        loading: false,
        draft: buildDraft(summary)
      });
    } catch (error) {
      this.setData({
        loading: false,
        errorMessage: error && error.message ? error.message : "读取偏好失败"
      });
    }
  },

  handleInput(event) {
    const field = event && event.currentTarget && event.currentTarget.dataset
      ? event.currentTarget.dataset.field
      : "";
    if (!field) {
      return;
    }
    this.setData({
      draft: Object.assign({}, this.data.draft, {
        [field]: event && event.detail ? event.detail.value : ""
      })
    });
  },

  async handleSave() {
    this.setData({ saving: true, errorMessage: "" });
    try {
      await api.updateProfilePreferences(payloadFromDraft(this.data.draft));
      this.setData({ saving: false });
      showToast("偏好已保存", "success");
      if (typeof wx !== "undefined" && wx.navigateBack) {
        wx.navigateBack({ delta: 1 });
      }
    } catch (error) {
      this.setData({
        saving: false,
        errorMessage: error && error.message ? error.message : "保存偏好失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(preferencesEditPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    buildDraft,
    payloadFromDraft,
    preferencesEditPageConfig
  };
}
