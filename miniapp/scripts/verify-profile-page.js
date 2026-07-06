const path = require("path");

const root = path.resolve(__dirname, "..");
const apiPath = path.join(root, "utils/api.js");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function loadPage(relativePath, apiStub) {
  const pagePath = path.join(root, relativePath);
  const originalPage = global.Page;
  let pageConfig = null;

  delete require.cache[require.resolve(apiPath)];
  delete require.cache[require.resolve(pagePath)];
  require.cache[require.resolve(apiPath)] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: apiStub
  };

  global.Page = (config) => {
    pageConfig = config;
  };

  const exported = require(pagePath);

  if (typeof originalPage === "undefined") {
    delete global.Page;
  } else {
    global.Page = originalPage;
  }

  return { pageConfig, exported };
}

function createPageInstance(pageConfig) {
  return Object.assign({}, pageConfig, {
    data: clone(pageConfig.data || {}),
    setData(patch) {
      this.data = Object.assign({}, this.data, patch);
    }
  });
}

const initialSummary = {
  user: {
    user_public_id: "usr_test",
    nickname: "明明",
    onboarding_status: "completed"
  },
  profile: {
    gender: "female",
    height_cm: 165,
    weight_kg: 52.5,
    body_notes: "想让通勤更利落",
    skin_notes: "偏好低饱和色",
    hair_notes: "希望好打理",
    face_shape: "方圆脸",
    upper_body_notes: "肩线偏窄",
    lower_body_notes: "偏好利落裤装",
    size_notes: "上衣 M，鞋码 37",
    lifestyle_scenarios: ["通勤", "周末见朋友"],
    style_goal_summary: "更利落"
  },
  profile_photos: [
    {
      public_id: "pph_head",
      asset_public_id: "ast_head",
      photo_type: "headshot",
      angle: "front",
      note: "正面自然光",
      sort_order: 10,
      image: { object_key: "users/12/profile/ast_head.jpg" }
    },
    {
      public_id: "pph_half",
      asset_public_id: "ast_half",
      photo_type: "half_body",
      angle: "side",
      note: "半身侧面",
      sort_order: 20,
      image: { url: "https://example.test/half.jpg" }
    },
    {
      public_id: "pph_full",
      asset_public_id: "ast_full",
      photo_type: "full_body",
      angle: "front",
      note: "全身正面",
      sort_order: 30,
      image: { url: "https://example.test/full.jpg" }
    }
  ],
  preferences: {
    style_goals: ["更利落"],
    avoidances: ["过甜"],
    scenario_preferences: ["通勤更正式"]
  },
  memory_summary: {
    fact_count: 4,
    preference_count: 2,
    avoidance_count: 1,
    inference_count: 3,
    pending_confirmation_count: 1
  },
  quick_entries: [
    { key: "profile", title: "我的档案", summary: "基础信息与场景" },
    { key: "preferences", title: "偏好与禁忌", summary: "风格目标与不想要的方向" },
    { key: "report", title: "报告与路线", summary: "初版报告已生成" },
    { key: "privacy", title: "隐私与数据", summary: "本地登录与删除入口" }
  ]
};

