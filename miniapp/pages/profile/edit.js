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
    weight_kg: profile.weight_kg || profile.weight_kg === 0 ? String(profile.weight_kg) : "",
    body_notes: normalizeText(profile.body_notes),
    skin_notes: normalizeText(profile.skin_notes),
    hair_notes: normalizeText(profile.hair_notes),
    face_shape: normalizeText(profile.face_shape),
    upper_body_notes: normalizeText(profile.upper_body_notes),
    lower_body_notes: normalizeText(profile.lower_body_notes),
    size_notes: normalizeText(profile.size_notes),
    scenarioText: joinList(profile.lifestyle_scenarios)
  };
}

const photoObservationMeta = [
  {
    key: "face",
    title: "脸型与五官比例",
    noteField: "face_shape",
    noteLabel: "脸型与五官比例备注",
    notePlaceholder: "可不填。不确定脸型时，系统会结合这些照片识别。",
    slots: [
      { key: "headshot_front", label: "正面头肩照", photoType: "headshot", angle: "front" },
      { key: "headshot_left_45", label: "左 45 度头肩照", photoType: "headshot", angle: "left_45" },
      { key: "headshot_right_45", label: "右 45 度头肩照", photoType: "headshot", angle: "right_45" },
      { key: "headshot_side", label: "侧面头肩照", photoType: "headshot", angle: "side" }
    ]
  },
  {
    key: "body",
    title: "身形与比例",
    noteField: "body_notes",
    noteLabel: "身形与比例备注",
    notePlaceholder: "可不填。系统会根据全身照识别比例、线条和适合的廓形方向。",
    slots: [
      { key: "full_body_front", label: "正面全身照", photoType: "full_body", angle: "front" },
      { key: "full_body_side", label: "侧面全身照", photoType: "full_body", angle: "side" },
      { key: "full_body_back", label: "背面全身照", photoType: "full_body", angle: "back" },
      { key: "full_body_natural", label: "日常站姿全身照", photoType: "full_body", angle: "natural" }
    ]
  },
  {
    key: "beauty",
    title: "肤色、妆发与发型",
    noteField: "skin_notes",
    noteLabel: "肤色、妆发与发型备注",
    notePlaceholder: "可不填。系统会结合照片判断用色、妆感和发型方向。",
    slots: [
      { key: "headshot_natural", label: "自然光近照", photoType: "headshot", angle: "natural" },
      { key: "headshot_other", label: "日常妆发照", photoType: "headshot", angle: "other" },
      { key: "half_body_hair_side", label: "发型侧面照", photoType: "half_body", angle: "side" },
      { key: "half_body_hair_back", label: "发型背面照", photoType: "half_body", angle: "back" }
    ]
  }
];

const photoGroupMeta = [
  { key: "headshot", title: "自拍/头肩照", defaultAngle: "front" },
  { key: "half_body", title: "半身照", defaultAngle: "front" },
  { key: "full_body", title: "全身照", defaultAngle: "front" }
];

const genderOptions = [
  { label: "女", value: "female" },
  { label: "男", value: "male" }
];

function genderIndex(value) {
  const index = genderOptions.findIndex((item) => item.value === normalizeText(value));
  return index >= 0 ? index : 0;
}

function genderLabel(value) {
  return genderOptions[genderIndex(value)].label;
}

function genderValue(value) {
  return genderOptions[genderIndex(value)].value;
}

function withNormalizedGender(draft) {
  const source = draft || {};
  return Object.assign({}, source, {
    gender: genderValue(source.gender)
  });
}

function photoImageURL(photo) {
  const image = photo && photo.image ? photo.image : {};
  return image.url || image.preview_url || "";
}

function normalizePhoto(photo) {
  const source = photo || {};
  return {
    public_id: normalizeText(source.public_id),
    asset_public_id: normalizeText(source.asset_public_id),
    photo_type: normalizeText(source.photo_type),
    angle: normalizeText(source.angle),
    note: normalizeText(source.note),
    sort_order: source.sort_order || 0,
    image: source.image || {},
    url: photoImageURL(source)
  };
}

function buildPhotoGroups(summary) {
  const photos = Array.isArray(summary && summary.profile_photos) ? summary.profile_photos.map(normalizePhoto) : [];
  return photoGroupMeta.reduce((result, group) => {
    result[group.key] = {
      key: group.key,
      title: group.title,
      defaultAngle: group.defaultAngle,
      photos: photos.filter((photo) => photo.photo_type === group.key)
    };
    return result;
  }, {});
}

function photoSlotKey(photoType, angle) {
  return `${normalizeText(photoType)}:${normalizeText(angle)}`;
}

