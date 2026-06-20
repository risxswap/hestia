# Hestia 数据库逻辑设计

日期：2026-06-20

## 背景

本文档定义 Hestia 第一版服务端从零实现时使用的 MySQL 逻辑表设计。当前服务端尚未实现业务数据层，因此本文不讨论历史迁移，也不需要兼容旧表结构。

本设计基于已确认的产品架构：

```text
形象路线中枢 + 领域能力挂载
```

数据库需要支撑以下核心闭环：

```text
轻量 onboarding
→ 初版形象路线报告
→ 今日完整建议
→ 用户反馈
→ 记忆更新
→ 下一次建议变准
```

第一版主数据对象是 `image_routes`。用户画像、衣橱、风格库、报告、推荐、反馈和记忆都围绕个人形象路线协同工作。

## 总体策略

### 数据库与 ID

第一版使用 MySQL。

核心表使用：

- `id BIGINT`：内部主键。
- `public_id`：外部暴露 ID。

原则：

- 表之间内部关联优先使用 `*_id BIGINT`。
- 前端、分享链接、外部 API 暴露 `public_id`。
- `public_id` 可使用 ULID、NanoID 或类似短 ID。
- 分享 token、上传 token、会话 token 使用安全随机值，不使用自增 ID。

通用字段含义：

- `id`：数据库内部主键。
- `public_id`：对前端、分享链接或外部 API 暴露的稳定 ID。
- `created_at`：记录创建时间。
- `updated_at`：记录最后更新时间。
- `deleted_at`：软删除时间；为空表示未删除。
- `status`：当前业务状态，不同表的状态枚举不同。
- `source`：数据来源，例如 `onboarding`、`chat`、`feedback`、`system`、`admin`。
- `user_id`：终端用户内部 ID。
- `profile_id`：用户当前画像内部 ID。
- `target_type`：被操作、关联或反馈的对象类型。
- `target_id`：被操作、关联或反馈对象的内部 ID。
- `target_public_id`：被操作、关联或反馈对象的外部 ID 快照。
- `sort_order`：展示或处理排序，数值越小越靠前。
- `created_by_admin_id`：创建记录的管理员 ID。
- `updated_by_admin_id`：最近更新记录的管理员 ID。

### 建模策略

数据库采用：

```text
结构化核心实体 + JSON 快照
```

结构化存储：

- 账号。
- 用户画像。
- 资产与核心衣橱。
- 风格库。
- 形象路线。
- 报告。
- 推荐请求与推荐结果。
- 反馈。
- 长期记忆。
- 系统配置。
- AI 调用和任务。

JSON 快照存储：

- 报告正文。
- 推荐正文。
- 推荐生成上下文。
- 报告生成上下文。
- AI 输入输出摘要。
- 形象路线策略字段。
- 风格库引用摘要。

这样可以兼顾查询、隐私删除、结构演进和 AI 输出灵活性。

### 外键策略

第一版不依赖复杂数据库外键约束。

原则：

- 表之间保留清晰的 `*_id BIGINT` 关联字段。
- MySQL 负责持久化、唯一约束、必要索引和事务。
- 跨表存在性校验、业务级联、删除前检查由应用层代码保障。
- 关键写操作必须通过 application/domain 统一入口完成。
- 需要跨表一致性的操作使用数据库事务。
- 删除和软删除由应用层显式编排，不依赖数据库级联删除。

不做复杂外键的原因：

- 第一版业务变化快，复杂外键会增加数据修复和模型调整成本。
- 软删除、隐私删除、任务审计和 AI 快照需要更细的应用层控制。
- 管理端和用户侧服务端分离后，应用层领域规则比数据库级联更容易表达业务边界。

### 会话策略

用户会话不存 MySQL。

- 用户 session 存 Redis。
- 管理端 session 也优先存 Redis。
- MySQL 只存用户账号、管理员账号、任务记录和 AI 调用摘要。

### 表命名策略

表名优先短，但核心中枢对象必须保留产品语义。

- 使用 `image_routes`，因为这里的 `image` 表示个人形象，不是图片资源。
- 图片、照片和文件统一使用 `assets`。
- 推荐请求和推荐结果使用短名：`rec_requests`、`recs`。

## 表域总览

第一版表域：

```text
账号与管理
users
admin_users

画像
profiles
profile_attrs

资产与衣橱
assets
asset_links
wardrobe_items
wardrobe_item_assets
wardrobe_gaps

风格库
style_subjects
style_samples
style_tags
style_sample_tags
style_sample_assets

形象路线
image_routes
image_route_events
image_route_styles

报告
reports
report_image_routes

推荐
rec_requests
recs

反馈与记忆
feedbacks
memories
memory_sources

系统配置、AI 与任务
system_configs
ai_calls
jobs

分享与指标
share_tokens
metrics_events
```

第一版明确不建：

- `user_sessions`：会话放 Redis。
- `user_auth_bindings`：第一版微信身份和可选手机号直接放 `users`。
- `admin_roles`：第一版管理权限保持极简。
- `image_route_versions`：路线历史先由 `image_route_events` 解释。
- `memory_events`：记忆当前态 + 来源足够。
- `rec_options`：第一版一条请求默认最多一条推荐结果，替代方案放 JSON。
- `report_versions`：报告生成后作为快照；更新时新增一条 `reports`。
- `ai_config_versions`：系统配置第一版用 `system_configs`。
- `operate_logs`：前期不建，减少首版后台复杂度。
- `user_delete_requests`：第一版删除由应用层直接编排，不做请求流转表。

