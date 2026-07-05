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

const photoGroupMeta = [
  { key: "headshot", title: "自拍/头肩照", defaultAngle: "front" },
  { key: "half_body", title: "半身照", defaultAngle: "front" },
  { key: "full_body", title: "全身照", defaultAngle: "front" }
];

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
    gender: normalizeText(source.gender),
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
    draft: buildDraft(null),
    photoGroups: buildPhotoGroups(null)
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
        draft: buildDraft(summary),
        photoGroups: buildPhotoGroups(summary)
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
      this.setData({
        photoGroups: groups,
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
      this.setData({
        photoGroups: buildPhotoGroups({ profile_photos: photos }),
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
