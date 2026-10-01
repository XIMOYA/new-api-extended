<!--
 docs/narrafork-dashboard-token-normalization-phase-2.md
 项目：NarraFork 看板 Token 统计归一化（二阶段）
 状态：规划中，暂不实施
 目标：统一不同厂商、不同日志来源的 Token 统计语义，让总 Token、缓存分项和缓存命中率可审计、可比较。
-->

# NarraFork 看板 Token 统计归一化（二阶段）

> 状态：**规划中，暂不实施**
> 建议前置条件：第一阶段统计口径修复已上线并观察稳定
> 适用范围：模型调用分析、Token 统计、缓存命中率、日志聚合与数据看板

## 1. 项目背景

当前模型调用分析页面存在两套 Token 统计链路：

- 原版统计卡片读取 `quota_data`，累加 `token_used`。
- NarraFork 缓存统计卡片读取 `logs`，根据 `prompt_tokens`、`completion_tokens` 和 `other` 中的缓存字段重新计算。

第一阶段已经将新增卡片的总 Token 统一为：

```text
总 Token = prompt_tokens + completion_tokens
```

并将缓存命中、缓存写入保留为独立分项，避免重复计入总数。

该方案可以解决当前最明显的数字膨胀问题，但不同上游厂商对以下字段的定义并不完全一致：

- `prompt_tokens`
- `input_tokens`
- `cache_tokens`
- `cache_creation_tokens`
- `cache_write_tokens`
- `total_tokens`

因此，第一阶段解决的是“看板总数不要重复相加”，并没有彻底解决“不同厂商 Token 语义统一”的问题。

二阶段的目标，是把 Token 语义归一化前置到请求结算和日志记录阶段，让看板只负责读取统一后的结果，不再根据原始字段猜测含义。

---

## 2. 当前代码现状

### 2.1 原版看板统计链路

前端入口：

```text
web/src/features/dashboard/components/models/log-stat-cards.tsx
```

调用：

```text
GET /api/data
GET /api/data/self
```

前端聚合：

```text
web/src/features/dashboard/lib/stats.ts
```

核心逻辑：

```ts
totalTokens += Number(item.token_used) || 0
```

后端数据来源：

```text
model/usedata.go
```

读取：

```text
quota_data.token_used
```

`token_used` 在消费日志记录时生成：

```text
model/log.go
```

当前含义：

```go
TokenUsed = PromptTokens + CompletionTokens
```

### 2.2 NarraFork 缓存卡片统计链路

前端入口：

```text
web/src/features/dashboard/components/models/cache-hit-rate-card.tsx
```

调用：

```text
GET /api/data/cache-hit-rate
GET /api/data/cache-hit-rate/self
```

后端入口：

```text
controller/usedata.go
```

数据聚合：

```text
model/narrafork_usage.go
```

当前读取：

```text
logs.prompt_tokens
logs.completion_tokens
logs.other.cache_tokens
logs.other.cache_write_tokens
logs.other.cache_creation_tokens*
```

二阶段之前，缓存卡片的计算方式曾经是：

```text
cache_input_tokens = input_tokens + cache_hit_tokens + cache_write_tokens
total_tokens = cache_input_tokens + output_tokens
```

第一阶段已经将其修正为：

```text
cache_input_tokens = input_tokens
total_tokens = input_tokens + output_tokens
```

### 2.3 现有厂商归一化逻辑

项目已经存在部分厂商使用量转换：

```text
service/billing_usage.go
```

主要函数包括：

```text
usageFromOpenAIBillingUsage
usageFromClaudeBillingUsage
usageFromGeminiBillingUsage
```

其中 Claude 的转换已经明确体现出缓存 Token 可能不包含在基础 `PromptTokens` 中：

```text
PromptTokens = InputTokens
InputTokens  = InputTokens + CacheReadInputTokens + CacheCreationInputTokens
```

文本结算流程还会生成部分扩展字段：

```text
service/text_quota.go
```

包括：

```text
other.cache_tokens
other.cache_write_tokens
other.input_tokens_total
other.usage_semantic
```

但是这些字段目前没有形成统一、强约束的日志协议，缓存看板也没有完整使用它们。

---

## 3. 二阶段目标

### 3.1 核心目标

建立统一的 Token 统计事实源：

```text
请求完成时归一化
→ 日志保存归一化结果
→ 后端统一聚合
→ 所有看板读取同一统计 DTO
```

### 3.2 统一字段目标

每条消费记录最终应能得到以下字段：

