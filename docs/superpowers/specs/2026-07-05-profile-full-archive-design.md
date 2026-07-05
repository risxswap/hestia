# 完整用户档案设计

## 背景

当前小程序 `pages/profile` 已经具备基础档案编辑能力，后端 `profile` 域也支持昵称、性别表达、身高、身形备注、肤色备注、发型备注和常见场景。随着 Hestia 的核心闭环走向“持续记忆和进化”，档案需要从轻量表单升级为用户明确资料的核心资料库。

本设计聚焦“我的档案”能力：记录会影响穿搭、发型、妆容和场景建议的用户本人信息，并通过多角度本人照片为后续 AI 解析和建议生成提供可靠上下文。档案照片不包含核心衣橱照片；衣橱仍由衣服域管理。

## 目标

- 完善用户明确填写的档案字段，覆盖身高、体重、肤色、脸型、身形比例、发型发量和尺码备注等信息。
- 支持用户上传和管理多角度本人照片，包括自拍/头肩照、半身照和全身照。
- 每张档案照片记录照片类型、角度、备注、排序和删除状态。
- 复用现有文件上传与资产表能力，档案只保存图片引用和档案语义。
- 在 `profile` 域中保持结构化字段，方便后续记忆更新、AI 取数和隐私删除。
- 保持表达中性、可修正，不输出医疗、减肥、整形等高风险建议。

## 非目标

- 不把核心衣橱照片放进用户档案。
- 不做完整衣橱库存管理。
- 不做电商导购或 SKU 推荐。
- 不做自动抓取公开明星图片。
- 不做“你像某明星”的相貌对比系统。
- 不做减肥、医疗、植发、皮肤病、整形等需要专业资质的建议。
- 不把图片二进制存进 `profiles` 或 `profile_photos` 表。

## 产品原则

档案是“用户明确资料”的承载层，不是 AI 黑箱判断层。

- 用户明确填写的信息写入档案字段，并同步沉淀为事实类记忆。
- AI 推断仍进入记忆或推断体系，带置信度和可修正状态，不直接覆盖用户明确字段。
- 真实反馈优先级高于初始诊断和相似度匹配。
- 敏感信息允许删除；照片、档案字段和反馈数据的删除入口需要在隐私管理中保留。

## 字段范围

### 基础信息

第一版支持：

- 昵称。
- 性别表达。
- 身高 `height_cm`。
- 体重 `weight_kg`，可选、可删除，只用于比例、版型和尺码上下文，不用于健康或减脂判断。
- 常见生活场景 `lifestyle_scenarios`。

### 形象要素

保留现有备注字段：

- 身形/比例备注 `body_notes`。
- 肤色/用色备注 `skin_notes`。
- 发型/发量备注 `hair_notes`。

新增结构化或半结构化字段：

- 脸型 `face_shape`，例如圆脸、长脸、方圆脸、鹅蛋脸，也允许用户自定义。
- 肩颈/上半身特点 `upper_body_notes`。
- 腿型/下半身特点 `lower_body_notes`。
- 穿衣尺码备注 `size_notes`，例如上衣 M、裤装 27、鞋码 37，允许留空。

所有新增字段都是可选字段，用户可以清空。前端文案必须避免羞辱式表达，例如不要使用“缺陷”“显胖问题”“肤色差”等词。

## 档案照片

### 分组

档案照片按拍摄尺度分组：

1. 自拍/头肩照 `headshot`
2. 半身照 `half_body`
3. 全身照 `full_body`

第一版不把核心衣橱照片放在档案照片中。衣橱照片继续使用衣服域的图片能力。

### 角度

每张照片记录 `angle`。建议枚举：

- `front`：正面。
- `left_45`：左 45 度。
- `right_45`：右 45 度。
- `side`：侧面。
- `back`：背面。
- `natural`：自然日常。
- `sitting`：坐姿。
- `other`：其他。

各分组推荐角度：

- 自拍/头肩照：正面、左 45 度、右 45 度、侧面。
- 半身照：正面、侧面、自然站姿或坐姿。
- 全身照：正面、侧面、背面、自然日常。

### 展示与数量

第一版建议每个分组最多 6 张，总数最多 18 张。超过数量时前端阻止继续添加，后端兜底校验。

每张照片展示：

- 缩略图。
- 类型。
- 角度。
- 备注。
- 删除入口。

照片删除默认删除档案引用，不立即物理删除对象存储文件。资产级删除在隐私与数据管理中统一处理。

## 数据库设计

### 扩展 `profiles`

新增字段建议：

```sql
ALTER TABLE profiles
  ADD COLUMN weight_kg DECIMAL(5,2) NULL AFTER height_cm,
  ADD COLUMN face_shape VARCHAR(80) NULL AFTER hair_notes,
  ADD COLUMN upper_body_notes VARCHAR(220) NULL AFTER face_shape,
  ADD COLUMN lower_body_notes VARCHAR(220) NULL AFTER upper_body_notes,
  ADD COLUMN size_notes VARCHAR(220) NULL AFTER lower_body_notes;
```

约束建议：

- `weight_kg` 后端校验范围建议为 20 到 300，允许 `NULL`。
- 文本字段最大 220 个字符。
- `face_shape` 最大 80 个字符，允许用户自定义。

### 新增 `profile_photos`

新增表保存档案照片引用和语义：

```sql
CREATE TABLE IF NOT EXISTS profile_photos (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  public_id VARCHAR(64) NOT NULL UNIQUE,
  user_id BIGINT NOT NULL,
  profile_id BIGINT NOT NULL,
  asset_public_id VARCHAR(64) NOT NULL,
  photo_type VARCHAR(32) NOT NULL,
  angle VARCHAR(32) NOT NULL,
  note VARCHAR(220) NULL,
  sort_order INT NOT NULL DEFAULT 0,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at DATETIME(3) NULL,
  KEY idx_profile_photos_user_profile (user_id, profile_id, deleted_at),
  KEY idx_profile_photos_asset (asset_public_id),
  KEY idx_profile_photos_type_angle (user_id, photo_type, angle, deleted_at)
);
```

