const DEFAULT_API_BASE_URL = "http://127.0.0.1:8080";
const TOKEN_KEY = "user_token";

class ApiError extends Error {
  constructor(message, options) {
    super(message || "请求失败");
    this.name = "ApiError";
    this.code = options && options.code ? options.code : "api.request_failed";
    this.statusCode = options && options.statusCode ? options.statusCode : 0;
    this.data = options ? options.data : null;
  }
}

function getApiBaseUrl() {
  const app = typeof getApp === "function" ? getApp() : null;
  const appBaseUrl = app && app.globalData && app.globalData.apiBaseUrl;
  if (appBaseUrl) {
    return trimTrailingSlash(appBaseUrl);
  }

  if (typeof wx !== "undefined" && wx.getStorageSync) {
    const storedBaseUrl = wx.getStorageSync("api_base_url");
    if (storedBaseUrl) {
      return trimTrailingSlash(storedBaseUrl);
    }
  }

  return DEFAULT_API_BASE_URL;
}

function trimTrailingSlash(value) {
  return String(value || "").replace(/\/+$/, "");
}

function normalizePath(path) {
  return String(path || "").startsWith("/") ? path : `/${path}`;
}

function storageToken() {
  if (typeof wx === "undefined" || !wx.getStorageSync) {
    return "";
  }
  return wx.getStorageSync(TOKEN_KEY) || wx.getStorageSync("token") || "";
}

function setStorage(key, value) {
  if (typeof wx !== "undefined" && wx.setStorageSync) {
    wx.setStorageSync(key, value);
  }
}

function request(options) {
  const config = options || {};
  const auth = config.auth !== false;
  const token = storageToken();
  const header = Object.assign({}, config.header || {});
  if (auth && token) {
    header.Authorization = `Bearer ${token}`;
  }

  return new Promise((resolve, reject) => {
    if (typeof wx === "undefined" || !wx.request) {
      reject(new ApiError("当前环境不支持 wx.request", { code: "api.wx_unavailable" }));
      return;
    }

    wx.request({
      url: `${getApiBaseUrl()}${normalizePath(config.path)}`,
      method: config.method || "GET",
      data: config.data,
      header,
      responseType: config.responseType,
      success(response) {
        const body = response && response.data;
        const statusCode = response && response.statusCode ? response.statusCode : 0;

        if (statusCode >= 200 && statusCode < 300 && body && body.code === "ok") {
          resolve(body.data);
          return;
        }

        reject(new ApiError(body && body.message ? body.message : "请求失败", {
          code: body && body.code ? body.code : "api.request_failed",
          statusCode,
          data: body && Object.prototype.hasOwnProperty.call(body, "data") ? body.data : body
        }));
      },
      fail(error) {
        reject(new ApiError(error && error.errMsg ? error.errMsg : "网络请求失败", {
          code: "api.network_failed",
          data: error
        }));
      }
    });
  });
}

async function ensureDevSession(options) {
  const config = options || {};
  const existing = !config.force && storageToken();
  if (existing) {
    return {
      token: existing,
      user_public_id: wx.getStorageSync("user_public_id") || "",
      onboarding_status: wx.getStorageSync("onboarding_status") || ""
    };
  }

  const devKey = config.devKey || wx.getStorageSync("dev_key") || "miniapp-local";
  const nickname = config.nickname || wx.getStorageSync("dev_nickname") || "小程序本地用户";
  const result = await request({
    path: "/api/user/dev-login",
    method: "POST",
    auth: false,
    data: {
      dev_key: devKey,
      nickname
    }
  });

  setStorage(TOKEN_KEY, result.token);
  setStorage("token", result.token);
  setStorage("user_public_id", result.user_public_id);
  setStorage("onboarding_status", result.onboarding_status || "");
  return result;
}

async function authorizedRequest(options) {
  await ensureDevSession();
  return request(options);
}

function getLatestReport() {
  return authorizedRequest({
    path: "/api/user/reports/latest"
  });
}

function getProfileSummary() {
  return authorizedRequest({
    path: "/api/user/profile/summary"
  });
}

function updateProfile(data) {
  return authorizedRequest({
    path: "/api/user/profile",
    method: "PATCH",
    data
  });
}

function updateProfilePreferences(data) {
  return authorizedRequest({
    path: "/api/user/profile/preferences",
    method: "PATCH",
    data
  });
}

