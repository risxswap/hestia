const api = require("../../utils/api");

const STEP_ORDER = ["welcome", "goals", "scenarios", "basic", "wardrobe", "submit"];
const GOAL_OPTIONS = ["通勤更有质感", "日常更显精神", "约会自然好看", "重要场合稳重专业", "找到稳定风格方向", "减少穿搭试错"];
const SCENARIO_OPTIONS = ["工作日通勤", "见客户或面试", "朋友聚会", "约会", "周末出行", "快速出门"];
const AVOIDANCE_OPTIONS = ["过度甜美", "太紧身", "显成熟或显老", "不好打理", "大面积高饱和色", "依赖高跟鞋"];
const WARDROBE_CATEGORY_OPTIONS = [
  { value: "top", label: "上装" },
  { value: "bottom", label: "下装" },
  { value: "outerwear", label: "外套" },
  { value: "dress", label: "连衣裙" },
  { value: "shoes", label: "鞋" },
  { value: "accessory", label: "配饰" }
];

const baseInitialData = {
  loading: false,
  submitting: false,
  statusMessage: "",
  errorMessage: "",
  currentStep: "welcome",
  stepOrder: STEP_ORDER,
  steps: ["基础特征", "风格目标与禁忌", "核心衣橱", "生成报告"],
  selectedGoals: [],
  customGoal: "",
  selectedScenarios: [],
  selectedAvoidances: [],
  basic: {
    gender: "",
    height_cm: "",
    body_notes: "",
    skin_notes: "",
    hair_notes: ""
  },
  wardrobeItems: [],
  wardrobeDraft: {
    name: "",
    category: "top",
    color: ""
  },
  styleGoal: {
    goalsText: "",
    avoidancesText: "",
    scenariosText: "",
    referenceStylesText: ""
  },
  wardrobeText: ""
};

function optionState(options, selectedValues) {
  const selectedMap = {};
  splitText(selectedValues).forEach((item) => {
    selectedMap[item] = true;
  });
  return options.map((option) => {
    if (typeof option === "string") {
      return {
        value: option,
        label: option,
        active: Boolean(selectedMap[option])
      };
    }
    return Object.assign({}, option, {
      active: Boolean(selectedMap[option.value])
    });
  });
}

