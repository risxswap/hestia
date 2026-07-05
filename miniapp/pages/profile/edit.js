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
  const user = summary && summary.user ? summary.user : {};
  const profile = summary && summary.profile ? summary.profile : {};
  return {
    nickname: normalizeText(user.nickname),
    gender: normalizeText(profile.gender),
    height_cm: profile.height_cm || profile.height_cm === 0 ? String(profile.height_cm) : "",
    body_notes: normalizeText(profile.body_notes),
    skin_notes: normalizeText(profile.skin_notes),
    hair_notes: normalizeText(profile.hair_notes),
    scenarioText: joinList(profile.lifestyle_scenarios)
  };
}

function payloadFromDraft(draft) {
  const source = draft || {};
  const heightText = normalizeText(source.height_cm);
  const heightValue = heightText ? Number(heightText) : null;
  return {
    nickname: normalizeText(source.nickname),
    gender: normalizeText(source.gender),
    height_cm: Number.isFinite(heightValue) ? heightValue : null,
    body_notes: normalizeText(source.body_notes),
    skin_notes: normalizeText(source.skin_notes),
    hair_notes: normalizeText(source.hair_notes),
    lifestyle_scenarios: splitTextList(source.scenarioText)
  };
}

function showToast(title, icon) {
  if (typeof wx !== "undefined" && wx.showToast) {
    wx.showToast({ title, icon: icon || "none" });
  }
}

const profileEditPageConfig = {
  data: {
    loading: false,
    saving: false,
    errorMessage: "",
    draft: buildDraft(null)
  },

  onLoad() {
    return this.loadProfile();
  },

  async loadProfile() {
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
        errorMessage: error && error.message ? error.message : "读取档案失败"
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
      await api.updateProfile(payloadFromDraft(this.data.draft));
      this.setData({ saving: false });
      showToast("档案已保存", "success");
      if (typeof wx !== "undefined" && wx.navigateBack) {
        wx.navigateBack({ delta: 1 });
      }
    } catch (error) {
      this.setData({
        saving: false,
        errorMessage: error && error.message ? error.message : "保存档案失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(profileEditPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    buildDraft,
    payloadFromDraft,
    profileEditPageConfig
  };
}
