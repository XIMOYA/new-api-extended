<!--
 docs/request-content-audit-development.md
 项目：请求内容审计与请求记录页面
 状态：开发设计，暂不实施
 目标：记录用户实际提交给 New API 的请求内容，并提供受控查看、脱敏、图片渲染、使用日志双向联动和自动清理能力。
 重要边界：只记录用户提交后的请求内容，不记录键盘输入、按键事件、草稿或输入框变化。
-->

# 请求内容审计与请求记录页面开发文档

> 状态：**开发设计，暂不实施**  
> 内容范围：用户提交请求后的文本、图片和多模态内容  
> 核心原则：记录提交内容，不记录键盘操作；权限最小化；原文加密；图片懒加载；与使用日志双向定位。

---

## 1. 需求确认

### 1.1 必须记录的内容

记录用户实际提交给 New API 的请求内容：

- 用户在输入框中输入文本后按下回车提交的内容；
- 用户点击发送按钮提交的内容；
- 请求中的多轮消息；
- 请求中的图片内容；
- 请求中的多模态内容；
- 请求对应的模型、Token、渠道和时间等审计元数据。

### 1.2 明确不记录的内容

本功能严禁记录：

- 键盘按键事件；
- 用户输入过程；
- 输入框草稿；
- 鼠标移动；
- 光标位置；
- 用户没有提交的文字；
- 前端输入框实时内容上报。

系统只在请求已经提交、进入 New API 请求处理流程后记录内容。

### 1.3 用户权限要求

| 用户类型 | 查看范围 | 查看内容 |
|---|---|---|
| Root 用户 | 所有用户 | 完整文本、完整图片、完整请求内容 |
| 普通用户 | 自己的请求记录 | 自己的完整内容 |
| 管理员 | Root 授权范围内的其他用户 | 脱敏文本、脱敏图片或缩略图 |
| 未授权管理员 | 不允许查看其他用户 | 无访问权限 |
| 普通用户 | 其他用户 | 无访问权限 |

管理员能否查看其他用户内容，不由普通管理员角色自动决定，必须由 Root 在系统设置中逐个授予。

### 1.4 用户同意机制

按照当前需求，不增加“用户是否同意记录内容”的开关，也不因为用户未点击同意而绕过记录功能。

功能由 Root 统一控制：

- Root 开启后，按系统策略记录提交请求；
- 用户不能通过前端设置关闭审计记录；
- 用户只能受 Root 设置影响，查看或不查看自己的历史内容。

可以在页面中提供说明性提示，但说明文字不能成为记录功能的授权门槛。

---

## 2. 总体架构

```text
用户提交请求
    ↓
New API 鉴权
    ↓
读取并解析请求体
    ↓
提取原始用户消息和多模态内容
    ↓
脱敏扫描 / 风险识别 / 内容归一化
    ↓
写入请求内容审计记录
    ↓
图片解码、去元数据、生成缩略图
    ↓
转发给上游渠道
    ↓
写入使用日志关联信息
```

请求内容审计和现有消费日志是两个独立模块：

```text
消费日志：记录费用、Token、模型、渠道和结算信息
请求内容审计：记录用户实际提交的文本和多模态内容
```

不能把完整请求正文直接塞入现有的 `Log.Content` 或 `Log.Other`。

---

## 3. 请求捕获位置

当前请求体已经在公共 Relay 流程中被读取和解析，可以复用现有请求解析链路：

```text
controller/relay.go
relay/compatible_handler.go
relay/responses_handler.go
relay/claude_handler.go
relay/gemini_handler.go
common.GetRequestBody(...)
```

### 3.1 捕获时机

推荐在以下时机捕获：

```text
完成鉴权
→ 已成功读取请求体
→ 已完成基础协议解析
→ 尚未加入管理员系统提示词
→ 尚未转发上游
```

这样保存的是用户真正提交的原始请求，而不是：

- New API 后续拼接的系统提示词；
- 渠道侧转换后的请求；
- 上游渠道改写后的请求；
- 仅用于计费的摘要内容。

### 3.2 统一审计服务

不要在每个渠道 Handler 中直接写数据库，建议新增统一服务：

```text
service.RecordRequestContentAudit(...)
```

不同协议的 Handler 只负责提供标准化内容：

