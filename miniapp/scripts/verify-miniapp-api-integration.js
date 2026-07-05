const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const apiPath = path.join(root, "utils/api.js");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function read(relativePath) {
  return fs.readFileSync(path.join(root, relativePath), "utf8");
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function loadPage(relativePath, apiStub) {
  const pagePath = path.join(root, relativePath);
  const originalPage = global.Page;
  let config = null;

  delete require.cache[require.resolve(apiPath)];
  delete require.cache[require.resolve(pagePath)];
  require.cache[require.resolve(apiPath)] = {
    id: apiPath,
    filename: apiPath,
    loaded: true,
    exports: apiStub || {}
  };

  global.Page = (pageConfig) => {
    config = pageConfig;
  };

  const exported = require(pagePath);

  if (typeof originalPage === "undefined") {
    delete global.Page;
  } else {
    global.Page = originalPage;
  }

  return { config, exported };
}

function createPageInstance(config) {
  return Object.assign({}, config, {
    data: clone(config.data || {}),
    setData(patch) {
      this.data = Object.assign({}, this.data, patch);
    }
  });
}

function withWx(wxStub, fn) {
  const originalWx = global.wx;
  const originalGetApp = global.getApp;
  const app = { globalData: {} };
  global.wx = wxStub || {};
  global.getApp = () => app;
  return Promise.resolve()
    .then(() => fn(app))
    .finally(() => {
      if (typeof originalWx === "undefined") {
        delete global.wx;
      } else {
        global.wx = originalWx;
      }
      if (typeof originalGetApp === "undefined") {
        delete global.getApp;
      } else {
        global.getApp = originalGetApp;
      }
    });
}

async function verifyRoutesAndStaticContracts() {
  const appJson = JSON.parse(read("app.json"));
  [
    "pages/clothes/list",
    "pages/clothes/detail",
    "pages/clothes/edit",
    "pages/hair/list",
    "pages/hair/detail",
    "pages/hair/edit",
    "pages/makeup/list",
    "pages/makeup/detail",
    "pages/makeup/edit",
    "pages/profile/index",
    "pages/profile/edit",
    "pages/preferences/edit",
    "pages/memory/index",
    "pages/privacy/index"
  ].forEach((pagePath) => {
    assert(appJson.pages.includes(pagePath), `app.json should register ${pagePath}`);
  });
  [
    "pages/wardrobe/wardrobe",
    "pages/wardrobe-detail/wardrobe-detail",
    "pages/wardrobe-edit/wardrobe-edit",
    "pages/references/references",
    "pages/profile/profile"
  ].forEach((pagePath) => {
    assert(!appJson.pages.includes(pagePath), `app.json should remove old route ${pagePath}`);
  });
  assert(appJson.tabBar.list[3].pagePath === "pages/profile/index", "profile tab should use pages/profile/index");

  const collectionMarkup = read("pages/collection/collection.wxml");
  const collectionScript = read("pages/collection/collection.js");
  assert(collectionMarkup.includes("保存衣服、发型和妆容"), "collection copy should mention three domains");
  assert(!collectionMarkup.includes("参考图"), "collection copy should remove references");
  assert(!collectionScript.includes("references"), "collection script should remove references");

  const clothesDetailMarkup = read("pages/clothes/detail.wxml");
  assert(!clothesDetailMarkup.includes("<input"), "clothes detail should not include inputs");
  assert(!clothesDetailMarkup.includes("<textarea"), "clothes detail should not include textareas");
  assert(!clothesDetailMarkup.includes("保存"), "clothes detail should not include save action");
  assert(read("pages/clothes/edit.wxml").includes("保存"), "clothes edit should include save action");
  const profileMarkup = read("pages/profile/index.wxml");
  assert(!profileMarkup.includes("<input"), "profile index should not expose inline inputs");
  assert(!profileMarkup.includes("<textarea"), "profile index should not expose inline textareas");
  assert(!profileMarkup.includes("我的形象档案"), "profile index should not show top identity title");
  assert(!profileMarkup.includes("用户："), "profile index should not show user id copy");
  assert(!profileMarkup.includes("本地开发用户"), "profile index should not show local user fallback copy");
  assert(!profileMarkup.includes("onboarding"), "profile index should not show onboarding status copy");
  assert(!profileMarkup.includes("补充档案"), "profile index should not show supplemental profile action");
  assert(!profileMarkup.includes("handleStartOnboarding"), "profile index should not bind onboarding action");
  assert(!profileMarkup.includes("档案摘要"), "profile index should not duplicate profile summary");
  assert(!profileMarkup.includes("长期记忆摘要"), "profile index should not duplicate memory summary");
  assert(profileMarkup.includes('class="quick-list"'), "profile index should render entries as rows");
}

async function verifyCollectionPage() {
  const { config, exported } = loadPage("pages/collection/collection.js", {
    getCollectionSummary: async () => ({
      types: [
        { type: "clothes", label: "衣服", count: 2, entry_path: "/pages/clothes/list" },
        { type: "hair", label: "发型", count: 1, entry_path: "/pages/hair/list" },
        { type: "makeup", label: "妆容", count: 1, entry_path: "/pages/makeup/list" }
      ],
      recent_items: [
        {
          type: "clothes",
          public_id: "clo_shirt",
          title: "米白衬衫",
          subtitle: "上装",
          entry_path: "/pages/clothes/detail?public_id=clo_shirt"
        }
      ]
    })
  });
  assert(config, "collection page should register Page config");
  assert(exported.DEFAULT_TYPES.length === 3, "collection page should have three type entries");
  assert(exported.DEFAULT_TYPES.map((item) => item.type).join(",") === "clothes,hair,makeup", "collection types should be clothes,hair,makeup");
  const page = createPageInstance(config);
  await page.loadCollection.call(page);
  assert(page.data.types.length === 3, "collection instance should render three types");
  assert(page.data.types[0].entryPath === "/pages/clothes/list", "clothes type should enter clothes list");
  assert(page.data.recentItems[0].entryPath === "/pages/clothes/detail?public_id=clo_shirt", "recent item should enter detail page");

  const navigations = [];
  await withWx({
    navigateTo(options) {
      navigations.push(options.url);
    }
  }, async () => {
    page.handleOpenType.call(page, { currentTarget: { dataset: { entryPath: "/pages/hair/list" } } });
    page.handleOpenRecent.call(page, { currentTarget: { dataset: { entryPath: "/pages/clothes/detail?public_id=clo_shirt" } } });
  });
  assert(navigations.join(",") === "/pages/hair/list,/pages/clothes/detail?public_id=clo_shirt", `collection navigation mismatch: ${navigations.join(",")}`);
}

async function verifyClothesPages() {
  const clothesItemNames = ["米白衬衫"];
  const clothesItemRequests = [];
  const apiStub = {
    getLatestReport: async () => ({ content_json: { clothes_gaps: ["浅色短外套"] } }),
    getClothesOptions: async () => ({ categories: ["上装", "下装"], colors: ["米白"] }),
    getClothesItems: async () => ({
      items: [{
        public_id: "clo_shirt",
        name: "米白衬衫",
        category: "上装",
        color: "米白",
        is_core: true,
        recommendation_status: "normal",
        recognition_status: "succeeded"
      }]
    }),
    getClothesItem: async (publicID) => ({
      public_id: "clo_shirt",
      name: clothesItemNames[Math.min(clothesItemRequests.length, clothesItemNames.length - 1)],
      category: "上装",
      color: "米白",
      is_core: true,
      recommendation_status: "normal",
      recognition_status: "succeeded"
    }),
    createClothesItem: async (payload) => Object.assign({
      public_id: "clo_new",
      category: "其他",
      recognition_status: "pending",
      recommendation_status: "normal"
    }, payload),
    updateClothesItem: async (_publicID, payload) => Object.assign({
      public_id: "clo_shirt",
      recognition_status: "succeeded"
    }, payload),
    deleteClothesItem: async () => ({ ok: true }),
    recognizeClothesItemImage: async () => ({ name: "识别衣服", category: "上装" }),
    uploadFileToQiniu: async () => ({ asset_public_id: "ast_photo" })
  };

  const list = loadPage("pages/clothes/list.js", apiStub);
  assert(list.config, "clothes list should register Page config");
  const listPage = createPageInstance(list.config);
  await listPage.loadClothes.call(listPage);
  assert(listPage.data.items.length === 1, "clothes list should load items");
  assert(listPage.data.gaps.length === 1, "clothes list should load clothes gaps");
  const listNavigations = [];
  await withWx({
    navigateTo(options) {
      listNavigations.push(options.url);
    }
  }, async () => {
    listPage.handleOpenDetail.call(listPage, { currentTarget: { dataset: { publicId: "clo_shirt" } } });
  });
  assert(
    listNavigations[0] === "/pages/clothes/detail?public_id=clo_shirt&source=clothes",
    "clothes list should navigate to detail with clothes source",
  );

  listPage.handleOpenCreate.call(listPage);
  listPage.setData({
    draft: Object.assign({}, listPage.data.draft, {
      name: "黑色西装",
      category: "外套",
      asset_public_ids: ["ast_photo"]
    })
  });
  await listPage.handleSaveItem.call(listPage);
  assert(listPage.data.items.some((item) => item.public_id === "clo_new"), "clothes list create modal should append created item");

  const detail = loadPage("pages/clothes/detail.js", apiStub);
  assert(detail.config, "clothes detail should register Page config");
  const detailPage = createPageInstance(detail.config);
  apiStub.getClothesItem = async (publicID) => {
    clothesItemRequests.push(publicID);
    return {
      public_id: "clo_shirt",
      name: clothesItemNames[Math.min(clothesItemRequests.length - 1, clothesItemNames.length - 1)],
      category: "上装",
      color: "米白",
      images: [
        { asset_public_id: "ast_front", preview_url: "https://img.example.test/front.webp" },
        { asset_public_id: "ast_side", preview_url: "https://img.example.test/side.webp" }
      ],
      is_core: true,
      recommendation_status: "normal",
      recognition_status: "succeeded"
    };
  };
  await detailPage.onLoad.call(detailPage, { public_id: "clo_shirt", source: "clothes" });
  assert(detailPage.data.item.name === "米白衬衫", "clothes detail should load item");
  assert(detailPage.data.item.imageSlides.length === 2, "clothes detail should normalize all item images for swiper");
  assert(detailPage.data.item.hasMultipleImages === true, "clothes detail should mark multiple image items");
  assert(detailPage.data.backLabel === "返回衣服", "clothes detail opened from clothes should show clothes back label");
  assert(detailPage.data.backUrl === "/pages/clothes/list", "clothes detail opened from clothes should return to clothes list");
  const collectionDetailPage = createPageInstance(detail.config);
  await collectionDetailPage.onLoad.call(collectionDetailPage, { public_id: "clo_shirt" });
  assert(collectionDetailPage.data.backLabel === "返回私藏", "clothes detail opened without source should keep collection back label");
  assert(collectionDetailPage.data.backUrl === "/pages/collection/collection", "clothes detail opened without source should return to collection");
  const detailNavigations = [];
  await withWx({
    navigateTo(options) {
      detailNavigations.push(options.url);
    }
  }, async () => {
    detailPage.handleOpenEdit.call(detailPage);
  });
  assert(detailNavigations[0] === "/pages/clothes/edit?public_id=clo_shirt", "clothes detail should navigate to edit");
  clothesItemNames.push("米白衬衫更新");
  await withWx({}, async (app) => {
    app.globalData.clothesDirty = true;
    await detailPage.onShow.call(detailPage);
    assert(app.globalData.clothesDirty === false, "clothes detail refresh should clear dirty flag after reload");
  });
  assert(detailPage.data.item.name === "米白衬衫更新", "clothes detail should reload item after edit page marks dirty");
  assert(
    clothesItemRequests.filter((publicID) => publicID === "clo_shirt").length >= 2,
    "clothes detail onShow should fetch current item when dirty",
  );

  const edit = loadPage("pages/clothes/edit.js", apiStub);
  assert(edit.config, "clothes edit should register Page config");
  const editPage = createPageInstance(edit.config);
  await editPage.loadClothesItem.call(editPage, "clo_shirt");
  editPage.setData({
    draft: Object.assign({}, editPage.data.draft, {
      name: "米白衬衫更新"
    })
  });
  await editPage.handleSaveItem.call(editPage);
  assert(editPage.data.errorMessage === "", "clothes edit should save without error");
}

async function verifyTypedPages(domain) {
  const isHair = domain === "hair";
  const apiStub = isHair
    ? {
      getHairItems: async () => ({ items: [{ public_id: "hai_1", name: "锁骨发", length: "锁骨", recommendation_status: "normal" }] }),
      getHairItem: async () => ({ public_id: "hai_1", name: "锁骨发", length: "锁骨", recommendation_status: "normal" }),
      createHairItem: async (payload) => Object.assign({ public_id: "hai_new", recommendation_status: "normal" }, payload),
      updateHairItem: async (_publicID, payload) => Object.assign({ public_id: "hai_1", recommendation_status: "normal" }, payload),
      deleteHairItem: async () => ({ ok: true })
    }
    : {
      getMakeupItems: async () => ({ items: [{ public_id: "mkp_1", name: "通勤淡妆", makeup_type: "通勤", recommendation_status: "normal" }] }),
      getMakeupItem: async () => ({ public_id: "mkp_1", name: "通勤淡妆", makeup_type: "通勤", recommendation_status: "normal" }),
      createMakeupItem: async (payload) => Object.assign({ public_id: "mkp_new", recommendation_status: "normal" }, payload),
      updateMakeupItem: async (_publicID, payload) => Object.assign({ public_id: "mkp_1", recommendation_status: "normal" }, payload),
      deleteMakeupItem: async () => ({ ok: true })
    };

  const list = loadPage(`pages/${domain}/list.js`, apiStub);
  const listPage = createPageInstance(list.config);
  await (isHair ? listPage.loadHair : listPage.loadMakeup).call(listPage);
  assert(listPage.data.items.length === 1, `${domain} list should load items`);
  listPage.handleOpenCreate.call(listPage);
  listPage.setData({
    draft: Object.assign({}, listPage.data.draft, {
      name: isHair ? "短发" : "约会妆"
    })
  });
  await listPage.handleSaveCreate.call(listPage);
  assert(listPage.data.items.length === 2, `${domain} list create modal should append item`);

  const navigations = [];
  await withWx({
    navigateTo(options) {
      navigations.push(options.url);
    }
  }, async () => {
    listPage.handleOpenDetail.call(listPage, { currentTarget: { dataset: { publicId: isHair ? "hai_1" : "mkp_1" } } });
  });
  assert(navigations[0] === `/pages/${domain}/detail?public_id=${isHair ? "hai_1" : "mkp_1"}`, `${domain} list should navigate to detail`);

  const detailMarkup = read(`pages/${domain}/detail.wxml`);
  assert(!detailMarkup.includes("<input"), `${domain} detail should not include inputs`);
  assert(!detailMarkup.includes("<textarea"), `${domain} detail should not include textareas`);
  assert(detailMarkup.includes("编辑"), `${domain} detail should expose edit action`);

  const detail = loadPage(`pages/${domain}/detail.js`, apiStub);
  const detailPage = createPageInstance(detail.config);
  await detailPage.loadItem.call(detailPage, isHair ? "hai_1" : "mkp_1");
  const detailNavigations = [];
  await withWx({
    navigateTo(options) {
      detailNavigations.push(options.url);
    }
  }, async () => {
    detailPage.handleOpenEdit.call(detailPage);
  });
  assert(detailNavigations[0] === `/pages/${domain}/edit?public_id=${isHair ? "hai_1" : "mkp_1"}`, `${domain} detail should navigate to edit`);

  const edit = loadPage(`pages/${domain}/edit.js`, apiStub);
  const editPage = createPageInstance(edit.config);
  await editPage.loadItem.call(editPage, isHair ? "hai_1" : "mkp_1");
  editPage.setData({
    draft: Object.assign({}, editPage.data.draft, {
      name: isHair ? "锁骨发更新" : "通勤淡妆更新"
    })
  });
  await editPage.handleSave.call(editPage);
  assert(editPage.data.errorMessage === "", `${domain} edit should save without error`);
}

async function verifyProfileSubpages() {
  const apiCalls = [];
  const apiStub = {
    getProfileSummary: async () => ({
      user: { user_public_id: "usr_test", nickname: "明明", onboarding_status: "completed" },
      profile: { height_cm: 165, lifestyle_scenarios: ["通勤"], body_notes: "利落" },
      preferences: { style_goals: ["更利落"], avoidances: ["过甜"], scenario_preferences: ["通勤"] },
      memory_summary: { fact_count: 1, preference_count: 2, avoidance_count: 1, inference_count: 1, pending_confirmation_count: 0 },
      quick_entries: []
    }),
    updateProfile: async (payload) => {
      apiCalls.push({ name: "updateProfile", payload });
      return {};
    },
    updateProfilePreferences: async (payload) => {
      apiCalls.push({ name: "updateProfilePreferences", payload });
      return {};
    }
  };

  const profile = loadPage("pages/profile/index.js", apiStub);
  const profilePage = createPageInstance(profile.config);
  await profilePage.loadProfile.call(profilePage);
  const navigations = [];
  await withWx({
    navigateTo(options) {
      navigations.push(options.url);
    }
  }, async () => {
    profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "profile" } } });
    profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "preferences" } } });
    profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "memory" } } });
    profilePage.handleQuickEntry.call(profilePage, { currentTarget: { dataset: { key: "privacy" } } });
  });
  assert(navigations.join(",") === "/pages/profile/edit,/pages/preferences/edit,/pages/memory/index,/pages/privacy/index", "profile entries should navigate to independent pages");

  const profileEdit = loadPage("pages/profile/edit.js", apiStub);
  const profileEditPage = createPageInstance(profileEdit.config);
  await profileEditPage.loadProfile.call(profileEditPage);
  profileEditPage.setData({ draft: Object.assign({}, profileEditPage.data.draft, { nickname: "新昵称" }) });
  await profileEditPage.handleSave.call(profileEditPage);

  const preferencesEdit = loadPage("pages/preferences/edit.js", apiStub);
  const preferencesPage = createPageInstance(preferencesEdit.config);
  await preferencesPage.loadPreferences.call(preferencesPage);
  preferencesPage.setData({ draft: Object.assign({}, preferencesPage.data.draft, { styleGoalsText: "更利落、轻松" }) });
  await preferencesPage.handleSave.call(preferencesPage);

  assert(apiCalls.some((call) => call.name === "updateProfile"), "profile edit should call updateProfile");
  assert(apiCalls.some((call) => call.name === "updateProfilePreferences"), "preferences edit should call updateProfilePreferences");
}

async function main() {
  await verifyRoutesAndStaticContracts();
  await verifyCollectionPage();
  await verifyClothesPages();
  await verifyTypedPages("hair");
  await verifyTypedPages("makeup");
  await verifyProfileSubpages();
  console.log("miniapp api integration verification passed");
}

main().catch((error) => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