function buildPhotoObservations(summary, draft) {
  const photos = Array.isArray(summary && summary.profile_photos) ? summary.profile_photos.map(normalizePhoto) : [];
  const draftSource = draft || {};
  const photoBySlot = photos.reduce((result, photo) => {
    const key = photoSlotKey(photo.photo_type, photo.angle);
    result[key] = photo;
    return result;
  }, {});
  return photoObservationMeta.map((group) => Object.assign({}, group, {
    noteValue: normalizeText(draftSource[group.noteField]),
    slots: group.slots.map((slot) => Object.assign({}, slot, {
      photo: photoBySlot[photoSlotKey(slot.photoType, slot.angle)] || null
    })),
    photos: group.slots.map((slot) => Object.assign({}, slot, {
      photo: photoBySlot[photoSlotKey(slot.photoType, slot.angle)] || null
    })).filter((slot) => slot.photo)
  }));
}

function flattenObservationPhotos(observations) {
  return (observations || []).reduce((items, group) => (
    items.concat((group.slots || []).map((slot) => slot.photo).filter(Boolean))
  ), []);
}

function nextPhotoSortOrder(group) {
  const photos = group && Array.isArray(group.photos) ? group.photos : [];
  const maxOrder = photos.reduce((max, photo) => Math.max(max, Number(photo.sort_order) || 0), 0);
  return maxOrder + 10;
}

function payloadFromDraft(draft) {
  const source = draft || {};
  const heightText = normalizeText(source.height_cm);
  const heightValue = heightText ? Number(heightText) : null;
  const weightText = normalizeText(source.weight_kg);
  const weightValue = weightText ? Number(weightText) : null;
  return {
    nickname: normalizeText(source.nickname),
    gender: genderValue(source.gender),
    height_cm: Number.isFinite(heightValue) ? heightValue : null,
    weight_kg: Number.isFinite(weightValue) ? weightValue : null,
    body_notes: normalizeText(source.body_notes),
    skin_notes: normalizeText(source.skin_notes),
    hair_notes: normalizeText(source.hair_notes),
    face_shape: normalizeText(source.face_shape),
    upper_body_notes: normalizeText(source.upper_body_notes),
    lower_body_notes: normalizeText(source.lower_body_notes),
    size_notes: normalizeText(source.size_notes),
    lifestyle_scenarios: splitTextList(source.scenarioText)
  };
}

function getDataset(event) {
  return event && event.currentTarget && event.currentTarget.dataset ? event.currentTarget.dataset : {};
}

function getUploadFile(event) {
  const detail = event && event.detail ? event.detail : {};
  if (detail.file) {
    return detail.file;
  }
  const files = detail.files || detail.fileList || detail.currentFiles;
  if (Array.isArray(files) && files.length) {
    return files[0];
  }
  return null;
}

function chooseImageFile() {
  if (typeof wx === "undefined") {
    return Promise.resolve(null);
  }
  if (wx.chooseMedia) {
    return new Promise((resolve) => {
      wx.chooseMedia({
        count: 1,
        mediaType: ["image"],
        sourceType: ["album", "camera"],
        success(result) {
          const files = result && result.tempFiles ? result.tempFiles : [];
          resolve(files[0] || null);
        },
        fail() {
          resolve(null);
        }
      });
    });
  }
  if (wx.chooseImage) {
    return new Promise((resolve) => {
      wx.chooseImage({
        count: 1,
        sourceType: ["album", "camera"],
        success(result) {
          const paths = result && result.tempFilePaths ? result.tempFilePaths : [];
          resolve(paths[0] ? { tempFilePath: paths[0] } : null);
        },
        fail() {
          resolve(null);
        }
      });
    });
  }
  return Promise.resolve(null);
}

function chooseObservationSlot(group) {
  const slots = group && Array.isArray(group.slots) ? group.slots : [];
  if (!slots.length) {
    return Promise.resolve(null);
  }
  if (typeof wx === "undefined" || !wx.showActionSheet) {
    return Promise.resolve(slots[0]);
  }
  return new Promise((resolve) => {
    wx.showActionSheet({
      itemList: slots.map((slot) => slot.label),
      success(result) {
        resolve(slots[result && Number.isInteger(result.tapIndex) ? result.tapIndex : 0] || null);
      },
      fail() {
        resolve(null);
      }
    });
  });
}

