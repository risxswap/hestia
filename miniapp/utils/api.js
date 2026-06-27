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

function getWardrobeItems(filters) {
  const params = filters || {};
  const query = Object.keys(params)
    .filter((key) => params[key] !== undefined && params[key] !== null && params[key] !== "")
    .map((key) => `${encodeURIComponent(key)}=${encodeURIComponent(params[key])}`)
    .join("&");

  return authorizedRequest({
    path: `/api/user/wardrobe/items${query ? `?${query}` : ""}`
  });
}

function createWardrobeItem(data) {
  return authorizedRequest({
    path: "/api/user/wardrobe/items",
    method: "POST",
    data
  });
}

function updateWardrobeItem(publicID, data) {
  return authorizedRequest({
    path: `/api/user/wardrobe/items/${publicID}`,
    method: "PATCH",
    data
  });
}

function deleteWardrobeItem(publicID) {
  return authorizedRequest({
    path: `/api/user/wardrobe/items/${publicID}`,
    method: "DELETE"
  });
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
  await ensureDevSession();
  const raw = await rawRequest({
    path: "/api/user/agent/stream",
    method: "POST",
    data: {
      text
    },
    header: {
      Accept: "text/event-stream"
    }
  });
  return parseSSEEvents(raw);
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
  getWardrobeItems,
  createWardrobeItem,
  updateWardrobeItem,
  deleteWardrobeItem,
  getOnboardingDraft,
  saveOnboardingDraft,
  submitOnboarding,
  sendImageRouteFeedback,
  sendAgentMessage,
  parseSSEEvents
};