## 账号与管理

### users

用途：终端用户主账号。

关键字段：

- `id`
- `public_id`
- `wechat_openid`
- `wechat_unionid`
- `phone`
- `nickname`
- `avatar_url`
- `onboarding_status`
- `status`
- `last_active_at`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `wechat_openid`：微信小程序用户在当前小程序下的 openid。
- `wechat_unionid`：微信开放平台 unionid，可为空。
- `phone`：用户手机号，可为空。
- `nickname`：用户昵称。
- `avatar_url`：用户头像 URL，和业务资产分开处理。
- `onboarding_status`：onboarding 进度状态。
- `last_active_at`：用户最近活跃时间。

索引建议：

- `uk_users_public_id`
- `uk_users_wechat_openid`
- `uk_users_phone`
- `idx_users_status`
- `idx_users_last_active_at`

说明：

- 小程序微信登录直接落到 `users`。
- `phone` 允许为空；如果启用手机号登录，需要唯一索引。
- 用户头像和业务资产分开处理，头像 URL 不进入 `assets`。

### admin_users

用途：管理端账号。

关键字段：

- `id`
- `public_id`
- `username`
- `password_hash`
- `display_name`
- `is_super_admin`
- `status`
- `last_login_at`
- `created_at`
- `updated_at`

字段说明：

- `username`：管理端登录用户名。
- `password_hash`：管理端密码哈希，不保存明文。
- `display_name`：管理端显示名称。
- `is_super_admin`：是否为超级管理员。
- `last_login_at`：最近登录时间。

索引建议：

- `uk_admin_users_public_id`
- `uk_admin_users_username`
- `idx_admin_users_status`

说明：

- 第一版不建 `admin_roles`。
- 用 `is_super_admin` 和 `status` 控制最小管理能力。

## 画像

### profiles

用途：用户当前画像摘要，一人一份当前画像。

关键字段：

- `id`
- `public_id`
- `user_id`
- `status`
- `gender`
- `height_cm`
- `body_notes`
- `skin_notes`
- `hair_notes`
- `lifestyle_scenarios`
- `style_goal_summary`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `gender`：用户性别或表达倾向，可为空。
- `height_cm`：用户身高，单位厘米。
- `body_notes`：用户或系统记录的身形相关中性描述。
- `skin_notes`：用户或系统记录的肤色、明度、冷暖倾向等中性描述。
- `hair_notes`：发量、发质、长度、日常打理等摘要。
- `lifestyle_scenarios`：常见生活、职业和社交场景 JSON。
- `style_goal_summary`：用户想呈现的整体形象目标摘要。

索引建议：

- `uk_profiles_public_id`
- `uk_profiles_user_id`
- `idx_profiles_status`

JSON 字段：

- `lifestyle_scenarios`

说明：

- `profiles` 只存稳定摘要，不再保存画像摘要冗余字段。
- 用户事实、AI 推断、明确偏好和禁忌统一放入 `profile_attrs`，通过 `attr_type` 区分。

### profile_attrs

用途：统一存储用户画像属性，包括用户明确事实、AI 推断、用户偏好和禁忌。

关键字段：

- `id`
- `user_id`
- `profile_id`
- `attr_type`
- `attr_key`
- `attr_value`
- `polarity`
- `confidence`
- `source`
- `source_asset_id`
- `source_job_id`
- `status`
- `confirmed_by_user`
- `confirmed_at`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `attr_type`：属性类型，可为 `fact`、`inference`、`pref`。
- `attr_key`：属性键名，例如 `occupation`、`hair_texture`、`face_shape_tendency`、`low_heels`。
- `attr_value`：属性值 JSON。
- `polarity`：偏好方向，仅偏好类常用，可为 `like`、`dislike`、`prefer`、`avoid`、`neutral`。
- `confidence`：置信度；用户确认事实通常最高，AI 推断需带不确定性。
- `source_asset_id`：支撑该属性的图片资源 ID，可为空。
- `source_job_id`：产生该属性的任务 ID，可为空。
- `confirmed_by_user`：用户是否确认过该推断。
- `confirmed_at`：用户确认该属性的时间。

索引建议：

- `idx_profile_attrs_user_type_key`
- `idx_profile_attrs_profile`
- `idx_profile_attrs_status`
- `idx_profile_attrs_polarity`
- `idx_profile_attrs_source_asset`
- `idx_profile_attrs_source_job`

JSON 字段：

- `attr_value`

说明：

- `attr_type=fact` 存用户明确说过或确认过的事实，例如职业、发质自述、常见场景。
- `attr_type=inference` 存脸型倾向、肤色倾向、体型比例线索、发量发质等 AI 推断。
- `attr_type=pref` 存用户明确偏好和禁忌，例如颜色、版型、成熟度、露肤度、场景偏好。
- `status` 可为 `active`、`rejected`、`superseded`。
- 用户否定 AI 推断或偏好后标记 `rejected`，避免反复使用同样结论。
- `profile_attrs` 是画像层属性；`memories` 是顾问从反馈中沉淀出的长期记忆和策略规则。

## 资产与衣橱

### assets

用途：统一存储照片和文件资源元数据。

关键字段：

