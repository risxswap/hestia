# 大模型配置设计

## 背景

当前服务端只有一个很薄的 `llm.Client` 接口，业务侧直接依赖注入后的客户端能力。衣服图片识别已经需要按业务选择支持视觉能力的大模型，后续聊天、onboarding 总结、风格参考解析也需要各自选择供应商、模型和调用参数。

第一版不做管理后台和管理 API，配置通过数据库 seed 或 SQL 维护。

## 目标

- 支持配置多个大模型供应商。
- 支持在供应商下维护可用模型。
- 支持不同业务场景绑定不同供应商、模型和参数。
- 第一版 token 接受数据库明文存储。
- 业务使用配置复用 `system_configs`，不新增独立 `llm_usages` 表。

## 数据模型

### `llm_providers`

供应商配置表，保存供应商基础信息和明文 token。

字段：

- `id`
- `code`：稳定编码，例如 `openai`、`qwen`、`deepseek`
- `name`：展示名称
- `api_base_url`：供应商接口根地址
- `token`：第一版明文保存
- `auth_type`：认证方式，默认 `bearer`
- `status`：`active` / `disabled`
- `created_at`
- `updated_at`

约束：

- `code` 唯一。
- 运行时只读取 `status = active` 的供应商。

### `llm_models`

供应商模型表，保存每个供应商下可用的大模型。

字段：

- `id`
- `provider_id`
- `model_code`：供应商真实模型名，例如 `qwen-vl-plus`
- `name`：展示名称
- `caps_json`：能力列表 JSON，例如 `["text","vision","json","stream"]`
- `max_input_tokens`
- `max_output_tokens`
- `status`
- `created_at`
- `updated_at`

约束：

- `(provider_id, model_code)` 唯一。
- 运行时只读取 `status = active` 的模型。
- `caps_json` 第一版使用字符串数组，避免过早拆成多张能力表。

## 业务使用配置

业务绑定配置放在 `system_configs`：

- `group = "llm.usages"`
- `key = 固定业务场景`
- `value_type = "json"`
- `status = "active"`

示例：

```json
{
  "provider_code": "qwen",
  "model_code": "qwen-vl-plus",
  "params": {
    "temperature": 0.2,
    "max_tokens": 1200,
    "response_format": "json_object"
  },
  "prompt_version": "v1"
}
```

第一版固定 usage key：

- `wardrobe_image_recognition`：衣服图片识别，需要 `vision`、`json`
- `agent_chat`：聊天，需要 `text`，后续可启用 `stream`
- `onboarding_summary`：onboarding 总结，需要 `text`、`json`

## 运行时读取流程

1. 业务按固定 usage key 请求大模型配置。
2. 从 `system_configs` 读取 `group = "llm.usages"` 的 active 配置。
3. 根据 `provider_code` 查 `llm_providers` active 记录。
4. 根据 `provider_id + model_code` 查 `llm_models` active 记录。
5. 校验模型 `caps_json` 是否满足业务能力要求。
6. 合并业务参数，发起大模型调用。

## 接口方向

现有 `llm.Client` 只有：

```go
Complete(ctx context.Context, input string) (string, error)
```

后续实现时应升级为按 usage key 调用的结构化接口，例如：

```go
Generate(ctx context.Context, req llm.Request) (llm.Response, error)
```

其中 `Request` 至少包含：

- `UsageKey`
- `Messages`
- `ImageURLs`
- `ResponseFormat`
- `Params`

图片识别、聊天、总结都通过 resolver 读取配置后调用统一入口。

## 安全说明

第一版 token 明文入库，适合开发和早期内测。后续如果进入真实生产环境，应增加应用层加密或接入密钥管理服务，并避免通过任何查询接口返回 token 明文。

## 非目标

- 第一版不做管理后台。
- 第一版不做管理 API。
- 第一版不做 token 加密。
- 第一版不做多模型灰度、AB 测试、按用户分流。