function confirmDeletePhoto() {
  if (typeof wx === "undefined" || !wx.showModal) {
    return Promise.resolve(true);
  }
  return new Promise((resolve) => {
    wx.showModal({
      title: "删除照片",
      content: "删除后这张照片不会再用于档案建议。",
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

const profileEditPageConfig = {
  data: {
    loading: false,
    saving: false,
    imageUploading: false,
    errorMessage: "",
    imageUploadError: "",
    draft: withNormalizedGender(buildDraft(null)),
    photoGroups: buildPhotoGroups(null),
    photoObservations: buildPhotoObservations(null, withNormalizedGender(buildDraft(null))),
    genderOptions: genderOptions.map((item) => item.label),
    genderIndex: 0,
    genderLabel: genderOptions[0].label
  },

  onLoad() {
    return this.loadProfile();
  },

  async loadProfile() {
    this.setData({ loading: true, errorMessage: "" });
    try {
      const summary = await api.getProfileSummary();
      const draft = withNormalizedGender(buildDraft(summary));
      this.setData({
        loading: false,
        draft,
        photoGroups: buildPhotoGroups(summary),
        photoObservations: buildPhotoObservations(summary, draft),
        genderIndex: genderIndex(draft.gender),
        genderLabel: genderLabel(draft.gender)
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
    const draft = Object.assign({}, this.data.draft, {
      [field]: event && event.detail ? event.detail.value : ""
    });
    this.setData({
      draft,
      photoObservations: buildPhotoObservations({
        profile_photos: flattenObservationPhotos(this.data.photoObservations)
      }, draft)
    });
  },

  handleGenderChange(event) {
    const index = Number(event && event.detail ? event.detail.value : 0);
    const option = genderOptions[index] || genderOptions[0];
    this.setData({
      genderIndex: index >= 0 && index < genderOptions.length ? index : 0,
      genderLabel: option.label,
      draft: Object.assign({}, this.data.draft, {
        gender: option.value
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
  },

  async handleChoosePhoto(event) {
    const file = await chooseImageFile();
    if (!file) {
      return null;
    }
    return this.handlePhotoUpload({
      currentTarget: event && event.currentTarget ? event.currentTarget : { dataset: getDataset(event) },
      detail: { file }
    });
  },

  async handleAddObservationPhoto(event) {
    const groupKey = normalizeText(getDataset(event).groupKey);
    const group = (this.data.photoObservations || []).find((item) => item.key === groupKey);
    const slot = await chooseObservationSlot(group);
    if (!slot) {
      return null;
    }
    const file = await chooseImageFile();
    if (!file) {
      return null;
    }
    return this.handlePhotoUpload({
      currentTarget: {
        dataset: {
          type: slot.photoType,
          photoType: slot.photoType,
          angle: slot.angle
        }
      },
      detail: { file }
    });
  },

  async handlePhotoUpload(event) {
    const dataset = getDataset(event);
    const photoType = normalizeText(dataset.type || dataset.photoType) || "headshot";
    const group = this.data.photoGroups && this.data.photoGroups[photoType] ? this.data.photoGroups[photoType] : { defaultAngle: "front" };
    const angle = normalizeText(dataset.angle) || group.defaultAngle || "front";
    const file = getUploadFile(event);
    if (!file) {
      return null;
    }
    this.setData({ imageUploading: true, imageUploadError: "" });
    try {
      const uploaded = await api.uploadFileToQiniu(file, { assetType: "profile_photo" });
      const photo = await api.createProfilePhoto({
        asset_public_id: uploaded.asset_public_id,
        photo_type: photoType,
        angle,
        note: "",
        sort_order: nextPhotoSortOrder(group)
      });
      const groups = buildPhotoGroups({
        profile_photos: Object.keys(this.data.photoGroups || {}).reduce((items, key) => (
          items.concat((this.data.photoGroups[key].photos || []))
        ), []).concat(photo)
      });
      const observations = buildPhotoObservations({
        profile_photos: flattenObservationPhotos(this.data.photoObservations).filter((item) => (
          photoSlotKey(item.photo_type, item.angle) !== photoSlotKey(photo.photo_type, photo.angle)
        )).concat(photo)
      }, this.data.draft);
      this.setData({
        photoGroups: groups,
        photoObservations: observations,
        imageUploading: false,
        imageUploadError: ""
      });
      showToast("照片已保存", "success");
      return photo;
    } catch (error) {
      const message = error && error.message ? error.message : "照片上传失败";
      this.setData({
        imageUploading: false,
        imageUploadError: message
      });
      return null;
    }
  },

  async handleDeletePhoto(event) {
    const publicID = normalizeText(getDataset(event).publicId || getDataset(event).public_id);
    if (!publicID) {
      return null;
    }
    const confirmed = await confirmDeletePhoto();
    if (!confirmed) {
      return null;
    }
    try {
      await api.deleteProfilePhoto(publicID);
      const photos = Object.keys(this.data.photoGroups || {}).reduce((items, key) => (
        items.concat((this.data.photoGroups[key].photos || []))
      ), []).filter((photo) => photo.public_id !== publicID);
      const observationPhotos = flattenObservationPhotos(this.data.photoObservations)
        .filter((photo) => photo.public_id !== publicID);
      this.setData({
        photoGroups: buildPhotoGroups({ profile_photos: photos }),
        photoObservations: buildPhotoObservations({ profile_photos: observationPhotos }, this.data.draft),
        imageUploadError: ""
      });
      showToast("照片已删除", "success");
      return publicID;
    } catch (error) {
      this.setData({
        imageUploadError: error && error.message ? error.message : "删除照片失败"
      });
      return null;
    }
  }
};

if (typeof Page === "function") {
  Page(profileEditPageConfig);
}

if (typeof module !== "undefined") {
  module.exports = {
    buildDraft,
    buildPhotoGroups,
    payloadFromDraft,
    profileEditPageConfig
  };
}