- `id`
- `public_id`
- `owner_user_id`
- `bucket`
- `object_key`
- `mime_type`
- `file_size`
- `width`
- `height`
- `asset_type`
- `source`
- `status`
- `review_status`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `owner_user_id`：资源所属用户；系统或管理端资源可为空。
- `bucket`：对象存储 bucket。
- `object_key`：对象存储 key，不是完整访问 URL。
- `mime_type`：文件 MIME 类型。
- `file_size`：文件大小，单位字节。
- `width`：图片宽度，非图片可为空。
- `height`：图片高度，非图片可为空。
- `asset_type`：资源类型，例如自拍、半身照、衣橱单品图、参考图。
- `review_status`：内容审核状态。

索引建议：

- `uk_assets_public_id`
- `uk_assets_bucket_object_key`
- `idx_assets_owner_type`
- `idx_assets_status`

说明：

- `asset_type` 可为 `selfie`、`body_photo`、`wardrobe_item`、`reference_image`、`style_sample`。
- 用户资产用 `owner_user_id`，管理端风格样本图可以为空或用系统归属。
- 删除资产时先软删，再创建资源清理任务。
- 业务表不要直接存对象存储完整 URL。

### asset_links

用途：通用资源关联表。

关键字段：

- `id`
- `asset_id`
- `target_type`
- `target_id`
- `usage_type`
- `sort_order`
- `created_at`

字段说明：

- `asset_id`：被关联的资源 ID。
- `usage_type`：资源在目标对象上的用途，例如 `primary`、`reference`、`analysis_source`。

索引建议：

- `idx_asset_links_asset`
- `idx_asset_links_target`

说明：

- 适合自拍关联画像、参考图关联风格解析等通用关系。
- 强关系可以用专表，例如 `wardrobe_item_assets`、`style_sample_assets`。

### wardrobe_items

用途：用户核心衣橱单品。

关键字段：

- `id`
- `public_id`
- `user_id`
- `name`
- `category`
- `color`
- `silhouette`
- `material`
- `thickness`
- `pattern`
- `season`
- `formality`
- `scene_tags`
- `ai_attrs`
- `user_notes`
- `is_core`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `name`：用户或系统给单品取的名称。
- `category`：单品类别，例如外套、上衣、下装、鞋、包。
- `color`：主色或颜色描述。
- `silhouette`：廓形，例如直筒、修身、宽松、短款。
- `material`：材质描述。
- `thickness`：厚薄程度。
- `pattern`：图案，例如纯色、条纹、格纹。
- `season`：适合季节。
- `formality`：正式度。
- `scene_tags`：适合场景标签 JSON。
- `ai_attrs`：AI 识别出的单品属性 JSON。
- `user_notes`：用户补充说明。
- `is_core`：是否为核心高频单品。

索引建议：

- `uk_wardrobe_items_public_id`
- `idx_wardrobe_items_user_category`
- `idx_wardrobe_items_user_status`

JSON 字段：

- `scene_tags`
- `ai_attrs`

说明：

- 第一版只管理高频核心单品，不做完整库存。
- AI 识别结果和用户修正先合并在该表字段中。

### wardrobe_item_assets

用途：衣橱单品和图片的明确关联。

关键字段：

- `id`
- `wardrobe_item_id`
- `asset_id`
- `is_primary`
- `sort_order`
- `created_at`

字段说明：

- `wardrobe_item_id`：衣橱单品 ID。
- `asset_id`：单品图片资源 ID。
- `is_primary`：是否为该单品主图。

索引建议：

- `idx_wardrobe_item_assets_item`
- `idx_wardrobe_item_assets_asset`

说明：

- 一个单品可有多张图。
- 主图用 `is_primary` 标记。

### wardrobe_gaps

用途：记录系统识别出的衣橱缺口。

关键字段：

- `id`
- `public_id`
- `user_id`
- `gap_type`
- `title`
- `description`
- `reason`
- `priority`
- `source_report_id`
- `source_rec_id`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `gap_type`：缺口类型，例如 `item`、`color`、`shoe`、`outerwear`。
- `title`：缺口标题。
- `description`：缺口描述。
- `reason`：为什么需要这个缺口单品。
- `priority`：优先级。
- `source_report_id`：产生该缺口的报告 ID。
- `source_rec_id`：产生该缺口的推荐 ID。

索引建议：

- `uk_wardrobe_gaps_public_id`
- `idx_wardrobe_gaps_user_status`
- `idx_wardrobe_gaps_source_report`
- `idx_wardrobe_gaps_source_rec`

说明：

- 只记录品类和特征，不记录商品链接。
- `status` 可为 `active`、`dismissed`、`resolved`。

## 风格库

### style_subjects

用途：明星、博主、内部参考对象或用户参考对象主表。

关键字段：

- `id`
- `public_id`
- `name`
- `subject_type`
- `gender`
- `description`
- `status`
- `created_by_admin_id`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `name`：参考对象名称，例如明星、博主、内部风格对象名称。
- `subject_type`：参考对象类型，例如 `celebrity`、`blogger`、`internal`、`user_reference`。
- `gender`：参考对象性别或表达倾向，可为空。
- `description`：参考对象说明。

索引建议：

- `uk_style_subjects_public_id`
- `idx_style_subjects_type_status`
- `idx_style_subjects_name`

说明：

- `subject_type` 可为 `celebrity`、`blogger`、`internal`、`user_reference`。
- 第一版不自动抓取公开图片。
- 这里是参考对象，不是相似明星。

### style_samples

用途：参考对象下的一条结构化风格样本。

