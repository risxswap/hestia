const api = require("../../utils/api");

const initialData = {
  loading: false,
  submitting: false,
  statusMessage: "",
  errorMessage: "",
  steps: ["基础特征", "风格目标与禁忌", "核心衣橱", "生成报告"],
  basic: {
    gender: "",
    height_cm: "",
    body_notes: "",
    skin_notes: "",
    hair_notes: ""
  },
  styleGoal: {
    goalsText: "",
    avoidancesText: "",
    scenariosText: "",
    referenceStylesText: ""
  },
  wardrobeText: ""
};

function splitText(value) {
  return String(value || "")
    .split(/[\n,，、]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function parseHeight(value) {
  const parsed = Number.parseInt(String(value || "").trim(), 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

function parseWardrobeText(value) {
  return String(value || "")
    .split(/\n+/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const parts = line.split(/[,，]/).map((item) => item.trim());
      return {
        name: parts[0] || "",
        category: parts[1] || "item",
        color: parts[2] || "",
        silhouette: parts[3] || "",
        material: parts[4] || "",
        season: parts[5] || "",
        notes: parts.slice(6).join("，")
      };
    })
    .filter((item) => item.name);
}

function buildDraftPayload(data) {
  return {
    basic: {
      gender: data.basic.gender,
      height_cm: parseHeight(data.basic.height_cm),
      body_notes: data.basic.body_notes,
      skin_notes: data.basic.skin_notes,
      hair_notes: data.basic.hair_notes
    },
    style_goal: {
      goals: splitText(data.styleGoal.goalsText),
      avoidances: splitText(data.styleGoal.avoidancesText),
      scenarios: splitText(data.styleGoal.scenariosText),
      reference_styles: splitText(data.styleGoal.referenceStylesText)
    },
    wardrobe: {
      items: parseWardrobeText(data.wardrobeText)
    }
  };
}

function draftToPageData(draftData) {
  const basic = draftData.basic || {};
  const styleGoal = draftData.style_goal || {};
  const wardrobe = draftData.wardrobe || {};
  const items = Array.isArray(wardrobe.items) ? wardrobe.items : [];

  return {
    basic: {
      gender: basic.gender || "",
      height_cm: basic.height_cm ? String(basic.height_cm) : "",
      body_notes: basic.body_notes || "",
      skin_notes: basic.skin_notes || "",
      hair_notes: basic.hair_notes || ""
    },
    styleGoal: {
      goalsText: splitText(styleGoal.goals).join("，"),
      avoidancesText: splitText(styleGoal.avoidances).join("，"),
      scenariosText: splitText(styleGoal.scenarios).join("，"),
      referenceStylesText: splitText(styleGoal.reference_styles).join("，")
    },
    wardrobeText: items
      .map((item) => [item.name, item.category, item.color, item.silhouette, item.material, item.season, item.notes].filter(Boolean).join("，"))
      .join("\n")
  };
}

function validateDraftPayload(payload) {
  if (!payload.style_goal.goals.length) {
    return "请至少填写一个风格目标。";
  }
  if (!payload.wardrobe.items.length) {
    return "请至少填写一件核心衣橱单品。";
  }
  return "";
}

const onboardingPageConfig = {
  data: initialData,

  onLoad() {
    return this.loadDraft();
  },

  async loadDraft() {
    this.setData({
      loading: true,
      errorMessage: "",
      statusMessage: ""
    });

    try {
      const draft = await api.getOnboardingDraft();
      this.setData(Object.assign({
        loading: false
      }, draftToPageData(draft.draft_data || {})));
    } catch (error) {
      this.setData({
        loading: false,
        errorMessage: error && error.message ? error.message : "读取 onboarding 草稿失败"
      });
    }
  },

  handleBasicInput(event) {
    const field = event.currentTarget.dataset.field;
    this.setData({
      basic: Object.assign({}, this.data.basic, {
        [field]: event.detail.value
      })
    });
  },

  handleStyleInput(event) {
    const field = event.currentTarget.dataset.field;
    this.setData({
      styleGoal: Object.assign({}, this.data.styleGoal, {
        [field]: event.detail.value
      })
    });
  },

  handleWardrobeInput(event) {
    this.setData({
      wardrobeText: event.detail.value
    });
  },

  async handleSave() {
    const payload = buildDraftPayload(this.data);
    this.setData({
      loading: true,
      errorMessage: "",
      statusMessage: ""
    });

    try {
      await api.saveOnboardingDraft("wardrobe", payload);
      this.setData({
        loading: false,
        statusMessage: "草稿已保存。"
      });
    } catch (error) {
      this.setData({
        loading: false,
        errorMessage: error && error.message ? error.message : "保存草稿失败"
      });
    }
  },

  async handleSubmit() {
    const payload = buildDraftPayload(this.data);
    const validationMessage = validateDraftPayload(payload);
    if (validationMessage) {
      this.setData({
        errorMessage: validationMessage,
        statusMessage: ""
      });
      return;
    }

    this.setData({
      submitting: true,
      errorMessage: "",
      statusMessage: ""
    });

    try {
      await api.saveOnboardingDraft("wardrobe", payload);
      const result = await api.submitOnboarding();
      this.setData({
        submitting: false,
        statusMessage: "初版报告已生成。"
      });
      if (typeof wx !== "undefined" && wx.navigateTo) {
        wx.navigateTo({
          url: `/pages/report/report?public_id=${result.report_public_id}`
        });
      }
    } catch (error) {
      this.setData({
        submitting: false,
        errorMessage: error && error.message ? error.message : "生成初版报告失败"
      });
    }
  }
};

if (typeof Page === "function") {
  Page(onboardingPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    splitText,
    parseWardrobeText,
    buildDraftPayload,
    onboardingPageConfig
  };
}
