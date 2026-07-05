const path = require("path");
const fs = require("fs");

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

async function main() {
  const calls = [];
  const apiStub = {
    getMemoryItems() {
      calls.push({ name: "getMemoryItems" });
      return Promise.resolve({
        items: [
          {
            public_id: "mem_1",
            memory_type: "preference",
            type_label: "偏好",
            memory_key: "style_goal",
            memory_value: "通勤更利落",
            display_text: "通勤更利落",
            polarity: "positive",
            confidence: 0.82,
            source_label: "用户确认",
            updated_at: "2026-07-05T10:00:00Z"
          }
        ]
      });
    },
    updateMemoryItem(publicID, data) {
      calls.push({ name: "updateMemoryItem", publicID, data });
      return Promise.resolve({
        public_id: publicID,
        memory_type: "preference",
        type_label: "偏好",
        memory_key: "style_goal",
        memory_value: data.memory_value,
        display_text: data.memory_value,
        correction_note: data.correction_note,
        source_label: "用户修正"
      });
    },
    deleteMemoryItem(publicID) {
      calls.push({ name: "deleteMemoryItem", publicID });
      return Promise.resolve({ public_id: publicID });
    }
  };

  const memory = loadPage("pages/memory/index.js", apiStub);
  assert(memory.pageConfig, "memory/index.js should register Page config");
  assert(typeof memory.exported.normalizeMemoryItems === "function", "memory page should export normalizeMemoryItems");
  assert(typeof memory.pageConfig.loadMemory === "function", "memory page should load memories");
  assert(typeof memory.pageConfig.handleOpenEdit === "function", "memory page should open edit modal");
  assert(typeof memory.pageConfig.handleSaveEdit === "function", "memory page should save edits");
  assert(typeof memory.pageConfig.handleDelete === "function", "memory page should delete items");

  const page = createPageInstance(memory.pageConfig);
  await page.loadMemory.call(page);
  assert(calls[0].name === "getMemoryItems", "memory page should call getMemoryItems");
  assert(page.data.items.length === 1, "memory page should hydrate one memory item");
  assert(page.data.items[0].display_text === "通勤更利落", "memory item should keep display text");
  assert(page.data.items[0].confidenceText === "置信度 82%", "memory item should format confidence");

  page.handleOpenEdit.call(page, { currentTarget: { dataset: { publicId: "mem_1" } } });
  assert(page.data.editVisible === true, "edit modal should open");
  assert(page.data.draft.memory_value === "通勤更利落", "edit draft should use selected memory value");

  page.setData({ draft: { memory_value: "通勤想要更清爽利落" } });
  await page.handleSaveEdit.call(page);
  const updateCall = calls.find((call) => call.name === "updateMemoryItem");
  assert(updateCall && updateCall.publicID === "mem_1", "save should update selected memory");
  assert(updateCall.data.memory_value === "通勤想要更清爽利落", "save should send edited value");
  assert(page.data.items[0].display_text === "通勤想要更清爽利落", "updated item should replace list item");

  const originalWx = global.wx;
  global.wx = {
    showModal(options) {
      options.success({ confirm: true });
    },
    showToast() {}
  };
  await page.handleDelete.call(page, { currentTarget: { dataset: { publicId: "mem_1" } } });
  global.wx = originalWx;
  const deleteCall = calls.find((call) => call.name === "deleteMemoryItem");
  assert(deleteCall && deleteCall.publicID === "mem_1", "delete should call deleteMemoryItem");
  assert(page.data.items.length === 0, "deleted item should be removed from list");

  const markup = fs.readFileSync(path.join(root, "pages/memory/index.wxml"), "utf8");
  const styles = fs.readFileSync(path.join(root, "pages/memory/index.wxss"), "utf8");
  assert(markup.includes("bind:tap=\"handleOpenEdit\""), "memory markup should expose edit action");
  assert(markup.includes("bind:tap=\"handleDelete\""), "memory markup should expose delete action");
  assert(markup.includes("editVisible"), "memory markup should include edit modal");
  assert(styles.includes(".memory-list"), "memory styles should define list layout");

  console.log("memory page verification passed");
}

main().catch((error) => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
