const categoryOptions = [
  { label: "全部", value: "all" },
  { label: "上装", value: "上装" },
  { label: "下装", value: "下装" },
  { label: "外套", value: "外套" },
  { label: "鞋", value: "鞋" },
  { label: "包", value: "包" },
  { label: "配饰", value: "配饰" },
  { label: "运动", value: "运动" },
  { label: "家居", value: "家居" },
  { label: "其他", value: "其他" }
];

const categoryLabels = categoryOptions.reduce((result, option) => {
  result[option.value] = option.label;
  return result;
}, {});

const defaultClothesOptions = {
  categories: ["上装", "下装", "外套", "鞋", "包", "配饰", "运动", "家居", "其他"],
  colors: ["黑色", "白色", "米白", "灰色", "深蓝", "浅蓝", "棕色", "卡其", "红色", "绿色", "其他"],
  materials: ["棉", "亚麻", "羊毛", "针织", "牛仔", "真丝", "皮革", "聚酯纤维", "混纺", "其他"],
  seasons: ["春夏", "春秋", "秋冬", "夏季", "冬季", "四季"],
  silhouettes: ["修身", "合身", "微宽松", "宽松", "直筒", "A 字", "短款", "长款", "高腰", "其他"]
};

const recommendationLabels = {
  preferred: "优先推荐",
  normal: "正常推荐",
  paused: "暂不推荐"
};

const emptyDraft = {
  name: "",
  category: "上装",
  color: "",
  silhouette: "",
  material: "",
  season: "",
  scene_tags: [],
  sceneText: "",
  user_notes: "",
  is_core: true,
  recommendation_status: "normal",
  primary_asset_public_id: "",
  asset_public_ids: []
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
    .map((item) => {
      const value = String(typeof item === "string" ? item : item && item.value ? item.value : "").trim();
      return { label: value, value };
    })
    .filter((item) => item.label && item.value);
}

function normalizeClothesOptions(options) {
  const source = options || {};
  return {
    categories: normalizeOptionList(source.categories, defaultClothesOptions.categories),
    colors: normalizeOptionList(source.colors, defaultClothesOptions.colors),
    materials: normalizeOptionList(source.materials, defaultClothesOptions.materials),
    seasons: normalizeOptionList(source.seasons, defaultClothesOptions.seasons),
    silhouettes: normalizeOptionList(source.silhouettes, defaultClothesOptions.silhouettes)
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
  return "";
}

function mergeRecognizedFieldsIntoDraft(draft, recognized, clothesOptions) {
  const result = cloneDraft(draft || {});
  const source = recognized || {};
  const options = normalizeClothesOptions(clothesOptions);
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

function overwriteDraftWithRecognizedFields(draft, recognized, clothesOptions) {
  const result = cloneDraft(draft || {});
  const source = recognized || {};
  const options = normalizeClothesOptions(clothesOptions);
  [
    "name",
    "color",
    "user_notes"
  ].forEach((field) => {
    if (hasDraftValue(source[field])) {
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
    if (matched) {
      result[field] = matched;
    }
  });

  const recognizedSceneTags = normalizeSceneTags(source.scene_tags);
  if (recognizedSceneTags.length) {
    result.scene_tags = recognizedSceneTags;
    result.sceneText = recognizedSceneTags.join("，");
  }

  return result;
}

function normalizeClothesItems(response) {
  const body = response && response.data ? response.data : response;
  const items = Array.isArray(body) ? body : body && Array.isArray(body.items) ? body.items : [];
  return items.map((item) => decorateClothesItem(item));
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

function decorateClothesItem(item) {
  const source = item || {};
  const sceneTags = normalizeTextList(source.scene_tags);
  const recommendationStatus = source.recommendation_status || "normal";
  const recognitionStatus = source.recognition_status || "succeeded";
  const primaryImage = source.primary_image || null;
  const rawImages = Array.isArray(source.images) && source.images.length
    ? source.images
    : primaryImage
      ? [primaryImage]
      : [];
  const imageSlides = rawImages
    .map((image) => {
      const src = image && (image.preview_url || image.original_url) ? image.preview_url || image.original_url : "";
      return Object.assign({}, image || {}, {
        src
      });
    })
    .filter((image) => image.src);
  const primaryImageSrc = primaryImage && primaryImage.preview_url
    ? primaryImage.preview_url
    : imageSlides.length
      ? imageSlides[0].src
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
    recognition_status: recognitionStatus,
    isRecognizing: recognitionStatus === "pending",
    recognitionLabel: recognitionStatus === "pending" ? "识别中" : "",
    recommendationLabel: recommendationLabels[recommendationStatus] || "正常推荐",
    categoryLabel: categoryLabels[category] || category || "未分类",
    metaText,
    primary_image: primaryImage,
    images: rawImages,
    imageSlides,
    hasMultipleImages: imageSlides.length > 1,
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

function normalizeClothesGaps(report) {
  const content = report && report.content_json ? report.content_json : {};
  if (!Array.isArray(content.clothes_gaps)) {
    return [];
  }
  return content.clothes_gaps
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
    recommendation_status: source.recommendation_status || "normal",
    recognition_status: source.recognition_status || ""
  };

  if (Object.prototype.hasOwnProperty.call(source, "primary_asset_public_id")) {
    payload.primary_asset_public_id = String(source.primary_asset_public_id).trim();
  }
  if (Array.isArray(source.asset_public_ids)) {
    payload.asset_public_ids = source.asset_public_ids.map((item) => String(item || "").trim()).filter(Boolean);
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

  const name = source.name || String(source.object_key || url).split("/").filter(Boolean).pop() || "衣服图片";
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
  const decorated = decorateClothesItem(item);
  return cloneDraft({
    name: decorated.name,
    category: decorated.category || "上装",
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
      : "",
    asset_public_ids: decorated.primary_image && decorated.primary_image.asset_public_id
      ? [decorated.primary_image.asset_public_id]
      : []
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
  defaultClothesOptions,
  cloneDraft,
  normalizeSceneTags,
  normalizeClothesOptions,
  withEmptyOption,
  optionLabel,
  optionIndex,
  matchOptionValue,
  mergeRecognizedFieldsIntoDraft,
  overwriteDraftWithRecognizedFields,
  normalizeClothesItems,
  decorateClothesItem,
  filterItems,
  priorityItems,
  normalizeClothesGaps,
  buildPayload,
  imageFilesFromItem,
  imageFilesFromAsset,
  itemToDraft,
  categoryOptionsWithCounts,
  styleLogicForItem
};