```text
OpenAI Chat Completions
OpenAI Responses
Claude Messages
Gemini GenerateContent
兼容接口
任务类请求
```

统一服务负责：

- 检查 Root 开关；
- 校验请求 ID；
- 解析消息内容；
- 处理文本和图片；
- 生成摘要和哈希；
- 加密保存；
- 写入关联信息；
- 创建图片处理任务。

---

## 4. 数据模型

### 4.1 请求内容审计表

建议新增：

```text
request_content_audits
```

建议字段：

| 字段 | 说明 |
|---|---|
| `id` | 审计记录 ID |
| `request_id` | 与消费日志关联的稳定请求 ID |
| `upstream_request_id` | 上游请求 ID |
| `user_id` | 用户 ID |
| `username` | 用户名快照 |
| `token_id` | 使用的 Token ID |
| `channel_id` | 渠道 ID |
| `model_name` | 模型名称 |
| `request_type` | chat、responses、claude、gemini、task 等 |
| `created_at` | 请求时间 |
| `expires_at` | 记录过期时间 |
| `content_ciphertext` | 加密后的标准化请求内容 |
| `content_hash` | 内容哈希，用于去重和追踪 |
| `content_size` | 内容大小 |
| `message_count` | 消息数量 |
| `asset_count` | 图片/附件数量 |
| `risk_level` | 风险等级 |
| `redaction_version` | 脱敏规则版本 |
| `capture_status` | complete、partial、failed |
| `normalization_version` | 内容归一化版本 |

### 4.2 图片资源表

图片和二进制内容单独保存：

```text
request_content_assets
```

建议字段：

| 字段 | 说明 |
|---|---|
| `id` | 资源 ID |
| `audit_id` | 所属审计记录 |
| `asset_type` | image、audio、file 等 |
| `mime_type` | 实际 MIME 类型 |
| `original_size` | 原始大小 |
| `width` | 图片宽度 |
| `height` | 图片高度 |
| `sha256` | 资源哈希 |
| `original_ciphertext` | 加密原图或原始资源 |
| `thumbnail_path` | 缩略图路径 |
| `redacted_thumbnail_path` | 脱敏缩略图路径 |
| `created_at` | 创建时间 |
| `expires_at` | 过期时间 |

### 4.3 访问审计表

所有查看行为都记录：

```text
request_content_access_logs
```

建议字段：

```text
id
viewer_user_id
viewer_role
target_user_id
audit_id
access_mode
created_at
ip
request_id
```

`access_mode` 示例：

```text
self_full
root_full
admin_redacted
asset_thumbnail
asset_original
```

---

## 5. 请求内容格式

建议保存统一的标准化结构：

```json
{
  "messages": [
    {
      "role": "system",
      "parts": [
        {
          "type": "text",
          "text": "用户实际提交的系统消息"
        }
      ]
    },
    {
      "role": "user",
      "parts": [
        {
          "type": "text",
          "text": "用户提交的文字"
        },
        {
          "type": "image",
          "asset_id": "asset_01H...",
          "mime_type": "image/png",
          "width": 1024,
          "height": 768,
          "sha256": "..."
        }
      ]
    }
  ],
  "request_type": "chat",
  "original_format": "openai_chat",
  "normalization_version": 1
}
```

保存时必须保留用户实际提交内容的顺序和角色，不要只拼接成一段纯文本，否则后续无法判断：

- 哪一段是用户输入；
- 哪一段是工具结果；
- 哪一段是系统消息；
- 哪一段来自图片或其他多模态资源。

---

## 6. 图片和 Base64 处理

### 6.1 绝对禁止直接把 Base64 返回前端

请求中的图片可能是：

```text
data:image/png;base64,...
```

不能直接把 Base64 放进：

- 列表接口响应；
- 页面状态；
- React 组件属性；
- 使用日志详情接口；
- URL 查询参数。

否则容易造成：

- 页面卡死；
- 浏览器内存暴涨；
- 接口响应过大；
- 列表滚动严重卡顿；
- 内容被浏览器开发工具直接复制；
- 多图片请求导致页面崩溃。

### 6.2 Base64 处理流程

