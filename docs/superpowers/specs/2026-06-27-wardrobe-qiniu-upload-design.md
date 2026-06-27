# 衣橱图片七牛上传设计

## 背景

图库式衣橱已经支持通过 `primary_asset_public_id` 关联主图，但新增和编辑弹窗里仍然需要用户手动填写“主图资产 ID”。这不符合真实使用方式，也阻塞了衣橱图库成为以图片为核心的界面。

本设计补齐图片上传链路：用户在小程序里选择或拍摄衣服图片，图片存储在七牛云对象存储，后端只负责签发上传凭证、登记资产、生成短期可访问图片 URL，并继续通过现有衣橱字段绑定主图。

图片属于敏感个人数据。第一版按私有 bucket 设计，不把对象 key 当作长期公开访问地址。

## 目标

- 新增衣服和编辑衣服时，可以在弹窗内选择、上传、替换或移除主图。
- 使用项目已有 TDesign 小程序组件能力，优先采用 `t-upload`，不把聊天输入组件搬进衣橱表单。
- 后端接入七牛云对象存储，业务服务端签发上传凭证，小程序直传七牛。
- 上传成功后登记到现有 `assets` 表，并把 `asset_public_id` 写入衣橱的 `primary_asset_public_id`。
- 衣橱列表和详情页返回短期签名图片 URL，让私有 bucket 图片可以被小程序展示。
- 上传失败、确认失败、图片过大等情况有明确反馈，不能保存半吊子的图片绑定。

## 非目标

- 不做完整文件管理后台。
- 不做批量衣橱导入。
- 不做视频上传。
- 不做自动识别衣服分类、颜色、材质。
- 不做电商商品图抓取或商品链接推荐。
- 删除衣服时不在本轮物理删除七牛对象；仅保留后续资产清理能力。

## 推荐方案

采用“小程序直传七牛 + 后端登记资产”的方案。

流程：

1. 用户在衣橱新增或编辑弹窗点击上传区域。
2. `t-upload` 调起相册或相机，限制单张图片。
3. 小程序向后端申请上传凭证。
4. 后端生成 `asset_public_id`、`object_key`、上传 token、上传地址和过期时间。
5. 小程序使用 `wx.uploadFile` 直传七牛。
6. 七牛返回成功后，小程序调用后端确认接口。
7. 后端写入 `assets` 表，状态为 `active`、审核状态为 `pending`。
8. 小程序把确认后的 `asset_public_id` 写入当前草稿的 `primary_asset_public_id`。
9. 用户保存衣服时，现有衣橱创建或更新接口绑定该主图资产。

不采用“图片先传业务后端再转发七牛”，因为它会增加服务端带宽和延迟。不采用“前端自己控制七牛 key/token”，因为密钥和对象路径控制权不能放到小程序端。

## 后端设计

### 配置

在服务端配置中增加七牛字段：

- `QINIU_ACCESS_KEY`
- `QINIU_SECRET_KEY`
- `QINIU_BUCKET`
- `QINIU_UPLOAD_HOST`
- `QINIU_PRIVATE_DOMAIN`
- `QINIU_UPLOAD_TOKEN_TTL_SECONDS`
- `QINIU_DOWNLOAD_URL_TTL_SECONDS`

TOML 配置可增加 `[qiniu]` 段，字段与环境变量对应。环境变量优先级继续沿用现有配置加载策略。

### 上传凭证接口

新增受登录保护的接口：

`POST /api/user/assets/upload-token`

请求体：

```json
{
  "asset_type": "wardrobe_item_photo",
  "mime_type": "image/jpeg",
  "file_size": 123456,
  "file_ext": ".jpg"
}
```

服务端行为：

- 校验用户已登录。
- 只允许图片 MIME 类型。
- 限制单文件大小，第一版建议 10 MB。
- 只允许明确的资产类型，衣橱主图使用 `wardrobe_item_photo`。
- 生成 `asset_public_id`，沿用现有 `ast` public id 前缀。
- 生成服务端控制的 `object_key`，例如：

```text
users/{user_id}/wardrobe/{asset_public_id}.jpg
```

- 生成七牛上传凭证，限定 bucket 和 object key。
- 返回上传所需信息，不返回七牛密钥。

响应：

```json
{
  "asset_public_id": "ast_xxx",
  "bucket": "hestia-private",
  "object_key": "users/123/wardrobe/ast_xxx.jpg",
  "upload_url": "https://upload.qiniup.com",
  "upload_token": "token",
  "expires_at": "2026-06-27T12:00:00Z"
}
```

### 上传确认接口

新增受登录保护的接口：

`POST /api/user/assets/confirm`

请求体：

```json
{
  "asset_public_id": "ast_xxx",
  "bucket": "hestia-private",
  "object_key": "users/123/wardrobe/ast_xxx.jpg",
  "mime_type": "image/jpeg",
  "file_size": 123456,
  "width": 1200,
  "height": 1600,
  "asset_type": "wardrobe_item_photo"
}
```

服务端行为：

- 校验当前用户只能确认自己路径下、由服务端签发的对象 key。
- 登记到现有 `assets` 表。
- 对同一 `asset_public_id` 的重复确认保持幂等：如果已存在且属于当前用户、bucket 和 object key 一致，则直接返回已有资产；如果归属或 object key 不一致，则拒绝。
- `source` 使用 `miniapp_upload`。
- `status` 使用 `active`。
- `review_status` 使用 `pending`。
- `metadata_json` 记录上传来源、确认时间、原始尺寸等轻量信息。