| 字段 | 含义 |
|---|---|
| `input_tokens_total` | 本次请求的总输入 Token |
| `output_tokens` | 本次请求的输出 Token |
| `cache_hit_tokens` | 缓存读取/命中 Token |
| `cache_write_tokens` | 缓存创建/写入 Token |
| `total_tokens` | `input_tokens_total + output_tokens` |
| `usage_source` | OpenAI Chat、OpenAI Responses、Claude、Gemini 等来源 |
| `usage_semantic` | OpenAI、Anthropic、Gemini、Unknown |
| `usage_complete` | 是否具备完整、可信的归一化数据 |
| `normalization_version` | 归一化规则版本 |

总 Token 的唯一计算方式：

```text
 total_tokens = input_tokens_total + output_tokens
```

缓存字段只用于：

- 展示缓存命中量；
- 展示缓存写入量；
- 计算缓存命中率；
- 计费或运营分析中的专项统计。

不得再次无条件加入 `total_tokens`。

---

## 4. 厂商归一化规则

二阶段不应在看板查询阶段根据字段名称猜测语义，而应由请求适配和结算阶段明确写入归一化结果。

### 4.1 OpenAI Chat Completions / Responses

建议规则：

1. 优先使用适配器明确提供的 `input_tokens`。
2. 如果没有明确的 `input_tokens`，回退到 `prompt_tokens`。
3. `cache_hit_tokens` 作为输入 Token 的子集记录。
4. 不因存在 `cache_hit_tokens` 再次增加 `input_tokens_total`。
5. `cache_write_tokens` 单独保存，是否计入输入总量由适配器的明确语义决定，不能统一相加。

目标示例：

```text
prompt_tokens = 100
cache_hit_tokens = 30
output_tokens = 10

input_tokens_total = 100
total_tokens = 110
```

### 4.2 Anthropic / Claude

Claude 的上游响应可能分别提供：

```text
基础输入 Token
缓存读取 Token
缓存创建 Token
输出 Token
```

建议规则：

```text
input_tokens_total =
    base_input_tokens
    + cache_read_input_tokens
    + cache_creation_input_tokens

total_tokens = input_tokens_total + output_tokens
```

其中：

```text
cache_hit_tokens = cache_read_input_tokens
cache_write_tokens = cache_creation_input_tokens
```

目标示例：

```text
base_input_tokens = 100
cache_hit_tokens = 30
cache_write_tokens = 20
output_tokens = 10

input_tokens_total = 150
total_tokens = 160
```

### 4.3 Gemini

建议使用 Gemini usage metadata 中已经归一化的：

- Prompt token count；
- Tool-use prompt token count；
- Cached content token count；
- Candidate/output token count；
- Thought/reasoning token count。

需要在适配器层明确：

- `PromptTokenCount` 是否已经包含 cached content；
- cached content 是否是 prompt 的子集；
- tool-use prompt 是否已经包含在 prompt 总数中。

看板层不得重复推导。

### 4.4 未知厂商和旧日志

当日志缺少完整归一化信息时：

```text
input_tokens_total = prompt_tokens
output_tokens = completion_tokens
total_tokens = prompt_tokens + completion_tokens
usage_complete = false
```

旧日志不能因为发现 `cache_tokens` 就强行修改历史总数，否则会造成历史数据在升级后突然跳变。

---

## 5. 日志存储设计

### 5.1 推荐的日志结构

建议在现有 `logs.other` 中增加统一的嵌套对象，避免继续散落多个同级字段：

```json
{
  "token_stats": {
    "version": 1,
    "input_tokens_total": 100,
    "output_tokens": 10,
    "cache_hit_tokens": 30,
    "cache_write_tokens": 0,
    "total_tokens": 110,
    "usage_source": "oai_chat",
    "usage_semantic": "openai",
    "usage_complete": true
  }
}
```

同时保留现有字段：

```json
{
  "cache_tokens": 30,
  "cache_write_tokens": 0,
  "input_tokens_total": 100
}
```

保留旧字段的原因：

- 兼容已有日志详情页；
- 兼容现有统计接口；
- 降低一次性迁移风险；
- 支持分阶段回滚。

### 5.2 是否新增数据库列

建议分两步：

#### 第一阶段：继续使用 `other.token_stats`

优点：

- 不需要立即修改多种数据库迁移；
- 兼容 SQLite、MySQL、PostgreSQL；
- 可以快速验证归一化规则。

缺点：

- 聚合时需要解析 JSON；
- 大量日志查询需要控制时间范围和内存。

#### 后续优化：增加结构化列

当数据量和查询性能需要时，再给日志或聚合表增加：

```text
input_tokens_total BIGINT
output_tokens BIGINT
cache_hit_tokens BIGINT
cache_write_tokens BIGINT
total_tokens BIGINT
usage_semantic VARCHAR
usage_complete BOOLEAN
normalization_version INTEGER
```

结构化列需要配套：

- 数据库迁移；
- 新旧数据兼容；
- 回填脚本；
- 多数据库测试；
- 回滚方案。

