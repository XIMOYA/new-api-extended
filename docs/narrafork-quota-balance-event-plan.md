# NarraFork 额度事件适配方案

> 适用项目：new-api
>
> 文档状态：方案设计，尚未修改源码
>
> 更新时间：2026-08-08

## 一、背景

NarraFork 支持从第三方 OpenAI-compatible 或 Anthropic-compatible 服务的流式 SSE 响应中读取额度信息，并在对话窗口中实时显示余额。

目标是在 new-api 中增加可选的 `quotaBalanceEvent` SSE 事件，使 NarraFork 能够从 new-api 获取当前请求对应的额度信息。

目标事件格式：

```text
event: quotaBalanceEvent
data: {"quotaBalance":"$12.34","detailedQuotaBalance":"余额: $12.34\\n本次消耗: $0.56"}

```

该功能应满足：

- 支持 OpenAI Chat Completions；
- 支持 OpenAI Responses；
- 支持 Anthropic Messages；
- 不破坏现有普通客户端；
- 事件出现在流终止事件之前；
- 支持全局配置和渠道级覆盖；
- 避免重复转发或重复生成额度事件；
- 不泄露用户、令牌和上游密钥等敏感信息。

## 二、当前代码分析

### 2.1 全局设置体系

全局配置主要经过以下路径：

```text
setting/config/config.go
model/option.go
controller/option.go
/api/option/
```

前端系统设置采用 section registry 组织：

```text
web/src/features/system-settings/models/section-registry.tsx
web/src/components/layout/config/system-settings.config.ts
```

NarraFork 额度事件属于模型网关和协议转发行为，建议放在：

```text
系统设置 → Models & Routing → NarraFork Gateway Events
```

不建议放在 Billing & Payment 下，因为该功能的核心是响应协议扩展，而不是支付网关配置。

### 2.2 渠道配置体系

渠道自身已经存在 `OtherSettings` 或类似扩展字段，适合增加渠道级 NarraFork 配置：

```text
model/channel.go
relaykit/dto/channel_settings.go
web/src/features/channels/types.ts
web/src/features/channels/components/drawers/channel-mutate-drawer.tsx
```

渠道级配置可以覆盖全局默认值，适合处理不同供应商或不同客户端的差异。

### 2.3 流式响应入口

主要流式处理入口包括：

```text
relay/channel/openai/relay-openai.go
relay/channel/openai/relay_responses.go
relay/channel/openai/chat_via_responses.go
relay/channel/claude/relay-claude.go
relay/helper/stream_scanner.go
```

当前 `StreamScannerHandler` 主要处理 `data:` 行，对 `event:` 行的保留并不完整。

如果直接把上游的自定义事件交给现有转换器，可能出现以下问题：

- `event:` 行被丢弃；
- 自定义事件被当成普通响应 chunk 解析；
- OpenAI 与 Anthropic 格式转换时事件名称丢失；
- 事件被错误地放在终止事件之后。

### 2.4 当前计费时序

当前文本请求大致经过以下流程：

```text
预扣额度
  ↓
请求上游
  ↓
发送流式内容
  ↓
发送 [DONE] 或 message_stop
  ↓
计算实际 usage
  ↓
PostTextConsumeQuota
  ↓
SettleBilling
```

因此，不能简单地在流处理器返回之后追加 `quotaBalanceEvent`。

如果追加位置晚于 `[DONE]` 或 `message_stop`，客户端可能已经认为流结束，NarraFork 也可能无法正确识别额度事件。

## 三、总体方案

推荐采用以下架构：

```text
全局配置 + 渠道覆盖
          ↓
解析 NarraFork 事件配置
          ↓
写入 RelayInfo
          ↓
处理上游 SSE
          ↓
转换为目标协议响应
          ↓
在终止事件之前注入 quotaBalanceEvent
          ↓
发送 [DONE] 或 message_stop
          ↓
执行现有计费结算
```

核心原则：

1. 额度事件功能默认关闭；
2. 只对 SSE 流式响应注入；
3. 使用统一的 SSE 事件写出函数；
4. 事件必须位于流终止事件之前；
5. 不改变现有计费的最终结算流程；
6. 使用预测的结算后余额，避免显示预扣后的临时余额；
7. 对已存在的上游额度事件进行去重。

