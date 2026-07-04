# 私藏独立架构设计

## 背景

`私藏` 是用户保存形象相关资料的总览入口，但它不应复用 `wardrobe` 页面和接口做包装。衣服、发型、妆容、参考是平级业务能力：衣服继续使用已有 `wardrobe` 领域，发型、妆容、参考后续各自拥有独立领域和接口。

本设计将 `私藏` 拆成独立聚合层：小程序 tab 进入 `collection` 首页，服务端提供 `/api/user/collection` 聚合接口；各类型详情页跳转到自己的独立页面和服务端领域。

## 目标

- `私藏` tab 使用独立小程序页面，不再指向 `pages/wardrobe/wardrobe`。
- 服务端新增 `collection` 聚合领域，提供 `/api/user/collection`。
- 衣服继续使用已有 `wardrobe` 领域和 `/api/user/wardrobe/*` 接口。
- 发型、妆容、参考分别作为一级领域预留：
  - `/api/user/hair/*`
  - `/api/user/makeup/*`
  - `/api/user/references/*`
- 私藏首页展示四类入口：衣服、发型、妆容、参考。
- 私藏首页展示最近收录，但第一阶段可以只聚合衣服最近项。

## 非目标

- 不把衣服迁移到 `clothes` 领域。
- 不新增 `/api/user/collection/clothes`、`/api/user/collection/hair` 等子路由。
- 不在本轮完整实现发型、妆容、参考的数据落库和编辑闭环。
- 不删除现有 `wardrobe` 接口。
- 不做电商导购或商品链接。

## 服务端设计

### collection 聚合接口

新增领域：

`server/internal/domain/collection`

注册路由：

`GET /api/user/collection`

响应结构：

```json
{
  "types": [
    {
      "type": "wardrobe",
      "label": "衣服",
      "count": 12,
      "hint": "常穿单品",
      "enabled": true,
      "entry_path": "/pages/wardrobe/wardrobe"
    },
    {
      "type": "hair",
      "label": "发型",
      "count": 0,
      "hint": "常用发型",
      "enabled": true,
      "entry_path": "/pages/hair/hair"
    },
    {
      "type": "makeup",
      "label": "妆容",
      "count": 0,
      "hint": "妆容方向",
      "enabled": true,
      "entry_path": "/pages/makeup/makeup"
    },
    {
      "type": "references",
      "label": "参考",
      "count": 0,
      "hint": "参考图",
      "enabled": true,
      "entry_path": "/pages/references/references"
    }
  ],
  "recent_items": [
    {
      "type": "wardrobe",
      "public_id": "wdi_xxx",
      "title": "米白衬衫",
      "subtitle": "上装 · 米白",
      "image": {
        "preview_url": "https://example.com/image.webp"
      },
      "entry_path": "/pages/wardrobe-detail/wardrobe-detail?public_id=wdi_xxx"
    }
  ]
}
```

第一阶段 `collection` 可以通过已有 `wardrobe.Service` 读取衣服列表，并派生衣服数量和最近衣服。发型、妆容、参考数量先返回 0，但路由和页面入口保持独立。

### wardrobe

保留现有：

- `GET /api/user/wardrobe/options`
- `GET /api/user/wardrobe/items`
- `POST /api/user/wardrobe/items`
- `POST /api/user/wardrobe/items/recognize`
- `PATCH /api/user/wardrobe/items/:public_id`
- `DELETE /api/user/wardrobe/items/:public_id`

`wardrobe` 页面是衣服页，不再承担私藏总览职责。

### hair

新增一级领域：

`server/internal/domain/hair`

第一阶段接口：

- `GET /api/user/hair`

返回空列表和轻量能力状态即可：

```json
{
  "items": [],
  "enabled": true
}
```

后续再扩展 `POST /api/user/hair`、详情、编辑和删除。

### makeup

新增一级领域：

`server/internal/domain/makeup`

第一阶段接口：

- `GET /api/user/makeup`

返回空列表和轻量能力状态。

### references

新增一级领域：

`server/internal/domain/reference` 或 `references`。

路由使用复数：

- `GET /api/user/references`

返回空列表和轻量能力状态。

## 小程序设计

### tab 页面

新增：

`miniapp/pages/collection/collection`

tabBar 第三个入口指向：

`pages/collection/collection`

文案：

- tab：私藏
- 页面标题：我的私藏
- 副文案：保存衣服、发型、妆容和参考图，作为 Hestia 给你建议的依据。

页面内容：

- 类型入口：衣服、发型、妆容、参考。
- 最近收录：展示 `GET /api/user/collection` 返回的 `recent_items`。
- 新增入口：第一阶段可以进入类型选择；衣服跳转到 `wardrobe` 新增流程，其他类型跳转到对应页面空状态。

### 衣服页

`miniapp/pages/wardrobe/wardrobe` 回到衣服页定位：

- 标题：衣服。
- 副文案：管理会参与建议的常穿单品。
- 使用 `/api/user/wardrobe/*`。
- 保留衣服分类筛选、图库、新增、详情和建议补齐。

### 发型页

新增：

`miniapp/pages/hair/hair`

第一阶段：

- 调用 `GET /api/user/hair`。
- 展示空状态：还没有发型私藏。
- 不承诺医疗、植发、病理判断。

### 妆容页

新增：

`miniapp/pages/makeup/makeup`

第一阶段：

- 调用 `GET /api/user/makeup`。
- 展示空状态：还没有妆容私藏。

### 参考页

新增：

`miniapp/pages/references/references`

第一阶段：

- 调用 `GET /api/user/references`。
- 展示空状态：还没有参考私藏。
- 文案强调“参考造型逻辑”，不是相貌对比。

## 数据边界

- `collection` 不拥有明细数据，只聚合各领域摘要。
- `wardrobe` 拥有衣服数据。
- `hair`、`makeup`、`references` 后续各自拥有自己的数据表和业务规则。
- 第一阶段可以用空列表响应建立接口契约，不急于落库。

## 已确认决策

- 接口不使用 `private` 前缀。
- `collection` 只做聚合，不做各类型父路由。
- 衣服继续使用 `wardrobe` 领域和路由。
- 发型、妆容、参考是 `/api/user` 下的一级资源。
- 小程序 `私藏` tab 使用独立 `collection` 页面。