function createProfilePhoto(data) {
  return authorizedRequest({
    path: "/api/user/profile/photos",
    method: "POST",
    data
  });
}

function updateProfilePhoto(publicID, data) {
  return authorizedRequest({
    path: `/api/user/profile/photos/${publicID}`,
    method: "PATCH",
    data
  });
}

function deleteProfilePhoto(publicID) {
  return authorizedRequest({
    path: `/api/user/profile/photos/${publicID}`,
    method: "DELETE"
  });
}

function getCollectionSummary() {
  return authorizedRequest({
    path: "/api/user/collection"
  });
}

function getMemoryItems() {
  return authorizedRequest({
    path: "/api/user/memories"
  });
}

function updateMemoryItem(publicID, data) {
  return authorizedRequest({
    path: `/api/user/memories/${publicID}`,
    method: "PATCH",
    data
  });
}

function deleteMemoryItem(publicID) {
  return authorizedRequest({
    path: `/api/user/memories/${publicID}`,
    method: "DELETE"
  });
}

function getHairItems() {
  return authorizedRequest({
    path: "/api/user/hair"
  });
}

function getHairItem(publicID) {
  return authorizedRequest({
    path: `/api/user/hair/${publicID}`
  });
}

function createHairItem(data) {
  return authorizedRequest({
    path: "/api/user/hair",
    method: "POST",
    data
  });
}

function updateHairItem(publicID, data) {
  return authorizedRequest({
    path: `/api/user/hair/${publicID}`,
    method: "PATCH",
    data
  });
}

function deleteHairItem(publicID) {
  return authorizedRequest({
    path: `/api/user/hair/${publicID}`,
    method: "DELETE"
  });
}

function getMakeupItems() {
  return authorizedRequest({
    path: "/api/user/makeup"
  });
}

function getMakeupItem(publicID) {
  return authorizedRequest({
    path: `/api/user/makeup/${publicID}`
  });
}

function createMakeupItem(data) {
  return authorizedRequest({
    path: "/api/user/makeup",
    method: "POST",
    data
  });
}

function updateMakeupItem(publicID, data) {
  return authorizedRequest({
    path: `/api/user/makeup/${publicID}`,
    method: "PATCH",
    data
  });
}

function deleteMakeupItem(publicID) {
  return authorizedRequest({
    path: `/api/user/makeup/${publicID}`,
    method: "DELETE"
  });
}

function getClothesItems(filters) {
  const params = filters || {};
  const query = Object.keys(params)
    .filter((key) => params[key] !== undefined && params[key] !== null && params[key] !== "")
    .map((key) => `${encodeURIComponent(key)}=${encodeURIComponent(params[key])}`)
    .join("&");

  return authorizedRequest({
    path: `/api/user/clothes/items${query ? `?${query}` : ""}`
  });
}

function getClothesItem(publicID) {
  return authorizedRequest({
    path: `/api/user/clothes/items/${publicID}`
  });
}

function getClothesOptions() {
  return authorizedRequest({
    path: "/api/user/clothes/options"
  });
}

function createClothesItem(data) {
  return authorizedRequest({
    path: "/api/user/clothes/items",
    method: "POST",
    data
  });
}

function updateClothesItem(publicID, data) {
  return authorizedRequest({
    path: `/api/user/clothes/items/${publicID}`,
    method: "PATCH",
    data
  });
}

function deleteClothesItem(publicID) {
  return authorizedRequest({
    path: `/api/user/clothes/items/${publicID}`,
    method: "DELETE"
  });
}

function recognizeClothesItemImage(assetPublicID, options) {
  const config = options || {};
  return authorizedRequest({
    path: "/api/user/clothes/items/recognize",
    method: "POST",
    data: {
      asset_public_id: assetPublicID,
      item_public_id: config.itemPublicID || config.item_public_id || "",
      overwrite: config.overwrite === true
    }
  });
}

function createFileUploadToken(data) {
  return authorizedRequest({
    path: "/api/user/files/upload-token",
    method: "POST",
    data
  });
}

function confirmFileUpload(data) {
  return authorizedRequest({
    path: "/api/user/files/confirm",
    method: "POST",
    data
  });
}