关键字段：

- `id`
- `public_id`
- `subject_id`
- `name`
- `style_keywords`
- `suitable_scenes`
- `clothing_structure`
- `color_logic`
- `hair_makeup_points`
- `transferable_elements`
- `non_transferable_risks`
- `suitable_profile_notes`
- `status`
- `published_at`
- `created_by_admin_id`
- `updated_by_admin_id`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `subject_id`：所属参考对象 ID。
- `name`：风格样本名称。
- `style_keywords`：风格关键词 JSON。
- `suitable_scenes`：适合场景 JSON。
- `clothing_structure`：服装结构和单品组合逻辑。
- `color_logic`：色彩逻辑。
- `hair_makeup_points`：发型和妆容要点。
- `transferable_elements`：可迁移元素 JSON，例如比例、廓形、色彩、发型方向。
- `non_transferable_risks`：不可迁移风险 JSON。
- `suitable_profile_notes`：适合哪些画像特征的说明。
- `published_at`：发布时间。

索引建议：

- `uk_style_samples_public_id`
- `idx_style_samples_subject_status`
- `idx_style_samples_status`

JSON 字段：

- `style_keywords`
- `suitable_scenes`
- `clothing_structure`
- `transferable_elements`
- `non_transferable_risks`

说明：

- `status` 可为 `draft`、`pending_publish`、`published`、`offline`。
- 发布前必须有可迁移元素和不可迁移风险。
- 不存“用户像某明星”字段。

### style_tags

用途：风格标签字典。

关键字段：

- `id`
- `public_id`
- `name`
- `tag_type`
- `description`
- `status`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `name`：标签名称。
- `tag_type`：标签类型，例如 `style`、`scene`、`color`、`silhouette`、`hair`、`makeup`。
- `description`：标签说明。

索引建议：

- `uk_style_tags_public_id`
- `uk_style_tags_type_name`
- `idx_style_tags_status`

说明：

- `tag_type` 可为 `style`、`scene`、`color`、`silhouette`、`hair`、`makeup`。

### style_sample_tags

用途：风格样本和标签的多对多关系。

关键字段：

- `id`
- `style_sample_id`
- `style_tag_id`
- `created_at`

字段说明：

- `style_sample_id`：风格样本 ID。
- `style_tag_id`：风格标签 ID。

索引建议：

- `uk_style_sample_tags_sample_tag`
- `idx_style_sample_tags_tag`

### style_sample_assets

用途：风格样本和图片资源关联。

关键字段：

- `id`
- `style_sample_id`
- `asset_id`
- `usage_type`
- `sort_order`
- `created_at`

字段说明：

- `style_sample_id`：风格样本 ID。
- `asset_id`：样本图片资源 ID。
- `usage_type`：图片用途，例如 `cover`、`detail`、`reference`。

索引建议：

- `idx_style_sample_assets_sample`
- `idx_style_sample_assets_asset`

说明：

- 可关联管理端上传的样本图或参考图。
- 若图片版权或授权边界不清，前端不应直接展示给用户端。

## 形象路线

### image_routes

用途：当前可用于推荐的个人形象路线，是第一版的业务中枢对象。

关键字段：

- `id`
- `public_id`
- `user_id`
- `profile_id`
- `name`
- `route_role`
- `status`
- `weight`
- `target_impression`
- `suitable_scenes`
- `hair_strategy`
- `makeup_strategy`
- `outfit_strategy`
- `avoid_points`
- `reason`
- `source`
- `created_from_report_id`
- `created_from_job_id`
- `activated_at`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `name`：形象路线名称，例如利落亲和、轻熟松弛。
- `route_role`：路线角色，可为主路线、场景路线、探索路线。
- `weight`：路线推荐权重，用于排序和选择。
- `target_impression`：目标形象感受 JSON。
- `suitable_scenes`：适合场景 JSON。
- `hair_strategy`：发型策略 JSON。
- `makeup_strategy`：妆容或气色策略 JSON。
- `outfit_strategy`：穿搭策略 JSON。
- `avoid_points`：避雷项 JSON。
- `reason`：生成或保留该路线的原因 JSON。
- `created_from_report_id`：生成该路线的报告 ID。
- `created_from_job_id`：生成该路线的任务 ID。
- `activated_at`：路线被用户确认并激活的时间。

索引建议：

- `uk_image_routes_public_id`
- `idx_image_routes_user_status`
- `idx_image_routes_user_role`
- `idx_image_routes_profile`

JSON 字段：

- `target_impression`
- `suitable_scenes`
- `hair_strategy`
- `makeup_strategy`
- `outfit_strategy`
- `avoid_points`
- `reason`

说明：

- `status` 可为 `candidate`、`active`、`refinement`、`paused`、`archived`。
- `route_role` 可为 `primary`、`scene`、`explore`。
- 每个用户 active/refinement/candidate 路线总数第一版最多 3 条，由应用层控制。

### image_route_events

用途：记录形象路线生命周期事件。

关键字段：

- `id`
- `image_route_id`
- `user_id`
- `event_type`
- `event_value`
- `source`
- `source_feedback_id`
- `source_rec_id`
- `source_report_id`
- `created_by`
- `created_at`

字段说明：