```text
data:image/png;base64,...
    ↓
解析 Data URI
    ↓
校验 MIME 类型
    ↓
解码 Base64
    ↓
限制解码后大小
    ↓
检测真实图片格式
    ↓
检测图片尺寸
    ↓
去除 EXIF 等元数据
    ↓
保存加密原图
    ↓
生成缩略图
    ↓
生成管理员脱敏缩略图
```

### 6.3 资源存储规则

数据库只保存资源引用：

```json
{
  "type": "image",
  "asset_id": "asset_xxx",
  "mime_type": "image/png",
  "width": 1024,
  "height": 768,
  "size": 123456,
  "sha256": "..."
}
```

原图和缩略图通过鉴权接口单独获取：

```text
GET /api/request-records/{audit_id}/assets/{asset_id}
```

前端只在用户打开详情时懒加载图片，不在列表中预加载全部资源。

### 6.4 建议限制

初始默认值建议：

```text
单张图片最大：10 MB
单条请求最大：20 MB
缩略图最长边：1024px
列表单页：20～50 条
详情单次最大加载资源：10 个
```

图片必须进行：

- MIME 校验；
- 图片尺寸校验；
- 图片炸弹防护；
- EXIF 清理；
- SVG 默认禁止或转换后展示；
- 非图片扩展名拒绝；
- 解码超时和内存上限控制。

外部图片 URL 不建议服务器自动抓取，否则会产生 SSRF 风险。默认只保存 URL 和元信息；如需抓取，必须使用受限的图片下载器。

---

## 7. 加密和脱敏

### 7.1 原文保存

用户本人和 Root 需要查看完整内容，因此不能只保存哈希或摘要。

建议：

```text
content_ciphertext = AES-GCM 加密后的内容
```

加密密钥：

- 不存储在数据库；
- 不写入普通日志；
- 优先使用环境变量或密钥服务；
- 支持后续密钥轮换。

### 7.2 管理员脱敏

授权管理员只能查看脱敏内容：

```text
sk-xxxxxxxxxxxx
Bearer eyJhbGci...
api_key=********
password=********
```

建议识别和处理：

- API Key；
- Bearer Token；
- Cookie；
- 邮箱；
- 手机号；
- 身份证号；
- 数据库连接串；
- 私钥；
- 常见密码字段；
- 云服务密钥。

### 7.3 图片脱敏

管理员查看图片时：

- 只返回脱敏缩略图；
- 原图不返回；
- 可以添加水印；
- 可以降低分辨率；
- 去除 EXIF；
- 不允许原图下载。

图片自动识别人脸、证件和敏感信息属于后续增强能力，第一版不能宣称可以完全脱敏。

---

## 8. Root 系统设置

新增 Root 专属页面：

```text
系统设置 → 请求内容审计
```

页面只对 Root 显示和可操作。

### 8.1 全局开关

```text
enabled
```

关闭后：

- 不记录新请求内容；
- 历史记录仍按权限可查看；
- 不影响模型请求转发；
- 不删除已有审计记录。

### 8.2 允许用户本人查看

```text
allow_user_self_view
```

开启：

- 用户可以在“请求记录”中查看自己的完整内容。

关闭：

- 用户仍然可以正常使用；
- 用户无法查看自己的完整请求正文；
- Root 仍然可以查看。

### 8.3 授权管理员列表

```text
allowed_admin_user_ids
```

默认：

```json
[]
```

Root 可以：

- 搜索管理员用户；
- 添加授权；
- 撤销授权；
- 查看授权变更历史。

普通管理员不能授予自己权限，也不能授予其他管理员权限。

### 8.4 记录保留天数

```text
retention_days
```

建议范围：

```text
1～365 天
```

建议默认：

```text
30 天
```

自动清理：

- 清理过期审计记录；
- 清理过期原图；
- 清理过期缩略图；
- 清理访问审计日志；
- 保留必要的统计摘要和哈希。

### 8.5 图片保存策略

```text
store_images
```

可选：

- 保存文本和图片；
- 保存文本及图片元信息，不保存图片原文；
- 完全不保存图片，仅记录存在图片。

---

## 9. 用户端“请求记录”页面

### 9.1 侧边栏位置

在用户端侧边栏中：

```text
使用日志
请求记录
任务日志
```

“请求记录”必须放在“使用日志”和“任务日志”之间。

