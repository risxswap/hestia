const categoryOptions = [
  { label: "全部", value: "all" },
  { label: "上装", value: "top" },
  { label: "下装", value: "bottom" },
  { label: "外套", value: "outerwear" },
  { label: "鞋", value: "shoes" },
  { label: "包", value: "bag" },
  { label: "配饰", value: "accessory" },
  { label: "运动", value: "sport" },
  { label: "家居", value: "home" },
  { label: "其他", value: "other" }
];

const categoryLabels = categoryOptions.reduce((result, option) => {
  result[option.value] = option.label;
  return result;
}, {});

const defaultWardrobeOptions = {
  categories: categoryOptions.filter((option) => option.value !== "all"),
  materials: [
    { label: "棉", value: "cotton" },
    { label: "亚麻", value: "linen" },
    { label: "羊毛", value: "wool" },
    { label: "针织", value: "knit" },
    { label: "牛仔", value: "denim" },
    { label: "真丝", value: "silk" },
    { label: "皮革", value: "leather" },
    { label: "聚酯纤维", value: "polyester" },
    { label: "混纺", value: "blend" },
    { label: "其他", value: "other" }
  ],
  seasons: [
    { label: "春夏", value: "spring_summer" },
    { label: "春秋", value: "spring_autumn" },
    { label: "秋冬", value: "autumn_winter" },
    { label: "夏季", value: "summer" },
    { label: "冬季", value: "winter" },
    { label: "四季", value: "all_season" }
  ],
  silhouettes: [
    { label: "修身", value: "fitted" },
    { label: "合身", value: "regular" },
    { label: "微宽松", value: "slightly_relaxed" },
    { label: "宽松", value: "relaxed" },
    { label: "直筒", value: "straight" },
    { label: "A 字", value: "a_line" },
    { label: "短款", value: "cropped" },
    { label: "长款", value: "longline" },
    { label: "高腰", value: "high_waist" },
    { label: "其他", value: "other" }
  ]
};

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

function hasDraftValue(value) {
  if (Array.isArray(value)) {
    return value.length > 0;
  }
  return String(value || "").trim() !== "";
}

function normalizeOptionList(value, fallback) {
  const source = Array.isArray(value) && value.length ? value : fallback || [];
  return source
    .map((item) => ({
      label: String(item && item.label ? item.label : item && item.value ? item.value : "").trim(),
      value: String(item && item.value ? item.value : "").trim()
    }))
    .filter((item) => item.label && item.value);
}

function normalizeWardrobeOptions(options) {
  const source = options || {};
  return {
    categories: normalizeOptionList(source.categories, defaultWardrobeOptions.categories),
    materials: normalizeOptionList(source.materials, defaultWardrobeOptions.materials),
    seasons: normalizeOptionList(source.seasons, defaultWardrobeOptions.seasons),
    silhouettes: normalizeOptionList(source.silhouettes, defaultWardrobeOptions.silhouettes)
  };
}

function withEmptyOption(options) {
  return [{ label: "不选择", value: "" }].concat(options || []);
}

function optionLabel(options, value, emptyLabel) {
  const target = String(value || "").trim();
  if (!target) {
    return emptyLabel || "不选择";
  }
  const found = (options || []).find((item) => item.value === target);
  return found ? found.label : target;
}

function optionIndex(options, value) {
  const target = String(value || "").trim();
  const index = (options || []).findIndex((item) => item.value === target);
  return index >= 0 ? index : 0;
}

function matchOptionValue(options, value) {
  const target = String(value || "").trim();
  if (!target) {
    return "";
  }
  const byValue = (options || []).find((item) => item.value === target);
  if (byValue) {
    return byValue.value;
  }
  const byLabel = (options || []).find((item) => item.label === target);
  return byLabel ? byLabel.value : "";
}

function mergeRecognizedFieldsIntoDraft(draft, recognized, wardrobeOptions) {
  const result = cloneDraft(draft || {});
  const source = recognized || {};
  const options = normalizeWardrobeOptions(wardrobeOptions);
  [
    "name",
    "color",
    "user_notes"
  ].forEach((field) => {
    if (!hasDraftValue(result[field]) && hasDraftValue(source[field])) {
      result[field] = String(source[field]).trim();
    }
  });
  [
    ["category", options.categories],
    ["silhouette", options.silhouettes],
    ["material", options.materials],
    ["season", options.seasons]
  ].forEach(([field, optionList]) => {
    const matched = matchOptionValue(optionList, source[field]);
    if (!hasDraftValue(result[field]) && matched) {
      result[field] = matched;
    }
  });

  const recognizedSceneTags = normalizeSceneTags(source.scene_tags);
  if (!hasDraftValue(result.sceneText) && !hasDraftValue(result.scene_tags) && recognizedSceneTags.length) {
    result.scene_tags = recognizedSceneTags;
    result.sceneText = recognizedSceneTags.join("，");
  }

  return result;
}

function normalizeWardrobeItems(response) {
  const body = response && response.data ? response.data : response;
  const items = Array.isArray(body) ? body : body && Array.isArray(body.items) ? body.items : [];
  return items.map((item) => decorateWardrobeItem(item));
}

function statusTone(recommendationStatus, isCore, sceneTags) {
  if (recommendationStatus === "preferred") {
    return "优先推荐";
  }
  if (recommendationStatus === "paused") {
    return "暂不推荐";
  }
  if (isCore) {
    return "核心单品";
  }
  return sceneTags[0] || "已记录";
}