- `image_route_id`：形象路线 ID。
- `event_type`：路线事件类型，例如 `generated`、`liked`、`adjust_requested`。
- `event_value`：事件详情 JSON。
- `source_feedback_id`：触发事件的反馈 ID。
- `source_rec_id`：触发事件的推荐 ID。
- `source_report_id`：触发事件的报告 ID。
- `created_by`：事件创建方，例如 `user`、`system`、`admin`。

索引建议：

- `idx_image_route_events_route`
- `idx_image_route_events_user_created`
- `idx_image_route_events_type`

JSON 字段：

- `event_value`

说明：

- `event_type` 可为 `generated`、`liked`、`disliked`、`adjust_requested`、`activated`、`paused`、`archived`、`weight_changed`、`strategy_updated`。
- 用于解释路线从哪里来、为什么变化。
- 第一版不建 `image_route_versions`。

### image_route_styles

用途：记录形象路线引用了哪些风格样本。

关键字段：

- `id`
- `image_route_id`
- `style_sample_id`
- `reason`
- `transfer_snapshot`
- `risk_snapshot`
- `weight`
- `created_at`

字段说明：

- `image_route_id`：形象路线 ID。
- `style_sample_id`：被引用的风格样本 ID。
- `reason`：引用该样本的原因。
- `transfer_snapshot`：引用时可迁移元素快照 JSON。
- `risk_snapshot`：引用时不可迁移风险快照 JSON。
- `weight`：该风格样本对路线的参考权重。

索引建议：

- `idx_image_route_styles_route`
- `idx_image_route_styles_sample`

JSON 字段：

- `transfer_snapshot`
- `risk_snapshot`

说明：

- 保存引用理由、可迁移元素快照、不可迁移风险快照和权重。
- 不表达“用户像某明星”。

## 报告

### reports

用途：初版路线报告或阶段性复盘报告。

关键字段：

- `id`
- `public_id`
- `user_id`
- `profile_id`
- `report_type`
- `status`
- `title`
- `summary`
- `content_json`
- `context_snapshot`
- `style_refs_json`
- `ai_call_id`
- `job_id`
- `generated_at`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `report_type`：报告类型，例如初版路线报告、阶段复盘。
- `title`：报告标题。
- `summary`：报告摘要。
- `content_json`：报告正文 JSON。
- `context_snapshot`：生成报告时使用的画像、路线、衣橱、记忆等上下文快照。
- `style_refs_json`：报告引用的风格样本摘要 JSON。
- `ai_call_id`：生成报告的 AI 调用 ID。
- `job_id`：生成报告的任务 ID。
- `generated_at`：报告生成完成时间。

索引建议：

- `uk_reports_public_id`
- `idx_reports_user_status`
- `idx_reports_profile`
- `idx_reports_generated`

JSON 字段：

- `content_json`
- `context_snapshot`
- `style_refs_json`

说明：

- `report_type` 可为 `initial_route_report`、`stage_review`。
- `status` 可为 `generating`、`ready`、`failed`、`expired`、`hidden`。
- 报告生成后作为不可编辑快照。若需要更新，新增一条 `reports`。
- 管理端最多修改状态或隐藏，不直接编辑报告正文。

### report_image_routes

用途：记录某份报告引用了哪些形象路线，并保存当时路线快照。

关键字段：

- `id`
- `report_id`
- `image_route_id`
- `route_role`
- `sort_order`
- `route_snapshot`
- `created_at`

字段说明：

- `report_id`：报告 ID。
- `image_route_id`：报告引用的形象路线 ID。
- `route_role`：该路线在报告中的角色。
- `route_snapshot`：报告生成时的路线快照 JSON。

索引建议：

- `idx_report_image_routes_report`
- `idx_report_image_routes_route`

JSON 字段：

- `route_snapshot`

说明：

- 历史报告不受当前路线后续变化影响。
- 可记录报告中路线的主路线、场景路线、探索路线角色。

## 推荐

### rec_requests

用途：用户或系统提出的一次建议需求。

关键字段：

- `id`
- `public_id`
- `user_id`
- `source`
- `status`
- `input_text`
- `input_assets`
- `scenario`
- `trigger_context`
- `parent_request_id`
- `parent_rec_id`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `source`：请求来源，例如今日页自动生成、聊天文本、拍照问搭配。
- `input_text`：用户输入原文或系统触发说明。
- `input_assets`：输入图片或文件资源引用 JSON。
- `scenario`：结构化场景 JSON，例如天气、日期、正式度、地点类型。
- `trigger_context`：触发上下文 JSON，例如点击“更正式一点”。
- `parent_request_id`：上一条相关请求 ID。
- `parent_rec_id`：基于哪条推荐继续调整。

索引建议：

- `uk_rec_requests_public_id`
- `idx_rec_requests_user_status`
- `idx_rec_requests_user_created`
- `idx_rec_requests_parent_rec`

JSON 字段：

- `input_assets`
- `scenario`
- `trigger_context`

说明：

- `source` 可为 `today_auto`、`today_scene_change`、`today_adjustment`、`chat_text`、`chat_photo`、`report_action`、`onboarding_initial`。
- `status` 可为 `pending`、`processing`、`completed`、`failed`、`cancelled`。
- 第一版一条请求默认最多生成一条 `recs`。

### recs

用途：系统针对某次请求生成的一套完整建议。

关键字段：