## 四、配置设计

### 4.1 全局配置

建议增加新的配置命名空间：

```text
narrafork_setting.enabled
narrafork_setting.activation_mode
narrafork_setting.balance_source
narrafork_setting.include_detailed
narrafork_setting.duplicate_policy
narrafork_setting.expose_extra
```

默认值：

```json
{
  "enabled": false,
  "activation_mode": "header_or_user_agent",
  "balance_source": "effective",
  "include_detailed": true,
  "duplicate_policy": "skip",
  "expose_extra": false
}
```

字段说明：

| 配置项 | 说明 |
|---|---|
| `enabled` | NarraFork 额度事件总开关，默认关闭 |
| `activation_mode` | 请求级触发方式，默认要求 Header 或 User-Agent 命中 |
| `balance_source` | 额度来源 |
| `include_detailed` | 是否发送 `detailedQuotaBalance` |
| `duplicate_policy` | 上游已有额度事件时的处理策略 |
| `expose_extra` | 是否发送机器可读的 `extra` 信息 |

### 4.2 额度来源

建议支持以下来源：

```text
effective      当前请求实际使用的额度来源
user_quota     用户账户余额
token_quota    API Token 剩余额度
custom         管理员配置的固定值
```

默认使用 `effective`。

原因是 new-api 可能根据用户配置使用不同资金来源，例如：

- 用户钱包余额；
- 用户订阅额度；
- API Token 剩余额度；
- 其他计费来源。

不建议默认读取 `Channel.Balance`，因为渠道余额通常表示上游供应商余额，不一定属于当前用户，也不一定实时。

### 4.3 渠道级配置

建议在渠道的 `OtherSettings` 中增加：

```json
{
  "narrafork": {
    "enabled": null,
    "activation_mode": null,
    "balance_source": null,
    "include_detailed": null,
    "duplicate_policy": null,
    "custom_quota_balance": null,
    "custom_detailed_quota_balance": null,
    "custom_extra": null
  }
}
```

字段使用三态逻辑：

```text
null   继承全局配置
true   仅该渠道开启
false  仅该渠道关闭
```

这样可以实现：

- 全局关闭，只为 NarraFork 渠道开启；
- 全局开启，关闭普通客户端渠道；
- 不同渠道使用不同额度来源；
- 为测试渠道设置固定额度信息。

### 4.4 请求级触发条件

为避免同一实例同时服务普通客户端和 NarraFork 时影响严格校验 SSE 事件类型的 SDK，建议增加请求级触发条件。

默认策略建议为：

```text
只有满足以下任一条件时，才发送 quotaBalanceEvent：

1. 请求 Header 中存在 X-NarraFork-Quota-Event: true；
2. User-Agent 中包含 NarraFork。
```

匹配规则建议：

- Header 名称大小写不敏感；
- Header 值去除首尾空格后，大小写不敏感地匹配 `true`；
- User-Agent 使用大小写不敏感的子串匹配；
- 全局 `enabled=false` 时，即使命中 Header 或 User-Agent，也不发送事件；
- 渠道级 `enabled=false` 时，优先于请求级触发条件，不发送事件；
- `X-NarraFork-Quota-Event` 仅作为 new-api 本地控制 Header，不应转发给上游供应商。

建议支持以下 `activation_mode`：

| 模式 | 行为 |
|---|---|
| `header_or_user_agent` | Header 或 User-Agent 任一命中即可发送，推荐默认值 |
| `header_only` | 仅 Header 命中时发送 |
| `user_agent_only` | 仅 User-Agent 命中时发送 |
| `always` | 全部符合全局和渠道开关的流式请求都发送 |
| `never` | 强制关闭请求级发送 |

推荐的最终判断顺序：

```text
全局 enabled
  ↓
渠道 enabled 覆盖
  ↓
activation_mode
  ↓
Header / User-Agent 匹配
  ↓
仅对 SSE 流式响应注入 quotaBalanceEvent
```

该 Header 不是安全认证机制，任何调用方都可以主动请求额度事件；它只影响响应格式，不应改变额度查询权限和计费逻辑。

## 五、额度计算方案

### 5.1 不直接读取请求开始时的余额

请求开始时的 `RelayInfo.UserQuota` 可能是旧值，并且 new-api 在请求开始前已经执行了预扣额度。