当前侧边栏入口位置参考：

```text
web/src/hooks/use-sidebar-data.ts
```

新增页面路由建议：

```text
/request-records
```

对应 TanStack Router 文件建议：

```text
web/src/routes/_authenticated/request-records/index.tsx
web/src/routes/_authenticated/request-records/$section.tsx
```

如果第一版不需要子分类，也可以只实现：

```text
/request-records
```

### 9.2 页面内容

请求记录页面用于展示当前用户提交过的每一次请求：

列表展示：

- 请求时间；
- 请求 ID；
- 模型；
- 请求类型；
- 请求状态；
- 输入内容摘要；
- 消息数量；
- 图片数量；
- Token 数；
- 是否命中风险规则；
- 是否存在对应使用日志。

列表不一次性渲染完整正文，而是：

```text
列表显示摘要
点击后打开详情
详情页显示完整内容
```

这样既满足用户完整查看，也避免大量长文本和图片同时渲染导致页面卡顿。

### 9.3 请求记录详情

详情页或详情抽屉展示：

- 完整消息列表；
- 消息角色；
- 完整文本；
- 图片缩略图；
- 点击图片查看受控大图；
- 请求时间；
- 模型；
- Token；
- 使用日志关联状态；
- 请求 ID；
- 上游请求 ID；
- 请求状态。

图片使用懒加载：

```text
列表不加载图片
打开详情才加载缩略图
点击缩略图才加载受控大图
```

---

## 10. 使用日志和请求记录双向联动

这是本功能的重点之一。

### 10.1 关联主键

两个页面必须使用稳定的：

```text
request_id
```

作为关联依据。

不要使用前端列表中的展示序号作为关联 ID，因为当前日志列表可能会重新分页和重新编号。

必要时同时保存：

```text
request_id
upstream_request_id
log_id
```

但页面跳转和高亮优先使用 `request_id`。

### 10.2 请求记录 → 使用日志

请求记录详情或列表操作提供：

```text
转到使用日志
```

跳转地址建议：

```text
/usage-logs/common?request_id={request_id}&focus=1
```

到达使用日志页面后：

1. 将 `request_id` 加入日志筛选条件；
2. 定位对应日志行；
3. 自动滚动到目标行；
4. 添加高亮边框；
5. 执行 2～3 次闪烁动画；
6. 动画结束后保留轻微高亮状态；
7. 提供键盘和屏幕阅读器可识别的定位状态。

### 10.3 使用日志 → 请求记录

使用日志每一条消费记录的详细信息中增加：

```text
我的请求内容
```

该区域包含：

- 内容摘要；
- 消息数量；
- 图片数量；
- 是否存在完整请求记录；
- 转到请求记录按钮。

按钮跳转：

```text
/request-records?request_id={request_id}&focus=1
```

请求记录页面执行同样的：

1. 筛选；
2. 定位；
3. 自动滚动；
4. 高亮；
5. 闪烁标记。

### 10.4 高亮动画要求

不能只依靠颜色区分，建议同时使用：

- `outline`；
- 左侧标记条；
- 轻微背景色；
- `aria-current` 或 `data-focused`；
- 自动聚焦目标行。

动画建议：

```text
持续时间：1.5～2.5 秒
闪烁次数：2～3 次
结束后保留：低强度边框
```

需要兼容：

```text
prefers-reduced-motion: reduce
```

减少动态效果时改为静态高亮，不强制闪烁。

### 10.5 找不到目标记录

如果目标记录因以下原因无法找到：

- 已超过保留期限；
- 用户权限发生变化；
- 日志被删除；
- 查询时间范围不匹配；
- 历史数据没有 `request_id`；

页面应显示明确状态：

```text
未找到对应的请求记录
```

不能显示成普通的“暂无数据”。

---

## 11. 使用日志详情中的“我的请求内容”

现有使用日志详情页面参考：

```text
web/src/features/usage-logs/components/dialogs/details-dialog.tsx
```

### 11.1 展示位置

在使用日志详细信息弹窗中增加独立区域：

```text
我的请求内容
```

推荐放在基础请求信息之后、计费详情之前或之后，具体以详情弹窗现有布局为准。

### 11.2 内容预览

由于使用日志详情弹窗宽度有限，只显示摘要：