---

## 6. 聚合服务设计

### 6.1 统一后端 DTO

建议新增统一的看板统计结构：

```go
type DashboardTokenSummary struct {
    InputTokensTotal int64   `json:"input_tokens_total"`
    OutputTokens     int64   `json:"output_tokens"`
    TotalTokens      int64   `json:"total_tokens"`
    CacheHitTokens   int64   `json:"cache_hit_tokens"`
    CacheWriteTokens int64   `json:"cache_write_tokens"`
    CacheHitRate     float64 `json:"cache_hit_rate"`
    Complete         bool    `json:"complete"`
    Available        bool    `json:"available"`
}
```

统一不再使用含义模糊的：

```text
cache_input_tokens
```

如果必须保留该字段，应明确注释为：

```text
缓存命中率计算使用的总输入 Token
```

### 6.2 统一接口

建议提供统一接口：

```text
GET /api/data/token-summary
GET /api/data/token-summary/self
```

请求参数：

```text
start_timestamp
end_timestamp
username
model_name
```

返回：

```json
{
  "success": true,
  "data": {
    "input_tokens_total": 100,
    "output_tokens": 10,
    "total_tokens": 110,
    "cache_hit_tokens": 30,
    "cache_write_tokens": 0,
    "cache_hit_rate": 30,
    "complete": true,
    "available": true
  }
}
```

### 6.3 两个看板统一消费

以下组件应改为使用同一个统计接口或共享查询结果：

```text
web/src/features/dashboard/components/models/log-stat-cards.tsx
web/src/features/dashboard/components/models/cache-hit-rate-card.tsx
```

推荐增加共享 Hook：

```text
web/src/features/dashboard/hooks/use-dashboard-token-summary.ts
```

统一得到：

```text
input_tokens_total
output_tokens
total_tokens
cache_hit_tokens
cache_write_tokens
cache_hit_rate
```

这样不会出现：

- 一个卡片读 `quota_data`；
- 一个卡片读 `logs`；
- 两边自行计算不同结果。

---

## 7. 数据源选择

### 7.1 推荐顺序

二阶段建议先以 `logs` 作为归一化事实源，因为它包含：

- 原始输入/输出 Token；
- 缓存字段；
- 厂商语义；
- 请求时间；
- 用户、模型、渠道等完整维度。

先实现统一聚合接口，确保结果正确。

### 7.2 大数据量优化

如果直接扫描 `logs` 影响看板性能，应按以下顺序优化：

1. 强制时间范围；
2. 限制最大查询跨度；
3. 确认 `type + created_at + user_id` 索引；
4. 后端 SQL 只取必要字段；
5. 使用流式/分批读取，避免一次性加载全部日志；
6. 增加短时缓存；
7. 最终将归一化结果写入小时级聚合表。

### 7.3 `quota_data` 的后续扩展

如果最终需要继续使用 `quota_data`，应扩展其聚合字段：

```text
input_tokens_total
output_tokens
cache_hit_tokens
cache_write_tokens
total_tokens
```

并修改：

```text
model/usedata.go
model/log.go
```

让 `LogQuotaData` 在生成聚合数据时携带归一化 Token，而不是继续只传：

```text
PromptTokens + CompletionTokens
```

---

## 8. 缓存命中率规则

统一公式：

```text
cache_hit_rate = cache_hit_tokens / input_tokens_total * 100
```

边界规则：

- 输入 Token 为 0：命中率不可用；
- 缓存字段缺失：`complete=false`；
- 缓存命中大于输入：按厂商语义处理，无法确认时标记不完整；
- 最终展示范围限制在 `0 ~ 100`；
- 不因为缓存写入 Token 增加命中率分母，除非该厂商明确规定缓存写入属于同一输入统计口径。

前端应区分：

```text
有数据但统计不完整
没有数据
接口失败
```

不要把所有情况都显示成 `0%`。

---

## 9. 兼容和迁移策略

### 9.1 新日志

新请求完成时写入：

```text
token_stats.version = 1
```

新日志优先使用归一化字段。

### 9.2 老日志

老日志读取优先级：

1. `other.token_stats.total_tokens`；
2. `other.input_tokens_total + completion_tokens`；
3. `prompt_tokens + completion_tokens`。

老日志不主动根据 `cache_tokens` 修改总 Token。

### 9.3 回填策略

不建议默认全量回填历史日志。

原因：

- 不同厂商旧字段语义不完整；
- 部分日志缺少 `usage_semantic`；
- 回填可能造成历史看板数字跳变；
- 大表 JSON 扫描成本高。

如果确有对账需求，再提供显式回填命令：

```text
仅处理指定时间范围
支持 dry-run
输出无法归一化的记录数量
保存归一化版本
支持中断和继续
```

---