因此不能直接使用：

```text
RelayInfo.UserQuota
```

作为本次请求结束后的余额。

### 5.2 预测结算后余额

对于已经预扣的额度，可以使用以下逻辑：

```text
结算后余额 = 预扣后的当前余额 + 预扣额度 - 本次实际消费额度
```

这样可以在终止事件发送之前，计算出接近最终结算结果的余额。

建议通过 `BillingSession` 暴露计费快照，而不是在额度事件逻辑中重复实现计费规则。

### 5.3 计费计算与结算拆分

建议把现有计费逻辑拆成两个阶段：

```text
CalculateActualQuota
  ↓
BuildQuotaBalancePreview
  ↓
Write quotaBalanceEvent
  ↓
SettleBilling
```

其中：

- `CalculateActualQuota` 只计算本次真实消费额度；
- `BuildQuotaBalancePreview` 只生成余额展示数据；
- `SettleBilling` 继续由现有流程执行；
- 额度事件不能触发第二次扣费。

需要特别关注订阅额度、钱包额度和 Token 额度之间的差异，避免将订阅余额错误展示为钱包余额。

## 六、SSE 事件实现

### 6.1 统一事件结构

建议新增：

```text
relay/helper/gateway_event.go
```

定义统一结构：

```go
type StreamEvent struct {
    Event string
    Data  []byte
}
```

同时提供统一写出函数，负责：

- JSON 序列化；
- 写入 `event:` 行；
- 写入 `data:` 行；
- 写入结尾空行；
- 刷新响应缓冲区。

### 6.2 事件格式

最终输出必须是标准 SSE：

```text
event: quotaBalanceEvent
data: {"quotaBalance":"$12.34","detailedQuotaBalance":"余额: $12.34\\n本次消耗: $0.56"}

```

不要直接拼接未转义的 JSON 或换行符。

### 6.3 OpenAI Chat Completions

推荐顺序：

```text
普通响应 chunk
↓
usage chunk
↓
event: quotaBalanceEvent
↓
data: {...}
↓
data: [DONE]
```

注入点：

```text
relay/channel/openai/relay-openai.go
```

应位于 `helper.Done(c)` 之前。

### 6.4 OpenAI Responses

目标格式为 OpenAI 时：

```text
response.output_text.delta
response.completed
quotaBalanceEvent
[DONE]
```

目标格式为 Anthropic 时：

```text
message_delta
quotaBalanceEvent
message_stop
```

涉及文件：

```text
relay/channel/openai/relay_responses.go
relay/channel/openai/chat_via_responses.go
```

### 6.5 Anthropic Messages

推荐顺序：

```text
message_start
content_block_delta
message_delta
quotaBalanceEvent
message_stop
```

注入点：

```text
relay/channel/claude/relay-claude.go
```

## 七、事件去重

如果上游已经提供：

```text
event: quotaBalanceEvent
```

建议默认策略为：

```text
skip
```

即保留上游事件，不再生成重复事件。

可选策略：

| 策略 | 行为 |
|---|---|
| `skip` | 上游已有事件时不再注入 |
| `replace` | 丢弃上游事件，使用 new-api 生成的事件 |
| `always` | 上游事件和 new-api 事件都发送 |

默认使用 `skip`，避免 NarraFork 余额重复刷新。

## 八、`extra` 字段设计

默认不发送敏感信息。

允许发送：

```json
{
  "source": "effective",
  "billingSource": "wallet",
  "requestQuota": 560,
  "estimated": true
}
```

禁止发送：

```text
用户邮箱
用户 ID
Token Key
Channel Key
上游 API Key
数据库内部字段
```

`custom_extra` 需要限制 JSON 大小，并且必须经过 JSON 序列化，避免破坏 SSE 格式。

## 九、建议修改文件

### 9.1 后端配置

```text
setting/config/config.go
model/option.go
controller/option.go
setting/narrafork_setting/narrafork_setting.go
```

### 9.2 渠道配置

```text
model/channel.go
relaykit/dto/channel_settings.go
relay/common/relay_info.go
```

### 9.3 SSE 公共层

```text
relay/helper/stream_scanner.go
relay/helper/gateway_event.go
relay/helper/stream_result.go
```

### 9.4 OpenAI 和 Responses