- `id`
- `public_id`
- `request_id`
- `user_id`
- `adopted_route_id`
- `report_id`
- `status`
- `rec_date`
- `scene_key`
- `title`
- `summary`
- `outfit_advice`
- `hair_advice`
- `makeup_advice`
- `avoid_notes`
- `alternatives`
- `wardrobe_item_refs`
- `wardrobe_gap_refs`
- `context_snapshot`
- `ai_call_id`
- `job_id`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `request_id`：对应的推荐请求 ID。
- `adopted_route_id`：本次推荐采用的形象路线 ID。
- `report_id`：本次推荐关联的报告 ID，可为空。
- `rec_date`：推荐归属日期，用于今日页查询。
- `scene_key`：主场景键名，例如 `commute`、`date`、`client_meeting`。
- `title`：推荐标题。
- `summary`：推荐摘要。
- `outfit_advice`：穿搭主方案 JSON。
- `hair_advice`：发型建议 JSON。
- `makeup_advice`：妆容或气色建议 JSON。
- `avoid_notes`：今天不优先或需要避开的内容 JSON。
- `alternatives`：可替换方案 JSON。
- `wardrobe_item_refs`：本次推荐使用的衣橱单品引用 JSON。
- `wardrobe_gap_refs`：本次推荐提到的衣橱缺口引用 JSON。
- `context_snapshot`：生成推荐时使用的路线、画像、衣橱、记忆上下文快照。
- `ai_call_id`：生成推荐的 AI 调用 ID。
- `job_id`：生成推荐的任务 ID。

索引建议：

- `uk_recs_public_id`
- `idx_recs_request`
- `idx_recs_user_status`
- `idx_recs_user_date_scene`
- `idx_recs_adopted_route`
- `idx_recs_report`

JSON 字段：

- `outfit_advice`
- `hair_advice`
- `makeup_advice`
- `avoid_notes`
- `alternatives`
- `wardrobe_item_refs`
- `wardrobe_gap_refs`
- `context_snapshot`

说明：

- `status` 可为 `generating`、`ready`、`failed`、`expired`、`hidden`。
- 今日页查询同一 `user_id + rec_date + scene_key` 下最新 ready 推荐。
- 用户换场景、点击调整、聊天追问或拍照问搭配时，新建 request 和 rec。
- 第一版不建 `rec_options`；替代方案放 `alternatives`。

## 反馈与记忆

### feedbacks

用途：记录用户对建议、路线、报告、单品、记忆等对象的一次反馈。

关键字段：

- `id`
- `public_id`
- `user_id`
- `target_type`
- `target_id`
- `target_public_id`
- `feedback_type`
- `feedback_text`
- `feedback_value`
- `source`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `feedback_type`：反馈类型，例如采纳、拒绝、实际穿了感觉不错。
- `feedback_text`：用户自由文本反馈。
- `feedback_value`：结构化反馈 JSON。

索引建议：

- `uk_feedbacks_public_id`
- `idx_feedbacks_user_created`
- `idx_feedbacks_target`
- `idx_feedbacks_type`

JSON 字段：

- `feedback_value`

说明：

- `target_type` 可为 `image_route`、`rec`、`report`、`wardrobe_item`、`memory`、`profile_attr`。
- `feedback_type` 可为 `accepted`、`rejected`、`modified`、`worn_good`、`worn_bad`、`external_positive`、`external_negative`、`correction`。
- 真实穿着反馈优先级高于初始 AI 判断。

### memories

用途：当前可用于推荐的长期记忆，用户可见、可修正、可删除。

关键字段：

- `id`
- `public_id`
- `user_id`
- `memory_type`
- `memory_key`
- `memory_value`
- `polarity`
- `confidence`
- `visibility`
- `status`
- `last_reinforced_at`
- `user_corrected_at`
- `correction_note`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `memory_type`：记忆类型，可为事实、偏好、反馈、策略。
- `memory_key`：记忆键名，例如 `client_meeting_outerwear_preference`。
- `memory_value`：记忆内容 JSON。
- `polarity`：记忆方向，例如 `like`、`dislike`、`prefer`、`avoid`、`neutral`。
- `confidence`：记忆置信度。
- `visibility`：用户是否可见。
- `last_reinforced_at`：最近一次被反馈强化的时间。
- `user_corrected_at`：用户修正该记忆的时间。
- `correction_note`：用户修正说明。

索引建议：

- `uk_memories_public_id`
- `idx_memories_user_type_status`
- `idx_memories_user_status`

JSON 字段：

- `memory_value`

说明：

- `memory_type` 可为 `fact`、`preference`、`feedback`、`strategy`。
- `polarity` 可为 `like`、`dislike`、`prefer`、`avoid`、`neutral`。
- `visibility` 可为 `visible`、`hidden`。
- `status` 可为 `active`、`rejected`、`superseded`、`deleted`。
- 用户修正后直接更新当前值，并记录 `user_corrected_at`、`correction_note`。
- 第一版不建 `memory_events`。

### memory_sources

用途：记录记忆来源，用于解释“为什么记住”。

关键字段：

- `id`
- `memory_id`
- `source_type`
- `source_id`
- `source_public_id`
- `weight`
- `created_at`

字段说明：

- `memory_id`：长期记忆 ID。
- `source_type`：记忆来源类型，例如 `feedback`、`rec`、`profile_attr`。
- `source_id`：来源对象内部 ID。
- `source_public_id`：来源对象外部 ID 快照。
- `weight`：该来源对记忆的贡献权重。

索引建议：

- `idx_memory_sources_memory`
- `idx_memory_sources_source`

说明：

