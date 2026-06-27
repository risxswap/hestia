const api = require("../../utils/api");

const emptyProfileDraft = {
  nickname: "",
  gender: "",
  height_cm: "",
  body_notes: "",
  skin_notes: "",
  hair_notes: "",
  scenarioText: ""
};

const emptyPreferencesDraft = {
  styleGoalsText: "",
  avoidancesText: "",
  scenarioPreferencesText: ""
};

const fallbackQuickEntries = [
  {
    key: "profile",
    title: "我的档案",
    summary: "基础信息与场景",
    action: "编辑"
  },
  {
    key: "preferences",
    title: "偏好与禁忌",
    summary: "风格目标与不想要的方向",
    action: "编辑"
  },
  {
    key: "report",
    title: "报告与路线",
    summary: "查看当前行动建议",
    action: "查看"
  },
  {
    key: "privacy",
    title: "隐私与数据",
    summary: "本地登录与删除入口",
    action: "查看"
  }
];

const quickEntryFallbackByKey = fallbackQuickEntries.reduce((result, entry) => {
  result[entry.key] = entry;
  return result;
}, {});

function normalizeText(value) {
  return String(value || "").trim();
}

function normalizeList(value) {
  if (Array.isArray(value)) {
    return value.map((item) => normalizeText(item)).filter(Boolean);
  }
  return splitTextList(value);
}