async function main() {
  const apiCalls = [];
  const apiStub = {
    getProfileSummary() {
      apiCalls.push({ name: "getProfileSummary" });
      return Promise.resolve(clone(initialSummary));
    },
    updateProfile(data) {
      apiCalls.push({ name: "updateProfile", data });
      return Promise.resolve(clone(initialSummary));
    },
    updateProfilePreferences(data) {
      apiCalls.push({ name: "updateProfilePreferences", data });
      return Promise.resolve(clone(initialSummary));
    },
    uploadFileToQiniu(file, options) {
      apiCalls.push({ name: "uploadFileToQiniu", file, options });
      return Promise.resolve({
        asset_public_id: "ast_uploaded",
        object_key: "users/12/profile/ast_uploaded.jpg",
        asset_type: "profile_photo"
      });
    },
    createProfilePhoto(data) {
      apiCalls.push({ name: "createProfilePhoto", data });
      return Promise.resolve({
        public_id: "pph_uploaded",
        asset_public_id: data.asset_public_id,
        photo_type: data.photo_type,
        angle: data.angle,
        note: data.note || "",
        sort_order: data.sort_order || 0,
        image: { url: "https://example.test/uploaded.jpg" }
      });
    },
    deleteProfilePhoto(publicID) {
      apiCalls.push({ name: "deleteProfilePhoto", publicID });
      return Promise.resolve({ public_id: publicID });
    }
  };

  const profile = loadPage("pages/profile/index.js", apiStub);
  assert(profile.pageConfig, "profile/index.js should register Page config");
  assert(typeof profile.pageConfig.loadProfile === "function", "profile index should load profile summary");
  assert(typeof profile.pageConfig.handleQuickEntry === "function", "profile index should handle quick entries");
  assert(typeof profile.exported.normalizeProfileSummary === "function", "profile index should export normalizeProfileSummary");

  const normalizedEmpty = profile.exported.normalizeProfileSummary(null);
  assert(normalizedEmpty.quickEntries.map((entry) => entry.key).join(",") === "profile,preferences,memory,report,privacy", "profile fallback entries should include independent memory domain");

  const profilePage = createPageInstance(profile.pageConfig);
  await profilePage.loadProfile.call(profilePage);
  assert(profilePage.data.user.nickname === "明明", "profile index should hydrate user nickname");
  assert(profilePage.data.profileDraft.scenarioText === "通勤、周末见朋友", "profile index should hydrate scenario summary");
  assert(profilePage.data.preferencesDraft.avoidancesText === "过甜", "profile index should hydrate avoidance summary");
  assert(profilePage.data.quickEntries.map((entry) => entry.key).join(",") === "profile,preferences,memory,report,privacy", "profile entries should keep memory before privacy");

  const profileMarkup = require("fs").readFileSync(path.join(root, "pages/profile/index.wxml"), "utf8");
  const profileStyles = require("fs").readFileSync(path.join(root, "pages/profile/index.wxss"), "utf8");
  assert(!profileMarkup.includes("我的形象档案"), "profile index should not show top identity title");
  assert(!profileMarkup.includes("用户："), "profile index should not show user id copy");
  assert(!profileMarkup.includes("本地开发用户"), "profile index should not show local user fallback copy");
  assert(!profileMarkup.includes("onboarding"), "profile index should not show onboarding status copy");
  assert(!profileMarkup.includes("补充档案"), "profile index should not show supplemental profile action");
  assert(!profileMarkup.includes("handleStartOnboarding"), "profile index should not bind onboarding action");
  assert(!profileMarkup.includes("档案摘要"), "profile index should not duplicate profile summary");
  assert(!profileMarkup.includes("长期记忆摘要"), "profile index should not duplicate memory summary");
  assert(!Object.prototype.hasOwnProperty.call(profilePage.data, "memoryItems"), "profile index should not prepare removed memory summary data");
  assert(profileMarkup.includes('class="quick-list"'), "profile index should render quick entries as a single-column list");
  assert(!profileMarkup.includes('class="quick-grid"'), "profile index should no longer render quick entries as a grid");
  assert(profileStyles.includes(".quick-list"), "profile index styles should define single-column quick list");
  assert(!profileStyles.includes(".quick-grid"), "profile index styles should remove quick grid layout");
  assert(!profileStyles.includes(".memory-grid"), "profile index styles should remove memory summary grid");

  const navigations = [];
  const originalWx = global.wx;
  global.wx = {
    navigateTo(options) {
      navigations.push(options.url);
    }
  };
  profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "profile" } } });
  profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "preferences" } } });
  profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "memory" } } });
  profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "privacy" } } });
  global.wx = originalWx;
  assert(navigations.join(",") === "/pages/profile/edit,/pages/preferences/edit,/pages/memory/index,/pages/privacy/index", `profile quick entry routes mismatch: ${navigations.join(",")}`);

  const profileEdit = loadPage("pages/profile/edit.js", apiStub);
  assert(profileEdit.pageConfig, "profile/edit.js should register Page config");
  assert(typeof profileEdit.exported.payloadFromDraft === "function", "profile edit should export payloadFromDraft");
  assert(profileEdit.exported.payloadFromDraft({ gender: "custom" }).gender === "female", "profile edit payload should normalize unknown gender to allowed option");
  const profileEditPage = createPageInstance(profileEdit.pageConfig);
  await profileEditPage.loadProfile.call(profileEditPage);
  assert(profileEditPage.data.draft.weight_kg === "52.5", "profile edit should hydrate weight");
  assert(profileEditPage.data.draft.face_shape === "方圆脸", "profile edit should hydrate face shape");
  assert(profileEditPage.data.draft.upper_body_notes === "肩线偏窄", "profile edit should hydrate upper body notes");
  assert(profileEditPage.data.draft.lower_body_notes === "偏好利落裤装", "profile edit should hydrate lower body notes");
  assert(profileEditPage.data.draft.size_notes === "上衣 M，鞋码 37", "profile edit should hydrate size notes");
  assert(profileEditPage.data.photoGroups.headshot.photos.length === 1, "profile edit should group headshot photos");
  assert(profileEditPage.data.photoGroups.headshot.photos[0].url === "", "profile edit should not use object_key as image URL");
  assert(profileEditPage.data.photoGroups.half_body.photos.length === 1, "profile edit should group half body photos");
  assert(profileEditPage.data.photoGroups.full_body.photos.length === 1, "profile edit should group full body photos");
  assert(profileEditPage.data.photoObservations.length === 3, "profile edit should render three fixed observation groups");
  assert(profileEditPage.data.photoObservations.map((group) => group.title).join(",") === "脸型与五官比例,身形与比例,肤色、妆发与发型", "profile edit should use fixed observation group titles");
  assert(profileEditPage.data.photoObservations.map((group) => group.noteLabel).join(",") === "脸型与五官比例备注,身形与比例备注,肤色、妆发与发型备注", "profile edit should label observation note fields as notes");
  assert(!profileEditPage.data.photoObservations.map((group) => group.noteLabel).join(",").includes("描述"), "profile edit note labels should not use description wording");
  assert(profileEditPage.data.photoObservations.map((group) => group.noteField).join(",") === "face_shape,body_notes,skin_notes", "profile edit should attach note fields to photo observation groups");
  assert(profileEditPage.data.photoObservations.map((group) => group.noteValue).join("|") === "方圆脸|想让通勤更利落|偏好低饱和色", "profile edit should expose observation note values without dynamic template lookup");
  assert(profileEditPage.data.photoObservations.every((group) => Array.isArray(group.photos)), "profile edit should expose uploaded photos per observation group");
  assert(profileEditPage.data.photoObservations[0].photos.length === 1, "profile edit face group should show uploaded photos instead of all empty slots");
  assert(profileEditPage.data.photoObservations[1].photos.length === 1, "profile edit body group should show uploaded photos instead of all empty slots");
  const observationSlots = profileEditPage.data.photoObservations.reduce((items, group) => items.concat(group.slots), []);
  assert(observationSlots.map((slot) => slot.label).includes("正面头肩照"), "profile edit should define front headshot slot");
  assert(observationSlots.map((slot) => slot.label).includes("左 45 度头肩照"), "profile edit should define left 45 headshot slot");
  assert(observationSlots.map((slot) => slot.label).includes("右 45 度头肩照"), "profile edit should define right 45 headshot slot");
  assert(observationSlots.map((slot) => slot.label).includes("侧面头肩照"), "profile edit should define side headshot slot");
  assert(observationSlots.map((slot) => slot.label).includes("正面全身照"), "profile edit should define front full body slot");
  assert(observationSlots.map((slot) => slot.label).includes("侧面全身照"), "profile edit should define side full body slot");
  assert(observationSlots.map((slot) => slot.label).includes("背面全身照"), "profile edit should define back full body slot");
  assert(observationSlots.map((slot) => slot.label).includes("日常站姿全身照"), "profile edit should define natural full body slot");
  assert(observationSlots.map((slot) => slot.label).includes("自然光近照"), "profile edit should define natural light close-up slot");
  assert(observationSlots.map((slot) => slot.label).includes("日常妆发照"), "profile edit should define daily makeup slot");
  assert(observationSlots.map((slot) => slot.label).includes("发型侧面照"), "profile edit should define side hair slot");
  assert(observationSlots.map((slot) => slot.label).includes("发型背面照"), "profile edit should define back hair slot");
  assert(observationSlots.every((slot) => slot.photoType && slot.angle), "profile edit fixed photo slots should carry upload type and angle");
  assert(observationSlots.map((slot) => slot.photoType).includes("headshot"), "profile edit fixed slots should include headshot upload type");
  assert(observationSlots.map((slot) => slot.photoType).includes("half_body"), "profile edit fixed slots should include half body upload type");
  assert(observationSlots.map((slot) => slot.photoType).includes("full_body"), "profile edit fixed slots should include full body upload type");
  profileEditPage.setData({
    draft: Object.assign({}, profileEditPage.data.draft, {
      nickname: "新的昵称",
      weight_kg: "53",
      face_shape: "鹅蛋脸",
      upper_body_notes: "肩线清晰",
      lower_body_notes: "喜欢直筒裤",
      size_notes: "上衣 M"
    })
  });
  await profileEditPage.handleSave.call(profileEditPage);
  const profileSave = apiCalls.find((call) => call.name === "updateProfile");
  assert(profileSave, "profile edit should call updateProfile");
  assert(profileSave.data.nickname === "新的昵称", "profile edit should pass nickname");
  assert(profileSave.data.gender === "female", "profile edit should preserve loaded gender before picker changes");
  assert(profileSave.data.lifestyle_scenarios.join(",") === "通勤,周末见朋友", "profile edit should preserve loaded scenarios without rendering a scenario input");
  assert(profileSave.data.weight_kg === 53, "profile edit should pass numeric weight");
  assert(profileSave.data.face_shape === "鹅蛋脸", "profile edit should pass face shape");
  assert(profileSave.data.upper_body_notes === "肩线清晰", "profile edit should pass upper body notes");
  assert(profileSave.data.lower_body_notes === "喜欢直筒裤", "profile edit should pass lower body notes");
  assert(profileSave.data.size_notes === "上衣 M", "profile edit should pass size notes");

  const profileEditMarkup = require("fs").readFileSync(path.join(root, "pages/profile/edit.wxml"), "utf8");
  const profileEditStyles = require("fs").readFileSync(path.join(root, "pages/profile/edit.wxss"), "utf8");
  assert(!profileEditMarkup.includes("性别表达"), "profile edit should rename gender expression to gender");
  assert(profileEditMarkup.includes("<picker"), "profile edit should use picker for gender");
  assert(profileEditMarkup.includes('range="{{genderOptions}}"'), "profile edit gender picker should use genderOptions");
  assert(profileEditStyles.includes(".picker-field"), "profile edit should style gender picker field");
  assert(/\.picker-field\s*\{[^}]*display:\s*flex;[^}]*align-items:\s*center;/s.test(profileEditStyles), "profile edit gender picker value should be vertically centered");
  assert(!profileEditMarkup.includes("常见场景"), "profile edit should remove confusing scenario input");
  assert(!profileEditMarkup.includes('data-field="scenarioText"'), "profile edit should not render scenarioText input");
  assert(profileEditMarkup.includes("照片档案"), "profile edit should render photo archive section");
  assert(!profileEditMarkup.includes("照片档案与备注"), "profile edit should not call photo archive section notes");
  assert(!profileEditMarkup.includes("尺码备注"), "profile edit should not render size notes extra field");
  assert(!profileEditMarkup.includes("发型补充"), "profile edit should not render hair extra field");
  assert(!profileEditMarkup.includes("上半身补充"), "profile edit should not render upper body extra field");
  assert(!profileEditMarkup.includes("下半身补充"), "profile edit should not render lower body extra field");
  assert(!profileEditMarkup.includes("archive-extra-grid"), "profile edit should remove extra archive grid");
  assert(!profileEditMarkup.includes("按角度补充"), "profile edit should not use generic angle guidance copy");
  assert(profileEditMarkup.includes('class="photo-observations"'), "profile edit should render fixed photo observation container");
  assert(profileEditMarkup.includes('wx:for="{{photoObservations}}"'), "profile edit should render observations from fixed model");
  assert(profileEditMarkup.includes('wx:for="{{group.photos}}"'), "profile edit should render uploaded photos inside each observation group");
  assert(profileEditMarkup.includes('class="photo-add-tile"'), "profile edit should render one add tile per observation group");
  assert(profileEditMarkup.includes('bind:tap="handleAddObservationPhoto"'), "profile edit should choose photo type when adding from group plus");
  assert(!profileEditMarkup.includes('wx:for="{{group.slots}}"'), "profile edit should not display every fixed photo slot upfront");
  assert(profileEditMarkup.includes('value="{{group.noteValue}}"'), "profile edit should bind observation note value directly");
  assert(profileEditMarkup.includes('data-field="{{group.noteField}}"'), "profile edit should render note input inside photo observation group");
  assert(!profileEditMarkup.includes("核心衣橱"), "profile edit should not include core wardrobe photos");
  assert(!profileEditMarkup.includes("显胖"), "profile edit copy should avoid anxiety wording");
  const profileEditScript = require("fs").readFileSync(path.join(root, "pages/profile/edit.js"), "utf8");
  assert(profileEditScript.includes('sourceType: ["album", "camera"]'), "profile edit photo picker should support camera capture");
  assert(Array.isArray(profileEditPage.data.genderOptions), "profile edit should expose gender options");
  assert(profileEditPage.data.genderOptions.join(",") === "女,男", "profile edit gender options should only be female and male labels");

  profileEditPage.handleGenderChange.call(profileEditPage, { detail: { value: 1 } });
  assert(profileEditPage.data.draft.gender === "male", "gender picker should write male value");
  assert(profileEditPage.data.genderLabel === "男", "gender picker should render selected label");
  await profileEditPage.handleSave.call(profileEditPage);
  const profileSaves = apiCalls.filter((call) => call.name === "updateProfile");
  const genderSave = profileSaves[profileSaves.length - 1];
  assert(genderSave.data.gender === "male", "profile edit should save gender selected from picker");

  const actionSheets = [];
  const chooseMediaOptions = [];
  global.wx = {
    showActionSheet(options) {
      actionSheets.push(options.itemList);
      options.success({ tapIndex: 1 });
    },
    chooseMedia(options) {
      chooseMediaOptions.push(options);
      options.success({
        tempFiles: [{
          tempFilePath: "wxfile://typed-profile-photo",
          size: 2048,
          fileType: "image"
        }]
      });
    },
    showToast() {}
  };
  await profileEditPage.handleAddObservationPhoto.call(profileEditPage, {
    currentTarget: { dataset: { groupKey: "face" } }
  });
  assert(actionSheets[0].join(",") === "正面头肩照,左 45 度头肩照,右 45 度头肩照,侧面头肩照", "profile edit should ask user what photo type they are uploading");
  assert(chooseMediaOptions[0].sourceType.join(",") === "album,camera", "profile edit chooseMedia should allow album and camera");
  const typedCreatePhotoCall = apiCalls.filter((call) => call.name === "createProfilePhoto").pop();
  assert(typedCreatePhotoCall.data.photo_type === "headshot", "typed profile photo upload should use selected photo type");
  assert(typedCreatePhotoCall.data.angle === "left_45", "typed profile photo upload should use selected angle");
  global.wx = originalWx;

  await profileEditPage.handlePhotoUpload.call(profileEditPage, {
    currentTarget: { dataset: { type: "full_body" } },
    detail: {
      file: {
        url: "wxfile://profile-photo",
        size: 2048,
        type: "image/jpeg"
      }
    }
  });
  const uploadCall = apiCalls.find((call) => call.name === "uploadFileToQiniu");
  assert(uploadCall, "profile edit should upload selected profile photo");
  assert(uploadCall.options.assetType === "profile_photo", "profile photo upload should use profile_photo asset type");
  const createPhotoCall = apiCalls.filter((call) => call.name === "createProfilePhoto").pop();
  assert(createPhotoCall, "profile edit should create profile photo reference after upload");
  assert(createPhotoCall.data.asset_public_id === "ast_uploaded", "profile photo create should pass uploaded asset id");
  assert(createPhotoCall.data.photo_type === "full_body", "profile photo create should pass photo type");
  assert(createPhotoCall.data.angle === "front", "profile photo create should default angle");

  global.wx = {
    showModal(options) {
      assert(options.title === "删除照片", "profile photo delete should show confirmation");
      options.success({ confirm: false });
    },
    showToast() {}
  };
  await profileEditPage.handleDeletePhoto.call(profileEditPage, {
    currentTarget: { dataset: { publicId: "pph_full" } }
  });
  assert(!apiCalls.find((call) => call.name === "deleteProfilePhoto"), "profile edit should not delete when confirmation is cancelled");

  global.wx = {
    showModal(options) {
      assert(options.title === "删除照片", "profile photo delete should show confirmation");
      options.success({ confirm: true });
    },
    showToast() {}
  };
  await profileEditPage.handleDeletePhoto.call(profileEditPage, {
    currentTarget: { dataset: { publicId: "pph_full" } }
  });
  global.wx = originalWx;
  const deletePhotoCall = apiCalls.find((call) => call.name === "deleteProfilePhoto");
  assert(deletePhotoCall && deletePhotoCall.publicID === "pph_full", "profile edit should delete profile photo reference");

  const preferencesEdit = loadPage("pages/preferences/edit.js", apiStub);
  assert(preferencesEdit.pageConfig, "preferences/edit.js should register Page config");
  assert(typeof preferencesEdit.exported.payloadFromDraft === "function", "preferences edit should export payloadFromDraft");
  const preferencesPage = createPageInstance(preferencesEdit.pageConfig);
  await preferencesPage.loadPreferences.call(preferencesPage);
  preferencesPage.setData({
    draft: {
      styleGoalsText: "更利落、轻松",
      avoidancesText: "过甜 / 太紧身",
      scenarioPreferencesText: "通勤，周末"
    }
  });
  await preferencesPage.handleSave.call(preferencesPage);
  const preferencesSave = apiCalls.find((call) => call.name === "updateProfilePreferences");
  assert(preferencesSave, "preferences edit should call updateProfilePreferences");
  assert(preferencesSave.data.style_goals.length === 2, "preferences edit should split style goals");
  assert(preferencesSave.data.avoidances.length === 2, "preferences edit should split avoidances");
  assert(preferencesSave.data.scenario_preferences.length === 2, "preferences edit should split scenario preferences");

  console.log("profile page verification passed");
}

main().catch((error) => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