- `source_type` 可为 `feedback`、`rec`、`profile_attr`、`onboarding`、`chat`。
- 一个记忆可以来自多次反馈。
- 真实穿着反馈权重大于初始 onboarding。

## 系统配置、AI 与任务

### system_configs

用途：通用 K-V 系统配置。

关键字段：

- `id`
- `group`
- `key`
- `value`
- `value_type`
- `description`
- `status`
- `updated_by_admin_id`
- `created_at`
- `updated_at`

字段说明：

- `group`：配置分组，例如 `ai`、`upload`、`feature`、`onboarding`。
- `key`：分组内配置键名。
- `value`：配置值。
- `value_type`：配置值类型，例如 `string`、`number`、`bool`、`json`。
- `description`：配置说明。
- `updated_by_admin_id`：最近更新配置的管理员 ID。

索引建议：

- `uk_system_configs_group_key`
- `idx_system_configs_status`

说明：

- `group` 示例：`ai`、`upload`、`feature`、`onboarding`、`recommendation`。
- `key` 是组内配置名，例如 `default_model`、`max_image_size_mb`、`today_auto_enabled`。
- `value_type` 可为 `string`、`number`、`bool`、`json`。
- 复杂配置值存 JSON。
- `group` 和 `key` 是 SQL 关键字风险词；实际 DDL 需使用反引号，或实现时改为 `config_group`、`config_key`。逻辑设计按产品确认使用 `group`、`key`。
- 配置历史第一版不建版本表，后续如需审计再补专门的操作日志能力。

### ai_calls

用途：AI 调用审计和成本摘要。

关键字段：

- `id`
- `public_id`
- `user_id`
- `job_id`
- `provider`
- `model`
- `call_type`
- `input_summary`
- `output_summary`
- `token_usage`
- `cost_amount`
- `latency_ms`
- `status`
- `error_message`
- `created_at`

字段说明：

- `job_id`：关联任务 ID。
- `provider`：AI 服务提供方。
- `model`：模型名称。
- `call_type`：调用类型，例如 `extraction`、`route_generation`、`recommendation`、`memory_summary`。
- `input_summary`：输入摘要 JSON，不保存完整敏感原文。
- `output_summary`：输出摘要 JSON，不保存完整敏感原文。
- `token_usage`：token 用量 JSON。
- `cost_amount`：调用成本。
- `latency_ms`：调用耗时，单位毫秒。
- `error_message`：失败原因摘要。

索引建议：

- `uk_ai_calls_public_id`
- `idx_ai_calls_user_created`
- `idx_ai_calls_job`
- `idx_ai_calls_status`

JSON 字段：

- `input_summary`
- `output_summary`
- `token_usage`

说明：

- 不保存完整敏感原文和完整图片地址。
- 不普通软删，作为审计和排障记录。

### jobs

用途：异步任务持久化记录。

关键字段：

- `id`
- `public_id`
- `job_type`
- `status`
- `queue_name`
- `related_type`
- `related_id`
- `user_id`
- `input_summary`
- `output_summary`
- `error_message`
- `retry_count`
- `next_retry_at`
- `started_at`
- `finished_at`
- `created_at`
- `updated_at`

字段说明：

- `job_type`：任务类型，例如 `image_analysis`、`report_generation`、`rec_generation`、`memory_update`。
- `queue_name`：队列名称。
- `related_type`：任务关联对象类型。
- `related_id`：任务关联对象 ID。
- `input_summary`：任务输入摘要 JSON。
- `output_summary`：任务输出摘要 JSON。
- `error_message`：任务失败原因摘要。
- `retry_count`：已重试次数。
- `next_retry_at`：下次重试时间。
- `started_at`：任务开始时间。
- `finished_at`：任务结束时间。

索引建议：

- `uk_jobs_public_id`
- `idx_jobs_status_next_retry`
- `idx_jobs_type_status`
- `idx_jobs_related`
- `idx_jobs_user`

JSON 字段：

- `input_summary`
- `output_summary`

说明：

- Redis 负责队列和锁，MySQL 记录状态。
- 不普通软删，作为排障记录。
- 隐私删除后，摘要字段不能保留敏感原文。

## 分享与指标

### share_tokens

用途：报告或建议分享页的访问 token。

关键字段：

- `id`
- `public_id`
- `token`
- `owner_user_id`
- `target_type`
- `target_id`
- `target_public_id`
- `scope`
- `expires_at`
- `revoked_at`
- `created_at`
- `updated_at`
- `deleted_at`

字段说明：

- `token`：安全随机分享 token。
- `owner_user_id`：分享所有者用户 ID。
- `scope`：分享范围 JSON，例如可见模块、脱敏级别。
- `expires_at`：过期时间。
- `revoked_at`：撤销时间。

索引建议：

- `uk_share_tokens_public_id`
- `uk_share_tokens_token`
- `idx_share_tokens_owner`
- `idx_share_tokens_target`
- `idx_share_tokens_expires`

JSON 字段：

- `scope`

说明：

- `token` 必须是安全随机值。
- `target_type` 可为 `report`、`rec`。
- 第一版分享页只展示脱敏摘要。

### metrics_events

用途：基础埋点事件。

关键字段：

- `id`
- `user_id`
- `event_name`
- `event_source`
- `target_type`
- `target_id`
- `properties`
- `occurred_at`
- `created_at`

字段说明：

- `event_name`：事件名称。
- `event_source`：事件来源，例如 `miniapp`、`admin`、`server`。
- `properties`：事件属性 JSON，不写入敏感原文。
- `occurred_at`：事件发生时间。