## 10. 测试矩阵

### 10.1 归一化单元测试

#### OpenAI

```text
prompt_tokens = 100
cache_hit_tokens = 30
output_tokens = 10
```

期望：

```text
input_tokens_total = 100
total_tokens = 110
```

#### Claude

```text
base_input_tokens = 100
cache_hit_tokens = 30
cache_write_tokens = 20
output_tokens = 10
```

期望：

```text
input_tokens_total = 150
total_tokens = 160
```

#### Gemini

覆盖：

- prompt token；
- cached content token；
- tool-use prompt token；
- thoughts/reasoning token。

#### 未知厂商

期望：

```text
total_tokens = prompt_tokens + completion_tokens
usage_complete = false
```

### 10.2 聚合测试

覆盖：

- 单用户；
- 全站；
- 用户名筛选；
- 模型筛选；
- 时间范围；
- 混合厂商；
- 空日志；
- 缺少 `other`；
- JSON 损坏；
- 缓存命中大于输入；
- 老日志和新日志混合。

### 10.3 接口测试

验证：

- 普通用户只能查询自己的数据；
- 管理员可按用户名查询；
- 时间范围非法时拒绝；
- 超大时间范围受限制；
- 两个接口返回结构一致；
- 不会因为缺少缓存字段导致总 Token 变成 0。

### 10.4 前端测试

验证：

- 两个卡片的总 Token 使用相同值；
- 缓存命中和写入只显示为分项；
- `complete=false` 时显示统计不完整状态；
- 接口失败和无数据状态区分；
- 长数字格式化和 tooltip 数值一致；
- 用户筛选、模型筛选、时间筛选保持一致。

### 10.5 性能测试

至少测试：

- 10 万条日志；
- 100 万条日志；
- 30 天查询；
- 管理员全站查询；
- 多用户并发打开看板。

验收重点：

- 不出现无界内存增长；
- 不一次性把全部日志载入内存；
- 查询耗时不会阻塞主请求；
- 必要索引能够命中。

---

## 11. 实施顺序

### 阶段 A：归一化函数

1. 整理 `service/billing_usage.go` 的厂商转换结果；
2. 新增统一 Token 统计结构；
3. 为 OpenAI、Claude、Gemini、未知厂商补测试；
4. 定义 `normalization_version`。

### 阶段 B：日志写入

1. 在结算完成时生成统一 Token 统计；
2. 写入 `other.token_stats`；
3. 保留旧字段；
4. 为旧日志读取提供 fallback。

### 阶段 C：统一聚合接口

1. 新增统一后端 DTO；
2. 实现统一统计服务；
3. 处理权限、用户筛选和时间范围；
4. 加入性能保护。

### 阶段 D：前端接入

1. 两个卡片共用统计接口；
2. 移除各自独立的 Token 总数计算；
3. 优化缓存分项文案；
4. 增加统计不完整提示。

### 阶段 E：聚合表优化

1. 观察日志实时聚合性能；
2. 确认是否需要扩展 `quota_data`；
3. 若需要，新增迁移和回填工具；
4. 完成多数据库验证。

---

## 12. 回滚方案

二阶段应通过能力开关逐步上线：

```text
normalized_dashboard_token_stats=false
```

回滚顺序：

1. 前端恢复旧统计接口；
2. 保留新日志字段，不删除；
3. 关闭新聚合任务；
4. 保留归一化版本和失败记录；
5. 确认旧看板数据正常后再排查问题。

不要在回滚时删除新字段或清理历史日志，避免无法恢复。

---

## 13. 验收标准

二阶段完成后必须满足：

- 相同筛选条件下，所有看板的总 Token 一致；
- OpenAI 缓存命中不会被重复计入总 Token；
- Claude 缓存读取和写入可以正确纳入总输入 Token；
- 未知厂商不会被错误推断；
- 缓存命中率始终在 `0 ~ 100`；
- 老日志仍能显示总 Token；
- 统计不完整时有明确标识；
- 普通用户和管理员权限隔离正确；
- 大范围查询不会造成数据库或内存压力异常；
- 多数据库迁移测试通过；
- 可以关闭归一化开关并安全回滚。

---

## 14. 暂不实施原因

目前第一阶段已经能够满足日常看板使用：

- 总 Token 不再重复计算缓存命中；
- 缓存数据仍可作为分项观察；
- 不影响实际计费逻辑；
- 不需要立即修改数据库结构；
- 可以避免在厂商语义尚未完全确认时扩大改动范围。

因此二阶段作为后续项目保留，等出现以下需求时再启动：

- 需要精确对账；
- 需要跨厂商比较缓存命中率；
- 需要统一多个看板数据源；
- 日志量增长导致当前聚合性能不足；
- 需要回填和审计历史 Token 数据。
