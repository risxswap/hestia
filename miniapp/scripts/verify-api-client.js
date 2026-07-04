const path = require("path");

const root = path.resolve(__dirname, "..");
const apiPath = path.join(root, "utils/api.js");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

async function withGlobals(globals, fn) {
  const originalWx = global.wx;
  const originalGetApp = global.getApp;

  try {
    global.wx = globals.wx;
    global.getApp = globals.getApp;
    return await fn();
  } finally {
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
  }
}

function requireFreshApi() {
  delete require.cache[require.resolve(apiPath)];
  return require(apiPath);
}

async function main() {
  const api = requireFreshApi();

  [
    "request",
    "ensureDevSession",
    "getLatestReport",
    "getOnboardingDraft",
    "saveOnboardingDraft",
    "submitOnboarding",
    "sendAgentMessage",
    "parseSSEEvents",
    "getCollectionSummary",
    "getHairItems",
    "getMakeupItems",
    "getReferenceItems",
    "getWardrobeItems",
    "getWardrobeOptions",
    "createWardrobeItem",
    "updateWardrobeItem",
    "deleteWardrobeItem",
    "recognizeWardrobeItemImage",
    "createFileUploadToken",
    "confirmFileUpload",
    "uploadFileToQiniu",
    "getProfileSummary",
    "updateProfile",
    "updateProfilePreferences"
  ].forEach((name) => {
    assert(typeof api[name] === "function", `api.js should export ${name}`);
  });

  const stored = {};
  const calls = [];
  const wx = {
    getStorageSync(key) {
      return stored[key] || "";
    },
    setStorageSync(key, value) {
      stored[key] = value;
    },
    request(options) {
      calls.push(options);
      if (options.url.endsWith("/api/user/dev-login")) {
        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              token: "dev_token",
              user_public_id: "usr_dev",
              onboarding_status: "not_started"
            }
          }
        });
        return;
      }

      options.success({
        statusCode: 200,
        data: {
          code: "ok",
          data: {
            public_id: "rpt_test"
          }
        }
      });
    }
  };

  await withGlobals({
    getApp: () => ({
      globalData: {
        apiBaseUrl: "http://127.0.0.1:8080/"
      }
    }),
    wx
  }, async () => {
    const session = await api.ensureDevSession();
    assert(session.token === "dev_token", "ensureDevSession should return token");
    assert(stored.user_token === "dev_token", "ensureDevSession should persist token");

    const report = await api.getLatestReport();
    assert(report.public_id === "rpt_test", "getLatestReport should return response data");
  });

  assert(calls.length === 2, `expected 2 wx.request calls, got ${calls.length}`);
  assert(
    calls[0].url === "http://127.0.0.1:8080/api/user/dev-login",
    `dev login url should trim base slash, got ${calls[0].url}`
  );
  assert(calls[0].method === "POST", "dev login should use POST");
  assert(!calls[0].header.Authorization, "dev login should not send Authorization");
  assert(
    calls[1].url === "http://127.0.0.1:8080/api/user/reports/latest",
    `latest report url mismatch: ${calls[1].url}`
  );
  assert(
    calls[1].header.Authorization === "Bearer dev_token",
    "authorized request should send persisted bearer token"
  );

  const events = api.parseSSEEvents(
    "event: status\n" +
      "data: {\"text\":\"准备好了\"}\n\n" +
      "event: done\n" +
      "data: {\"job_public_id\":\"job_1\"}\n\n"
  );
  assert(events.length === 2, "parseSSEEvents should parse two events");
  assert(events[0].event === "status", "first event should keep event name");
  assert(events[0].data.text === "准备好了", "first event should parse JSON data");

  const collectionCalls = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "collection_token";
      },
      request(options) {
        collectionCalls.push(options);
        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              items: [],
              types: []
            }
          }
        });
      }
    }
  }, async () => {
    await api.getCollectionSummary();
    await api.getHairItems();
    await api.getMakeupItems();
    await api.getReferenceItems();
  });

  const collectionPaths = collectionCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(collectionCalls.every((call) => call.method === "GET"), "collection and asset-type APIs should use GET");
  assert(collectionPaths[0] === "/api/user/collection", `collection path mismatch: ${collectionPaths[0]}`);
  assert(collectionPaths[1] === "/api/user/hair", `hair path mismatch: ${collectionPaths[1]}`);
  assert(collectionPaths[2] === "/api/user/makeup", `makeup path mismatch: ${collectionPaths[2]}`);
  assert(collectionPaths[3] === "/api/user/references", `references path mismatch: ${collectionPaths[3]}`);
  assert(
    collectionPaths.every((apiPath) => !apiPath.includes("/private") && !apiPath.includes("/collection/")),
    `collection-related APIs should not use private prefix or nested collection routes: ${collectionPaths.join(",")}`
  );

  const wardrobeCalls = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "wardrobe_token";
      },
      request(options) {
        wardrobeCalls.push(options);
        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              items: []
            }
          }
        });
      }
    }
  }, async () => {
    const createPayload = { name: "米白衬衫", category: "上装" };
    const updatePayload = { color: "米白" };
    await api.getWardrobeItems();
    await api.getWardrobeItems({ category: "上装" });
    await api.getWardrobeOptions();
    await api.createWardrobeItem(createPayload);
    await api.updateWardrobeItem("wdi_test", updatePayload);
    await api.deleteWardrobeItem("wdi_test");
    await api.recognizeWardrobeItemImage("ast_test", { itemPublicID: "wdi_test", overwrite: true });
  });

  const wardrobePaths = wardrobeCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(wardrobeCalls[0].method === "GET", "getWardrobeItems should use GET");
  assert(wardrobePaths[0] === "/api/user/wardrobe/items", `wardrobe list path mismatch: ${wardrobePaths[0]}`);
  assert(wardrobeCalls[1].method === "GET", "filtered getWardrobeItems should use GET");
  assert(wardrobePaths[1] === `/api/user/wardrobe/items?category=${encodeURIComponent("上装")}`, `wardrobe filtered path mismatch: ${wardrobePaths[1]}`);
  assert(wardrobeCalls[2].method === "GET", "getWardrobeOptions should use GET");
  assert(wardrobePaths[2] === "/api/user/wardrobe/options", `wardrobe options path mismatch: ${wardrobePaths[2]}`);
  assert(wardrobeCalls[3].method === "POST", "createWardrobeItem should use POST");
  assert(wardrobePaths[3] === "/api/user/wardrobe/items", `wardrobe create path mismatch: ${wardrobePaths[3]}`);
  assert(wardrobeCalls[3].data.name === "米白衬衫", "createWardrobeItem should pass create payload name");
  assert(wardrobeCalls[3].data.category === "上装", "createWardrobeItem should pass create payload category");
  assert(wardrobeCalls[4].method === "PATCH", "updateWardrobeItem should use PATCH");
  assert(wardrobePaths[4] === "/api/user/wardrobe/items/wdi_test", `wardrobe update path mismatch: ${wardrobePaths[4]}`);
  assert(wardrobeCalls[4].data.color === "米白", "updateWardrobeItem should pass update payload");
  assert(wardrobeCalls[5].method === "DELETE", "deleteWardrobeItem should use DELETE");
  assert(wardrobePaths[5] === "/api/user/wardrobe/items/wdi_test", `wardrobe delete path mismatch: ${wardrobePaths[5]}`);
  assert(wardrobeCalls[6].method === "POST", "recognizeWardrobeItemImage should use POST");
  assert(wardrobePaths[6] === "/api/user/wardrobe/items/recognize", `wardrobe recognize path mismatch: ${wardrobePaths[6]}`);
  assert(wardrobeCalls[6].data.asset_public_id === "ast_test", "recognizeWardrobeItemImage should pass asset_public_id");
  assert(wardrobeCalls[6].data.item_public_id === "wdi_test", "recognizeWardrobeItemImage should pass item_public_id");
  assert(wardrobeCalls[6].data.overwrite === true, "recognizeWardrobeItemImage should pass overwrite");

  const profileCalls = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "profile_token";
      },
      request(options) {
        profileCalls.push(options);
        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              ok: true
            }
          }
        });
      }
    }
  }, async () => {
    await api.getProfileSummary();
    await api.updateProfile({ nickname: "明明" });
    await api.updateProfilePreferences({ style_goals: ["更利落"] });
  });

  const profilePaths = profileCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(profileCalls[0].method === "GET", "getProfileSummary should use GET");
  assert(profilePaths[0] === "/api/user/profile/summary", `profile summary path mismatch: ${profilePaths[0]}`);
  assert(profileCalls[1].method === "PATCH", "updateProfile should use PATCH");
  assert(profilePaths[1] === "/api/user/profile", `update profile path mismatch: ${profilePaths[1]}`);
  assert(profileCalls[1].data.nickname === "明明", "updateProfile should pass profile payload");
  assert(profileCalls[2].method === "PATCH", "updateProfilePreferences should use PATCH");
  assert(profilePaths[2] === "/api/user/profile/preferences", `update preferences path mismatch: ${profilePaths[2]}`);
  assert(profileCalls[2].data.style_goals[0] === "更利落", "updateProfilePreferences should pass preferences payload");

  let rejected = false;
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "token";
      },
      request(options) {
        options.success({
          statusCode: 401,
          data: {
            code: "auth.unauthorized",
            message: "请先登录",
            data: null
          }
        });
      }
    }
  }, async () => {
    try {
      await api.getLatestReport();
    } catch (error) {
      rejected = error.code === "auth.unauthorized" && error.statusCode === 401;
    }
  });
  assert(rejected, "request should reject non-ok API responses with code and statusCode");

  const assetCalls = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "asset_token";
      },
      request(options) {
        assetCalls.push(options);
        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              ok: true
            }
          }
        });
      }
    }
  }, async () => {
    await api.createFileUploadToken({
      asset_type: "wardrobe_item_photo",
      mime_type: "image/png",
      file_size: 1024,
      file_ext: "png"
    });
    await api.confirmFileUpload({
      asset_public_id: "ast_1",
      bucket: "hestia-assets",
      object_key: "users/u1/assets/ast_1.png",
      mime_type: "image/png",
      file_size: 1024,
      asset_type: "wardrobe_item_photo"
    });
  });

  const assetPaths = assetCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(assetCalls[0].method === "POST", "createFileUploadToken should use POST");
  assert(assetPaths[0] === "/api/user/files/upload-token", `file upload-token path mismatch: ${assetPaths[0]}`);
  assert(assetCalls[0].data.asset_type === "wardrobe_item_photo", "createFileUploadToken should pass asset_type");
  assert(assetCalls[1].method === "POST", "confirmFileUpload should use POST");
  assert(assetPaths[1] === "/api/user/files/confirm", `file confirm path mismatch: ${assetPaths[1]}`);
  assert(assetCalls[1].data.object_key === "users/u1/assets/ast_1.png", "confirmFileUpload should pass object_key");

  const uploadRequests = [];
  const uploadFiles = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "asset_token";
      },
      request(options) {
        uploadRequests.push(options);
        if (options.url.endsWith("/api/user/files/upload-token")) {
          options.success({
            statusCode: 200,
            data: {
              code: "ok",
              data: {
                asset_public_id: "ast_upload",
                bucket: "hestia-assets",
                object_key: "users/u1/assets/ast_upload.webp",
                upload_url: "https://upload.qiniup.com",
                upload_token: "qiniu_token"
              }
            }
          });
          return;
        }

        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              asset_public_id: "ast_upload",
              object_key: "users/u1/assets/ast_upload.webp",
              asset_type: "wardrobe_item_photo"
            }
          }
        });
      },
      uploadFile(options) {
        uploadFiles.push(options);
        options.success({
          statusCode: 200,
          data: "{\"hash\":\"hash_1\"}"
        });
      }
    }
  }, async () => {
    const uploaded = await api.uploadFileToQiniu(
      {
        tempFilePath: "/tmp/wardrobe.webp",
        size: 2048,
        type: "image/webp"
      },
      {
        assetType: "wardrobe_item_photo",
        width: 640,
        height: 960
      }
    );

    assert(uploaded.asset_public_id === "ast_upload", "uploadFileToQiniu should return confirmed asset_public_id");
    assert(uploaded.object_key === "users/u1/assets/ast_upload.webp", "uploadFileToQiniu should return confirmed object_key");
    assert(!Object.prototype.hasOwnProperty.call(uploaded, "url"), "uploadFileToQiniu confirm result should not include expiring url");
    assert(uploaded.asset_type === "wardrobe_item_photo", "uploadFileToQiniu should return confirmed asset_type");
  });

  assert(uploadFiles.length === 1, `expected 1 wx.uploadFile call, got ${uploadFiles.length}`);
  assert(uploadFiles[0].url === "https://upload.qiniup.com", `upload url mismatch: ${uploadFiles[0].url}`);
  assert(uploadFiles[0].filePath === "/tmp/wardrobe.webp", `upload filePath mismatch: ${uploadFiles[0].filePath}`);
  assert(uploadFiles[0].name === "file", "uploadFileToQiniu should use file field name");
  assert(uploadFiles[0].formData.token === "qiniu_token", "wx.uploadFile formData should include upload token");
  assert(uploadFiles[0].formData.key === "users/u1/assets/ast_upload.webp", "wx.uploadFile formData should include object key");
  assert(uploadRequests.length === 2, `expected token and confirm requests, got ${uploadRequests.length}`);
  assert(uploadRequests[0].url.endsWith("/api/user/files/upload-token"), "uploadFileToQiniu should request file token endpoint");
  assert(uploadRequests[0].data.asset_type === "wardrobe_item_photo", "uploadFileToQiniu should request token with asset_type");
  assert(uploadRequests[0].data.mime_type === "image/webp", "uploadFileToQiniu should request token with inferred mime_type");
  assert(uploadRequests[0].data.file_size === 2048, "uploadFileToQiniu should request token with file size");
  assert(uploadRequests[0].data.file_ext === "webp", "uploadFileToQiniu should request token with inferred file_ext");
  assert(uploadRequests[1].url.endsWith("/api/user/files/confirm"), "uploadFileToQiniu should confirm file endpoint");
  assert(uploadRequests[1].data.asset_public_id === "ast_upload", "uploadFileToQiniu should confirm asset_public_id");
  assert(uploadRequests[1].data.bucket === "hestia-assets", "uploadFileToQiniu should confirm bucket");
  assert(uploadRequests[1].data.object_key === "users/u1/assets/ast_upload.webp", "uploadFileToQiniu should confirm object_key");
  assert(uploadRequests[1].data.mime_type === "image/webp", "uploadFileToQiniu should confirm mime_type");
  assert(uploadRequests[1].data.file_size === 2048, "uploadFileToQiniu should confirm file_size");
  assert(uploadRequests[1].data.width === 640, "uploadFileToQiniu should confirm width");
  assert(uploadRequests[1].data.height === 960, "uploadFileToQiniu should confirm height");
  assert(uploadRequests[1].data.asset_type === "wardrobe_item_photo", "uploadFileToQiniu should confirm asset_type");

  const tdesignUploadRequests = [];
  const tdesignUploadFiles = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "asset_token";
      },
      request(options) {
        tdesignUploadRequests.push(options);
        if (options.url.endsWith("/api/user/files/upload-token")) {
          options.success({
            statusCode: 200,
            data: {
              code: "ok",
              data: {
                asset_public_id: "ast_tdesign",
                bucket: "hestia-assets",
                object_key: "users/u1/assets/ast_tdesign.jpg",
                upload_url: "https://upload.qiniup.com",
                upload_token: "qiniu_token"
              }
            }
          });
          return;
        }

        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              asset_public_id: "ast_tdesign",
              object_key: "users/u1/assets/ast_tdesign.jpg",
              asset_type: "wardrobe_item_photo"
            }
          }
        });
      },
      uploadFile(options) {
        tdesignUploadFiles.push(options);
        options.success({
          statusCode: 200,
          data: "{}"
        });
      }
    }
  }, async () => {
    await api.uploadFileToQiniu(
      {
        url: "wxfile://tmp-no-extension",
        size: 4096,
        type: "image"
      },
      {
        assetType: "wardrobe_item_photo"
      }
    );
  });

  assert(tdesignUploadFiles[0].filePath === "wxfile://tmp-no-extension", "uploadFileToQiniu should support TDesign file.url");
  assert(tdesignUploadRequests[0].data.mime_type === "image/jpeg", "TDesign type=image should default to image/jpeg");
  assert(tdesignUploadRequests[0].data.file_ext === "jpg", "TDesign type=image should default to jpg extension");

  let missingAssetTypeRejected = false;
  try {
    await api.uploadFileToQiniu({ tempFilePath: "/tmp/wardrobe.png", size: 1 }, {});
  } catch (error) {
    missingAssetTypeRejected = error instanceof api.ApiError && error.code === "asset.asset_type_required";
  }
  assert(missingAssetTypeRejected, "uploadFileToQiniu should reject without assetType");

  let missingFilePathRejected = false;
  try {
    await api.uploadFileToQiniu({ size: 1, type: "image" }, { assetType: "wardrobe_item_photo" });
  } catch (error) {
    missingFilePathRejected = error instanceof api.ApiError && error.code === "asset.file_path_required";
  }
  assert(missingFilePathRejected, "uploadFileToQiniu should reject without a file path");

  let invalidImageRejected = false;
  try {
    await api.uploadFileToQiniu({ tempFilePath: "/tmp/file", size: 1, type: "video" }, { assetType: "wardrobe_item_photo" });
  } catch (error) {
    invalidImageRejected = error instanceof api.ApiError && error.code === "asset.invalid_image_type";
  }
  assert(invalidImageRejected, "uploadFileToQiniu should reject files without image mime or extension");

  let uploadStatusRejected = false;
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "asset_token";
      },
      request(options) {
        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              asset_public_id: "ast_failed",
              bucket: "hestia-assets",
              object_key: "users/u1/assets/ast_failed.jpg",
              upload_url: "https://upload.qiniup.com",
              upload_token: "qiniu_token"
            }
          }
        });
      },
      uploadFile(options) {
        options.success({
          statusCode: 500,
          data: "upload failed"
        });
      }
    }
  }, async () => {
    try {
      await api.uploadFileToQiniu({ tempFilePath: "/tmp/wardrobe.jpg", size: 1 }, { assetType: "wardrobe_item_photo" });
    } catch (error) {
      uploadStatusRejected = error instanceof api.ApiError && error.code === "asset.upload_failed" && error.statusCode === 500;
    }
  });
  assert(uploadStatusRejected, "uploadFileToQiniu should reject non-2xx wx.uploadFile responses");

  let uploadFailRejected = false;
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "asset_token";
      },
      request(options) {
        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              asset_public_id: "ast_network_failed",
              bucket: "hestia-assets",
              object_key: "users/u1/assets/ast_network_failed.jpg",
              upload_url: "https://upload.qiniup.com",
              upload_token: "qiniu_token"
            }
          }
        });
      },
      uploadFile(options) {
        options.fail({
          errMsg: "uploadFile:fail timeout"
        });
      }
    }
  }, async () => {
    try {
      await api.uploadFileToQiniu({ path: "/tmp/wardrobe.jpg", size: 1 }, { assetType: "wardrobe_item_photo" });
    } catch (error) {
      uploadFailRejected = error instanceof api.ApiError && error.code === "asset.upload_failed";
    }
  });
  assert(uploadFailRejected, "uploadFileToQiniu should reject wx.uploadFile fail callbacks");
}

main()
  .then(() => {
    console.log("api client verification passed");
  })
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
