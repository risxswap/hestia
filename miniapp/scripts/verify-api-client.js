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

function arrayBufferFromBytes(bytes) {
  const array = Uint8Array.from(bytes);
  return array.buffer.slice(array.byteOffset, array.byteOffset + array.byteLength);
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
    "streamAgentChat",
    "getCurrentAdviceDraft",
    "getAdviceDraftVersions",
    "confirmAdviceDraft",
    "discardAdviceDraft",
    "parseSSEEvents",
    "getCollectionSummary",
    "getHairItems",
    "getHairItem",
    "createHairItem",
    "updateHairItem",
    "deleteHairItem",
    "getMakeupItems",
    "getMakeupItem",
    "createMakeupItem",
    "updateMakeupItem",
    "deleteMakeupItem",
    "getClothesItems",
    "getClothesItem",
    "getClothesOptions",
    "createClothesItem",
    "updateClothesItem",
    "deleteClothesItem",
    "recognizeClothesItemImage",
    "createFileUploadToken",
    "confirmFileUpload",
    "uploadFileToQiniu",
    "getProfileSummary",
    "updateProfile",
    "updateProfilePreferences",
    "createProfilePhoto",
    "updateProfilePhoto",
    "deleteProfilePhoto"
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

  const agentCalls = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "agent_token";
      },
      request(options) {
        agentCalls.push(options);
        if (options.url.endsWith("/api/user/agent/chat")) {
          options.success({
            statusCode: 200,
            data: "event: message\n" +
              "data: {\"text\":\"先给你一版草稿\"}\n\n" +
              "event: draft\n" +
              "data: {\"draft_public_id\":\"drf_test\",\"sections\":[]}\n\n"
          });
          return {};
        }
        options.success({
          statusCode: 200,
          data: {
            code: "ok",
            data: {
              public_id: "drf_test"
            }
          }
        });
        return {};
      }
    }
  }, async () => {
    const chatEvents = await api.sendAgentMessage("今天怎么穿");
    assert(chatEvents.length === 2, `sendAgentMessage should collect SSE events, got ${chatEvents.length}`);
    await api.getCurrentAdviceDraft();
    await api.getAdviceDraftVersions("drf_test");
    await api.confirmAdviceDraft("drf_test");
    await api.discardAdviceDraft("drf_test");
  });

  const agentPaths = agentCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(agentPaths[0] === "/api/user/agent/chat", `agent chat path mismatch: ${agentPaths[0]}`);
  assert(agentCalls[0].method === "POST", "sendAgentMessage should use POST");
  assert(agentCalls[0].enableChunked === true, "agent chat should enable chunked streaming");
  assert(agentCalls[0].header.Accept === "text/event-stream", "agent chat should request SSE");
  assert(agentCalls[0].header.Authorization === "Bearer agent_token", "agent chat should send bearer token");
  assert(!agentPaths.some((apiPath) => apiPath.includes("/api/user/agent/stream")), `old agent stream path should not be used: ${agentPaths.join(",")}`);
  assert(agentPaths[1] === "/api/user/advice-drafts/current", `current draft path mismatch: ${agentPaths[1]}`);
  assert(agentPaths[2] === "/api/user/advice-drafts/drf_test/versions", `draft versions path mismatch: ${agentPaths[2]}`);
  assert(agentPaths[3] === "/api/user/advice-drafts/drf_test/confirm", `confirm draft path mismatch: ${agentPaths[3]}`);
  assert(agentPaths[4] === "/api/user/advice-drafts/drf_test/discard", `discard draft path mismatch: ${agentPaths[4]}`);

  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "chunk_token";
      },
      request(options) {
        let chunkHandler = null;
        const raw = "event: message\n" +
          "data: {\"text\":\"准备好了\"}\n\n";
        const bytes = Array.from(Buffer.from(raw, "utf8"));
        const splitAt = bytes.findIndex((byte) => byte >= 0xe0);
        const firstChunk = bytes.slice(0, splitAt + 1);
        const secondChunk = bytes.slice(splitAt + 1);
        return {
          onChunkReceived(handler) {
            chunkHandler = handler;
            Promise.resolve().then(() => {
              chunkHandler({ data: arrayBufferFromBytes(firstChunk) });
              chunkHandler({ data: arrayBufferFromBytes(secondChunk) });
              options.success({
                statusCode: 200,
                data: ""
              });
            });
          }
        };
      }
    }
  }, async () => {
    const chunkEvents = await api.streamAgentChat("测试分块").promise;
    assert(chunkEvents.length === 1, `chunked stream should produce one event, got ${chunkEvents.length}`);
    assert(chunkEvents[0].data.text === "准备好了", `chunked UTF-8 text mismatch: ${JSON.stringify(chunkEvents)}`);
  });

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
  });

  const collectionPaths = collectionCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(collectionCalls.every((call) => call.method === "GET"), "collection and asset-type APIs should use GET");
  assert(collectionPaths[0] === "/api/user/collection", `collection path mismatch: ${collectionPaths[0]}`);
  assert(collectionPaths[1] === "/api/user/hair", `hair path mismatch: ${collectionPaths[1]}`);
  assert(collectionPaths[2] === "/api/user/makeup", `makeup path mismatch: ${collectionPaths[2]}`);
  assert(collectionPaths.length === 3, `references API should be removed, got ${collectionPaths.join(",")}`);
  assert(
    collectionPaths.every((apiPath) => !apiPath.includes("/private") && !apiPath.includes("/collection/")),
    `collection-related APIs should not use private prefix or nested collection routes: ${collectionPaths.join(",")}`
  );

  const clothesCalls = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "clothes_token";
      },
      request(options) {
        clothesCalls.push(options);
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
    await api.getClothesItems();
    await api.getClothesItems({ category: "上装" });
    await api.getClothesItem("wdi_test");
    await api.getClothesOptions();
    await api.createClothesItem(createPayload);
    await api.updateClothesItem("wdi_test", updatePayload);
    await api.deleteClothesItem("wdi_test");
    await api.recognizeClothesItemImage("ast_test", { itemPublicID: "wdi_test", overwrite: true });
  });

  const clothesPaths = clothesCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(clothesCalls[0].method === "GET", "getClothesItems should use GET");
  assert(clothesPaths[0] === "/api/user/clothes/items", `clothes list path mismatch: ${clothesPaths[0]}`);
  assert(clothesCalls[1].method === "GET", "filtered getClothesItems should use GET");
  assert(clothesPaths[1] === `/api/user/clothes/items?category=${encodeURIComponent("上装")}`, `clothes filtered path mismatch: ${clothesPaths[1]}`);
  assert(clothesCalls[2].method === "GET", "getClothesItem should use GET");
  assert(clothesPaths[2] === "/api/user/clothes/items/wdi_test", `clothes item path mismatch: ${clothesPaths[2]}`);
  assert(clothesCalls[3].method === "GET", "getClothesOptions should use GET");
  assert(clothesPaths[3] === "/api/user/clothes/options", `clothes options path mismatch: ${clothesPaths[3]}`);
  assert(clothesCalls[4].method === "POST", "createClothesItem should use POST");
  assert(clothesPaths[4] === "/api/user/clothes/items", `clothes create path mismatch: ${clothesPaths[4]}`);
  assert(clothesCalls[4].data.name === "米白衬衫", "createClothesItem should pass create payload name");
  assert(clothesCalls[4].data.category === "上装", "createClothesItem should pass create payload category");
  assert(clothesCalls[5].method === "PATCH", "updateClothesItem should use PATCH");
  assert(clothesPaths[5] === "/api/user/clothes/items/wdi_test", `clothes update path mismatch: ${clothesPaths[5]}`);
  assert(clothesCalls[5].data.color === "米白", "updateClothesItem should pass update payload");
  assert(clothesCalls[6].method === "DELETE", "deleteClothesItem should use DELETE");
  assert(clothesPaths[6] === "/api/user/clothes/items/wdi_test", `clothes delete path mismatch: ${clothesPaths[6]}`);
  assert(clothesCalls[7].method === "POST", "recognizeClothesItemImage should use POST");
  assert(clothesPaths[7] === "/api/user/clothes/items/recognize", `clothes recognize path mismatch: ${clothesPaths[7]}`);
  assert(clothesCalls[7].data.asset_public_id === "ast_test", "recognizeClothesItemImage should pass asset_public_id");
  assert(clothesCalls[7].data.item_public_id === "wdi_test", "recognizeClothesItemImage should pass item_public_id");
  assert(clothesCalls[7].data.overwrite === true, "recognizeClothesItemImage should pass overwrite");

  const typedCalls = [];
  await withGlobals({
    getApp: () => ({ globalData: { apiBaseUrl: "http://127.0.0.1:8080" } }),
    wx: {
      getStorageSync() {
        return "typed_token";
      },
      request(options) {
        typedCalls.push(options);
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
    await api.getHairItem("hai_test");
    await api.createHairItem({ name: "锁骨发" });
    await api.updateHairItem("hai_test", { color: "深棕" });
    await api.deleteHairItem("hai_test");
    await api.getMakeupItem("mkp_test");
    await api.createMakeupItem({ name: "通勤淡妆" });
    await api.updateMakeupItem("mkp_test", { finish: "自然" });
    await api.deleteMakeupItem("mkp_test");
  });
  const typedPaths = typedCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(typedPaths.join(",") === [
    "/api/user/hair/hai_test",
    "/api/user/hair",
    "/api/user/hair/hai_test",
    "/api/user/hair/hai_test",
    "/api/user/makeup/mkp_test",
    "/api/user/makeup",
    "/api/user/makeup/mkp_test",
    "/api/user/makeup/mkp_test"
  ].join(","), `hair/makeup CRUD paths mismatch: ${typedPaths.join(",")}`);
  assert(typedCalls[1].method === "POST", "createHairItem should use POST");
  assert(typedCalls[2].method === "PATCH", "updateHairItem should use PATCH");
  assert(typedCalls[3].method === "DELETE", "deleteHairItem should use DELETE");
  assert(typedCalls[5].method === "POST", "createMakeupItem should use POST");
  assert(typedCalls[6].method === "PATCH", "updateMakeupItem should use PATCH");
  assert(typedCalls[7].method === "DELETE", "deleteMakeupItem should use DELETE");

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
    await api.createProfilePhoto({ asset_public_id: "ast_profile", photo_type: "full_body", angle: "front" });
    await api.updateProfilePhoto("pph_test", { angle: "side" });
    await api.deleteProfilePhoto("pph_test");
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
  assert(profileCalls[3].method === "POST", "createProfilePhoto should use POST");
  assert(profilePaths[3] === "/api/user/profile/photos", `create profile photo path mismatch: ${profilePaths[3]}`);
  assert(profileCalls[3].data.asset_public_id === "ast_profile", "createProfilePhoto should pass asset id");
  assert(profileCalls[4].method === "PATCH", "updateProfilePhoto should use PATCH");
  assert(profilePaths[4] === "/api/user/profile/photos/pph_test", `update profile photo path mismatch: ${profilePaths[4]}`);
  assert(profileCalls[4].data.angle === "side", "updateProfilePhoto should pass patch payload");
  assert(profileCalls[5].method === "DELETE", "deleteProfilePhoto should use DELETE");
  assert(profilePaths[5] === "/api/user/profile/photos/pph_test", `delete profile photo path mismatch: ${profilePaths[5]}`);

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
      asset_type: "clothes_item_photo",
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
      asset_type: "clothes_item_photo"
    });
  });

  const assetPaths = assetCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(assetCalls[0].method === "POST", "createFileUploadToken should use POST");
  assert(assetPaths[0] === "/api/user/files/upload-token", `file upload-token path mismatch: ${assetPaths[0]}`);
  assert(assetCalls[0].data.asset_type === "clothes_item_photo", "createFileUploadToken should pass asset_type");
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
              asset_type: "clothes_item_photo"
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
        tempFilePath: "/tmp/clothes.webp",
        size: 2048,
        type: "image/webp"
      },
      {
        assetType: "clothes_item_photo",
        width: 640,
        height: 960
      }
    );

    assert(uploaded.asset_public_id === "ast_upload", "uploadFileToQiniu should return confirmed asset_public_id");
    assert(uploaded.object_key === "users/u1/assets/ast_upload.webp", "uploadFileToQiniu should return confirmed object_key");
    assert(!Object.prototype.hasOwnProperty.call(uploaded, "url"), "uploadFileToQiniu confirm result should not include expiring url");
    assert(uploaded.asset_type === "clothes_item_photo", "uploadFileToQiniu should return confirmed asset_type");
  });

  assert(uploadFiles.length === 1, `expected 1 wx.uploadFile call, got ${uploadFiles.length}`);
  assert(uploadFiles[0].url === "https://upload.qiniup.com", `upload url mismatch: ${uploadFiles[0].url}`);
  assert(uploadFiles[0].filePath === "/tmp/clothes.webp", `upload filePath mismatch: ${uploadFiles[0].filePath}`);
  assert(uploadFiles[0].name === "file", "uploadFileToQiniu should use file field name");
  assert(uploadFiles[0].formData.token === "qiniu_token", "wx.uploadFile formData should include upload token");
  assert(uploadFiles[0].formData.key === "users/u1/assets/ast_upload.webp", "wx.uploadFile formData should include object key");
  assert(uploadRequests.length === 2, `expected token and confirm requests, got ${uploadRequests.length}`);
  assert(uploadRequests[0].url.endsWith("/api/user/files/upload-token"), "uploadFileToQiniu should request file token endpoint");
  assert(uploadRequests[0].data.asset_type === "clothes_item_photo", "uploadFileToQiniu should request token with asset_type");
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
  assert(uploadRequests[1].data.asset_type === "clothes_item_photo", "uploadFileToQiniu should confirm asset_type");

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
              asset_type: "clothes_item_photo"
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
        assetType: "clothes_item_photo"
      }
    );
  });

  assert(tdesignUploadFiles[0].filePath === "wxfile://tmp-no-extension", "uploadFileToQiniu should support TDesign file.url");
  assert(tdesignUploadRequests[0].data.mime_type === "image/jpeg", "TDesign type=image should default to image/jpeg");
  assert(tdesignUploadRequests[0].data.file_ext === "jpg", "TDesign type=image should default to jpg extension");

  let missingAssetTypeRejected = false;
  try {
    await api.uploadFileToQiniu({ tempFilePath: "/tmp/clothes.png", size: 1 }, {});
  } catch (error) {
    missingAssetTypeRejected = error instanceof api.ApiError && error.code === "asset.asset_type_required";
  }
  assert(missingAssetTypeRejected, "uploadFileToQiniu should reject without assetType");

  let missingFilePathRejected = false;
  try {
    await api.uploadFileToQiniu({ size: 1, type: "image" }, { assetType: "clothes_item_photo" });
  } catch (error) {
    missingFilePathRejected = error instanceof api.ApiError && error.code === "asset.file_path_required";
  }
  assert(missingFilePathRejected, "uploadFileToQiniu should reject without a file path");

  let invalidImageRejected = false;
  try {
    await api.uploadFileToQiniu({ tempFilePath: "/tmp/file", size: 1, type: "video" }, { assetType: "clothes_item_photo" });
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
      await api.uploadFileToQiniu({ tempFilePath: "/tmp/clothes.jpg", size: 1 }, { assetType: "clothes_item_photo" });
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
      await api.uploadFileToQiniu({ path: "/tmp/clothes.jpg", size: 1 }, { assetType: "clothes_item_photo" });
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