- 最大字符数，例如 500～1000 字符；
- 最大显示行数，例如 6～8 行；
- 超出部分使用省略号；
- 多消息显示消息数量；
- 图片显示图片数量和第一张缩略图；
- 不在使用日志详情弹窗中直接嵌入完整 Base64。

示例：

```text
用户：请帮我分析这段代码……
用户：另外请根据这张图片……
[还有 3 条消息，包含 2 张图片]
```

### 11.3 操作按钮

增加：

```text
转到请求记录
```

按钮行为：

- 关闭当前详情弹窗；
- 跳转到请求记录页面；
- 带上 `request_id`；
- 自动定位并高亮对应记录。

如果当前用户没有权限查看完整内容：

- 可以显示脱敏摘要；
- 不显示原文；
- 仍可显示请求记录存在状态；
- 不允许通过前端参数绕过权限。

---

## 12. API 设计

### 12.1 用户接口

```text
GET /api/request-records
GET /api/request-records/:id
GET /api/request-records/:id/assets/:asset_id
```

行为：

- 只能查询当前用户自己的记录；
- 是否允许查看完整内容由 Root 设置控制；
- 用户无法通过修改 `user_id` 查询他人记录。

### 12.2 管理员接口

```text
GET /api/admin/request-records
GET /api/admin/request-records/:id
GET /api/admin/request-records/:id/assets/:asset_id
```

权限：

```text
Root：完整内容
Root 授权管理员：脱敏内容
普通管理员：403
```

### 12.3 查询参数

建议支持：

```text
request_id
user_id
username
model_name
request_type
risk_level
start_timestamp
end_timestamp
page
page_size
```

用户接口必须忽略或拒绝 `user_id`、`username` 等越权筛选条件。

### 12.4 列表响应

列表接口禁止返回完整 Base64 和完整加密内容：

```json
{
  "id": "audit_123",
  "request_id": "req_123",
  "created_at": 1780000000,
  "model_name": "gpt-xxx",
  "request_type": "chat",
  "message_count": 4,
  "asset_count": 2,
  "preview": "请帮我分析这段代码……",
  "has_full_content": true,
  "has_usage_log": true
}
```

完整内容必须通过详情接口按需读取。

---

## 13. 权限和安全控制

### 13.1 Root 权限

Root 可以：

- 开启或关闭记录；
- 设置保留天数；
- 设置用户本人可见；
- 授权管理员；
- 撤销管理员授权；
- 查看所有用户完整内容；
- 查看原始图片；
- 查看访问审计；
- 删除过期或指定记录。

### 13.2 管理员权限

管理员只有在 Root 授权后才可以：

- 查看授权范围内用户；
- 查看脱敏文本；
- 查看脱敏缩略图；
- 查看风险标记。

管理员不能：

- 查看完整原文；
- 查看原始图片；
- 导出全部数据；
- 授权其他管理员；
- 修改 Root 配置。

### 13.3 用户权限

用户可以：

- 查看自己的完整请求内容；
- 查看自己的图片；
- 从使用日志跳转到请求记录；
- 从请求记录跳转到使用日志。

用户不能：

- 查看其他用户；
- 修改记录；
- 删除记录；
- 修改保留期限；
- 访问管理员脱敏接口。

### 13.4 防止越权

所有权限必须由后端验证：

- 不能只依赖前端隐藏按钮；
- 不能只验证管理员角色；
- 不能只验证请求参数中的用户 ID；
- 图片资源接口同样必须鉴权；
- 详情接口和列表接口必须使用同一套权限判断。

---

## 14. 滥用检测联动

记录内容的主要目标是追溯和保护上游账号，但单纯记录不能阻止滥用。

审计记录建议同时保存：

```text
risk_level
matched_rules
request_count
recent_failure_count
provider_error_count
```

后续可以联动：

```text
低风险：正常转发
中风险：限速或降低额度
高风险：拒绝请求并记录
重复违规：自动禁用用户 Token
```

还可以根据以下维度识别重复滥用：

- 用户；
- Token；
- IP；
- 模型；
- 渠道；
- 请求内容哈希；
- 短时间重复次数；
- 上游风控错误次数。

---

## 15. 保留和清理策略

建议 Root 设置：

```text
retention_days = 30
```