function styleLogicForItem(item) {
  const source = item || {};
  const sceneTags = normalizeTextList(source.scene_tags);
  const categoryLabel = categoryLabels[source.category] || source.category || "单品";
  const sceneCopy = sceneTags.length ? sceneTags.join("、") : "日常真实场景";
  const details = [source.color, source.silhouette, source.material].filter(Boolean).join("、");
  const status = source.recommendation_status || "normal";

  if (status === "paused") {
    return `这件${categoryLabel}目前标记为暂不推荐。后续建议会尽量避开它，除非你重新调整推荐状态或补充新的反馈。`;
  }

  if (details) {
    return `这件${categoryLabel}适合放进${sceneCopy}。${details}这些特征可以帮助建议更具体地控制比例、质感和场景表达。`;
  }

  return `这件${categoryLabel}适合放进${sceneCopy}。继续补充颜色、廓形或材质后，后续搭配建议会更稳定。`;
}

function recentFeedbackText(item) {
  const source = item || {};
  if (source.latest_feedback_text) {
    return source.latest_feedback_text;
  }
  if (source.latest_feedback && source.latest_feedback.summary) {
    return source.latest_feedback.summary;
  }
  return "还没有针对这件衣服的实际反馈。之后采纳、拒绝或修改建议时，会优先沉淀到这里。";
}

function decorateWardrobeItem(item) {
  const source = item || {};
  const sceneTags = normalizeTextList(source.scene_tags);
  const recommendationStatus = source.recommendation_status || "normal";
  const primaryImage = source.primary_image || null;
  const primaryImageSrc = primaryImage && primaryImage.preview_url
    ? primaryImage.preview_url
    : "";
  const category = source.category || "";
  const isCore = source.is_core !== false;
  const metaText = [categoryLabels[category] || category || "未分类", source.color || ""].filter(Boolean).join(" · ");

  return Object.assign({}, source, {
    public_id: source.public_id || source.publicID || "",
    name: source.name || "未命名单品",
    category,
    color: source.color || "",
    colorText: source.color || "未记录",
    silhouette: source.silhouette || "",
    silhouetteText: source.silhouette || "未记录",
    material: source.material || "",
    materialText: source.material || "未记录",
    season: source.season || "",
    seasonText: source.season || "未记录",
    scene_tags: sceneTags,
    sceneText: sceneTags.join(" / "),
    sceneDisplayText: sceneTags.length ? sceneTags.join(" / ") : "未记录",
    user_notes: source.user_notes || source.notes || "",
    userNotesText: source.user_notes || source.notes || "未记录",
    is_core: isCore,
    recommendation_status: recommendationStatus,
    recommendationLabel: recommendationLabels[recommendationStatus] || "正常推荐",
    categoryLabel: categoryLabels[category] || category || "未分类",
    metaText,
    primary_image: primaryImage,
    primaryImageSrc,
    status: source.status || "active",
    isPreferred: recommendationStatus === "preferred",
    isPaused: recommendationStatus === "paused",
    keyStatus: statusTone(recommendationStatus, isCore, sceneTags),
    coreLabel: isCore ? "核心单品" : "普通单品",
    styleLogic: styleLogicForItem(source),
    recentFeedbackText: recentFeedbackText(source)
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
    .filter((item) => {
      const status = item.status || "active";
      const recommendationStatus = item.recommendation_status || "normal";
      return status === "active" && recommendationStatus !== "paused" && (
        recommendationStatus === "preferred" || item.is_core === true
      );
    })
    .sort((left, right) => {
      const leftPreferred = left.recommendation_status === "preferred";
      const rightPreferred = right.recommendation_status === "preferred";
      if (leftPreferred !== rightPreferred) {
        return leftPreferred ? -1 : 1;
      }
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

  if (Object.prototype.hasOwnProperty.call(source, "primary_asset_public_id")) {
    payload.primary_asset_public_id = String(source.primary_asset_public_id).trim();
  }

  return payload;
}

function imageFilesFromAsset(asset) {
  const source = asset || {};
  const previewUrl = source.preview_url || source.local_url || "";
  const originalUrl = source.original_url || "";
  const url = previewUrl || originalUrl;
  if (!url) {
    return [];
  }

  const name = source.name || String(source.object_key || url).split("/").filter(Boolean).pop() || "衣服主图";
  return [
    {
      url,
      preview_url: previewUrl,
      original_url: originalUrl,
      name,
      type: "image",
      status: "done",
      asset_public_id: source.asset_public_id || source.public_id || "",
      object_key: source.object_key || ""
    }
  ];
}

function imageFilesFromItem(item) {
  return imageFilesFromAsset(item && item.primary_image);
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

function categoryOptionsWithCounts(items) {
  const sourceItems = Array.isArray(items) ? items : [];
  return categoryOptions.map((option) => {
    const count = option.value === "all"
      ? sourceItems.length
      : sourceItems.filter((item) => item.category === option.value).length;
    return Object.assign({}, option, {
      count,
      countLabel: `${option.label} ${count}`
    });
  });
}

module.exports = {
  categoryOptions,
  categoryLabels,
  recommendationLabels,
  defaultWardrobeOptions,
  cloneDraft,
  normalizeSceneTags,
  normalizeWardrobeOptions,
  withEmptyOption,
  optionLabel,
  optionIndex,
  matchOptionValue,
  mergeRecognizedFieldsIntoDraft,
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
  styleLogicForItem
};