```text
relay/channel/openai/relay-openai.go
relay/channel/openai/relay_responses.go
relay/channel/openai/chat_via_responses.go
relay/channel/openai/helper.go
```

### 9.5 Anthropic

```text
relay/channel/claude/relay-claude.go
```

### 9.6 计费余额预测

```text
service/text_quota.go
service/billing.go
service/billing_session.go
model/user_cache.go
```

### 9.7 前端

```text
web/src/features/system-settings/types.ts
web/src/features/system-settings/models/section-registry.tsx
web/src/features/system-settings/models/narrafork-settings-section.tsx
web/src/features/channels/types.ts
web/src/features/channels/lib/channel-form.ts
web/src/features/channels/components/drawers/channel-mutate-drawer.tsx
```

同时需要补齐对应的前端多语言文件。

## 十、兼容性风险

### 10.1 普通客户端解析失败

部分 OpenAI SDK 或 Anthropic SDK 可能只期待标准事件类型，不支持自定义事件。

因此：

- NarraFork 特殊信息发送默认关闭；
- 全局开启后，默认仍要求请求 Header 或 User-Agent 命中；
- 同一实例同时服务普通客户端和 NarraFork 时，优先使用 `header_or_user_agent`；
- 建议只为 NarraFork 使用的渠道启用；
- 严格校验 SSE 事件类型的客户端不会收到未知事件。

推荐触发示例：

```http
X-NarraFork-Quota-Event: true
User-Agent: NarraFork/1.0
```

其中任一条件满足即可触发，具体由 `activation_mode` 决定。

### 10.2 事件位置错误

事件如果出现在以下内容之后，可能无法被正确识别：

```text
[DONE]
message_stop
```

因此必须在终止事件之前写入。

### 10.3 Responses 转 Anthropic

Responses 转 Anthropic 时不能简单统一追加到响应末尾，必须插入到 `message_stop` 之前。

### 10.4 重试渠道变化

如果请求重试并切换渠道，最终事件应使用最终成功渠道的 NarraFork 配置。

### 10.5 并发请求余额误差

多个请求同时扣费时，预测余额与数据库最终余额可能存在短暂差异。

建议在 `extra` 中标记：

```json
{
  "estimated": true
}
```

### 10.6 非流式响应

非 SSE 响应不发送 `quotaBalanceEvent`，避免改变普通 JSON 响应结构。

## 十一、测试方案

### 11.1 后端自动测试

```bash
go test ./relay/helper/...
go test ./relay/channel/openai/...
go test ./relay/channel/claude/...
go test ./service/...
go test ./...
```

### 11.2 流式接口手工测试

至少验证：

```text
/v1/chat/completions
/v1/responses
/v1/messages
```

### 11.3 测试断言

需要验证：

- `quotaBalanceEvent` 恰好出现一次；
- 事件出现在 `[DONE]` 之前；
- Anthropic 中事件出现在 `message_stop` 之前；
- 每个 SSE 事件以 `\\n\\n` 结束；
- 普通 chunk 顺序保持不变；
- 非流式响应不包含额度事件；
- 全局关闭时完全不注入；
- 全局开启但请求没有命中 Header 或 User-Agent 时不注入；
- `X-NarraFork-Quota-Event: true` 可以触发事件；
- User-Agent 包含 `NarraFork` 可以触发事件；
- Header 值不是 `true` 时不触发事件；
- `header_only`、`user_agent_only`、`always`、`never` 模式行为正确；
- 控制 Header 不会被转发给上游；
- 渠道级配置优先于全局配置；
- 上游已有事件时默认不重复发送；
- Token 无限制时不会错误显示为零余额；
- 订阅计费时不会错误读取用户钱包余额。

## 十二、最终建议

第一期建议实现：

```text
全局开关
渠道级覆盖
effective / user_quota / token_quota
quotaBalanceEvent
OpenAI Chat Completions
OpenAI Responses
Anthropic Messages
重复事件去重
安全 extra
```

暂时不建议把 `channel_balance` 作为默认来源，也不建议大范围修改现有计费结算时序。

推荐采用：

```text
结算后余额预测
  +
终止事件前注入
  +
现有结算流程保持不变
```

这样可以在改动范围、协议兼容性和计费安全之间取得较好的平衡。
