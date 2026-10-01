# NewAPI 全栈优化审查报告

- **审查日期**：2026 年 8 月 13 日
- **审查方式**：只读静态代码审查
- **审查范围**：前端 UI、可访问性、用户旅程、前端状态、后端逻辑、安全权限、计费支付、性能数据层、测试发布与运维
- **审查对象**：NewAPI 当前工作区代码
- **审查结论**：功能覆盖面较完整，但高风险集中在首次初始化、支付账本、额度并发扣减、异步任务补偿、权限边界和发布可靠性
- **代码状态**：本次审查未修改业务代码；本报告排除了此前 XIMOYA 版本展示相关的未提交改动

> 本报告用于后续选择性修复和排期。报告中的 P0/P1/P2 表示修复优先级，不代表所有问题都已经通过线上攻击、真实支付或浏览器真机验证。每条问题都尽量附带文件、行号、触发条件和代码证据；标记为“需要运行时验证”的项目，应在修复前补充集成测试或浏览器验收。

---

## 目录

1. [审查方法与优先级定义](#审查方法与优先级定义)
2. [总体结论](#总体结论)
3. [P0 立即处理项](#p0-立即处理项)
4. [P1 高优先级优化项](#p1-高优先级优化项)
5. [P2 中长期优化项](#p2-中长期优化项)
6. [按领域整理的问题清单](#按领域整理的问题清单)
7. [推荐修复路线](#推荐修复路线)
8. [建议建立的统一基础能力](#建议建立的统一基础能力)
9. [修复验收总清单](#修复验收总清单)
10. [审查限制与后续工作](#审查限制与后续工作)

---

## 审查方法与优先级定义

本次并发派出 8 个专项审查方向：

1. UI、视觉、一致性与无障碍
2. 普通用户与管理员用户旅程
3. 前端状态、请求、路由与国际化
4. Go 后端逻辑可靠性
5. 安全、认证、授权与敏感数据
6. 计费、额度、支付、订阅与账单
7. 性能、数据库、缓存与可扩展性
8. 测试、CI/CD、Docker、发布与运维

### 优先级定义

| 优先级 | 定义 | 处理建议 |
|---|---|---|
| P0 | 可能造成接管、资金损失、额度透支、数据泄露、服务不可用或高概率资源耗尽 | 进入最近一次修复窗口，修复前不要扩大部署规模 |
| P1 | 影响核心业务可靠性、管理员安全操作、账务一致性、用户信任或规模化性能 | 近期排期，最好在下一个稳定版本完成 |
| P2 | 体验、维护性、可观测性、成本或长期扩展性问题 | 结合版本计划持续治理 |

### 置信度说明

- **高**：代码路径和触发条件清楚，可以直接从源码确认
- **中高**：代码证据充分，但需要特定数据库、部署拓扑或浏览器环境复现
- **中**：存在明显风险，需补充集成测试或运行时数据确认

---

## 总体结论

### 当前最危险的五个区域

1. **首次初始化**：未初始化实例可能被首个访问者抢先创建 Root 管理员。
2. **支付与账务**：支付成功、回调处理、订单状态、额度入账和退款补偿之间缺少统一可靠状态机。
3. **额度并发扣减**：多个请求可能同时通过余额检查，再执行无条件扣减。
4. **异步任务**：任务提交、落库、轮询、结算、退款之间存在状态覆盖、重复创建和补偿缺失。
5. **权限与错误状态**：前端经常把失败显示成空数据、默认配置或余额 0；细粒度权限也没有完全映射到操作界面。

### 最值得先做的十项

1. 给首次初始化增加一次性 Bootstrap/Setup Token，并让初始化过程事务化。
2. 建立支付事件表、订单状态机、幂等键、入账事务和失败补偿队列。
3. 使用条件更新或行锁实现钱包、令牌和订阅额度的原子预扣。
4. 建立可持久化、可重试、可幂等的退款和差额补扣账本。
5. 修复 Midjourney 图片接口鉴权、Footer HTML 注入、OAuth/支付敏感日志和 SSRF。
6. 修复负数分页、无界日志/看板查询以及上游请求无限等待。
7. 统一前端“加载失败、业务失败、空数据、默认配置”的状态模型。
8. 给批量渠道操作、模型同步、余额刷新和支付流程增加预览、确认、进度和结果。
9. 建立真实 readiness、数据库迁移锁、发布质量门、备份恢复和指标体系。
10. 修复无障碍主内容地标、移动菜单焦点、浅色主题对比度和共享组件英文文案。

---

# P0 立即处理项

> P0 是本报告中最需要优先进入修复队列的问题。支付、额度、初始化和安全问题建议先冻结相关扩展功能，避免在风险未收敛时继续增加复杂度。

## P0-01 首次初始化可被匿名请求抢占 Root 管理员

- **领域**：安全、初始化、用户旅程
- **位置**：
  - `router/api-router.go:22-24`
  - `controller/setup.go:46-160`
  - `model/user.go:1442-1449`
- **问题**：`POST /api/setup` 匿名开放。接口检查系统是否已初始化后，直接创建 `RoleRootUser`，没有一次性安装密钥、安装者声明、回环地址限制或跨进程初始化锁。
- **触发条件**：新实例已经绑定公网或共享网络，但管理员还没有完成初始化。
- **影响**：攻击者可以抢先创建 Root 账户并完全接管系统。并发请求还可能同时通过“尚未初始化”的检查。
- **修复建议**：
  1. 增加一次性 `SETUP_TOKEN` 或启动时生成的 Bootstrap Token。
  2. Token 只允许使用一次，成功后立即失效。
  3. 初始化接口使用数据库事务和数据库级锁，避免检查与创建之间的竞态。
  4. 可选增加仅回环地址/显式 `ALLOW_REMOTE_SETUP=true` 的部署保护。
  5. 未配置初始化密钥时，如果服务监听非回环地址，启动阶段给出明确警告或拒绝启动。
- **验收标准**：
  - 未提供正确 Setup Token 时，所有初始化写请求均拒绝。
  - 两个并发初始化请求最多一个成功。
  - 初始化失败不会留下半成品 Root 或“已初始化”标记。
  - 初始化完成后旧 Token 无法再次使用。

## P0-02 支付入口和支付回调启用条件不一致，可能付款后永不入账

- **领域**：支付、充值、订阅
- **位置**：
  - `controller/topup_stripe.go:64-125,147-153`
  - `controller/topup.go:189-315`
  - `controller/topup_creem.go:66-165`
  - `controller/topup_waffo.go:143-148`
  - `controller/payment_webhook_availability.go:14-109`
  - `controller/subscription_payment_stripe.go:43-53`
  - `controller/subscription_payment_creem.go:53-60`
  - `controller/subscription_payment_waffo_pancake.go:43-53,76-92`
- **问题**：创建支付订单时使用一套“产品配置/合规开关”判断；回调时又根据当前钱包网关是否启用来决定是否接受回调。管理员在支付完成前关闭钱包配置，或者只配置订阅产品而没有钱包充值产品时，历史回调可能直接被拒绝。
- **触发条件**：用户已打开支付页或已付款，管理员随后关闭支付配置、删除充值产品配置，或只配置套餐级产品。
- **影响**：用户已经付款，但 webhook 返回拒绝，订单永久 pending，订阅无法激活，资金和本地权益无法对应。
- **修复建议**：
  1. “是否允许创建新订单”和“是否允许完成历史待处理订单”拆成两个判断。
  2. 回调只依赖签名、事件幂等键和本地待处理订单，不因当前停售而拒绝历史订单。
  3. 订阅回调使用订阅订单自己的产品快照，不依赖钱包充值产品配置。
  4. 订单保存创建时的网关、产品、币种、金额和配置快照。
- **验收标准**：
  - 关闭新订单入口后，旧订单仍可以合法完成回调。
  - 仅配置订阅产品也能完成订阅回调。
  - 关闭支付渠道不会影响已经验签且存在的待处理订单。

## P0-03 Stripe 入账失败仍返回 HTTP 200，支付商不会重试

- **领域**：支付可靠性
- **位置**：`controller/topup_stripe.go:176-189,273-284`
- **问题**：`fulfillOrder` 内部充值或订阅完成失败时只写日志，外层 webhook 无论本地处理是否成功都返回 200。
- **触发条件**：数据库暂时故障、订单不存在、计划配置错误、并发状态异常或本地事务提交失败。
- **影响**：Stripe 认为事件已经成功处理，不再重试；用户可能永久付款成功但未充值/未开通订阅。
- **修复建议**：
  1. 处理函数返回明确错误。
  2. 只有“本地已成功入账”或“已确认幂等完成”时返回 2xx。
  3. 失败事件先持久化，再由后台补偿。
  4. 事件表使用 provider event ID 唯一约束，避免重复消费。
- **验收标准**：
  - 数据库失败时返回非 2xx 或进入持久化待重试状态。
  - 同一 Stripe event 重试不会重复充值。
  - 管理后台可以查看事件状态、失败原因和下一次重试时间。

## P0-04 易支付回调先确认成功，再分步入账

- **领域**：支付、并发、分布式部署
- **位置**：`controller/topup.go:268-407`、`model/topup.go:72-80`
- **问题**：回调在真正更新订单和用户额度前就写出 `success`；订单状态更新、用户加额和回调响应不在同一事务，进程内锁也无法覆盖多实例。
- **触发条件**：支付商重复通知、负载均衡多实例、进程在入账中途崩溃、用户额度更新失败。
- **影响**：跨实例重复通知可能双倍充值；订单已经标记成功但额度没有增加；支付商也不会再次重试。
- **修复建议**：
  1. 用支付商交易 ID做全局幂等键。
  2. 在数据库事务内完成“锁定订单、校验 pending、更新订单、更新用户额度、写账本”。
  3. 事务提交后才返回成功。
  4. 失败写入补偿表，后台重试。
- **验收标准**：
  - 100 次重复回调只产生一次额度变更。
  - 多实例同时处理同一订单不会双充。
  - 额度更新失败时订单仍可恢复，不会出现永久成功但无额度。

## P0-05 钱包和令牌预扣存在并发透支

- **领域**：计费、额度、并发
- **位置**：
  - `service/billing_session.go:354-380`
  - `service/funding_source.go:36-44`
  - `service/quota.go:397-405`
  - `model/user.go:1286-1305`
  - `model/token.go:435-462`
- **问题**：请求先读取余额判断，再执行没有余额条件的扣减。多个请求可以同时读到相同余额，各自通过检查后进行无条件更新。
- **触发条件**：同一用户或令牌并发发起多个高价请求，尤其是流式请求、图片任务和异步任务。
- **影响**：钱包或令牌余额可能变成负数，用户得到超过实际额度的服务，账务和用量日志不一致。
- **修复建议**：
  1. 数据库使用 `UPDATE ... WHERE quota >= ?`，并检查 `RowsAffected`。
  2. 同一账务事务内锁定用户/令牌行，建立 reservation 记录。
  3. Redis 预扣使用 Lua 原子脚本，不要用普通 pipeline 模拟原子性。
  4. 预扣、结算、退款都使用同一个业务 request/task ID。
- **验收标准**：
  - 并发压力下余额永不低于允许下限。
  - 只有一个请求能成功占用最后一段额度。
  - DB、Redis、日志和 reservation 状态可以对账。

## P0-06 订阅差额结算失败后仍完成请求

- **领域**：订阅额度、账务
- **位置**：
  - `service/billing_session.go:47-78`
  - `service/funding_source.go:104-109`
  - `model/subscription.go:1507-1523`
  - `service/quota.go:354-381`
- **问题**：预扣额度小于实际输出时，订阅差额扣减失败只记录日志，仍写消费日志并把请求视为成功。
- **触发条件**：长输出、估算偏低、订阅余额接近耗尽、数据库或缓存短暂失败。
- **影响**：用户实际使用了服务但没有完整扣费，订阅账本、消费日志和真实额度不一致。
- **修复建议**：
  1. 预留可计费上限，避免差额无法支付。
  2. 差额不足时定义明确策略：钱包兜底、欠费账本、拒绝后续输出或冻结账户。
  3. 差额失败写入 durable settlement/reconcile 任务。
  4. 消费日志记录 `settlement_pending`，不要伪装成完全结算成功。
- **验收标准**：
  - 差额扣减失败不会被静默吞掉。
  - 管理员可以查看待补扣记录。
  - 补扣重试幂等，不会重复扣费。

## P0-07 Stripe recurring 订阅缺少完整生命周期处理

- **领域**：订阅、支付
- **位置**：
  - `controller/subscription_payment_stripe.go:115-126`
  - `controller/topup_stripe.go:176-186`
  - `model/subscription.go:214-228`
- **问题**：Checkout 使用订阅模式，但 webhook 主要处理 checkout 事件，没有完整处理 invoice、subscription 更新、续费失败、取消、退款和争议，也没有可靠保存 provider subscription ID。
- **触发条件**：首期订阅到期自动续费、续费失败、Stripe 侧取消、退款或拒付。
- **影响**：Stripe 已自动扣款但本地订阅不续期；或外部已退款而本地权益继续有效。
- **修复建议**：
  1. 如果产品是一次性期限包，改成一次性支付模式。
  2. 如果产品是真订阅，保存 subscription ID、invoice ID、customer ID。
  3. 处理续费成功、续费失败、取消、退款、争议和支付失败事件。
  4. 本地权益必须由支付状态机驱动，不直接依赖首期 checkout。
- **验收标准**：
  - 首期、续费、取消、失败、退款和拒付均有对应本地状态。
  - provider 重复事件不会重复延长权益。
  - 本地订阅状态可与支付商后台对账。

## P0-08 异步任务提交、结算、日志和落库不是可靠提交链

- **领域**：异步任务、任务计费
- **位置**：
  - `controller/relay.go:524-606`
  - `relay/channel/task/kling/adaptor.go:209-215`
  - `relay/relay_task.go:175-178,213-240`
- **问题**：上游任务可能已成功创建，控制器随后才做结算、写日志和插入任务；其中任一步失败只记录日志，客户端可能已经拿到成功 task ID。提交重试也没有稳定的上游幂等键。
- **触发条件**：上游响应成功但本地 DB 写入失败、网络断开导致响应解析失败、`RetryTimes > 0`。
- **影响**：本地没有任务可查询或退款；重试可能在上游创建多个任务，用户被重复计费或产生重复任务。
- **修复建议**：
  1. 先持久化 `SUBMITTING` 状态的本地任务。
  2. 对支持的平台传递稳定幂等键。
  3. 控制器完成本地状态推进后再返回最终成功响应。
  4. 对不支持幂等的任务 POST 默认不重试。
  5. 建立任务恢复扫描和人工对账入口。
- **验收标准**：
  - 本地任务记录缺失时，系统能从提交状态恢复。
  - 网络重试不会重复创建上游任务。
  - 任务成功、失败、退款和结算状态可追踪。

## P0-09 负数分页可能形成无界查询资源耗尽

- **领域**：性能、资源保护
- **位置**：
  - `common/page_info.go:41-81`
  - `model/log.go:511,602`
  - `controller/task.go:32,56`
- **问题**：分页只限制了过大的 `page_size`，没有拒绝负数。GORM 的 `Limit(-1)` 语义是取消 LIMIT。
- **触发条件**：手工构造 `page_size=-1` 查询日志、任务或其他列表接口。
- **影响**：用户可能拉取全部个人日志，管理员可能拉取全库数据，造成 DB 扫描、内存、序列化、带宽和 GC 压力。
- **修复建议**：分页解析层统一要求 `page_size` 为正整数，限制 `page`、`page * page_size` 乘法溢出，并为所有列表接口复用同一校验。
- **验收标准**：负数、0、超大值和整数溢出输入均返回明确 400；正常分页行为不变。

## P0-10 上游请求没有继承客户端 Context，可能无限等待

- **领域**：性能、可靠性、资源保护
- **位置**：
  - `relay/channel/api_request.go:320-379,497-595`
  - `service/http_client.go:74-109`
  - `common/init.go:108-114`
- **问题**：多处使用 `http.NewRequest` 而不是绑定客户端请求 Context；默认总超时可能为 0，Transport 也缺少响应头超时和每 Host 连接上限。
- **触发条件**：客户端断开、上游慢响应、代理黑洞、上游连接半开或未配置 `RELAY_TIMEOUT`。
- **影响**：请求、连接、文件描述符和 goroutine 长时间占用，故障上游可能放大成网关资源耗尽。
- **修复建议**：
  1. 使用 `http.NewRequestWithContext(c.Request.Context(), ...)`。
  2. 非流式请求设置总超时和响应头超时。
  3. 流式请求设置独立的首包/空闲超时。
  4. 增加按用户、渠道和全局的 in-flight 流限制。
- **验收标准**：客户端断开后上游请求能在短时间取消；慢上游会得到受控超时；连接数和 goroutine 有上限指标。

---

# P1 高优先级优化项

## 一、安全与权限

### P1-SEC-01 Midjourney 图片代理端点缺少鉴权和任务归属校验

- **位置**：`router/relay-router.go:208-228`、`relay/mjproxy_handler.go:29-93`、`model/midjourney.go:117-125`
- **问题**：图片路由在 TokenAuth 之前注册，处理器按全局 `mj_id` 查询，不验证任务所属用户。
- **影响**：知道或猜到任务 ID 的访问者可能读取其他用户的图片；接口还可能被滥用为出站资源拉取器。
- **建议**：将路由放入鉴权组；按 `user_id + mj_id` 查询；公开分享使用短期签名 URL、限速和安全 MIME 白名单。
- **验收**：用户 A 无法读取用户 B 的任务；无效/过期签名返回 404 或 403；图片下载有速率和大小限制。

### P1-SEC-02 Footer 原始 HTML 注入形成持久化 XSS

- **位置**：`router/api-router.go:191-196`、`controller/option.go:366-378`、`model/option.go:514-515`、`controller/misc.go:53-70`、`web/src/components/layout/components/footer.tsx:225-238`
- **问题**：Root 可以设置 Footer HTML，公开接口返回后，前端使用 `dangerouslySetInnerHTML` 直接注入。
- **影响**：恶意事件属性、危险 URL、SVG 或脚本可以在所有访客同源页面执行，影响访问令牌和管理操作。
- **建议**：优先改成纯文本；如果必须支持 HTML，统一使用严格 DOMPurify 白名单，禁止事件属性、脚本、危险协议、SVG 活动内容，并增加 CSP/Trusted Types。
- **验收**：XSS payload 在所有主题和公开页面均不执行；允许的链接、加粗、换行仍正常显示。

### P1-SEC-03 自定义 OAuth Discovery 存在 SSRF

- **位置**：`router/api-router.go:212-221`、`controller/custom_oauth.go:141-210`
- **问题**：只检查 URL scheme 和 host，随后使用普通 `http.Client` 请求，没有复用现有 SSRF 防护，也没有逐跳限制重定向。
- **影响**：Root 可请求内网服务、云元数据地址或管理接口，并把响应 JSON 返回给调用者。
- **建议**：使用统一 SSRF 安全客户端；解析每一次重定向；限制响应大小；默认只允许 HTTPS；必要时配置 issuer 域名白名单。
- **验收**：私网、回环、链路本地、云元数据和重定向到上述地址均被拒绝；公网合法 issuer 仍可使用。

### P1-SEC-04 渠道模型预览存在 SSRF 出站风险

- **位置**：`router/channel-router.go:61-63`、`controller/channel.go:1196-1316`、`controller/channel_upstream_update.go:307-324`、`controller/channel.go:473-538`
- **问题**：渠道管理者可提交任意 `base_url` 获取上游模型，未纳入 SSRF 出站限制；若结合 Header Override，风险更高。
- **影响**：受委派管理员可能探测或访问应用所在网络的内部服务。
- **建议**：将临时上游 URL 接入 SSRF 防护；非 Root 角色使用域名/网段允许列表；自建内网上游必须显式配置。
- **验收**：所有内部保留地址按策略阻断；允许列表中的自建上游可以正常测试；请求日志不泄露敏感 Header。

### P1-SEC-05 OAuth Token 响应被写入 Debug 日志

- **位置**：`oauth/generic.go:146-164`、`logger/logger.go:55-68,88-95`
- **问题**：Debug 模式记录 Token endpoint 响应体前 500 字符，响应体可能含 `access_token`、`refresh_token`、`id_token`。
- **影响**：日志读取者或日志平台可能直接获得第三方 OAuth 凭据。
- **建议**：只记录状态码、provider、请求 ID 和白名单字段；立即排查和轮换历史可能泄露的 Token；日志权限收紧。
- **验收**：任何日志等级都不输出 Token 字段；单元测试使用敏感字段断言日志中不存在原文。

### P1-SEC-06 Token、渠道密钥和 OAuth Secret 明文持久化

- **位置**：`model/token.go:14-18,287-308`、`model/channel.go:23-27`、`model/custom_oauth_provider.go:40-50`
- **问题**：用户 API Token、渠道密钥和 OAuth Client Secret 直接保存为普通字符串；Token 还按明文 key 查询。
- **影响**：数据库、备份或只读账号泄露会立即暴露所有凭据。
- **建议**：
  1. 渠道和 OAuth Secret 使用 KMS/主密钥信封加密。
  2. 用户 Token 改成“公开前缀 + HMAC/哈希检索”。
  3. 明文只在创建时展示，增加轮换和迁移方案。
- **验收**：数据库导出中不存在可直接使用的完整密钥；旧 Token 迁移后仍能验证且可以轮换。

### P1-SEC-07 pprof 端口默认开放所有网卡且无鉴权

- **位置**：`main.go:41,184-190`
- **问题**：`ENABLE_PPROF=true` 时监听 `0.0.0.0:8005`，没有独立鉴权或网络白名单。
- **影响**：泄露堆、goroutine、CPU 和执行信息，也可能被用于资源消耗。
- **建议**：默认绑定 `127.0.0.1`；支持显式 debug listen 地址；生产环境只通过管理网络、mTLS、反向代理认证或网络策略访问。
- **验收**：外部网卡无法访问；开发者仍能通过回环或端口转发使用 pprof。

### P1-SEC-08 Cookie Secure 默认关闭，Origin 防护也随之跳过

- **位置**：`common/constants.go:39-42`、`common/session_cookie.go:44-55`、`service/auth_session.go:303-312`、`middleware/auth_origin.go:18-23`
- **问题**：`SessionCookieSecure` 默认 false，Refresh Cookie 的 Secure 标志跟随配置；Origin Guard 在非 Secure 模式直接放行。
- **影响**：生产存在 HTTP 可达路径时，Refresh Cookie 可能暴露，刷新/登出接口也失去 Origin 校验。
- **建议**：非回环生产监听强制 Secure；配合 HSTS、`__Host-` Cookie 和可信反向代理配置。
- **验收**：生产配置缺少 Secure 时启动警告或失败；HTTPS 下 Cookie 包含 Secure、HttpOnly、SameSite 等正确属性。

### P1-SEC-09 默认信任全部私网反向代理地址

- **位置**：`middleware/trusted_proxies.go:13-26`、`middleware/auth.go:427-440`、`middleware/rate-limit.go:109-115`
- **问题**：未配置 `TRUSTED_PROXIES` 时信任 RFC1918 和 IPv6 ULA 网段。
- **影响**：共享 VPC/私网中的主体可能伪造转发 IP，绕过 IP 白名单、限流或审计。
- **建议**：生产必须显式配置精确代理 CIDR；未配置时默认不信任；入口代理负责覆盖客户端转发头。
- **验收**：非受信私网请求无法伪造真实客户端 IP；受信代理场景仍能正确获取源地址。

### P1-SEC-10 支付 Webhook 原始 body 和签名进入日志

- **位置**：
  - `controller/topup_stripe.go:155-169`
  - `controller/topup_creem.go:239-268`
  - `controller/topup_waffo.go:357-368`
  - `controller/topup_waffo_pancake.go:453-457`
  - `logger/logger.go:55-68`
- **问题**：多个支付回调直接记录原始请求体和签名。
- **影响**：支付用户信息、订单元数据和签名材料进入文件或日志平台。
- **建议**：只记录事件 ID、订单号、校验结果和哈希摘要；禁止完整 body/signature；收紧日志权限和保留周期。
- **验收**：日志中不出现付款邮箱、签名原文、完整 webhook body；失败诊断仍能依靠事件 ID完成。

### P1-SEC-11 自定义 OAuth Token/UserInfo endpoint 缺少 HTTPS 和 SSRF 策略

- **位置**：`controller/custom_oauth.go:214-252,341-354`、`oauth/generic.go:113-137,205-218`
- **问题**：端点原样保存，后续会向其发送 Client Secret、授权码或 Access Token。
- **影响**：误配 HTTP、内网地址或恶意端点时，凭据会被泄露。
- **建议**：保存时强制 HTTPS；禁止内网/保留地址；限制重定向；优先从受信 issuer discovery 获取端点。

### P1-SEC-12 Compose 默认弱口令、可变镜像、Root 运行

- **位置**：`docker-compose.yml:19,23-34,68-83`、`Dockerfile:30-41`
- **问题**：存在 `root:123456`、Redis `123456`、可变 `latest` 镜像，运行镜像未切换非 Root 用户。
- **影响**：容易被直接照抄到生产，构建不可复现，容器被攻破后的权限面过大。
- **建议**：密码改为必须注入的 secrets；镜像使用版本和 digest；运行镜像创建低权限用户。

### P1-SEC-13 `channel.secret_view` 权限没有真正生效

- **位置**：`service/authz/resources_channel.go:45-53`、`router/channel-router.go:23-29`、`web/src/features/channels/components/drawers/channel-mutate-drawer.tsx:638,3064-3121`
- **问题**：权限模型定义了 `secret_view`，但后端接口使用 RootAuth，前端判断超级管理员角色。
- **影响**：配置中授予该权限的管理员仍然看不到密钥，形成误导性权限项。
- **建议**：真正使用 `RequirePermission(ChannelSecretView)` 并保留二次验证，或删除该无效权限。

---

## 二、后端逻辑与异步任务

### P1-LOGIC-01 渠道缓存读取失败会用空快照覆盖健康缓存

- **位置**：`model/channel_cache.go:31-44,79-103,106-112`
- **问题**：查询 `channels`/`abilities` 时忽略 DB 错误，随后无条件替换全局缓存。
- **影响**：数据库瞬断会让全部可选渠道短时间消失，表现为全站无可用渠道。
- **建议**：两份查询都成功后才原子 swap；失败时保留 last-known-good 快照，并记录告警。

### P1-LOGIC-02 配置热更新与请求读取共享可变结构

- **位置**：`model/option.go:279-292,610-645`、`setting/config/config.go:203-269,280-282`、`setting/operation_setting/general_setting.go:40-42`、`relay/channel/api_request.go:524-527`
- **问题**：管理员更新配置时反射写入字段，请求侧直接读取同一指针，没有共同锁或整体快照交换。
- **影响**：可能出现数据竞态，以及多字段配置半更新。
- **建议**：构建不可变配置副本，用 `atomic.Value` 整体替换；getter 返回值副本；批量配置一次性校验和发布。

### P1-LOGIC-03 单项配置更新忽略 DB 写失败并返回假成功

- **位置**：`model/option.go:222-239`、`controller/option.go:366-378`
- **问题**：`FirstOrCreate`/`Save` 错误未检查，内存仍然更新并返回成功。
- **影响**：当前节点暂时生效，其他节点不生效；后续同步可能覆盖，形成配置漂移。
- **建议**：检查所有 DB 错误；使用事务/Upsert；DB 成功后再更新内存。

### P1-LOGIC-04 异步任务退款失败后无持久化补偿

- **位置**：`service/task_billing.go:166-203`、`service/task_polling.go:573-599`、`model/task.go:311-319`
- **问题**：任务已进入 FAILURE 终态后不再轮询，退款失败只记录日志，没有 durable retry。
- **影响**：用户任务失败但预扣额度永久滞留。
- **建议**：任务状态与退款状态分离；建立幂等 refund ledger 和补偿扫描队列。

### P1-LOGIC-05 任务状态失败更新可能覆盖并发成功终态

- **位置**：`service/task_polling.go:153-163,217-235,390-407`、`model/task.go:431-442`
- **问题**：轮询缓存缺失或渠道读取失败时，用无 CAS 的批量更新直接置 FAILURE。
- **影响**：并发轮询可能刚刚成功，随后又被失败路径覆盖；退款流程也可能被跳过。
- **建议**：使用逐条 CAS 状态推进；只有获胜者执行退款；缓存暂时失败应进入可重试状态，而不是直接终态失败。

### P1-LOGIC-06 任务 token 重算忽略提交时计费快照

- **位置**：快照定义 `model/task.go:115-123`，写入 `controller/relay.go:593-600`；重算 `service/task_billing.go:285-326`
- **问题**：结算时重新读取当前倍率，且把使用组当作用户组使用特殊倍率。
- **影响**：价格配置热更新后，补扣/退款与提交时预扣不一致。
- **建议**：完整使用 `BillingContext` 的模型价格、模型倍率、分组倍率和附加倍率；旧任务无快照时才回退当前配置。

### P1-LOGIC-07 Suno 状态没有转换为内部大写状态

- **位置**：`dto/suno.go:19-28`、`service/task_polling.go:283-297`、`model/task.go:34-42`
- **问题**：DTO 文档约定小写 `submitted/processing/success/failed`，内部状态机使用大写常量，轮询直接写入。
- **影响**：成功任务可能不会标记完成，失败任务也可能不触发退款。
- **建议**：增加显式状态映射；未知状态进入可重试错误并记录原始值。

### P1-LOGIC-08 任务轮询 HTTP 请求缺少取消上下文和强制 deadline

- **位置**：`service/task_polling.go:359-375,441-470`、`relay/channel/task/suno/adaptor.go:131-149`、`service/http_client.go:101-109`
- **问题**：轮询适配器没有传递 Context，未配置超时时可能无限等待。
- **影响**：半开连接长期占用轮询 goroutine，任务无法及时推进。
- **建议**：接口传入 `context.Context`，使用 `NewRequestWithContext`，轮询设置独立 deadline。

### P1-LOGIC-09 敏感词命中路径复用 nil err，返回 500

- **位置**：`controller/relay.go:139-145`、`relaykit/types/error.go:244-263`
- **问题**：敏感词命中时使用已为 nil 的 `err` 创建错误，默认状态码变成 500。
- **影响**：正常业务拒绝被伪装成系统故障，客户端可能错误重试。
- **建议**：创建明确的 400/403 业务错误，附加不可重试标记。

### P1-LOGIC-10 异步任务资金和令牌额度调整可部分成功

- **位置**：`service/task_billing.go:102-119,172-203,231-274`
- **问题**：资金调整成功后令牌调整失败只记录日志，随后可能清零任务 quota 或写消费日志。
- **影响**：钱包、订阅、令牌和任务账本长期不一致。
- **建议**：同库变更使用事务；跨库变更通过 durable compensation；失败前不要清理任务 quota 标记。

### P1-LOGIC-11 渠道删除或缓存失败会直接终止付费任务且不退款

- **位置**：`service/task_polling.go:217-235,390-407`、`model/task.go:431-442`
- **问题**：渠道消失或缓存短暂失败时直接把任务置 FAILURE，跳过退款。
- **影响**：管理员删除渠道或缓存故障可能导致付费任务失败且额度滞留。
- **建议**：渠道暂时不可用时进入 retryable 状态；终态推进和退款必须由 CAS 获胜者执行。

### P1-LOGIC-12 Suno 非 200 响应未及时关闭 Body

- **位置**：`service/task_polling.go:242-253`、`relay/relay_task.go:220-227`
- **问题**：状态码检查发生在 `defer resp.Body.Close()` 之前，非 200 分支可能遗漏关闭。
- **影响**：持续 429/5xx 时连接池逐步耗尽。
- **建议**：收到非 nil response 后第一时间注册 Close，再处理状态码；增加连接泄漏测试。

### P1-LOGIC-13 自动恢复渠道后未重新加入内存选路候选集

- **位置**：`model/channel_cache.go:271-294`、`service/channel.go:36-43`、`controller/channel-test.go:992-995`
- **问题**：禁用时从候选索引删除，自动恢复只更新状态，没有立即重建能力索引。
- **影响**：渠道已恢复但在下一次周期同步前不会承接请求。
- **建议**：恢复状态时增量加入该渠道，或统一通过 `UpdateChannelStatus` 刷新选路缓存。

### P1-LOGIC-14 JSON Scanner 只接受 []byte

- **位置**：`model/channel.go:169-173`、`model/task.go:87-94,149-155`
- **问题**：Scanner 类型断言失败时直接使用 nil 数据。
- **影响**：不同数据库驱动返回 string 时，渠道/任务 JSON 字段可能丢失或查询失败。
- **建议**：支持 `[]byte`、`string`、`nil` type switch；未知类型返回明确错误。

### P1-LOGIC-15 Suno JSON 比较通过排序原始字节，存在误判

- **位置**：`service/task_polling.go:335-346`
- **问题**：按字节排序后比较 JSON，字符集合相同但语义不同的内容可能被判断为相同。
- **影响**：任务 URL、结果字段更新可能不落库。
- **建议**：直接比较稳定序列化结果，或反序列化后做语义深比较。

### P1-LOGIC-16 异步任务重试可能重复创建上游任务

- **位置**：`controller/relay.go:524-571`、`relay/relay_task.go:175-240`、`relay/channel/task/kling/adaptor.go:147-158`
- **问题**：本地 task ID 不是上游稳定幂等键，响应丢失时按 RetryTimes 重试会再次创建任务。
- **影响**：上游出现多个任务，本地只保存最后一个 ID，用户可能被重复计费。
- **建议**：不支持幂等的任务创建禁止自动重试；支持的平台传递稳定幂等键；本地使用 `SUBMITTING` 恢复流程。

### P1-LOGIC-17 速率限制器可能长期保留低活跃 key

- **位置**：`common/rate-limit.go:14-42`
- **问题**：内存限流器周期扫描和队列尾部清理无法保证所有低活跃 key 被及时淘汰。
- **影响**：攻击者构造大量不同 key 后，map 长期增长。
- **建议**：使用带 TTL 的分片缓存、定期全量清理或上限淘汰；记录 key 数和清理耗时。

---

## 三、计费、额度、订阅与账单

### P1-BILL-01 支付金额没有单一事实源

- **位置**：`model/topup.go:14-24,143-145`、`model/subscription.go:214-228`、`controller/topup.go:228-252`、`controller/topup_stripe.go:88-111,356-415`、`controller/topup_creem.go:322-350`
- **问题**：金额使用 `float64`；网关金额格式化、优惠、税费、Stripe Price ID 和本地金额可能走不同公式。
- **影响**：支付页面、实际扣款、订单记录和到账额度可能不一致，无法可靠对账。
- **建议**：统一保存 `currency + minor_units(int64)`、预期金额、实际金额、产品 ID、入账额度和交易 ID。

### P1-BILL-02 负数倍率或兑换码额度可能造成负额度

- **位置**：`common/topup-ratio.go:25-30`、`controller/option.go:156-366`、`controller/topup_stripe.go:88-95,388-395`、`controller/redemption.go:64-168`、`model/redemption.go:137-179`
- **问题**：充值倍率和兑换码额度缺少统一正数、有限值和上限校验。
- **影响**：异常配置可能让支付后获得负额度，或负额度兑换码直接扣减用户余额。
- **建议**：服务端统一限制所有倍率/额度为有限正数并设置上限；订单创建和回调完成时再次验证快照。

### P1-BILL-03 支付/兑换直接写库，缓存更新存在时序漂移

- **位置**：`model/topup.go:122-150,405-455,480-516,543-577`、`model/redemption.go:151-179`、`model/user_cache.go:83-91,146-155`
- **问题**：部分支付/兑换路径直接更新用户 quota，没有同步缓存；通用额度增减函数又可能在 DB 成功前异步更新 Redis。
- **影响**：刚充值后可能仍显示余额不足；DB 失败时缓存可能暂时多额度。
- **建议**：统一进入账本服务；DB 事务提交后通过 outbox/invalidate 更新缓存，禁止 DB 成功前改变授权缓存。

### P1-BILL-04 退款/回滚不是可重试的持久化流程

- **位置**：`service/billing_session.go:81-122`、`service/funding_source.go:57-64`、`controller/relay.go:173-181`、`service/task_billing.go:166-203`
- **问题**：同步请求退款使用异步 goroutine，异步任务退款失败后没有持久化记录。
- **影响**：节点崩溃或短暂故障后，失败请求可能永久不退款。
- **建议**：使用 request/task 级 refund ledger、唯一业务键、状态机和后台重试；退款操作必须幂等。

### P1-BILL-05 订阅订单没有保存套餐权益快照

- **位置**：`model/subscription.go:214-228,484-621`
- **问题**：订单只保存 Plan ID/Money，回调完成时重新读取当前套餐。
- **影响**：管理员修改计划后，已付款用户可能得到与付款页面不同的额度、时长、分组或重置规则。
- **建议**：下单时持久化价格、币种、额度、时长、分组、重置、产品 ID 等不可变快照；完成时只按快照履约。

### P1-BILL-06 套餐购买上限可能被并发回调突破

- **位置**：`controller/subscription_payment_stripe.go:67-76`、`model/subscription.go:494-503`、`controller/topup.go:278-307`
- **问题**：购买次数使用普通 count，不同订单之间没有用户/套餐级 reservation 或唯一约束。
- **影响**：`MaxPurchasePerUser=1` 时并发支付仍可能获得多份订阅。
- **建议**：创建订单时占用购买名额，或完成事务时锁定用户/计划计数行并使用唯一约束。

### P1-BILL-07 额度使用 int，可能触发数据库 INT 边界

- **位置**：`common/constants.go:26`、`model/user.go:95-102`、`model/token.go:23-28`、`model/topup.go:498-512`
- **问题**：钱包、令牌和日志额度字段使用 `int`，默认 `QuotaPerUnit=500,000`，高额充值会接近或超过 32 位数据库 INT 上限。
- **影响**：写入失败、截断或变负数。
- **建议**：统一迁移到 int64/BIGINT；所有边界转换使用饱和和错误返回；增加数据库迁移回归。

### P1-BILL-08 缺少退款、拒付、争议的逆向结算链路

- **位置**：`common/constants.go:255-258`、`controller/topup_stripe.go:176-186`、`controller/topup_creem.go:275-282`、`controller/topup_waffo.go:373-387`
- **问题**：状态主要覆盖 pending/success/failed/expired，没有 refunded/disputed 等逆向状态，也没有回收额度或冻结权益流程。
- **影响**：外部款项已退款或拒付，本地额度/订阅仍继续有效。
- **建议**：建立支付账本和逆向状态机，处理 provider refund/dispute 事件，定义已用额度、欠费和冻结策略。

### P1-BILL-09 钱包预设价格未包含用户分组倍率

- **位置**：`web/src/features/wallet/lib/format.ts:78-96`、`recharge-form-card.tsx:230-276`、`controller/topup.go:149-176`
- **问题**：前端按基础 ratio 和折扣计算预览，后端实际报价还乘 `TopupGroupRatio`。
- **影响**：用户看到的预览金额与最终支付确认金额不一致。
- **建议**：由后端 `/topup/info` 返回当前用户报价快照，前端不要复制计费公式。

### P1-BILL-10 支付报价失败仍可能打开 `$0` 确认框

- **位置**：`web/src/features/wallet/hooks/use-payment.ts:60-104`、`web/src/features/wallet/index.tsx:173-190`、`payment-confirm-dialog.tsx:95-151`
- **问题**：报价请求失败时返回 0，选择支付方式后不验证金额即可打开确认框。
- **影响**：用户看到 `$0`，可能重复操作或进入错误支付状态。
- **建议**：报价必须返回明确状态；金额无效或小于等于 0 时禁止确认，显示失败原因和重试入口。

### P1-BILL-11 外部支付缺少待支付和到账确认闭环

- **位置**：`web/src/features/wallet/hooks/use-payment.ts:108-156`、`wallet/index.tsx:193-212`、`subscription-purchase-dialog.tsx:117-228`
- **问题**：打开支付页后立即关闭弹窗并刷新余额，没有订单状态、回跳处理、轮询或手动确认。
- **影响**：Webhook 尚未到账时用户以为失败，可能重复付款。
- **建议**：创建待支付订单卡，展示订单号、截止时间、刷新按钮；回站或页面重新聚焦后轮询状态。

### P1-BILL-12 API Token 批量明文复制缺少防误操作保护

- **位置**：`web/src/features/keys/components/api-keys-cells.tsx:58-74`、`api-keys-provider.tsx:76-108`、`data-table-bulk-actions.tsx:51-80`、`router/api-router.go:241-254`
- **问题**：Popover 打开就拉取完整 Token，批量复制会把所有选中 Token 写入剪贴板，缺少二次确认和内存清理。
- **影响**：屏幕共享、误点或残留选中行会暴露大量凭据。
- **建议**：单个展示和批量复制增加明确确认/二次验证；复制后清空内存中的明文；剪贴板设置短时清理提醒。

### P1-BILL-13 API Token IP 白名单非法输入会静默锁死 Token

- **位置**：`web/src/features/keys/lib/api-key-form.ts:35-97`、`api-keys-mutate-drawer.tsx:720-744`、`model/token.go:84-103`、`common/ip.go:33-50`、`middleware/auth.go:430-439`
- **问题**：前端保存原始 IP 文本，服务端解析时跳过非法条目；最终没有任何匹配项时直接拒绝所有请求。
- **影响**：用户看到保存成功，但 Token 立即全部 403。
- **建议**：前后端逐行校验和规范化；保存前展示有效条目数及当前出口 IP 匹配预览；提供测试按钮和回退窗口。

### P1-BILL-14 真实支付金额、支付产品和入账额度无法可靠对账

- **位置**：`model/topup.go`、`model/subscription.go`、各支付控制器和钱包历史组件
- **问题**：不同网关复用 `Amount/Money`，币种、支付金额、额度含义不统一，部分金额使用浮点数。
- **影响**：账单、收款和入账额度出现差异时无法判断源头。
- **建议**：拆分 `paid_amount`、`currency`、`credited_quota`、`product_type`、`plan_title`、`provider_transaction_id`，所有金额使用 minor units。

---

## 四、前端状态、错误处理与数据一致性

### P1-FE-01 `/api/status` 存在多套请求和缓存通道

- **位置**：`web/src/main.tsx:116-156`、`web/src/hooks/use-system-config.ts:105-171`、`web/src/hooks/use-status.ts:40-75`
- **问题**：启动直接 Axios、原生 fetch 和 React Query 都请求或映射 `/api/status`。
- **影响**：首屏重复请求、状态竞争、配置展示不一致。
- **建议**：建立唯一 Status DTO、Zod parser 和 React Query 数据源，所有组件消费统一映射结果。

### P1-FE-02 全局错误处理把局部 500 强制跳转 `/500`

- **位置**：`web/src/main.tsx:72-93`、`web/src/lib/http-client.ts:80-141`、`web/src/lib/handle-server-error.ts:25-50`
- **问题**：Axios、QueryCache、Mutation 默认处理可能重复 toast，并把后台局部请求失败升级为全局错误页。
- **影响**：用户丢失当前表单、筛选和页面上下文。
- **建议**：只让启动级关键请求进入全局错误页；局部查询保留页面，显示 ErrorState + Retry；统一 Toast 责任。

### P1-FE-03 默认 Query 重试会重试大多数 4xx

- **位置**：`web/src/main.tsx:55-70`
- **问题**：除 401/403 外，400、404、409、422 等客户端错误仍会继续重试。
- **影响**：无效请求被放大，用户重复收到错误提示。
- **建议**：仅重试网络错误、429 和可恢复 5xx；中间重试静默，最终失败再提示。

### P1-FE-04 业务失败被伪装为空列表

- **位置**：`web/src/features/users/components/users-table.tsx:156-169`、`usage-logs-table.tsx:140-153`、`api-keys-table.tsx:257-266`、`redemptions-table.tsx:110-119`、`channels-table.tsx:223-305`、`data-table-view.tsx:267-314`
- **问题**：`success:false` 或缺失 data 时返回空数组，表格渲染“暂无数据”。
- **影响**：管理员可能误以为用户、密钥、渠道或兑换码不存在，进而做错误管理操作。
- **建议**：统一 `unwrapApiResponse`，业务失败抛异常；区分 EmptyState 和 ErrorState，提供重试。

### P1-FE-05 初始化状态检查失败仍永久缓存“已检查”

- **位置**：`web/src/routes/__root.tsx:110-177`
- **问题**：`getSetupStatus().catch(() => null)` 后仍写 `setup_status_checked=true`。
- **影响**：网络临时失败后，浏览器后续跳过初始化守卫，可能进入错误登录路径。
- **建议**：只有有效成功响应才写缓存；失败不写，设置短 TTL，并展示可重试阻断页。

### P1-FE-06 Playground 聊天记录跨账号泄漏

- **位置**：`web/src/lib/auth-session.ts:197-203`、`web/src/features/playground/constants.ts:68-73`、`web/src/features/playground/lib/storage/storage.ts:341-397`
- **问题**：聊天记录使用固定 localStorage key，登出时清理函数未被调用，未按用户或会话隔离。
- **影响**：账号 B 可能看到账号 A 的提示词、回答和推理内容。
- **建议**：按 `userId/sid` 分区存储；身份切换时清理或隔离；退出时删除敏感历史。

### P1-FE-07 Playground 超限会静默清空或永久截断

- **位置**：`web/src/features/playground/lib/storage/storage.ts:56-65,195-218,341-383`
- **问题**：超过 1 MiB 时直接删除整份历史；加载时把单条内容截成 40k 字符并重新写回。
- **影响**：用户刷新后不可逆丢失长对话，没有提示和导出能力。
- **建议**：按预算逐条淘汰最旧记录；展示层截断不要覆写原始数据；增加导出和容量提示。

### P1-FE-08 系统设置跨标签页不同步

- **位置**：`web/src/features/system-settings/hooks/use-update-option.ts:45-60`、`web/src/main.tsx:68-70`、`web/src/lib/nav-modules.ts:145-210`、`web/src/routes/_authenticated/playground/index.tsx:25-29`
- **问题**：设置更新只失效当前标签的 Query/localStorage，其他标签没有广播。
- **影响**：另一标签页长期使用旧的导航、价格、模块开关和 Playground 门禁。
- **建议**：使用 BroadcastChannel 或 storage event 同步 status 失效；涉及权限/门禁时做新鲜校验。

### P1-FE-09 钱包请求失败被显示为余额 0

- **位置**：`web/src/lib/api.ts:41-46`、`web/src/features/wallet/index.tsx:112-125`、`wallet-stats-card.tsx:57-75`
- **问题**：失败只写 console，加载结束后卡片以 0 兜底。
- **影响**：用户误以为余额、用量和请求数都是 0。
- **建议**：迁移到 Query 状态机，提供 error/retry；保留 last-known-good，不用 0 表示不可用。

### P1-FE-10 钱包充值配置失败被显示为“未开启充值”

- **位置**：`web/src/features/wallet/hooks/use-topup-info.ts:171-215`、`recharge-form-card.tsx:480-488`
- **问题**：配置请求失败后返回空配置，组件将其解释为管理员没有开启充值。
- **影响**：网络故障被误解为业务配置，用户没有重试入口。
- **建议**：分开表示加载失败、功能关闭、没有支付方式和成功空配置。

### P1-FE-11 支付报价失败会进入 `$0` 确认流程

- **位置**：`web/src/features/wallet/hooks/use-payment.ts:60-104`、`web/src/features/wallet/index.tsx:173-190`
- **问题**：请求失败时返回 0，选中支付方式后仍打开确认弹窗。
- **影响**：金融流程状态错误，用户可能重复尝试。
- **建议**：报价失败不能进入确认；显示明确原因、重试和订单未创建说明。

### P1-FE-12 外部支付缺少到账闭环

- **位置**：`web/src/features/wallet/hooks/use-payment.ts:108-156`、`subscription-purchase-dialog.tsx:117-228`
- **问题**：支付页打开后立即关闭弹窗、刷新用户信息，没有待支付订单状态和到账轮询。
- **影响**：用户不知道是否成功，可能重复付款。
- **建议**：保存订单号，展示 pending 状态、截止时间、回跳处理、页面聚焦轮询和手动刷新。

### P1-FE-13 渠道细粒度权限没有驱动操作界面

- **位置**：`router/channel-router.go:40-78`、`web/src/features/channels/components/data-table-row-actions.tsx:99-252`、`channels-primary-buttons.tsx:210-252`
- **问题**：后端区分 read/operate/write/sensitive_write，前端多数只判断 sensitive_write。
- **影响**：受限管理员可以点击测试、启停、编辑、查余额，最后收到 403。
- **建议**：统一下发 capability；按钮按权限隐藏、禁用或显示缺失权限；编辑抽屉区分只读与可编辑字段。

### P1-FE-14 普通管理员看不到模型检测进度

- **位置**：`router/channel-router.go:77`、`web/src/features/channels/hooks/use-channel-upstream-updates.ts:246-267`、`web/src/routes/_authenticated/system-info/index.tsx:25-35`
- **问题**：渠道操作权限可以发起检测，但提示用户到只有超级管理员可访问的系统信息页查看进度。
- **影响**：任务启动后无法查看进度、失败原因或结果。
- **建议**：在渠道页显示任务卡、轮询状态和结果；或开放任务详情只读页。

### P1-FE-15 全量应用上游模型更新没有预览和回退

- **位置**：`web/src/features/channels/components/channels-primary-buttons.tsx:234-252`、`use-channel-upstream-updates.ts:164-205`、`controller/channel_upstream_update.go:986-1067`
- **问题**：点击即遍历所有渠道执行新增/移除模型；前端不展示渠道级结果和失败明细。
- **影响**：模型删除会立即影响生产路由，误操作后难以恢复。
- **建议**：先生成差异预览；移除模型二次确认；完成后展示结果抽屉、失败重试和导出。

### P1-FE-16 批量渠道操作缺少影响范围和进度

- **位置**：`channels-primary-buttons.tsx:210-230`、`data-table-bulk-actions.tsx:85-107`、`data-table-tag-row-actions.tsx:53-59`、`controller/channel-billing.go:454-496`
- **问题**：刷新全部余额、标签启停和批量启停点击后直接执行；刷新余额还可能自动禁用余额不大于零的渠道。
- **影响**：一次误点可能造成批量停服；没有渠道级结果、进度和审计链接。
- **建议**：展示受影响数量和列表；超过阈值要求强确认；改为异步任务并显示进度、结果和失败重试。

---

## 五、UI、可访问性与响应式

### P1-UI-01 浅色主题主按钮对比度不足

- **位置**：`web/src/styles/theme.css:112-113`、`web/src/components/ui/button.tsx:25-45`
- **问题**：浅色主题 `--primary` 配白色前景约 2.7:1，低于普通文本 AA 要求 4.5:1。
- **影响**：保存、创建、搜索等主操作可读性不足。
- **建议**：调整主色或前景色；对所有主题 token 建立自动对比度检查。

### P1-UI-02 Skip Link 指向不存在的主内容节点，且存在嵌套 main

- **位置**：`web/src/components/skip-to-main.tsx:24-29`、`sidebar.tsx:325-334`、`section-page-layout.tsx:81-114`、`main.tsx:25-35`、`public-layout.tsx:49-55`
- **问题**：Skip Link 使用 `#content`，但页面没有对应 id；外层和内部布局还可能输出多个 `<main>`。
- **影响**：键盘和读屏器用户无法跳过重复导航，主内容地标混乱。
- **建议**：每种布局只保留一个 `<main id="content" tabIndex={-1}>`，内部容器不再重复使用 main。

### P1-UI-03 移动端公开菜单不是可访问的模态层

- **位置**：`web/src/components/layout/components/public-header.tsx:108-113,306-416`
- **问题**：菜单使用 opacity/pointer-events 隐藏，缺少 `inert`、`aria-hidden`、焦点圈定、Escape 关闭和焦点恢复。
- **影响**：关闭菜单后 Tab 仍可能进入隐藏链接；打开后焦点可能跑到背景内容。
- **建议**：复用现有 Sheet/Dialog；补 `aria-expanded`、`aria-controls`、Escape、焦点陷阱和恢复。

### P1-UI-04 移动端公开页缺少语言切换和通知入口

- **位置**：`public-header.tsx:64-74,259-278,300-415`
- **问题**：语言和通知只在桌面导航渲染，移动菜单没有等价入口。
- **影响**：移动端国际用户无法切换语言，也难以访问通知。
- **建议**：把入口放入移动 Sheet，保持触控、键盘和读屏语义一致。

### P1-UI-05 设置开关缺少可访问名称关联

- **位置**：`web/src/features/system-settings/components/settings-form-layout.tsx:109-131`
- **问题**：Label 没有 `htmlFor`，Switch 没有 `aria-labelledby/aria-label`，说明文字也没有 `aria-describedby`。
- **影响**：读屏器只能读出“开关”，不知道控制什么；点击说明文字也不可靠。
- **建议**：生成稳定 ID，建立 label、control、description 的语义关联。

### P1-UI-06 列表错误显示为空数据

- **位置**：多个用户、密钥、兑换码、渠道和日志表格，见 `web/src/components/data-table/core/data-table-view.tsx:267-314`
- **问题**：组件把请求失败和空数组统一渲染成 EmptyState。
- **影响**：管理员会误判数据已被删除或筛选为空。
- **建议**：共享 ErrorState + Retry；后台刷新失败时保留旧数据并显示非阻塞错误。

### P1-UI-07 共享 UI 原语存在硬编码英文

- **位置**：
  - `web/src/components/ui/dialog.tsx:79-92,127-131`
  - `web/src/components/ui/sheet.tsx:94-107`
  - `web/src/components/ui/sidebar.tsx:218-220,282-307`
  - `web/src/components/ui/pagination.tsx:30-39,88-148`
  - `web/src/components/ui/command.tsx:52-70`
  - `web/src/components/command-menu.tsx:105`
- **问题**：Close、Toggle Sidebar、分页和命令面板文案没有通过 i18n。
- **影响**：非英文界面下可见文案和读屏器播报仍为英文。
- **建议**：共享原语接收本地化 label props，或在可见组件层使用 `useTranslation()`。

### P1-UI-08 OAuth 六标签在窄屏和长语言下可能溢出

- **位置**：`web/src/features/system-settings/auth/oauth-section.tsx:380-388`、`web/src/components/ui/tabs.tsx:72-81`
- **问题**：`grid-cols-6`、`whitespace-nowrap` 强制六个标签挤在一行，没有横向滚动或移动端降级。
- **影响**：手机、小窗口和法语/俄语环境下可能挤压、裁切或重叠。
- **建议**：复用支付设置页的横向滚动模式，或小屏改成选择器/两行布局；补 320/375px 多语言视觉测试。

### P1-UI-09 减少动态效果支持不完整

- **位置**：`web/src/components/ui/skeleton.tsx:21-28`、`web/src/styles/index.css:163-169`、`web/src/components/ui/form.tsx:72-104`
- **问题**：Skeleton 无条件 `animate-pulse`，错误定位强制 smooth scroll。
- **影响**：减少动画用户仍然看到闪烁或平滑移动。
- **建议**：Skeleton 增加 `motion-reduce:animate-none`；根据 `prefers-reduced-motion` 选择 `auto` 滚动。

### P1-UI-10 页面标题层级从 h2 开始

- **位置**：`web/src/components/layout/components/section-page-layout.tsx:81-90`、`web/src/features/keys/index.tsx:31-40`
- **问题**：通用页面标题固定使用 h2，很多后台页没有 h1。
- **影响**：读屏器标题导航缺少页面级顶级标题。
- **建议**：页面标题使用 h1，卡片和区块从 h2 开始；保持视觉样式不变。

### P1-UI-11 移动端外观设置入口被隐藏

- **位置**：`web/src/components/config-drawer.tsx:87-99`、`web/src/components/layout/components/app-header.tsx:122-145`
- **问题**：ConfigDrawer 触发器在 `md` 以下隐藏，没有移动端替代入口。
- **影响**：手机用户无法发现主题、字体、密度和方向等设置。
- **建议**：在个人菜单或移动菜单中增加外观设置入口。

### P1-UI-12 后台设置加载失败仍展示可编辑默认值

- **位置**：`web/src/features/system-settings/api.ts:37-40`、`settings-page.tsx:113-149`、`use-system-options.ts:102-106`、`basic-auth-section.tsx:85-106`
- **问题**：接口业务失败时仍被认为查询成功，页面只处理 loading，不处理 error。
- **影响**：管理员可能把默认值误认为真实配置并保存，覆盖线上配置。
- **建议**：查询失败必须进入错误态；失败期间禁用保存；提供重试和最近成功配置提示。

### P2-UI-01 高频头部操作点击区域过小

- **位置**：`web/src/components/ui/button.tsx:42-51`、`profile-dropdown.tsx:60-72`、`header.tsx:33-35`
- **问题**：`icon-sm` 为 28px，个人菜单触发器只有 24px，头部按钮约 32px。
- **影响**：移动端容易误触，难以点击。
- **建议**：视觉图标大小可以保持不变，但交互盒至少 40px，最好 44px。

### P2-UI-02 非 macOS 用户看到错误的搜索快捷键

- **位置**：`web/src/components/search.tsx:53-56`、`web/src/context/search-provider.tsx:37-46`
- **问题**：界面固定显示 `⌘ K`，实际同时支持 `Ctrl+K`。
- **建议**：根据平台显示 `⌘ K` 或 `Ctrl K`；无法判断时使用中性提示。

### P2-UI-03 多数认证后台页面缺少一级标题

- **位置**：`section-page-layout.tsx:81-90` 及依赖该布局的后台页面
- **问题**：页面层级从 h2 开始。
- **建议**：统一页面级 h1，并通过 CSS 保持当前视觉大小。

### P2-UI-04 注册页条件邮箱验证缺少字段级反馈

- **位置**：`web/src/features/auth/constants.ts:30-44`、`sign-up-form.tsx:141-157,298-345`
- **问题**：邮箱/验证码条件校验只 toast，没有字段错误、Label 和发送按钮禁用原因。
- **建议**：根据系统状态生成条件 Schema；使用 `form.setError`、`FormMessage`、明确 Label 和帮助文案。

---

## 六、前端存储、路由与契约

### P1-FE-STORE-01 localStorage 固定 key 导致 Playground 跨账号泄漏

见 [P1-FE-06](#p1-fe-06-playground-聊天记录跨账号泄漏)。该问题同时属于隐私隔离和前端状态设计问题，修复时需要同时覆盖登出、切换用户、刷新和多标签场景。

### P1-FE-STORE-02 localStorage 访问未防护

- **位置**：`web/src/features/channels/components/channels-provider.tsx:86-91`、`web/src/features/home/hooks/use-home-page-content.ts:42-47`、`web/src/features/dashboard/lib/filters.ts:40-43,85-92`
- **问题**：部分同步存储读写没有 try/catch。
- **影响**：Safari 私密模式、嵌入式 WebView 或受限存储策略下，频道页可能崩溃，首页内容可能卡在加载状态。
- **建议**：提供统一 `safeStorage` 适配器；Zustand persist 支持降级到内存；所有解析失败回退为可解释状态。

### P1-FE-STORE-03 前端缓存升级会清除大量未知数据

- **位置**：`web/src/lib/frontend-cache.ts:19-57`
- **问题**：版本变化时删除所有不在白名单中的 localStorage key。
- **影响**：表格偏好、通知状态、系统配置、Playground 数据等可能被整体清除。
- **建议**：使用项目命名空间；逐项迁移；未知 key 默认保留，只有明确归属和过期策略的 key 才删除。

### P1-FE-STORE-04 日志 queryKey 包含翻译函数

- **位置**：`web/src/features/usage-logs/components/usage-logs-table.tsx:119-129`
- **问题**：`t` 函数进入 queryKey，切换语言可能改变 key。
- **影响**：语言切换触发不必要的日志请求和重复缓存。
- **建议**：移除 `t`；只有服务端响应确实依赖语言时才使用稳定的语言码。

### P1-FE-STORE-05 原生 history.replaceState 绕过 TanStack Router

- **位置**：`web/src/routes/_authenticated/wallet/index.tsx:24-35`、`web/src/features/wallet/index.tsx:132-137`
- **问题**：路由 search 由 TanStack Router 管理，却使用原生 History 清理参数。
- **影响**：URL 已变化但 Router 内存状态可能仍旧，返回、再次跳转和弹窗状态不稳定。
- **建议**：使用 `navigate({ search, replace: true })`。

### P1-FE-STORE-06 分页参数缺少整数和最大值约束

- **位置**：`web/src/hooks/use-table-url-state.ts:32-46,158-180`、`web/src/routes/_authenticated/users/index.tsx:26-39`
- **问题**：URL 和 localStorage 中的 `pageSize` 只检查大于 0。
- **影响**：手工输入超大页大小可发起大请求，造成浏览器卡顿和服务端压力。
- **建议**：共享 Zod schema，要求 int/min/max；读取旧存储时 clamp。

### P1-FE-STORE-07 `/api/status` 契约多处弱类型化

- **位置**：`web/src/lib/api.ts:70-73`、`web/src/features/auth/types.ts:92-180`、`web/src/hooks/use-system-config.ts:36-114`、`web/src/features/chat/hooks/use-chat-presets.ts:42-65`
- **问题**：同一接口被 `Record<string, unknown>`、兼容结构和原生 fetch 分散消费。
- **影响**：字段改名或嵌套变化可能静默回退默认值，影响登录方式、配额、模块开关。
- **建议**：唯一 DTO、Zod parser、映射层；业务组件不直接猜后端字段形状。

### P1-FE-STORE-08 充值配置解析宽松，错误只写 console

- **位置**：`web/src/features/wallet/hooks/use-topup-info.ts:39-164,171-235`
- **问题**：支付方式、折扣和商品配置使用大量 unknown 和宽松转换。
- **影响**：配置格式变化时支付方式可能消失，用户只看到残缺界面。
- **建议**：用 Zod 校验完整响应；明确显示错误和重试，不要用空数组和零值降级。

### P1-FE-STORE-09 OAuth 回调 search 缺少 schema 校验

- **位置**：`web/src/routes/(auth)/oauth.tsx:31-36,70-72`、`web/src/routes/oauth/$provider.tsx:62-74,246-248`
- **问题**：依赖类型断言 `as`，未配置 `validateSearch`。
- **影响**：第三方回调缺参、重复参数或错误 provider 时，运行时错误路径不一致。
- **建议**：增加 Zod search schema、provider 校验和明确降级页面。

---

## 七、用户旅程与高风险操作体验

### P1-JOURNEY-01 渠道管理员“能点但不能做”

见 [P1-FE-13](#p1-fe-13-渠道细粒度权限没有驱动操作界面)。

### P1-JOURNEY-02 全量上游模型更新缺少差异预览

见 [P1-FE-15](#p1-fe-15-全量应用上游模型更新没有预览和回退)。

### P1-JOURNEY-03 批量余额刷新可能自动禁用渠道

见 [P1-FE-16](#p1-fe-16-批量渠道操作缺少影响范围和进度)。

### P1-JOURNEY-04 系统行为设置逐项保存，失败后形成混合状态

- **位置**：`web/src/features/system-settings/general/system-behavior-section.tsx:68-76`、`web/src/features/system-settings/hooks/use-update-option.ts:45-69`
- **问题**：三个配置逐个顺序提交，失败不会停止后续提交，也没有汇总或回滚。
- **影响**：管理员看到“保存完成”，实际只有部分全局行为生效。
- **建议**：提供批量设置接口并原子提交；至少停止后续操作、列出每项成功/失败并重新拉取真实配置。

### P1-JOURNEY-05 API Key 明文和批量复制缺少二次确认

见 [P1-BILL-12](#p1-bill-12-api-token-批量明文复制缺少防误操作保护)。

### P2-JOURNEY-01 注册条件邮箱验证体验不完整

见 [P2-UI-04](#p2-ui-04-注册页条件邮箱验证缺少字段级反馈)。

### P2-JOURNEY-02 钱包失败和充值关闭状态没有区分

见 [P1-FE-09](#p1-fe-09-钱包请求失败被显示为余额-0) 和 [P1-FE-10](#p1-fe-10-钱包充值配置失败被显示为未开启充值)。

---

## 八、性能、数据库与可扩展性

### P1-PERF-01 NarraFork 命中率计算在请求热路径全量读取日志并逐条解析

- **位置**：`service/narrafork_quota_details.go:99-130`、`model/narrafork_usage.go:59-157`、`setting/narrafork_setting/narrafork_setting.go:38-43`
- **问题**：按用户窗口读取全部消费日志，再逐条解析 `other`，并且发生在 SSE 额度事件构建路径。
- **影响**：成本随历史日志线性增长，高频用户会产生严重尾延迟。
- **建议**：写日志时维护用户+日/小时聚合表或 ClickHouse 物化视图；展示时只查聚合桶；增加短 TTL 缓存。

### P1-PERF-02 日志列表、总数和统计接口缺少时间窗

- **位置**：`controller/log.go:13-25,98-149`、`model/log.go:468-511,564-603,618-671`
- **问题**：允许时间为空；列表先 Count 再 Find，统计还会触发多次聚合。
- **影响**：全历史 COUNT、分页和聚合可能同时发生；模糊搜索进一步扩大扫描。
- **建议**：服务端强制默认/最大时间窗；request_id 精确查询可例外；使用 keyset 分页；总数改为可选、异步或近似值。

### P1-PERF-03 ClickHouse 排序键与高频筛选维度不匹配

- **位置**：`model/main.go:441-465`、`model/log.go:468-511,564-602`
- **问题**：日志按 `(created_at, request_id)` 排序，但常见查询按 user/channel/model/group/type 过滤。
- **影响**：日志规模增长后，查询需要读取大量无关行。
- **建议**：先用 `EXPLAIN indexes=1` 量化，再增加 `(user_id, created_at, request_id)` projection，必要时增加相关跳数索引。

### P1-PERF-04 配额/流量看板允许全历史高基数聚合

- **位置**：`controller/usedata.go:31-61,141-157`、`model/usedata.go:141-182`、`model/usedata_flow.go:57-92`
- **问题**：管理员接口缺少最大时间跨度、分页和结果上限。
- **影响**：用户×模型×小时×分组×渠道的聚合会同时放大 DB 内存、应用内存和 JSON 响应。
- **建议**：限制时间窗；预聚合；Top-N；大范围查询转异步导出任务。

### P1-PERF-05 配额看板缓存写入全局锁加逐条 N+1

- **位置**：`model/usedata.go:41-49,95-139`、`main.go:138-139`
- **问题**：持有全局锁时，对每个聚合键执行 First，再执行 Update/Create。
- **影响**：高吞吐下阻塞 LogQuotaData，周期 flush 产生 O(K) 到 O(2K) DB 往返，多节点还会放大写压力。
- **建议**：锁内只交换快照 map；锁外批量 Upsert；增加完整维度唯一键，失败时合并回待刷队列。

### P1-PERF-06 用户/令牌缓存 miss 没有请求合并

- **位置**：`model/user_cache.go:93-120`、`model/token.go:287-308`、`common/redis.go:19-21`
- **问题**：缓存 miss 直接回源 DB，回填异步，没有 singleflight、租约或 TTL 抖动。
- **影响**：热点 key 到期时大量并发回源，造成数据库和 Redis 写放大。
- **建议**：按 key 使用 singleflight；TTL 加随机抖动；跨节点使用短租约；保留 auth-version fence。

### P1-PERF-07 每个节点每 60 秒全量同步渠道并失效价格缓存

- **位置**：`main.go:102-130`、`model/channel_cache.go:26-112`、`model/pricing.go:66-75,180-241`
- **问题**：每轮全量读取 channels/abilities，即使没有变更也无条件失效价格缓存。
- **影响**：实例数、渠道数增长时，DB、解析和 GC 成本按节点数乘上升。
- **建议**：基于 updated_at/version 增量同步，或 Redis Pub/Sub 广播变更；仅变更时失效定价缓存。

### P1-PERF-08 流式单事件和累计文本内存上限过大

- **位置**：`relay/helper/stream_scanner.go:25-51,204-320`、`relay/channel/openai/helper.go:93-131`、`relay/channel/openai/relay-openai.go:117-184`、`common/init.go:176-182`
- **问题**：单个 SSE token/event 允许最高 128MB；没有 usage 时还累计完整文本、推理和工具参数。
- **影响**：长文本、异常上游或工具参数会让单连接占用大量内存，并发时可能 OOM。
- **建议**：降低单事件上限；累计文本设置预算；超限中止；采用增量 token 计数和可观测指标。

### P1-PERF-09 入站 HTTP 和流式 goroutine 缺少背压

- **位置**：`main.go:231-234`、`common/gopool.go:11-24`、`relay/helper/stream_scanner.go:154-257`
- **问题**：Server 没有 ReadHeaderTimeout/IdleTimeout/MaxHeaderBytes；流式任务池上限接近无限。
- **影响**：慢客户端和大量长流会长期占用 FD、连接和 goroutine。
- **建议**：增加连接级超时、最大 Header；增加全局/用户/渠道 in-flight stream semaphore；拒绝时返回明确 429。

### P2-PERF-01 任务结算日志存在额外用户/Token 查询

- **位置**：`model/log.go:419-445`、`model/token.go:270-284`、`service/task_polling.go:117-169`
- **问题**：每个任务记录日志时查询用户名，存在 TokenId 时还会查 Token。
- **影响**：批量处理 1000 条任务时产生大量额外查询。
- **建议**：批处理前预加载名称，或任务创建时保存展示快照。

### P2-PERF-02 消费日志重复读取用户设置

- **位置**：`middleware/auth.go:443-456`、`model/user_cache.go:29-46,123-143`、`model/log.go:343-359`
- **问题**：认证阶段已经将 user setting 放进 Context，记录日志时又读取 Redis/DB。
- **影响**：每个请求增加 Redis 访问；Redis 不可用时可能退化为 DB 查询。
- **建议**：优先读取 Context，只有缺失时 fallback。

### P2-PERF-03 修改订阅计划会 SCAN 并删除全部订阅详情缓存

- **位置**：`model/subscription.go:135-143`、`pkg/cachex/hybrid_cache.go:139-174`
- **问题**：`Purge()` 收集 namespace 下全部 key 再删除。
- **影响**：订阅用户多时，单次计划修改触发 Redis 峰值和全量冷启动。
- **建议**：版本化 namespace/generation key；维护反向索引；SCAN 分批流式删除。

### P2-PERF-04 日志筛选会触发冗余请求

- **位置**：`web/src/features/usage-logs/components/common-logs-filter-bar.tsx:188-227`、`usage-logs-table.tsx:119-153`、`common-logs-stats.tsx:56-76`
- **问题**：筛选导航改变 query key 后，又按前缀 invalidate 全部 logs/stats。
- **影响**：一次筛选可能触发旧条件和新条件请求。
- **建议**：依赖新 query key 获取数据；仅在 query key 不变时精准失效；传递 AbortSignal 取消旧请求。

### P2-PERF-05 日志表格重复 JSON.parse

- **位置**：`web/src/features/usage-logs/components/usage-logs-table.tsx:208-230`、`web/src/features/usage-logs/components/columns/common-logs-columns.tsx:335,549,629,652,700,713,732`
- **问题**：同一行 `other` 在多列和行染色中反复解析。
- **影响**：100 行、多列和频繁切换时产生大量对象分配。
- **建议**：Query select 阶段一次性规范化，或按 row id memo 缓存。

### P2-PERF-06 前端生产构建关闭 Tailwind 优化和构建缓存

- **位置**：`web/rsbuild.config.ts:26-27,84-88`
- **问题**：显式设置 `optimize:false`、`buildCache:false`。
- **影响**：可能增加 CSS 包体和 CI/本地构建时间。
- **建议**：先建立 bundle 基线，再恢复生产优化和可信 CI 构建缓存。

### P2-PERF-07 数据库连接池默认值在多节点双库部署下偏激进

- **位置**：`model/main.go:189-195,233-239`
- **问题**：主库和日志库都允许最多 1000 连接、100 空闲连接，生命周期仅 60 秒。
- **影响**：多实例时连接数快速叠加；频繁重连造成认证/TLS抖动。
- **建议**：按数据库总连接预算和实例数分配；分别配置主库/日志库；补 `ConnMaxIdleTime` 和 DBStats 告警。

---

## 九、测试、发布与运维

### P0-OPS-01 Dockerfile.dev 首次构建可能失败

- **位置**：`Dockerfile.dev:14-17`、`go.mod:165-170`；对比 `Dockerfile:20-24`
- **问题**：根模块通过 replace 引用本地 `relaykit`，但开发 Dockerfile 在 `go mod download` 前没有复制 `relaykit/go.mod`。
- **影响**：`docker compose -f docker-compose.dev.yml up --build` 可能在首次构建时失败，直接阻断开发环境。
- **建议**：开发 Dockerfile 与生产 Dockerfile 一致，先复制 `relaykit/go.mod` 再下载依赖，并设置 `GOWORK=off`。
- **验收**：干净 Docker 构建上下文中首次构建成功；修改 relaykit 代码仍能热更新。

### P0-OPS-02 预发布 tag 可能覆盖 Docker latest

- **位置**：`.github/workflows/docker-build.yml:4-7,83-90,125-145`、`docker-compose.yml:19`
- **问题**：Docker 发布工作流匹配几乎所有 tag，仅排除 nightly，但每次都会推送 `latest`。
- **影响**：alpha、rc 等预发布镜像覆盖稳定版 latest，使用 latest 的生产环境可能意外升级。
- **建议**：只有受保护的稳定 SemVer tag 更新 latest；预发布只发布自己的版本标签。
- **验收**：预发布 tag 的 workflow 不推送 latest；稳定 tag 才创建 latest manifest。

### P1-OPS-01 Docker 与二进制版本来源不一致

- **位置**：`.github/workflows/docker-build.yml:44-59`、`.github/workflows/release.yml:25-57`、`VERSION:1`
- **问题**：Docker 以 tag 覆盖 VERSION，二进制工作流优先读取仓库 VERSION。
- **影响**：同一发布 tag 的 Docker 镜像和二进制可能嵌入不同版本。
- **建议**：选择唯一版本来源；或者发布前强制检查 `VERSION == tag`，不一致即失败。

### P1-OPS-02 PR CI 和发布流程缺少完整质量门

- **位置**：`.github/workflows/ci.yml:43-58,81-88`、`web/package.json:7-19`、`.github/workflows/release.yml:35-67`
- **问题**：PR CI 没有前端生产构建；tag 发布直接构建发布，也未复用完整 Go 测试。
- **影响**：Rsbuild、路由生成、静态资源嵌入和根包逻辑可能到发布时才暴露。
- **建议**：建立可复用 verify workflow，包含 Go test/vet、relaykit 独立测试、前端 lint/test/build，并作为发布前置条件。

### P1-OPS-03 MySQL/PostgreSQL 迁移测试在 CI 中被跳过

- **位置**：`model/user_session_migration_test.go:128-153`、`.github/workflows/ci.yml:14-89`
- **问题**：测试依赖 `TEST_MYSQL_DSN`、`TEST_POSTGRES_DSN`，为空就 Skip；CI 没有对应服务。
- **影响**：支持的多数据库迁移分支可能只在客户环境暴露。
- **建议**：增加 MySQL/PostgreSQL service matrix；补迁移启动、重复迁移、失败恢复测试。

### P1-OPS-04 多节点可能并发执行迁移

- **位置**：`common/init.go:87-90`、`model/main.go:197-205,253-315`、`main.go:331-336`
- **问题**：非 slave 节点启动时执行 AutoMigrate，没有跨实例迁移锁。
- **影响**：滚动发布或扩容时可能并发 DDL、锁等待、启动失败。
- **建议**：独立 migration job 或 advisory lock；明确唯一迁移节点。

### P1-OPS-05 缺少备份、恢复和恢复演练链路

- **位置**：`docker-compose.yml:76-85,122-125`、`model/main.go:203-205`
- **问题**：有持久化卷，但没有自动逻辑备份、恢复脚本、升级前快照和定期 restore drill。
- **影响**：自动迁移或误操作后恢复时间不可控。
- **建议**：按数据库类型提供加密备份、异地保留、恢复验证和定期演练。

### P1-OPS-06 Compose healthcheck 不能代表服务就绪

- **位置**：`docker-compose.yml:62-66`、`router/api-router.go:22-27`、`controller/misc.go:25-44`
- **问题**：健康检查只访问 `/api/status`，不检查主库、日志库或 Redis。
- **影响**：数据库失联时容器仍可能显示 healthy，编排系统不会摘流。
- **建议**：拆分 `/livez` 和 `/readyz`；ready 检查主库、日志库、Redis，设置超时和 start_period。

### P1-OPS-07 Compose 未保证 120 秒优雅退出

- **位置**：`main.go:246-260`、`docker-compose.yml:17-66`
- **问题**：应用默认允许 120 秒处理流请求，但 Compose 未配置相应 stop grace period。
- **影响**：停机或滚动发布时长流可能被强杀，计量/缓存未落盘。
- **建议**：设置略大于应用超时的 `stop_grace_period`，负载均衡先摘流，并补 SSE 停机测试。

### P1-OPS-08 Compose 缺少必填 SESSION_SECRET 和安全默认值

- **位置**：`docker-compose.yml:29,34,41,72-83`、`common/constants.go:39-40`、`common/init.go:50-64`、`service/auth_token.go:54-58`
- **问题**：生产 Compose 使用弱口令，SESSION_SECRET 只是注释项；未设置时重启可能生成新密钥，使 JWT/Refresh Token 失效。
- **建议**：使用 `${SESSION_SECRET:?required}` 或 Docker/Kubernetes secrets；禁止可运行的默认弱密码；启动时校验。

### P1-OPS-09 Docker build context 可能包含 .env、数据和日志

- **位置**：`.dockerignore:1-13`、`Dockerfile:26`
- **问题**：`.dockerignore` 未排除 `.env`、`data`、`logs`、`*.db`，Dockerfile 又全量 `COPY . .`。
- **影响**：密钥、SQLite 数据和日志进入 build context 或 builder cache。
- **建议**：默认拒绝，按需放行；避免全量 COPY；敏感材料使用 BuildKit secrets。

### P1-OPS-10 缺少标准 metrics 输出

- **位置**：`router/api-router.go:35-40,223-232`、`pkg/perf_metrics/metrics.go:23-77`
- **问题**：性能数据主要存在内存/数据库并通过业务接口查询，没有标准 Prometheus/OTEL scrape/exporter。
- **影响**：难以采集错误率、上游延迟、DB/Redis 状态、goroutine 和任务积压。
- **建议**：增加受网络策略保护的 metrics endpoint，定义 SLI/SLO 和告警规则。

### P1-OPS-11 文件日志没有自动保留策略

- **位置**：`docker-compose.yml:22,25-27`、`logger/logger.go:27,55-72,112-119`、`controller/performance.go:267-352`
- **问题**：日志主要按累计条数轮转，缺少按时间/大小 retention。
- **影响**：长期运行可能耗尽磁盘，进一步影响应用写入。
- **建议**：容器优先 stdout，由日志系统收集；若保留文件，增加按大小/日期轮转和 retention。

### P2-OPS-01 make test 可能排除根包测试

- **位置**：`makefile:42-49`、`main_version_test.go:12-50`
- **问题**：测试命令过滤掉与模块名相同的根包。
- **影响**：根包回归测试可能没有进入 CI。
- **建议**：显式执行根包测试，或统一使用 `GOWORK=off go test ./...` 并准备 `web/dist` 占位文件。

### P2-OPS-02 当前时间桶指标重启后丢失

- **位置**：`pkg/perf_metrics/flush.go:26-61`、`pkg/perf_metrics/metrics.go:79-122,380-428`
- **问题**：当前小时桶主要在结束后落库，Redis 活跃桶合并函数已定义但没有调用。
- **影响**：进程重启会丢失当前时间段的性能统计。
- **建议**：关闭时 flush 当前桶；启动/查询时合并 Redis 活跃桶；补重启恢复测试。

### P2-OPS-03 文档默认值和实际代码漂移

- **位置**：`README.md:329-333`、`common/init.go:177-183`
- **问题**：文档写流扫描和请求体默认 64MB，代码实际 128MB。
- **影响**：运维评估内存和攻击面不准确。
- **建议**：单一配置源生成文档，或 CI 校验文档默认值。

### P2-OPS-04 运行镜像未切换非 Root，依赖固定不完整

- **位置**：`Dockerfile:30-41`、`Dockerfile.dev:4,24`、`docker-compose.yml:69,77`
- **问题**：最终镜像未设置 USER，开发基础镜像和 Redis 使用可变 tag。
- **影响**：容器攻破后权限面大，构建结果漂移。
- **建议**：创建非 Root 用户、只授予数据目录权限、镜像使用版本和 digest。

### P2-OPS-05 Makefile 偏 Unix，PR 阶段没有 Windows/macOS 验证

- **位置**：`makefile:17-24,45-73`、`.github/workflows/ci.yml:18,63`
- **问题**：依赖 grep、后台进程、sqlite3 等 Unix 工具，PR CI 仅 Ubuntu。
- **影响**：跨平台开发和构建问题推迟到发布后发现。
- **建议**：提供 Go/Bun/PowerShell 等跨平台脚本；至少增加 Windows 构建冒烟和关键测试矩阵。

---

# P2 中长期优化项

## P2-01 钱包失败状态被展示为 Pending

- **位置**：`web/src/features/wallet/types.ts:247-274`、`web/src/features/wallet/lib/billing.ts:36-56`、`common/constants.go:255-258`
- **问题**：后端存在 failed，前端 union 和状态映射缺失，未知值回退 pending。
- **影响**：用户误以为订单仍在处理中，可能重复付款。
- **建议**：补齐 failed、cancelled、refunded、disputed 状态和操作指引。

## P2-02 账单历史混用 Amount/Money 含义

- **位置**：`controller/topup_creem.go:107-117`、`model/topup.go:426-432`、`model/subscription.go:643-661`、`billing-history-dialog.tsx:230-258`
- **问题**：不同网关中 Amount 可能代表入账额度、支付金额或兼容字段，UI 统一按 USD 格式化。
- **影响**：Creem 或订阅记录可能显示错误额度、$0 或无币种金额。
- **建议**：按交易类型拆分字段和 UI 展示，增加币种、支付金额、入账额度、套餐标题。

## P2-03 普通用户只能查询近 30 天订单

- **位置**：`model/topup.go:162-203`、`controller/topup.go:440-463`、`web/src/features/wallet/hooks/use-billing-history.ts:59-65`
- **问题**：普通用户查询强制使用 30 天窗口，管理员不受此限制。
- **影响**：用户无法自助查历史付款、下载凭证或完成长期对账。
- **建议**：提供完整财务历史、时间筛选、归档查询和导出能力。

## P2-04 Waffo 使用伪造邮箱

- **位置**：`controller/topup_waffo.go:52-54,269-272`
- **问题**：支付资料使用 `${user.Id}@examples.com`，不使用真实用户邮箱。
- **影响**：收据/通知可能送不到用户，支付商侧身份关联质量差。
- **建议**：使用已验证邮箱；缺失时要求补充或明确不传，不要伪造地址。

## P2-05 钱包请求失败显示零值

见 [P1-FE-09](#p1-fe-09-钱包请求失败被显示为余额-0)。该项也属于长期数据表达规范：不可用不应等同于 0。

## P2-06 频道页/首页/localStorage 在受限环境下可能崩溃

见 [P1-FE-STORE-02](#p1-fe-store-02-localstorage-访问未防护)。

## P2-07 OAuth 标签、移动入口和外观设置的响应式缺口

见 [P1-UI-04](#p1-ui-04-移动端公开页缺少语言切换和通知入口)、[P1-UI-08](#p1-ui-08-oauth-六标签在窄屏和长语言下可能溢出)、[P1-UI-11](#p1-ui-11-移动端外观设置入口被隐藏)。

---

# 按领域整理的问题清单

## 安全修复优先级

### 第一优先级

1. `/api/setup` 首次初始化保护
2. `/mj/image/:id` 鉴权和任务归属
3. Footer HTML 清理或改为纯文本
4. OAuth Token 和支付回调日志脱敏
5. OIDC Discovery、Custom OAuth、渠道预览 SSRF 防护
6. pprof 回环监听和管理网隔离
7. 生产 Cookie Secure、Trusted Proxies 和强制 Secret
8. Compose 密码、镜像、容器用户和 build context

### 第二优先级

1. Token/渠道/OAuth Secret 加密和轮换
2. `channel.secret_view` 权限闭环
3. CORS、Origin、CSRF 和审计事件统一检查
4. 关键管理动作增加操作人、影响范围和变更前后快照

## 计费修复优先级

1. 统一支付事件表和订单状态机
2. 原子预扣和 reservation
3. 退款/补扣 outbox 和重试
4. 支付产品、金额、币种和套餐快照
5. 订阅续费、取消、退款和争议生命周期
6. 额度 int64/BIGINT 迁移
7. 账单历史字段拆分
8. 支付创建失败和 webhook 失败的可见状态

## UI 修复优先级

1. 修复加载失败、业务失败和空数据混淆
2. 修复系统设置失败仍可编辑默认配置
3. 修复 Skip Link、唯一 main 和移动菜单焦点
4. 修复浅色主题主色对比度
5. 统一共享组件 i18n
6. 按 capability 控制管理员操作按钮
7. 批量操作增加预览、确认、进度、结果和撤销
8. 完善移动端语言、通知、外观和小触控目标

## 性能修复优先级

1. 负数分页和所有列表上限
2. 上游 Context、超时、并发和背压
3. 日志/看板最大时间窗和异步导出
4. 配额看板快照交换、批量 Upsert
5. 渠道缓存增量同步和 last-known-good
6. 缓存 miss singleflight
7. 流式事件/累计文本大小限制
8. ClickHouse projection 和日志查询优化
9. 连接池预算和 DBStats 告警

## 测试与运维修复优先级

1. Dockerfile.dev 首次构建
2. 稳定版 latest 发布规则
3. 发布前完整质量门
4. MySQL/PostgreSQL/ClickHouse 集成测试
5. migration job/锁
6. `/livez`、`/readyz`
7. 备份恢复和演练
8. metrics、日志 retention、优雅退出
9. Windows/macOS 构建验证
10. 测试运行器统一，避免 `node:test` 与 Bun 冲突

---

# 推荐修复路线

## 第一阶段：先堵接管、资金和额度风险

### 目标

避免在任何用户体验改造前继续产生账户接管、资金丢失、双倍充值、额度透支或凭据泄露。

### 建议任务

1. 实现 Setup Token 和原子初始化。
2. 实现支付事件持久化、幂等和订单状态机。
3. 修复 Stripe webhook 错误返回码和历史订单回调门控。
4. 实现钱包/令牌条件扣减和 reservation。
5. 实现退款、差额补扣和支付失败补偿队列。
6. 加入 Midjourney 鉴权、Footer HTML 清理、SSRF 防护、敏感日志清理。
7. 修复默认 Cookie、Proxy、pprof、Compose Secret 和容器权限。

### 第一阶段完成条件

- 所有 P0 有对应回归测试。
- 支付事件可以重复投递而不重复入账。
- 并发请求不会产生负额度。
- 失败退款能在进程重启后继续。
- 新实例没有正确安装密钥时无法被远程初始化。

## 第二阶段：修复管理员误操作和关键用户流程

### 建议任务

1. 建立统一 API 响应解包：`success:false` 必须转为 Error。
2. 所有表格区分 Loading、Empty、Error、Refreshing。
3. 系统设置失败时禁用保存，增加 Retry。
4. 统一 `/api/status` 数据源。
5. 充值报价失败禁止打开确认框。
6. 外部支付增加 Pending Order、到账轮询和订单详情。
7. 渠道按钮按 capability 驱动。
8. 批量模型同步、余额刷新和启停增加预览、确认、进度和结果。
9. API Key 明文读取和批量复制增加确认与 IP 白名单校验。

### 第二阶段完成条件

- 任何请求失败不会被显示为空数据、默认配置或余额 0。
- 管理员看到的每个按钮都与实际权限一致。
- 高风险批量操作能在执行前看到影响范围。
- 支付用户可以看到订单状态和到账结果。

## 第三阶段：UI、无障碍和移动端

### 建议任务

1. 统一 `<main id="content">` 和 Skip Link。
2. 移动公开菜单改用 Sheet/Dialog。
3. 修复主色对比度。
4. 补全共享组件 i18n。
5. 修复设置开关 label 关联。
6. 后台页面补 h1 标题层级。
7. 移动端补语言、通知、外观设置。
8. 修复 OAuth Tab 横向布局。
9. 补 reduced-motion、点击区域和平台快捷键。

### 第三阶段完成条件

- 键盘可以跳过导航并稳定定位主内容。
- 读屏器能理解菜单、开关、分页、关闭按钮和错误提示。
- 375px、320px 和横屏下无关键操作裁切或横向溢出。
- 主题文本和主按钮达到可接受对比度。

## 第四阶段：性能、数据和运维闭环

### 建议任务

1. 修复所有分页和日志时间窗。
2. 为上游请求增加 Context、timeout 和并发背压。
3. 优化看板聚合、缓存 flush 和 ClickHouse projection。
4. 渠道缓存改为变更驱动、原子 last-known-good swap。
5. 增加缓存 miss 请求合并。
6. 建立 `/livez`、`/readyz`、Prometheus/OTEL metrics。
7. 建立备份、恢复、迁移锁和数据库集成测试。
8. 优化 Docker、日志 retention、优雅退出和跨平台 CI。

### 第四阶段完成条件

- 大范围日志/看板请求有明确上限或转异步任务。
- 长流和慢上游不会无限占用连接和 goroutine。
- 数据库断开不会用空快照覆盖健康缓存。
- 发布前能自动验证前端构建、Go 测试、relaykit 和多数据库迁移。
- 具备实际可执行、可验证的恢复流程。

---

# 建议建立的统一基础能力

## 1. 统一 API 响应解包

建议建立类似以下职责的前端基础函数：

- 检查 HTTP 状态
- 检查业务 `success`
- 提取稳定 `error_code`
- 保留 request ID
- 抛出统一 Error 类型
- 由页面决定 ErrorState 或 Toast

不要让每个表格自己把失败转换为空数组。

## 2. 统一错误状态协议

每个数据请求至少区分：

- `idle`
- `loading`
- `success-empty`
- `success-with-data`
- `refreshing`
- `error-without-data`
- `error-with-stale-data`

推荐 UI 行为：

| 状态 | UI |
|---|---|
| 首次加载 | Skeleton |
| 成功但为空 | EmptyState + 下一步操作 |
| 首次失败 | ErrorState + Retry |
| 有旧数据但刷新失败 | 保留旧数据 + 非阻塞错误提示 |
| 提交中 | 禁用重复提交 + 进度 |
| 提交失败 | 说明原因 + 修复路径 |

## 3. 统一账务状态机

建议把以下概念分开：

- Payment Order
- Provider Event
- Credit Ledger
- Debit Reservation
- Settlement
- Refund
- Reconciliation

每个对象都应该有：

- 唯一业务键
- 状态
- 重试次数
- 最后错误
- 下一次重试时间
- 创建/更新时间
- 操作来源和 request ID

## 4. 统一配置快照

配置读取和热更新建议采用：

1. 数据库存储原始配置
2. 加载并验证成不可变结构
3. 使用 `atomic.Value` 整体替换
4. 请求只读当前快照
5. DB 写成功后再发布快照
6. 多节点通过版本号、Pub/Sub 或事件通知刷新

## 5. 统一批量操作协议

所有全量/标签/批量操作都建议遵循：

1. 预览影响范围
2. 明确目标数量
3. 展示增删或启停差异
4. 超过阈值要求二次确认
5. 创建异步任务
6. 展示实时进度
7. 展示逐对象结果
8. 支持失败重试
9. 提供审计记录
10. 可撤销时提供 Undo 或回滚入口

## 6. 统一状态检查端点

建议区分：

- `/livez`：进程是否存活，不访问外部依赖
- `/readyz`：是否可以接流量，检查主库、日志库、Redis 和关键配置
- `/api/status`：面向用户界面的公开状态信息
- `/api/status/test`：管理员诊断接口

不要让公开状态接口同时承担容器 readiness 的职责。

---

# 修复验收总清单

## 初始化

- [ ] 未提供 Setup Token 无法创建 Root
- [ ] 并发初始化只有一个成功
- [ ] 初始化事务失败可安全重试
- [ ] 初始化状态缓存失败不会永久跳过向导

## 认证与安全

- [ ] Midjourney 图片按用户归属校验
- [ ] Footer payload 无法执行脚本、事件属性和危险协议
- [ ] OAuth/支付日志无完整 Token、签名和 body
- [ ] OAuth Discovery、Custom OAuth、渠道预览阻断 SSRF
- [ ] pprof 默认只回环或受管理网络保护
- [ ] Secure Cookie、Trusted Proxies、Session Secret 有安全默认值
- [ ] 生产 Compose 不使用弱口令和可变 latest
- [ ] 运行镜像非 Root

## 计费与支付

- [ ] 同一支付事件重复投递不会重复入账
- [ ] 历史订单在停售后仍可完成合法回调
- [ ] 支付失败返回非 2xx 或进入可重试队列
- [ ] 钱包/令牌扣减使用条件更新或行锁
- [ ] 退款、补扣、逆向结算可持久化和重试
- [ ] 订阅订单保存权益快照
- [ ] Stripe 续费、取消、退款、争议可同步
- [ ] 金额使用币种和 minor units
- [ ] 额度使用 int64/BIGINT
- [ ] 负倍率、负兑换码额度和超大值会被拒绝
- [ ] 账单历史字段按交易类型正确展示

## 异步任务

- [ ] 本地任务先持久化提交状态
- [ ] 上游任务创建有幂等键或禁止不安全重试
- [ ] 任务状态用 CAS 推进
- [ ] 渠道缓存暂时失败不会直接终止并跳过退款
- [ ] 任务失败退款有 durable retry
- [ ] Suno 状态有大小写映射
- [ ] 非 200 response body 总是关闭
- [ ] 结算使用提交时计费快照

## 前端状态

- [ ] `/api/status` 只有一个数据源
- [ ] 业务失败不会转为空数组
- [ ] 全局错误不会把局部错误强制导航到 `/500`
- [ ] 4xx 默认不重复重试
- [ ] 钱包失败不会显示为 0 或未开启
- [ ] 支付报价失败不能打开确认框
- [ ] Playground 按账号/会话隔离
- [ ] Playground 超限渐进淘汰并提示
- [ ] 跨标签配置变更能同步
- [ ] URL 状态只由 Router 管理
- [ ] 分页参数有整数和上限校验
- [ ] OAuth callback search 使用 schema

## UI 与无障碍

- [ ] Skip Link 能定位到唯一 `main#content`
- [ ] 页面不存在嵌套 main
- [ ] 移动菜单有 Dialog/Sheet 语义和焦点管理
- [ ] 移动端有语言、通知和外观设置入口
- [ ] 设置开关有关联 Label 和描述
- [ ] 共享组件的关闭、分页、Sidebar、Command 文案支持 i18n
- [ ] 浅色主题主操作达到目标对比度
- [ ] 主页面标题从 h1 开始
- [ ] OAuth Tab 在 320/375px 和长语言下可操作
- [ ] reduced-motion 能关闭 pulse 和 smooth scroll
- [ ] 关键触控目标至少 40px，优先 44px
- [ ] Windows/Linux 显示 Ctrl+K

## 性能与数据

- [ ] 所有 page_size 拒绝负数、0、超大值
- [ ] 列表和统计有最大时间窗
- [ ] 大范围日志/看板转异步导出
- [ ] 上游请求继承 Context
- [ ] 非流式和流式请求有独立 timeout
- [ ] 有全局/用户/渠道 in-flight 限制
- [ ] 流式单事件和累计输出有上限
- [ ] 配额看板使用快照交换和批量 Upsert
- [ ] 缓存 miss 有 singleflight
- [ ] 渠道缓存变更驱动同步
- [ ] ClickHouse 高频筛选有 projection 或索引策略
- [ ] 连接池按实例预算配置

## 测试与运维

- [ ] Dockerfile.dev 干净构建成功
- [ ] 预发布不会覆盖 latest
- [ ] Docker 与二进制版本来源一致
- [ ] PR CI 执行前端生产构建
- [ ] 发布前执行 Go/relaykit/前端测试
- [ ] MySQL/PostgreSQL/ClickHouse 集成测试进入 CI
- [ ] 迁移由独立 job 或锁控制
- [ ] `/livez` 和 `/readyz` 语义清楚
- [ ] Compose stop grace period 覆盖流式关闭时间
- [ ] 有自动备份、恢复和 restore drill
- [ ] 有 Prometheus/OTEL 指标和告警
- [ ] 日志有大小/时间 retention
- [ ] 根包测试不会被 make test 排除
- [ ] Bun 测试不再混用不兼容的 node:test 注册方式

---

# 审查限制与后续工作

1. 本次为只读静态审查，没有修改代码。
2. 没有执行真实支付、退款、拒付、Stripe/Creem/Waffo 回调演练。
3. 没有在浏览器中完整执行登录、管理员配置、批量渠道操作和移动端真机测试。
4. UI 对比度问题根据源码颜色计算，最终仍应在浏览器中对光/暗主题和不同显示器验证。
5. ClickHouse、MySQL、PostgreSQL 的性能和迁移问题应使用真实服务容器做集成复现。
6. 额度并发风险应使用确定性并发测试和数据库隔离级别测试确认，不应只依赖压力测试。
7. 所有安全修复都应补充回归测试，避免只改入口而遗漏其他路径。
8. 修复时不要一次性大范围重构。推荐按“初始化/支付/额度/安全”先收敛，再处理 UI 和性能。
9. 每个 P0/P1 修复完成后，应单独审查相关文件、运行定向测试，再进入下一项。

## 最终建议

先处理 P0，尤其是初始化接管、支付入账、并发额度和退款补偿。随后修复“失败显示为空/默认值”的前端状态规范，再做 UI 无障碍和批量操作体验。性能和运维工作应与发布质量门、数据库迁移测试、ready 检查和备份恢复一起推进，形成可持续的上线闭环。
