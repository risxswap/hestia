const recommendationLabels = {
  preferred: "优先推荐",
  normal: "正常推荐",
  paused: "暂不推荐"
};

function normalizeText(value) {
  return String(value || "").trim();
}

function normalizeTags(value) {
  if (Array.isArray(value)) {
    return value.map(normalizeText).filter(Boolean);
  }
  return String(value || "")
    .split(/[,\n，、；;]/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function imageSrc(item) {
  const image = item && item.primary_image ? item.primary_image : null;
  return image && image.preview_url ? image.preview_url : "";
}

function decorateHairItem(item) {
  const source = item || {};
  const sceneTags = normalizeTags(source.scene_tags);
  const recommendationStatus = source.recommendation_status || "normal";
  return Object.assign({}, source, {
    public_id: source.public_id || source.publicID || "",
    name: source.name || "未命名发型",
    length: source.length || "",
    shape: source.shape || "",
    bangs: source.bangs || "",
    color: source.color || "",
    care_time: source.care_time || "",
    scene_tags: sceneTags,
    sceneText: sceneTags.join("、"),
    sceneDisplayText: sceneTags.length ? sceneTags.join(" / ") : "未记录",
    suitability_notes: source.suitability_notes || "",
    avoidance_notes: source.avoidance_notes || "",
    user_notes: source.user_notes || "",
    recommendation_status: recommendationStatus,
    recommendationLabel: recommendationLabels[recommendationStatus] || "正常推荐",
    primaryImageSrc: imageSrc(source),
    metaText: [source.length, source.shape, source.color].filter(Boolean).join(" · ") || "未补充细节"
  });
}

function decorateMakeupItem(item) {
  const source = item || {};
  const sceneTags = normalizeTags(source.scene_tags);
  const recommendationStatus = source.recommendation_status || "normal";
  return Object.assign({}, source, {
    public_id: source.public_id || source.publicID || "",
    name: source.name || "未命名妆容",
    makeup_type: source.makeup_type || "",
    focus: source.focus || "",
    color_palette: source.color_palette || "",
    finish: source.finish || "",
    scene_tags: sceneTags,
    sceneText: sceneTags.join("、"),
    sceneDisplayText: sceneTags.length ? sceneTags.join(" / ") : "未记录",
    suitability_notes: source.suitability_notes || "",
    avoidance_notes: source.avoidance_notes || "",
    user_notes: source.user_notes || "",
    recommendation_status: recommendationStatus,
    recommendationLabel: recommendationLabels[recommendationStatus] || "正常推荐",
    primaryImageSrc: imageSrc(source),
    metaText: [source.makeup_type, source.focus, source.finish].filter(Boolean).join(" · ") || "未补充细节"
  });
}

function normalizeItems(response, decorator) {
  const body = response && response.data ? response.data : response;
  const items = Array.isArray(body) ? body : body && Array.isArray(body.items) ? body.items : [];
  return items.map(decorator);
}

function baseDraft(overrides) {
  const source = overrides || {};
  return Object.assign({
    name: "",
    sceneText: "",
    scene_tags: [],
    suitability_notes: "",
    avoidance_notes: "",
    user_notes: "",
    recommendation_status: "normal",
    primary_asset_public_id: "",
    asset_public_ids: []
  }, source, {
    scene_tags: normalizeTags(source.scene_tags),
    sceneText: Object.prototype.hasOwnProperty.call(source, "sceneText")
      ? source.sceneText
      : normalizeTags(source.scene_tags).join("、")
  });
}

function hairDraft(item) {
  const source = decorateHairItem(item);
  return baseDraft({
    name: source.name,
    length: source.length,
    shape: source.shape,
    bangs: source.bangs,
    color: source.color,
    care_time: source.care_time,
    scene_tags: source.scene_tags,
    suitability_notes: source.suitability_notes,
    avoidance_notes: source.avoidance_notes,
    user_notes: source.user_notes,
    recommendation_status: source.recommendation_status,
    primary_asset_public_id: source.primary_image && source.primary_image.asset_public_id ? source.primary_image.asset_public_id : "",
    asset_public_ids: source.primary_image && source.primary_image.asset_public_id ? [source.primary_image.asset_public_id] : []
  });
}

function makeupDraft(item) {
  const source = decorateMakeupItem(item);
  return baseDraft({
    name: source.name,
    makeup_type: source.makeup_type,
    focus: source.focus,
    color_palette: source.color_palette,
    finish: source.finish,
    scene_tags: source.scene_tags,
    suitability_notes: source.suitability_notes,
    avoidance_notes: source.avoidance_notes,
    user_notes: source.user_notes,
    recommendation_status: source.recommendation_status,
    primary_asset_public_id: source.primary_image && source.primary_image.asset_public_id ? source.primary_image.asset_public_id : "",
    asset_public_ids: source.primary_image && source.primary_image.asset_public_id ? [source.primary_image.asset_public_id] : []
  });
}

function payloadFromDraft(draft) {
  const source = draft || {};
  const sceneTags = normalizeTags(source.scene_tags && source.scene_tags.length ? source.scene_tags : source.sceneText);
  const payload = {};
  Object.keys(source).forEach((key) => {
    if (key.endsWith("Label") || key.endsWith("Index") || key === "sceneText") {
      return;
    }
    payload[key] = source[key];
  });
  payload.name = normalizeText(source.name);
  payload.scene_tags = sceneTags;
  payload.suitability_notes = normalizeText(source.suitability_notes);
  payload.avoidance_notes = normalizeText(source.avoidance_notes);
  payload.user_notes = normalizeText(source.user_notes);
  payload.recommendation_status = source.recommendation_status || "normal";
  payload.asset_public_ids = Array.isArray(source.asset_public_ids)
    ? source.asset_public_ids.map(normalizeText).filter(Boolean)
    : [];
  payload.primary_asset_public_id = normalizeText(source.primary_asset_public_id);
  return payload;
}

function imageFilesFromItem(item) {
  const image = item && item.primary_image ? item.primary_image : null;
  const src = image && (image.preview_url || image.original_url);
  if (!src) {
    return [];
  }
  return [{
    url: src,
    name: image.object_key ? image.object_key.split("/").pop() : "图片",
    type: "image",
    status: "done",
    asset_public_id: image.asset_public_id || image.public_id || "",
    object_key: image.object_key || ""
  }];
}

module.exports = {
  recommendationLabels,
  normalizeTags,
  normalizeItems,
  decorateHairItem,
  decorateMakeupItem,
  baseDraft,
  hairDraft,
  makeupDraft,
  payloadFromDraft,
  imageFilesFromItem
};