响应：

```json
{
  "asset_public_id": "ast_xxx",
  "object_key": "users/123/wardrobe/ast_xxx.jpg",
  "url": "https://private-domain/...",
  "asset_type": "wardrobe_item_photo"
}
```

第一版不强制服务端回查七牛对象元信息；如果后续发现确认接口被误调用，再增加七牛 stat 校验。

### 图片展示 URL

现有 `wardrobe.Item.PrimaryImage` 已有 `URL` 字段。衣橱列表和详情仍返回 `primary_image`，但后端需要在返回前把私有 bucket 图片转换为短期签名 URL：

```json
{
  "primary_image": {
    "asset_public_id": "ast_xxx",
    "object_key": "users/123/wardrobe/ast_xxx.jpg",
    "url": "https://private-domain/..."
  }
}
```

小程序继续使用 `primaryImageSrc = primary_image.url || primary_image.object_key`，但真实七牛图片应优先使用 `url`。

## 前端设计

### 组件选择

衣橱新增弹窗和详情编辑弹窗使用 TDesign `t-upload`：

- `max=1`
- `mediaType=['image']`
- 支持拍照和相册。
- 使用 `requestMethod` 自定义上传逻辑。
- `files` 由页面草稿状态控制。

这满足“使用现有组件”的要求，也能保持衣橱表单和聊天页解耦。

### 上传工具

在小程序 API 层增加三个能力：

- `createAssetUploadToken(input)`
- `confirmAssetUpload(input)`
- `uploadAssetToQiniu(file, options)`

`uploadAssetToQiniu` 封装：

1. 根据文件类型和大小申请 token。
2. 使用 `wx.uploadFile` 上传七牛。
3. 解析七牛响应。
4. 调用 confirm。
5. 返回 `{ asset_public_id, url, object_key }`。

### 弹窗交互

新增和编辑弹窗中的“主图资产 ID”输入框替换为图片上传块。

状态：

- 无图：显示上传入口。
- 上传中：显示进度或 loading。
- 成功：显示缩略图。
- 失败：显示错误提示，允许重试。
- 删除：清空 `draft.primary_asset_public_id` 和本地 `files`。

保存衣服时：

- 如果用户上传成功，提交 `primary_asset_public_id`。
- 如果用户移除主图，提交空字符串，让后端清除主图关系。
- 如果图片仍在上传中，阻止保存并提示等待上传完成。

### 图片回显

编辑已有衣服时：

- 如果当前单品有 `primary_image.url`，弹窗初始化 `files` 为该 URL。
- 如果只有 `asset_public_id` 但没有 URL，则展示占位并保留资产 ID。
- 替换图片后，草稿里的 `primary_asset_public_id` 更新为新资产。

## 错误处理

- 未配置七牛：上传凭证接口返回服务端错误，前端提示“图片上传暂不可用”。
- 文件过大：前端先拦截，后端再兜底拒绝。
- 非图片文件：前端限制媒体类型，后端兜底拒绝。
- 七牛上传失败：不调用确认接口，保留草稿文本字段。
- 确认失败：上传块显示失败，不把资产 ID 写入草稿。
- 保存衣服失败：已上传资产保留在用户资产表中，但不会绑定到衣服；后续可由清理任务处理未绑定资产。

## 隐私与安全

- 七牛 access key 和 secret key 只存在服务端。
- 小程序只能拿到短期上传 token 和短期下载 URL。
- object key 由服务端生成，不能由客户端指定完整路径。
- 私有下载 URL 设置短 TTL。
- `assets.owner_user_id` 必须写入当前登录用户 ID。
- 衣橱绑定主图时继续校验资产属于当前用户。
- 不在日志中记录上传 token。

## 测试与验证

后端需要覆盖：

- 上传凭证接口拒绝未登录请求。
- 上传凭证接口拒绝非图片 MIME 类型。
- 上传凭证接口生成 `ast` public id 和用户隔离的 object key。
- 上传确认接口写入 `assets` 表。
- 上传确认接口拒绝不属于当前用户路径的 object key。
- 衣橱列表返回主图短期 URL。

前端需要覆盖：

- API client 暴露上传 token、确认、七牛上传封装。
- 新增弹窗上传成功后写入 `draft.primary_asset_public_id`。
- 编辑弹窗能回显已有主图。
- 上传中不能保存。
- 移除图片会清空主图资产 ID。

手工验证需要覆盖：

- 新增衣服时拍照上传并保存。
- 新增衣服时从相册上传并保存。
- 衣橱图库展示真实图片。
- 进入详情页展示同一张主图。
- 编辑详情页替换主图后，图库和详情都刷新。
- 七牛配置缺失时前端有可理解错误，不影响文本字段录入。

## 已确认决策

- 七牛 bucket 按私有 bucket 设计。
- 小程序直传七牛，服务端签发上传凭证。
- 衣橱弹窗使用现有 TDesign 上传组件能力。
- 后端返回短期签名下载 URL 给图库和详情页展示。
- 本轮聚焦单张衣橱主图，不做批量导入和自动识别。