范围：

```text
1～365 天
```

清理任务需要同时处理：

- 审计正文；
- 原始图片；
- 缩略图；
- 脱敏缩略图；
- 内容访问日志；
- 失效的临时资源。

清理任务必须：

- 分批执行；
- 避免一次性删除大量数据；
- 记录删除数量；
- 失败可重试；
- 不删除仍在调查中的保留记录。

---

## 16. 前端性能要求

### 16.1 列表页

必须使用：

- 分页；
- 服务端筛选；
- 服务端排序；
- 列表摘要；
- 图片懒加载；
- 长文本截断；
- 必要时使用虚拟列表。

禁止：

- 一次性加载用户全部请求正文；
- 一次性渲染所有图片；
- 将 Base64 放入列表响应；
- 用 `localStorage` 保存完整审计正文；
- 把完整内容放入 URL。

### 16.2 详情页

详情页可以显示完整文本，但要：

- 使用滚动容器；
- 对长消息分段渲染；
- 图片按需加载；
- 限制同时打开的原图数量；
- 防止 Markdown/HTML 内容被当作代码执行；
- 默认按纯文本展示用户输入。

### 16.3 高亮联动

使用日志和请求记录跳转时：

- 通过 `request_id` 定位；
- 支持自动滚动；
- 高亮 2～3 次；
- 支持 reduced-motion；
- 目标不存在时显示明确错误状态。

推荐 URL：

```text
/usage-logs/common?request_id={request_id}&focus=1
/request-records?request_id={request_id}&focus=1
```

---

## 17. 推荐前端文件调整

### 17.1 侧边栏

```text
web/src/hooks/use-sidebar-data.ts
```

在以下两个入口之间插入：

```text
Usage Logs
Task Logs
```

新增：

```text
Request Records
```

### 17.2 新增请求记录功能

建议新增目录：

```text
web/src/features/request-records/
```

建议文件：

```text
index.tsx
api.ts
types.ts
components/request-records-table.tsx
components/request-record-detail.tsx
components/request-record-content.tsx
components/request-record-assets.tsx
components/request-record-filters.tsx
lib/request-record-links.ts
```

### 17.3 新增路由

```text
web/src/routes/_authenticated/request-records/index.tsx
```

如需要分类：

```text
web/src/routes/_authenticated/request-records/$section.tsx
```

### 17.4 修改使用日志

重点文件：

```text
web/src/features/usage-logs/components/dialogs/details-dialog.tsx
web/src/features/usage-logs/components/columns/common-logs-columns.tsx
web/src/features/usage-logs/components/usage-logs-table.tsx
```

新增：

- “我的请求内容”摘要区域；
- “转到请求记录”按钮；
- `request_id` 定位能力；
- 对应记录不存在时的状态显示。

### 17.5 系统设置

新增 Root 专属配置卡片，建议放到：

```text
web/src/features/system-settings/models/
```

需要同步：

- 后端配置结构；
- 默认值；
- Root 权限判断；
- 多语言文案；
- 设置保存接口；
- 设置校验。

---

## 18. 推荐后端文件调整

建议新增：

```text
model/request_content_audit.go
model/request_content_asset.go
service/request_content_audit.go
service/request_content_redaction.go
service/request_content_image.go
controller/request_content.go
controller/request_content_admin.go
```

需要修改：

```text
router/api-router.go
controller/relay.go
relay/compatible_handler.go
relay/responses_handler.go
relay/claude_handler.go
relay/gemini_handler.go
model/log.go
```

如果采用独立设置结构，还需要：

```text
setting/request_content_setting/
```

---

## 19. 失败处理策略

审计写入失败时不能静默吞掉。

建议记录：

```text
capture_status = failed
capture_error_code
request_id
user_id
created_at
```

默认建议：

- 审计数据库短暂失败时，正常请求不立即全部阻断；
- 记录系统告警；
- 对高风险渠道可配置为失败即拒绝转发；
- 图片处理失败不影响文本审计记录；
- 原文保存成功但缩略图失败时，显示资源处理失败状态。

该行为可以作为 Root 高级设置：

```text
capture_failure_mode = fail_open / fail_closed
```

第一版建议默认 `fail_open`，避免审计系统故障扩大为全站不可用；高风险渠道可以单独配置 `fail_closed`。