function splitTextList(value) {
  return String(value || "")
    .split(/[，,、/；;\n]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function joinList(value) {
  return normalizeList(value).join("、");
}

function normalizeQuickEntries(entries) {
  const source = Array.isArray(entries) && entries.length ? entries : fallbackQuickEntries;
  return source.slice(0, 4).map((entry, index) => {
    const fallback = quickEntryFallbackByKey[entry.key] || fallbackQuickEntries[index] || fallbackQuickEntries[0];
    return {
      key: entry.key || fallback.key,
      title: entry.title || fallback.title,
      summary: entry.summary || fallback.summary,
      action: entry.action || fallback.action
    };
  });
}

function normalizeMemorySummary(memorySummary) {
  const memory = memorySummary || {};
  return [
    { label: "事实", value: Number(memory.fact_count || 0) },
    { label: "偏好", value: Number(memory.preference_count || 0) },
    { label: "禁忌", value: Number(memory.avoidance_count || 0) },
    { label: "推断", value: Number(memory.inference_count || 0) },
    { label: "待确认", value: Number(memory.pending_confirmation_count || 0) }
  ];
}

function buildProfileDraft(summary) {
  const user = summary.user || {};
  const profile = summary.profile || {};
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

function buildPreferencesDraft(summary) {
  const profile = summary.profile || {};
  const preferences = summary.preferences || {};
  return {
    styleGoalsText: joinList(preferences.style_goals || profile.style_goal_summary),
    avoidancesText: joinList(preferences.avoidances),
    scenarioPreferencesText: joinList(preferences.scenario_preferences || profile.lifestyle_scenarios)
  };
}

function normalizeProfileSummary(rawSummary) {
  const summary = rawSummary || {};
  const user = summary.user || {};
  const profile = summary.profile || null;
  const latestReport = summary.latest_report || null;
  const hasSummary = Boolean(rawSummary);

  return {
    loading: false,
    savingProfile: false,
    savingPreferences: false,
    errorMessage: "",
    empty: !hasSummary,
    tokenReady: hasSummary,
    summary,
    user: {
      user_public_id: normalizeText(user.user_public_id),
      nickname: normalizeText(user.nickname) || "本地用户",
      onboarding_status: normalizeText(user.onboarding_status) || "not_started"
    },
    profile,
    latestReport,
    quickEntries: normalizeQuickEntries(summary.quick_entries),
    memoryItems: normalizeMemorySummary(summary.memory_summary),
    profileDraft: hasSummary ? buildProfileDraft(summary) : Object.assign({}, emptyProfileDraft),
    preferencesDraft: hasSummary ? buildPreferencesDraft(summary) : Object.assign({}, emptyPreferencesDraft)
  };
}

function profilePayloadFromDraft(draft) {
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

function preferencesPayloadFromDraft(draft) {
  const source = draft || {};
  return {
    style_goals: splitTextList(source.styleGoalsText),
    avoidances: splitTextList(source.avoidancesText),
    scenario_preferences: splitTextList(source.scenarioPreferencesText)
  };
}

function showToast(title, icon) {
  if (typeof wx !== "undefined" && wx.showToast) {
    wx.showToast({
      title,
      icon: icon || "none"
    });
  }
}

const profilePageConfig = {
  data: {
    loading: false,
    savingProfile: false,
    savingPreferences: false,
    errorMessage: "",
    empty: false,
    tokenReady: false,
    summary: null,
    user: {
      user_public_id: "",
      nickname: "",
      onboarding_status: ""
    },
    profile: null,
    latestReport: null,
    activeSection: "",
    quickEntries: fallbackQuickEntries,
    memoryItems: normalizeMemorySummary(),
    profileDraft: Object.assign({}, emptyProfileDraft),
    preferencesDraft: Object.assign({}, emptyPreferencesDraft)
  },

  onLoad() {
    return this.loadProfile();
  },

  async loadProfile() {
    this.setData({
      loading: true,
      errorMessage: ""
    });

    try {
      const summary = await api.getProfileSummary();
      this.setData(normalizeProfileSummary(summary));
    } catch (error) {
      this.setData({
        loading: false,
        tokenReady: false,
        errorMessage: error && error.message ? error.message : "登录状态读取失败"
      });
    }
  },

  handleProfileInput(event) {
    const field = event && event.currentTarget && event.currentTarget.dataset
      ? event.currentTarget.dataset.field
      : "";
    if (!field) {
      return;
    }
    const value = event && event.detail ? event.detail.value : "";
    this.setData({
      profileDraft: Object.assign({}, this.data.profileDraft, {
        [field]: value
      })
    });
  },

  handlePreferencesInput(event) {
    const field = event && event.currentTarget && event.currentTarget.dataset
      ? event.currentTarget.dataset.field
      : "";
    if (!field) {
      return;
    }
    const value = event && event.detail ? event.detail.value : "";
    this.setData({
      preferencesDraft: Object.assign({}, this.data.preferencesDraft, {
        [field]: value
      })
    });
  },

  async handleSaveProfile() {
    this.setData({
      savingProfile: true,
      errorMessage: ""
    });

    try {
      const summary = await api.updateProfile(profilePayloadFromDraft(this.data.profileDraft));
      this.setData(normalizeProfileSummary(summary));
      showToast("档案已保存", "success");
    } catch (error) {
      this.setData({
        savingProfile: false,
        errorMessage: error && error.message ? error.message : "保存档案失败"
      });
    }
  },

  async handleSavePreferences() {
    const submittedDraft = Object.assign({}, this.data.preferencesDraft);
    this.setData({
      savingPreferences: true,
      errorMessage: ""
    });

    try {
      const summary = await api.updateProfilePreferences(preferencesPayloadFromDraft(this.data.preferencesDraft));
      this.setData(Object.assign({}, normalizeProfileSummary(summary), {
        preferencesDraft: submittedDraft
      }));
      showToast("偏好已保存", "success");
    } catch (error) {
      this.setData({
        savingPreferences: false,
        errorMessage: error && error.message ? error.message : "保存偏好失败"
      });
    }
  },

  handleQuickEntry(event) {
    const key = event && event.currentTarget && event.currentTarget.dataset
      ? event.currentTarget.dataset.key
      : "";

    if (key === "report") {
      if (typeof wx !== "undefined" && wx.navigateTo) {
        wx.navigateTo({ url: "/pages/report/report" });
      }
      return;
    }

    const sectionSelectorMap = {
      profile: "#profile-section",
      preferences: "#preferences-section",
      privacy: "#privacy-section"
    };
    const selector = sectionSelectorMap[key];
    if (!selector) {
      return;
    }

    this.setData({
      activeSection: key
    });
    if (typeof wx !== "undefined" && wx.pageScrollTo) {
      wx.pageScrollTo({
        selector,
        duration: 240
      });
    }
  },

  handleStartOnboarding() {
    if (typeof wx !== "undefined" && wx.navigateTo) {
      wx.navigateTo({
        url: "/pages/onboarding/onboarding"
      });
    }
  },

  handleOpenReport() {
    if (typeof wx !== "undefined" && wx.navigateTo) {
      wx.navigateTo({
        url: "/pages/report/report"
      });
    }
  },

  handleClearLocalSession() {
    if (typeof wx !== "undefined" && wx.removeStorageSync) {
      ["user_token", "token", "user_public_id", "onboarding_status"].forEach((key) => {
        wx.removeStorageSync(key);
      });
    }
    this.setData({
      tokenReady: false
    });
    showToast("已清除本地登录");
  }
};

if (typeof Page === "function") {
  Page(profilePageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    normalizeProfileSummary,
    profilePageConfig
  };
}
