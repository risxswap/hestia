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
    "getWardrobeItems",
    "createWardrobeItem",
    "updateWardrobeItem",
    "deleteWardrobeItem"
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
    await api.getWardrobeItems();
    await api.getWardrobeItems({ category: "top" });
    await api.createWardrobeItem({ name: "米白衬衫", category: "top" });
    await api.updateWardrobeItem("wdi_test", { color: "米白" });
    await api.deleteWardrobeItem("wdi_test");
  });

  const wardrobePaths = wardrobeCalls.map((call) => call.url.replace("http://127.0.0.1:8080", ""));
  assert(wardrobeCalls[0].method === "GET", "getWardrobeItems should use GET");
  assert(wardrobePaths[0] === "/api/user/wardrobe/items", `wardrobe list path mismatch: ${wardrobePaths[0]}`);
  assert(wardrobeCalls[1].method === "GET", "filtered getWardrobeItems should use GET");
  assert(wardrobePaths[1] === "/api/user/wardrobe/items?category=top", `wardrobe filtered path mismatch: ${wardrobePaths[1]}`);
  assert(wardrobeCalls[2].method === "POST", "createWardrobeItem should use POST");
  assert(wardrobePaths[2] === "/api/user/wardrobe/items", `wardrobe create path mismatch: ${wardrobePaths[2]}`);
  assert(wardrobeCalls[3].method === "PATCH", "updateWardrobeItem should use PATCH");
  assert(wardrobePaths[3] === "/api/user/wardrobe/items/wdi_test", `wardrobe update path mismatch: ${wardrobePaths[3]}`);
  assert(wardrobeCalls[4].method === "DELETE", "deleteWardrobeItem should use DELETE");
  assert(wardrobePaths[4] === "/api/user/wardrobe/items/wdi_test", `wardrobe delete path mismatch: ${wardrobePaths[4]}`);

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
}

main()
  .then(() => {
    console.log("api client verification passed");
  })
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