实现时如果现有 `files/assets` 表存在内部数值 ID，也可以在服务层通过 `asset_public_id` 校验归属；本表第一版只保存 public id，降低跨表迁移耦合。

## 后端接口

### 档案摘要

扩展现有接口：

`GET /api/user/profile/summary`

新增响应字段：

```json
{
  "profile": {
    "weight_kg": 52.5,
    "face_shape": "方圆脸",
    "upper_body_notes": "肩线偏窄，适合更清晰的肩部线条",
    "lower_body_notes": "偏好利落裤装",
    "size_notes": "上衣 M，鞋码 37"
  },
  "profile_photos": [
    {
      "public_id": "pph_xxx",
      "asset_public_id": "ast_xxx",
      "photo_type": "full_body",
      "angle": "front",
      "note": "自然站姿",
      "sort_order": 10,
      "image": {
        "url": "https://...",
        "object_key": "users/1/profile/ast_xxx.jpg"
      }
    }
  ]
}
```

### 更新档案

扩展现有接口：

`PATCH /api/user/profile`

新增请求字段：

```json
{
  "weight_kg": 52.5,
  "face_shape": "方圆脸",
  "upper_body_notes": "肩线偏窄",
  "lower_body_notes": "偏好利落裤装",
  "size_notes": "上衣 M，鞋码 37"
}
```

更新成功后返回完整 summary。

### 新增档案照片

新增接口：

`POST /api/user/profile/photos`

请求：

```json
{
  "asset_public_id": "ast_xxx",
  "photo_type": "full_body",
  "angle": "front",
  "note": "自然站姿",
  "sort_order": 10
}
```

服务端行为：

- 校验用户已登录。
- 确保当前用户存在 active profile，不存在则创建。
- 校验资产存在且归属当前用户。
- 校验 `photo_type` 和 `angle` 在允许范围内。
- 校验当前分组和总数上限。
- 创建 `profile_photos` 记录。
- 返回创建后的照片对象，包含短期签名图片 URL。

### 更新档案照片

新增接口：

`PATCH /api/user/profile/photos/:public_id`

允许更新：

- `photo_type`
- `angle`
- `note`
- `sort_order`
- `status`

不允许通过该接口替换 `asset_public_id`；替换照片应删除旧引用并创建新引用。

### 删除档案照片

新增接口：

`DELETE /api/user/profile/photos/:public_id`

行为：

- 软删除 `profile_photos.deleted_at`。
- 不物理删除对象存储文件。
- 返回删除成功状态。

## 上传资产类型

复用现有文件上传能力，新增或开放资产类型：

- `profile_photo`

对象路径建议：

```text
users/{user_id}/profile/{asset_public_id}.{ext}
```

小程序流程：

1. 选择图片。
2. 以 `asset_type = profile_photo` 申请上传凭证。
3. 直传对象存储。
4. 调用文件确认接口。
5. 调用 `POST /api/user/profile/photos` 写入档案照片引用。

## 小程序设计

### 页面结构

`pages/profile/edit` 改为三个分区：

1. 基础信息。
2. 照片档案。
3. 形象要素。

基础信息字段：

- 昵称。
- 性别表达。
- 身高。
- 体重，可留空。
- 常见场景。

照片档案：

- 自拍/头肩照。
- 半身照。
- 全身照。

每个分组内展示多张照片上传格。上传时用户需要选择角度，默认可按分组给出建议角度。上传失败保留已填写表单，不创建照片引用。

形象要素字段：

- 身形/比例备注。
- 肤色/用色备注。
- 发型/发量备注。
- 脸型。
- 肩颈/上半身特点。
- 腿型/下半身特点。
- 尺码备注。

### 交互状态

- 加载中：显示读取档案状态。
- 保存中：禁用保存按钮，防止重复提交。
- 上传中：单个照片格显示 loading，不阻塞其他表单编辑。
- 上传成功：立即写入照片引用并更新分组。
- 上传失败：显示错误，可重试。
- 删除照片：二次确认后删除档案引用。

## 隐私与删除

档案照片属于敏感个人数据。

- 档案页删除照片只删除档案引用。
- 隐私页后续需要提供删除照片资产的入口。
- 删除档案字段时，应同步删除或更新对应用户明确事实记忆。
- 不得未经授权公开复用用户上传照片。

## 测试与验证

后端测试：

- `PATCH /profile` 支持新增字段，体重可选且范围校验有效。
- 文本字段超长时返回校验错误。
- `POST /profile/photos` 校验资产归属、类型、角度和数量上限。
- `PATCH /profile/photos/:public_id` 只允许更新元信息。
- `DELETE /profile/photos/:public_id` 软删除且 summary 不再返回。
- `GET /profile/summary` 返回新增字段和按分组排序后的照片。

小程序验证：

- 档案编辑页可以回显新增字段。
- 体重可留空、可保存、可清空。
- 照片按自拍/头肩照、半身照、全身照分组展示。
- 上传成功后显示缩略图并保留角度。
- 删除照片后分组更新。
- 保存失败时用户输入不丢失。

## 实施顺序

1. 新增数据库迁移：扩展 `profiles`，新增 `profile_photos`。
2. 扩展后端 `profile` 模型、校验、仓储和 summary。
3. 增加档案照片 CRUD 接口与测试。
4. 开放 `profile_photo` 资产类型。
5. 扩展小程序 API 封装。
6. 改造 `pages/profile/edit`。
7. 增加验证脚本和相关测试。