async function uploadFileToQiniu(file, options) {
  const source = file || {};
  const config = options || {};
  const assetType = config.assetType;
  if (!assetType) {
    throw new ApiError("缺少资产类型", { code: "asset.asset_type_required" });
  }

  const filePath = source.url || source.path || source.tempFilePath;
  if (!filePath) {
    throw new ApiError("缺少上传文件路径", { code: "asset.file_path_required" });
  }

  const rawExt = inferFileExt(filePath, source.mimeType || source.type);
  const mimeType = inferImageMimeType(source.mimeType || source.type, rawExt);
  if (!mimeType) {
    throw new ApiError("不支持的图片格式", { code: "asset.invalid_image_type" });
  }
  const fileExt = rawExt || mimeExt(mimeType);
  const fileSize = source.size;
  const token = await createFileUploadToken({
    asset_type: assetType,
    mime_type: mimeType,
    file_size: fileSize,
    file_ext: fileExt
  });

  await uploadRawFileToQiniu({
    uploadUrl: token.upload_url,
    uploadToken: token.upload_token,
    objectKey: token.object_key,
    filePath
  });

  return confirmFileUpload({
    asset_public_id: token.asset_public_id,
    file_public_id: token.file_public_id || token.asset_public_id,
    bucket: token.bucket,
    object_key: token.object_key,
    mime_type: mimeType,
    file_size: fileSize,
    width: firstDefined(config.width, source.width),
    height: firstDefined(config.height, source.height),
    asset_type: assetType
  });
}

function uploadRawFileToQiniu(options) {
  const config = options || {};
  return new Promise((resolve, reject) => {
    if (typeof wx === "undefined" || !wx.uploadFile) {
      reject(new ApiError("当前环境不支持 wx.uploadFile", { code: "api.wx_unavailable" }));
      return;
    }

    wx.uploadFile({
      url: config.uploadUrl,
      filePath: config.filePath,
      name: "file",
      formData: {
        token: config.uploadToken,
        key: config.objectKey
      },
      success(response) {
        const statusCode = response && response.statusCode ? Number(response.statusCode) : 0;
        if (statusCode >= 200 && statusCode < 300) {
          resolve(response);
          return;
        }

        reject(new ApiError("上传失败", {
          code: "asset.upload_failed",
          statusCode,
          data: response
        }));
      },
      fail(error) {
        reject(new ApiError(error && error.errMsg ? error.errMsg : "上传失败", {
          code: "asset.upload_failed",
          data: error
        }));
      }
    });
  });
}

function firstDefined(primary, fallback) {
  return primary !== undefined && primary !== null ? primary : fallback;
}

function inferFileExt(filePath, mimeType) {
  const cleanPath = String(filePath || "").split("?")[0].split("#")[0];
  const match = cleanPath.match(/\.([a-zA-Z0-9]+)$/);
  if (match) {
    const ext = match[1].toLowerCase();
    if (["jpg", "jpeg", "png", "webp", "heic"].indexOf(ext) >= 0) {
      return ext;
    }
  }
  return mimeExt(mimeType);
}

function inferImageMimeType(mimeType, fileExt) {
  const normalized = normalizeImageMimeType(mimeType);
  if (normalized) {
    return normalized;
  }
  if (String(mimeType || "").toLowerCase() === "image") {
    return "image/jpeg";
  }

  const ext = String(fileExt || "").toLowerCase();
  if (ext === "jpg" || ext === "jpeg") {
    return "image/jpeg";
  }
  if (ext === "png") {
    return "image/png";
  }
  if (ext === "webp") {
    return "image/webp";
  }
  if (ext === "heic") {
    return "image/heic";
  }
  return "";
}

function normalizeImageMimeType(mimeType) {
  const value = String(mimeType || "").toLowerCase();
  if (value === "image/jpg" || value === "image/jpeg") {
    return "image/jpeg";
  }
  if (value === "image/png") {
    return "image/png";
  }
  if (value === "image/webp") {
    return "image/webp";
  }
  if (value === "image/heic" || value === "image/heif") {
    return "image/heic";
  }
  return "";
}

function mimeExt(mimeType) {
  const normalized = normalizeImageMimeType(mimeType);
  if (normalized === "image/jpeg") {
    return "jpg";
  }
  if (normalized === "image/png") {
    return "png";
  }
  if (normalized === "image/webp") {
    return "webp";
  }
  if (normalized === "image/heic") {
    return "heic";
  }
  return "";
}

function getOnboardingDraft() {
  return authorizedRequest({
    path: "/api/user/onboarding"
  });
}