索引建议：

- `idx_metrics_events_user_time`
- `idx_metrics_events_name_time`
- `idx_metrics_events_target`

JSON 字段：

- `properties`

说明：

- 第一版只存必要产品事件，不做复杂 BI。
- 高敏感内容不要写入 `properties`。
- 不普通软删；隐私删除后要脱敏可识别内容。

## 删除与隐私策略

### 软删除表

敏感和核心业务表使用 `deleted_at` 软删除：

- `users`
- `profiles`
- `profile_attrs`
- `assets`
- `wardrobe_items`
- `wardrobe_gaps`
- `style_subjects`
- `style_samples`
- `style_tags`
- `image_routes`
- `reports`
- `rec_requests`
- `recs`
- `feedbacks`
- `memories`
- `share_tokens`

### 关联表

关联表通常不暴露普通删除，但应用层删除用户数据时要一起处理：

- `asset_links`
- `wardrobe_item_assets`
- `style_sample_tags`
- `style_sample_assets`
- `image_route_events`
- `image_route_styles`
- `report_image_routes`
- `memory_sources`

### 日志与指标

以下表不普通软删，但隐私删除后必须脱敏摘要：

- `jobs`
- `ai_calls`
- `metrics_events`

### 资产删除流程

资产删除流程：

```text
业务软删 assets
→ 创建资源清理 job
→ 后台删除对象存储文件
→ 更新 assets.status
```

### 用户隐私删除

用户隐私删除由应用层编排，必须覆盖：

- 照片和参考图。
- 画像属性。
- 偏好、反馈和记忆。
- 推荐和报告快照中的敏感摘要。
- AI 调用和任务摘要中的敏感内容。

## 查询与索引原则

每张有 `public_id` 的核心表：

```text
unique(public_id)
```

高频用户查询：

```text
(user_id, status)
(user_id, created_at)
```

今日推荐查询：

```text
recs(user_id, rec_date, scene_key, status)
```

形象路线查询：

```text
image_routes(user_id, status)
image_routes(user_id, route_role)
```

反馈和记忆查询：

```text
feedbacks(user_id, created_at)
feedbacks(target_type, target_id)
memories(user_id, memory_type, status)
memory_sources(memory_id)
```

系统配置查询：

```text
system_configs(group, key) unique
```

风格库查询：

```text
style_subjects(subject_type, status)
style_samples(subject_id, status)
style_sample_tags(style_tag_id)
```

AI 和任务查询：

```text
jobs(status, next_retry_at)
jobs(related_type, related_id)
ai_calls(user_id, created_at)
ai_calls(job_id)
```

## 推荐生成与数据写入流程

### Onboarding 后生成路线和报告

```text
1. 写入 users / profiles / profile_attrs
2. 上传图片写入 assets，并建立必要 asset_links
3. AI 生成候选路线，写入 image_routes
4. 路线引用风格样本，写入 image_route_styles
5. 记录 image_route_events: generated
6. 生成初版报告，写入 reports
7. 报告引用路线，写入 report_image_routes
```

### 用户确认路线

```text
1. 用户点击喜欢 / 不喜欢 / 想调整
2. 写入 feedbacks，target_type=image_route
3. 更新 image_routes.status / weight / activated_at
4. 写入 image_route_events
5. 必要时写入 memories 或等待记忆任务归纳
```

### 生成今日建议

```text
1. 创建 rec_requests，记录来源、场景和输入
2. 创建 jobs 处理推荐生成
3. 调用 AI，写入 ai_calls
4. 生成 recs，关联 adopted_route_id
5. 更新 rec_requests.status
```

今日页同一用户、同一日期、同一场景优先复用最新 ready `recs`。用户换场景、点击调整、聊天追问或拍照问搭配时，新建 `rec_requests` 和 `recs`。

### 用户反馈与记忆更新

```text
1. 用户反馈写入 feedbacks
2. 记忆任务读取 feedbacks 和上下文
3. 新增或更新 memories
4. 写入 memory_sources
5. 必要时更新 image_routes.weight 或 profile_attrs
```

真实穿着反馈优先级高于初始 AI 推断和参考风格匹配。

## MVP 验收标准

数据库逻辑设计满足第一版开发条件，当且仅当：

- 核心实体使用 BIGINT 内部 ID 和 `public_id`。
- 表之间使用 `*_id` 关联字段，但不依赖复杂数据库外键。
- 跨表一致性由应用层代码和事务保障。
- 用户 session 不进入 MySQL。
- 账号域不包含 `user_auth_bindings` 和 `admin_roles`。
- 敏感和核心业务表支持软删除。
- 任务、AI 调用用于审计和排障，不普通软删。
- 用户画像属性通过 profile_attrs 区分事实、AI 推断和明确偏好。
- 资产统一由 `assets` 管理对象存储元数据。
- 衣橱只建核心单品和缺口，不做完整库存。
- 风格库支持可迁移元素和不可迁移风险。
- `image_routes` 是形象路线中枢对象。
- 报告作为不可编辑快照，更新时新增 `reports`。
- 推荐请求和推荐结果分为 `rec_requests` 与 `recs`。
- 反馈和记忆支持来源追溯。
- 用户可查看、修正、删除长期记忆。
- 分享 token 只暴露脱敏范围。
- 系统配置通过 `system_configs` K-V 管理。