---

## 20. 测试要求

### 20.1 后端单元测试

覆盖：

- 文本消息解析；
- 多轮消息顺序保留；
- 系统、用户、助手、工具角色识别；
- OpenAI Chat 格式；
- Responses 格式；
- Claude 格式；
- Gemini 格式；
- 图片 URL；
- Base64 图片；
- 非法 Base64；
- 超大图片；
- 超大请求；
- 内容加密和解密；
- 脱敏规则；
- 哈希生成；
- 过期时间计算。

### 20.2 权限测试

必须验证：

- Root 可以查看所有完整记录；
- 用户只能查看自己的记录；
- 用户不能查询其他用户 ID；
- 未授权管理员返回 403；
- Root 授权管理员只能查看脱敏内容；
- 授权管理员不能查看原图；
- 撤销权限立即生效；
- 图片资源接口不能绕过主记录权限。

### 20.3 联动测试

验证：

- 请求记录跳转使用日志；
- 使用日志跳转请求记录；
- `request_id` 正确定位；
- 目标自动滚动；
- 目标高亮闪烁；
- reduced-motion 下仍有静态标记；
- 目标不存在时显示明确提示；
- 使用日志详情摘要与请求记录详情关联正确。

### 20.4 前端性能测试

覆盖：

- 1000 条请求记录分页；
- 单条包含 10 张图片；
- 单条包含超长文本；
- 多条记录同时存在 Base64 输入；
- 快速连续跳转；
- 浏览器后退/前进；
- 移动端详情页；
- 低性能设备。

验收要求：

- 列表打开不加载完整图片；
- 页面不会因为 Base64 直接卡死；
- 详情打开不会阻塞整个页面；
- 跳转高亮不会造成滚动抖动。

---

## 21. 分阶段实施计划

### 阶段一：数据和权限基础

1. 新增审计配置；
2. 新增审计数据表；
3. 新增权限判断；
4. 新增文本内容加密存储；
5. 新增 Root 授权管理员列表；
6. 新增保留期限清理任务。

### 阶段二：请求捕获

1. 接入公共 Relay 请求解析流程；
2. 提取原始用户提交内容；
3. 保留消息顺序和角色；
4. 生成 `request_id` 关联；
5. 保存文本内容；
6. 记录捕获完整性状态。

### 阶段三：图片处理

1. 解析 Base64 图片；
2. 校验 MIME 和大小；
3. 保存加密原图；
4. 生成缩略图；
5. 生成管理员脱敏缩略图；
6. 新增受控资源接口。

### 阶段四：用户端请求记录页面

1. 侧边栏增加“请求记录”；
2. 新增 `/request-records` 路由；
3. 新增列表分页；
4. 新增详情抽屉或详情页；
5. 文本完整展示；
6. 图片懒加载和大图查看。

### 阶段五：使用日志联动

1. 使用日志详情中增加“我的请求内容”；
2. 添加摘要和省略号；
3. 添加“转到请求记录”；
4. 请求记录增加“转到使用日志”；
5. 通过 `request_id` 定位；
6. 自动滚动和高亮闪烁；
7. 支持目标缺失状态。

### 阶段六：安全审计和滥用联动

1. 记录管理员查看行为；
2. 增加风险等级；
3. 增加规则命中记录；
4. 对异常用户进行限速或禁用；
5. 增加 Root 审计导出和清理能力；
6. 完成多用户和多角色验收。

---

## 22. 验收标准

### 功能

- 用户提交的文本可以被记录；
- 没有提交的草稿不会被记录；
- 不记录键盘操作；
- 用户可以查看自己的完整记录；
- Root 可以查看全部完整记录；
- Root 授权的管理员只能查看脱敏内容；
- 未授权管理员无法查看其他用户内容；
- Root 可以配置开关、保留天数、用户自查权限和管理员白名单。

### 页面

- 侧边栏顺序为“使用日志 → 请求记录 → 任务日志”；
- 请求记录页面展示用户每次提交请求；
- 使用日志每条记录可查看“我的请求内容”摘要；
- 请求记录和使用日志可以双向跳转；
- 跳转后目标记录自动定位并高亮闪烁；
- 大量文本使用摘要和详情分离；
- Base64 图片不会直接进入列表接口；
- 图片按需渲染，不造成页面卡死。