function saveOnboardingDraft(step, data) {
  return authorizedRequest({
    path: "/api/user/onboarding",
    method: "PUT",
    data: {
      step,
      data
    }
  });
}

function submitOnboarding() {
  return authorizedRequest({
    path: "/api/user/onboarding/submit",
    method: "POST"
  });
}

async function sendImageRouteFeedback(publicID, action, reason) {
  return authorizedRequest({
    path: `/api/user/image-routes/${publicID}/feedback`,
    method: "POST",
    data: {
      action,
      reason: reason || ""
    }
  });
}

async function sendAgentMessage(text) {
  const stream = streamAgentChat(text);
  return stream.promise;
}

function streamAgentChat(text, callbacks) {
  const events = [];
  const handlers = callbacks || {};
  let requestTask = null;
  let receivedChunk = false;

  const promise = ensureDevSession().then(() => new Promise((resolve, reject) => {
    const parser = createSSEParser((event) => {
      events.push(event);
      if (typeof handlers.onEvent === "function") {
        handlers.onEvent(event);
      }
      const eventHandler = handlers[`on${upperFirst(event.event)}`];
      if (typeof eventHandler === "function") {
        eventHandler(event.data, event);
      }
    });
    const chunkDecoder = createChunkDecoder();

    requestTask = wx.request({
      url: `${getApiBaseUrl()}${normalizePath("/api/user/agent/chat")}`,
      method: "POST",
      data: { text },
      enableChunked: true,
      header: agentStreamHeader(),
      success(response) {
        const statusCode = response && response.statusCode ? response.statusCode : 0;
        if (statusCode < 200 || statusCode >= 300) {
          reject(new ApiError("智能体请求失败", {
            code: "api.request_failed",
            statusCode,
            data: response && response.data
          }));
          return;
        }
        if (!receivedChunk && typeof response.data === "string") {
          parser.push(response.data);
        }
        parser.push(chunkDecoder.flush());
        parser.flush();
        resolve(events);
      },
      fail(error) {
        reject(new ApiError(error && error.errMsg ? error.errMsg : "网络请求失败", {
          code: "api.network_failed",
          data: error
        }));
      }
    });

    if (requestTask && typeof requestTask.onChunkReceived === "function") {
      requestTask.onChunkReceived((response) => {
        receivedChunk = true;
        parser.push(chunkDecoder.decode(response && response.data));
      });
    }
  }));

  return {
    promise,
    abort() {
      if (requestTask && typeof requestTask.abort === "function") {
        requestTask.abort();
      }
    }
  };
}

function agentStreamHeader() {
  const token = storageToken();
  const header = {
    Accept: "text/event-stream"
  };
  if (token) {
    header.Authorization = `Bearer ${token}`;
  }
  return header;
}

function getCurrentAdviceDraft() {
  return authorizedRequest({
    path: "/api/user/advice-drafts/current"
  });
}

function confirmAdviceDraft(publicID) {
  return authorizedRequest({
    path: `/api/user/advice-drafts/${publicID}/confirm`,
    method: "POST"
  });
}

function discardAdviceDraft(publicID) {
  return authorizedRequest({
    path: `/api/user/advice-drafts/${publicID}/discard`,
    method: "POST"
  });
}

function rawRequest(options) {
  const config = options || {};
  const token = storageToken();
  const header = Object.assign({}, config.header || {});
  if (token) {
    header.Authorization = `Bearer ${token}`;
  }

  return new Promise((resolve, reject) => {
    wx.request({
      url: `${getApiBaseUrl()}${normalizePath(config.path)}`,
      method: config.method || "GET",
      data: config.data,
      header,
      success(response) {
        const statusCode = response && response.statusCode ? response.statusCode : 0;
        if (statusCode >= 200 && statusCode < 300) {
          resolve(typeof response.data === "string" ? response.data : String(response.data || ""));
          return;
        }
        reject(new ApiError("请求失败", {
          code: "api.request_failed",
          statusCode,
          data: response && response.data
        }));
      },
      fail(error) {
        reject(new ApiError(error && error.errMsg ? error.errMsg : "网络请求失败", {
          code: "api.network_failed",
          data: error
        }));
      }
    });
  });
}

function parseSSEEvents(raw) {
  return String(raw || "")
    .split(/\n\n+/)
    .map((block) => parseSSEBlock(block))
    .filter(Boolean);
}