function splitText(value) {
  if (Array.isArray(value)) {
    return value
      .map((item) => String(item || "").trim())
      .filter(Boolean);
  }

  return String(value || "")
    .split(/[\n,，、]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function uniqueList(values) {
  const seen = {};
  return splitText(values).filter((item) => {
    if (seen[item]) {
      return false;
    }
    seen[item] = true;
    return true;
  });
}

function toggleListValue(list, value) {
  const normalizedValue = String(value || "").trim();
  if (!normalizedValue) {
    return Array.isArray(list) ? list.slice() : [];
  }

  const currentList = splitText(list);
  if (currentList.includes(normalizedValue)) {
    return currentList.filter((item) => item !== normalizedValue);
  }
  return currentList.concat(normalizedValue);
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

function normalizeWardrobeItems(items) {
  if (!Array.isArray(items)) {
    return [];
  }

  return items
    .map((item) => ({
      name: String(item && item.name ? item.name : "").trim(),
      category: String(item && item.category ? item.category : "item").trim() || "item",
      color: String(item && item.color ? item.color : "").trim(),
      silhouette: String(item && item.silhouette ? item.silhouette : "").trim(),
      material: String(item && item.material ? item.material : "").trim(),
      season: String(item && item.season ? item.season : "").trim(),
      notes: String(item && item.notes ? item.notes : "").trim()
    }))
    .filter((item) => item.name);
}

function wardrobeItemsToText(items) {
  return normalizeWardrobeItems(items)
    .map((item) => [item.name, item.category, item.color, item.silhouette, item.material, item.season, item.notes].filter(Boolean).join("，"))
    .join("\n");
}

function decorateWardrobeItems(items) {
  return normalizeWardrobeItems(items).map((item, index) => Object.assign({}, item, {
    displayMeta: [item.category, item.color].filter(Boolean).join(" · "),
    viewKey: [item.name, item.category, item.color, index].filter((part) => part !== "").join("-")
  }));
}

function hasBasicValue(basic) {
  return Object.keys(basic || {}).some((key) => Boolean(basic[key]));
}

function stepFromPageData(pageData) {
  if (pageData.wardrobeItems.length) {
    return "wardrobe";
  }
  if (hasBasicValue(pageData.basic)) {
    return "basic";
  }
  if (pageData.selectedScenarios.length || pageData.selectedAvoidances.length) {
    return "scenarios";
  }
  if (pageData.selectedGoals.length) {
    return "goals";
  }
  return "welcome";
}

function viewState(data) {
  const selectedGoals = uniqueList(data.selectedGoals);
  const selectedScenarios = uniqueList(data.selectedScenarios);
  const selectedAvoidances = uniqueList(data.selectedAvoidances);
  const wardrobeItems = decorateWardrobeItems(data.wardrobeItems);
  const currentStep = data.currentStep || "welcome";
  const currentStepIndex = STEP_ORDER.indexOf(currentStep);
  const currentStepNumber = Math.min(Math.max(currentStepIndex, 0) + 1, 5);

  return {
    selectedGoals,
    selectedScenarios,
    selectedAvoidances,
    wardrobeItems,
    topbarMeta: currentStep === "welcome" ? "开始前" : `${currentStepNumber}/5`,
    goalOptions: optionState(GOAL_OPTIONS, selectedGoals),
    scenarioOptions: optionState(SCENARIO_OPTIONS, selectedScenarios),
    avoidanceOptions: optionState(AVOIDANCE_OPTIONS, selectedAvoidances),
    wardrobeCategoryOptions: optionState(WARDROBE_CATEGORY_OPTIONS, [(data.wardrobeDraft || {}).category || "top"]),
    selectedGoalsText: selectedGoals.length ? selectedGoals.join("，") : "待补充",
    selectedScenariosText: selectedScenarios.length ? selectedScenarios.join("，") : "待补充",
    selectedAvoidancesText: selectedAvoidances.length ? selectedAvoidances.join("，") : "待补充"
  };
}

function withViewState(data, patch) {
  const nextData = Object.assign({}, data, patch);
  return Object.assign({}, patch, viewState(nextData));
}

const initialData = Object.assign({}, baseInitialData, viewState(baseInitialData));

function buildDraftPayload(data) {
  const basic = data.basic || {};
  const styleGoal = data.styleGoal || {};
  const selectedGoals = Array.isArray(data.selectedGoals) && data.selectedGoals.length
    ? data.selectedGoals
    : splitText(styleGoal.goalsText);
  const customGoals = splitText(data.customGoal);
  const selectedAvoidances = Array.isArray(data.selectedAvoidances) && data.selectedAvoidances.length
    ? data.selectedAvoidances
    : splitText(styleGoal.avoidancesText);
  const selectedScenarios = Array.isArray(data.selectedScenarios) && data.selectedScenarios.length
    ? data.selectedScenarios
    : splitText(styleGoal.scenariosText);
  const wardrobeItems = Array.isArray(data.wardrobeItems) && data.wardrobeItems.length
    ? data.wardrobeItems
    : parseWardrobeText(data.wardrobeText);

  return {
    basic: {
      gender: basic.gender || "",
      height_cm: parseHeight(basic.height_cm),
      body_notes: basic.body_notes || "",
      skin_notes: basic.skin_notes || "",
      hair_notes: basic.hair_notes || ""
    },
    style_goal: {
      goals: uniqueList(selectedGoals.concat(customGoals)),
      avoidances: uniqueList(selectedAvoidances),
      scenarios: uniqueList(selectedScenarios),
      reference_styles: splitText(styleGoal.referenceStylesText)
    },
    wardrobe: {
      items: normalizeWardrobeItems(wardrobeItems)
    }
  };
}

function draftToPageData(draftData) {
  const basic = draftData.basic || {};
  const styleGoal = draftData.style_goal || {};
  const wardrobe = draftData.wardrobe || {};
  const selectedGoals = uniqueList(styleGoal.goals);
  const selectedAvoidances = uniqueList(styleGoal.avoidances);
  const selectedScenarios = uniqueList(styleGoal.scenarios);
  const wardrobeItems = normalizeWardrobeItems(wardrobe.items);
  const pageBasic = {
    gender: basic.gender || "",
    height_cm: basic.height_cm ? String(basic.height_cm) : "",
    body_notes: basic.body_notes || "",
    skin_notes: basic.skin_notes || "",
    hair_notes: basic.hair_notes || ""
  };

  return {
    basic: pageBasic,
    selectedGoals,
    customGoal: "",
    selectedScenarios,
    selectedAvoidances,
    wardrobeItems,
    wardrobeDraft: {
      name: "",
      category: "top",
      color: ""
    },
    styleGoal: {
      goalsText: selectedGoals.join("，"),
      avoidancesText: selectedAvoidances.join("，"),
      scenariosText: selectedScenarios.join("，"),
      referenceStylesText: splitText(styleGoal.reference_styles).join("，")
    },
    wardrobeText: wardrobeItemsToText(wardrobeItems)
  };
}

function stepFromDraft(draft) {
  if (!draft) {
    return "welcome";
  }

  if (draft.status === "submitted") {
    return "submit";
  }

  const currentStep = draft.current_step || "";
  if (STEP_ORDER.includes(currentStep) && currentStep !== "welcome") {
    return currentStep;
  }

  const pageData = draftToPageData(draft.draft_data || {});
  const inferredStep = stepFromPageData(pageData);
  if (inferredStep !== "welcome") {
    return inferredStep;
  }
  if (STEP_ORDER.includes(currentStep)) {
    return currentStep;
  }
  return "welcome";
}

function currentStepForSave(data, payload) {
  const currentStep = data.currentStep || "welcome";
  if (currentStep !== "welcome") {
    return currentStep;
  }
  return stepFromPageData(draftToPageData(payload || buildDraftPayload(data)));
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
      this.setData(withViewState(this.data, Object.assign({
        loading: false
      }, draftToPageData(draft.draft_data || {}), {
        currentStep: stepFromDraft(draft)
      })));
    } catch (error) {
      this.setData({
        loading: false,
        errorMessage: error && error.message ? error.message : "读取 onboarding 草稿失败"
      });
    }
  },

  handleStart() {
    this.setData(withViewState(this.data, {
      currentStep: "goals",
      errorMessage: "",
      statusMessage: ""
    }));
  },

  handleSkip() {
    if (typeof wx !== "undefined") {
      if (wx.setStorageSync) {
        wx.setStorageSync("onboarding_skip", true);
      }
      if (wx.switchTab) {
        wx.switchTab({
          url: "/pages/home/home"
        });
      }
    }
  },

  handleBasicInput(event) {
    const field = event.currentTarget.dataset.field;
    this.setData(withViewState(this.data, {
      basic: Object.assign({}, this.data.basic, {
        [field]: event.detail.value
      })
    }));
  },

  handleStyleInput(event) {
    const field = event.currentTarget.dataset.field;
    const value = event.detail.value;
    const nextStyleGoal = Object.assign({}, this.data.styleGoal, {
      [field]: value
    });
    const nextData = {
      styleGoal: nextStyleGoal
    };

    if (field === "goalsText") {
      nextData.selectedGoals = splitText(value);
    }
    if (field === "avoidancesText") {
      nextData.selectedAvoidances = splitText(value);
    }
    if (field === "scenariosText") {
      nextData.selectedScenarios = splitText(value);
    }

    this.setData(withViewState(this.data, nextData));
  },

  handleGoalToggle(event) {
    const value = event.currentTarget.dataset.value;
    const selectedGoals = toggleListValue(this.data.selectedGoals, value);
    this.setData(withViewState(this.data, {
      selectedGoals,
      styleGoal: Object.assign({}, this.data.styleGoal, {
        goalsText: selectedGoals.join("，")
      })
    }));
  },

  handleScenarioToggle(event) {
    const value = event.currentTarget.dataset.value;
    const selectedScenarios = toggleListValue(this.data.selectedScenarios, value);
    this.setData(withViewState(this.data, {
      selectedScenarios,
      styleGoal: Object.assign({}, this.data.styleGoal, {
        scenariosText: selectedScenarios.join("，")
      })
    }));
  },

  handleAvoidanceToggle(event) {
    const value = event.currentTarget.dataset.value;
    const selectedAvoidances = toggleListValue(this.data.selectedAvoidances, value);
    this.setData(withViewState(this.data, {
      selectedAvoidances,
      styleGoal: Object.assign({}, this.data.styleGoal, {
        avoidancesText: selectedAvoidances.join("，")
      })
    }));
  },

  handleCustomInput(event) {
    this.setData(withViewState(this.data, {
      customGoal: event.detail.value
    }));
  },

  handleWardrobeInput(event) {
    const wardrobeText = event.detail.value;
    this.setData(withViewState(this.data, {
      wardrobeText,
      wardrobeItems: parseWardrobeText(wardrobeText)
    }));
  },

  handleWardrobeDraftInput(event) {
    const field = event.currentTarget.dataset.field;
    this.setData(withViewState(this.data, {
      wardrobeDraft: Object.assign({}, this.data.wardrobeDraft, {
        [field]: event.detail.value
      })
    }));
  },

  handleWardrobeCategory(event) {
    const category = event.currentTarget.dataset.value;
    this.setData(withViewState(this.data, {
      wardrobeDraft: Object.assign({}, this.data.wardrobeDraft, {
        category
      })
    }));
  },

  handleAddWardrobeItem() {
    const nextItems = normalizeWardrobeItems(this.data.wardrobeItems.concat(this.data.wardrobeDraft));
    this.setData(withViewState(this.data, {
      wardrobeItems: nextItems,
      wardrobeText: wardrobeItemsToText(nextItems),
      wardrobeDraft: {
        name: "",
        category: "top",
        color: ""
      }
    }));
  },

  handleRemoveWardrobeItem(event) {
    const index = Number(event.currentTarget.dataset.index);
    const nextItems = this.data.wardrobeItems.filter((_, itemIndex) => itemIndex !== index);
    this.setData(withViewState(this.data, {
      wardrobeItems: nextItems,
      wardrobeText: wardrobeItemsToText(nextItems)
    }));
  },

  async handleSave() {
    const payload = buildDraftPayload(this.data);
    this.setData({
      loading: true,
      errorMessage: "",
      statusMessage: ""
    });

    try {
      await api.saveOnboardingDraft(currentStepForSave(this.data, payload), payload);
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

  handleBack() {
    const currentIndex = this.data.stepOrder.indexOf(this.data.currentStep);
    if (currentIndex <= 0) {
      return;
    }
    this.setData(withViewState(this.data, {
      currentStep: this.data.stepOrder[currentIndex - 1],
      errorMessage: "",
      statusMessage: ""
    }));
  },

  async handleNext() {
    const currentIndex = this.data.stepOrder.indexOf(this.data.currentStep);
    if (currentIndex < 0 || currentIndex >= this.data.stepOrder.length - 1) {
      return this.handleSubmit();
    }

    const payload = buildDraftPayload(this.data);
    const nextStep = this.data.stepOrder[currentIndex + 1];
    this.setData({
      loading: true,
      errorMessage: "",
      statusMessage: ""
    });

    try {
      await api.saveOnboardingDraft(nextStep, payload);
      this.setData(withViewState(this.data, {
        loading: false,
        currentStep: nextStep,
        statusMessage: "草稿已保存。"
      }));
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
      this.setData(withViewState(this.data, {
        submitting: false,
        currentStep: "submit",
        statusMessage: "初版报告已生成。"
      }));
      if (typeof wx !== "undefined" && wx.removeStorageSync) {
        wx.removeStorageSync("onboarding_skip");
      }
      if (typeof wx !== "undefined" && wx.setStorageSync) {
        wx.setStorageSync("onboarding_status", "completed");
      }
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
    toggleListValue,
    buildDraftPayload,
    draftToPageData,
    stepFromDraft,
    currentStepForSave,
    withViewState,
    onboardingPageConfig
  };
}