### 安全

- 完整正文加密保存；
- 管理员只能看到脱敏内容；
- 原图接口独立鉴权；
- 所有查看行为有审计记录；
- 记录按 Root 设置的天数自动清理；
- 不在普通业务日志中输出原始请求内容；
- 不把完整内容放入 URL、localStorage 或前端全局状态。

---

## 23. 默认配置建议

```text
enabled = false
allow_user_self_view = true
allowed_admin_user_ids = []
retention_days = 30
store_images = true
max_image_size = 10 MB
max_request_content_size = 20 MB
capture_failure_mode = fail_open
```

说明：

- 现有安装默认关闭，避免升级后立即产生大量历史内容；
- 开启后不增加用户同意开关；
- 用户本人查看权限由 Root 控制；
- 管理员授权名单默认为空；
- 图片默认保存加密原图和缩略图，但原图只对 Root 和符合条件的用户开放；
- 审计系统故障默认不阻断正常请求，但必须告警。

---

## 24. 最终设计原则

```text
记录提交内容，不记录键盘行为

记录请求事实，不记录前端草稿

独立审计存储，不污染消费日志

Root 查看完整内容，管理员默认不可见

管理员必须由 Root 单独授权

用户只能查看自己的内容

原文加密，图片独立存储

列表摘要，详情完整

Base64 不直接返回前端

request_id 作为双向联动主键

所有访问行为可审计

记录按保留期限自动清理
```

该方案既能保留用户实际提交内容，帮助定位滥用来源和保护上游官方账号，也能避免把键盘监听、原始 Base64、普通日志权限和完整用户隐私混在一起。

---

## 25. 当前实现落地说明（2026-08-14）

当前实现已按本文件边界接入主库与前端：

- 捕获点位于 `controller/relay.go` 的请求体校验和 `RelayInfo` 生成之后，只处理已经提交给 New API 的 HTTP 请求，不读取输入框、草稿或键盘事件；OpenAI Realtime 的用户内容位于 WebSocket 帧中，当前不落空握手记录，Realtime 帧审计仍需单独的帧级接入。
- 主库保存 `request_content_audits`、`request_content_assets`、`request_content_objects` 三类轻量元数据；正文、原图和缩略图保存到审计目录，使用 Zstandard 分块压缩和 AES-GCM 分块加密。
- 为了让前端可以直接增量解析并保持多模态字段结构，当前正文容器输出版本化 JSON envelope 的原始字节流；HTTP 响应会先完成解密、解压、尾记录和 SHA-256 校验，再以分块 `Flush` 输出，避免篡改文件先泄露部分明文。
- JSON 中的 `data:` 图片、Gemini `inline_data.data`、Claude `source.data`、`b64_json` 以及 multipart JSON 字段中的 Base64 会被替换为资源引用；原件按 SHA-256 去重保存，并生成受像素上限保护的 PNG 缩略图。列表、预览和管理员脱敏接口均不会返回 Base64。
- 写入失败会生成 `capture_status=failed` 的元记录并记录错误代码；正常请求默认继续执行，系统日志保留失败告警。文件提交失败、数据库回滚、Multipart 临时文件和异常临时文件清理均不改变现有计费、重试和响应流程。
- Root 设置使用 `request_content_audit.*` 配置键，默认关闭审计和用户自查；启用前必须配置稳定的 `CRYPTO_SECRET` 或 `SESSION_SECRET`。已有记录时禁止直接切换 `storage_path`，避免旧正文失去可读根目录；非 Root 管理员只有被 Root 加入白名单后才能查看脱敏正文，当前默认不提供其他用户图片缩略图和原件。
- 所有请求记录 JSON/流式接口均设置 `private, no-store`；原件接口强制下载并设置 `nosniff`，避免用户上传的 HTML/SVG 通过应用源执行。
- 前端 `/request-records` 使用元数据分页，详情正文使用 `ReadableStream` 并限制浏览器展示状态为 8 MiB，图片使用受权限保护的缩略图 Blob 懒加载；侧边栏顺序为“使用日志 → 请求记录 → 任务日志”，双向跳转会通过 `request_id` 定位并短时闪烁目标行。