function createSSEParser(onEvent) {
  let buffer = "";
  return {
    push(chunk) {
      buffer += String(chunk || "");
      const blocks = buffer.split(/\r?\n\r?\n/);
      buffer = blocks.pop() || "";
      blocks.forEach((block) => {
        const event = parseSSEBlock(block);
        if (event) {
          onEvent(event);
        }
      });
    },
    flush() {
      const event = parseSSEBlock(buffer);
      buffer = "";
      if (event) {
        onEvent(event);
      }
    }
  };
}

function createChunkDecoder() {
  const decoder = typeof TextDecoder !== "undefined" ? new TextDecoder("utf-8") : null;
  let pendingBytes = [];
  return {
    decode(data) {
      if (!data) {
        return "";
      }
      if (typeof data === "string") {
        return data;
      }
      const bytes = new Uint8Array(data);
      if (decoder) {
        return decoder.decode(bytes, { stream: true });
      }
      const combined = pendingBytes.concat(Array.prototype.slice.call(bytes));
      const splitAt = completeUTF8PrefixLength(combined);
      pendingBytes = combined.slice(splitAt);
      return decodeUTF8Bytes(combined.slice(0, splitAt));
    },
    flush() {
      if (decoder) {
        return decoder.decode();
      }
      const text = decodeUTF8Bytes(pendingBytes);
      pendingBytes = [];
      return text;
    }
  };
}

function completeUTF8PrefixLength(bytes) {
  let index = 0;
  let lastComplete = 0;
  while (index < bytes.length) {
    const byte = bytes[index];
    const width = utf8SequenceWidth(byte);
    if (width === 0 || index + width > bytes.length) {
      break;
    }
    index += width;
    lastComplete = index;
  }
  return lastComplete;
}

function utf8SequenceWidth(byte) {
  if (byte <= 0x7f) return 1;
  if (byte >= 0xc2 && byte <= 0xdf) return 2;
  if (byte >= 0xe0 && byte <= 0xef) return 3;
  if (byte >= 0xf0 && byte <= 0xf4) return 4;
  return 0;
}

function decodeUTF8Bytes(bytes) {
  if (!bytes.length) {
    return "";
  }
  const encoded = bytes.map((byte) => `%${byte.toString(16).padStart(2, "0")}`).join("");
  try {
    return decodeURIComponent(encoded);
  } catch (error) {
    return bytes.map((byte) => String.fromCharCode(byte)).join("");
  }
}

function upperFirst(value) {
  const text = String(value || "");
  return text ? text.charAt(0).toUpperCase() + text.slice(1) : "";
}

function parseSSEBlock(block) {
  if (!String(block || "").trim()) {
    return null;
  }

  const lines = String(block || "").split(/\n/);
  let event = "message";
  const dataLines = [];

  lines.forEach((line) => {
    if (line.startsWith("event:")) {
      event = line.slice("event:".length).trim();
      return;
    }
    if (line.startsWith("data:")) {
      dataLines.push(line.slice("data:".length).trim());
    }
  });

  if (!event && dataLines.length === 0) {
    return null;
  }

  const rawData = dataLines.join("\n");
  let data = rawData;
  if (rawData) {
    try {
      data = JSON.parse(rawData);
    } catch (error) {
      data = rawData;
    }
  }

  return {
    event,
    data
  };
}

module.exports = {
  ApiError,
  request,
  ensureDevSession,
  getLatestReport,
  getProfileSummary,
  updateProfile,
  updateProfilePreferences,
  createProfilePhoto,
  updateProfilePhoto,
  deleteProfilePhoto,
  getCollectionSummary,
  getMemoryItems,
  updateMemoryItem,
  deleteMemoryItem,
  getHairItems,
  getHairItem,
  createHairItem,
  updateHairItem,
  deleteHairItem,
  getMakeupItems,
  getMakeupItem,
  createMakeupItem,
  updateMakeupItem,
  deleteMakeupItem,
  getClothesItems,
  getClothesItem,
  getClothesOptions,
  createClothesItem,
  updateClothesItem,
  deleteClothesItem,
  recognizeClothesItemImage,
  createFileUploadToken,
  confirmFileUpload,
  uploadFileToQiniu,
  getOnboardingDraft,
  saveOnboardingDraft,
  submitOnboarding,
  sendImageRouteFeedback,
  sendAgentMessage,
  streamAgentChat,
  getCurrentAdviceDraft,
  confirmAdviceDraft,
  discardAdviceDraft,
  parseSSEEvents
};
