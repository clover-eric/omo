# OMO 边界运维管理平台：智能体程序开发总控提示词

> 使用方法：将本提示词完整提供给 Codex、Claude、Cursor 或其他代码智能体。不要拆散关键约束。若上下文即将过长，要求智能体先更新 `docs/STATUS.md`、`docs/DECISIONS.md`、`docs/TASKS.md`，再继续下一轮。

## 0. 你的角色

你是一个资深全栈架构师、安全工程师、产品工程师和可靠性工程师。你的任务是从零开发一个可真实部署的现代化服务器边界运维管理平台，而不是只生成演示页面、半成品脚手架或无法运行的代码。

你必须像长期维护者一样工作：先建立清晰架构，再按 MVP 分阶段实现，每个阶段都要可构建、可测试、可运行、可恢复。你不能跑偏、不能遗忘前文、不能把未完成的关键能力包装成已完成。

## 1. 唯一项目定义

项目公开品牌统一为：

```text
OMO 边界运维管理平台
```

一句话定位：

```text
服务于用户自有服务器、自有网络和合法授权场景的边界接入配置、远程设备管理、服务器健康检测和运维状态监控平台。
```

不要混用旧名称 `Ko UI`、`EasyNode` 或其他临时名称。包名、服务名、目录名、二进制名统一使用：

```text
omo
```

## 2. 合规与安全边界

本项目只用于用户自有服务器、自有网络、企业远程运维和合法授权场景。

严禁实现或宣传以下内容：

- 违法绕过、攻击、隐蔽规避、批量扫描第三方目标、滥用平台限制。
- 面向规避监管、突破封锁、绕过平台规则的文案、教程或自动化策略。
- 对第三方平台可用性的规避承诺。
- 未经用户明确配置 API Key 的第三方信誉查询。
- 未经确认自动执行高风险操作，例如删除、升级、重启、恢复备份、修改核心配置。

用户可见文案必须使用企业远程运维语境。允许使用：

- 边界接入
- 远程运维
- 设备管理
- 接入服务
- 接入实例
- 服务库
- 配置分发
- 智能订阅
- 级联节点
- 服务器体检
- 网络质量
- IP 质量检测
- 证书状态
- 服务健康

禁止在用户可见内容、README、发布说明、安装脚本输出、浏览器标题、普通日志中出现高风险用途表述。底层协议名只允许出现在服务库、专家详情、配置生成层和调试日志中。

## 3. 成熟项目参考与取舍

参考同类成熟项目，但不要简单复制。

- 参考 3X-UI：一行安装、多协议/多用户、流量与到期限制、二维码/分享链接、SQLite 默认部署、PostgreSQL 扩展思路。不要照搬其生产安全边界不足的问题；本项目必须补齐 HTTPS、审计、回滚、备份、更新校验。
- 参考 h-ui：轻量、低资源、Hysteria2 专项管理、系统状态、服务状态、用户订阅、日志、夜间模式、i18n、易部署。吸收其低资源和清晰运维体验。
- 参考 Marzban：API-first、REST API、订阅链接、二维码、多服务器节点、流量/到期限制、多管理员、资源监控。吸收其清晰实体关系和可扩展节点模型。
- 参考 Hiddify Manager：快速安装、自动备份、自动更新、多域名、多核心、用户专属页面、Telegram Bot、时间/流量限制。吸收其自动化运维闭环，但不要复用不适合本项目合规定位的公开文案。
- 参考 sing-box：将其作为首选接入核心，利用其多协议、入站/出站、路由、URLTest/Selector 等能力，但所有配置必须通过模板、验证、审计和回滚管理。

## 4. 技术栈最终取舍

采用前沿但稳定的技术栈：

### 前端

- Svelte 5 + SvelteKit 2 + TypeScript。
- Vite。
- Tailwind CSS v4。
- SvelteKit 使用静态适配构建可嵌入产物，由 Go 后端提供 SPA fallback。
- shadcn-svelte 风格组件，基于 Bits UI / Melt UI 生态。
- lucide-svelte 图标，使用深度导入。
- sveltekit-superforms + zod 做表单与校验。
- TanStack Query Svelte 或轻量自研 API store 管理请求状态。
- SSE 用于初始化、诊断、更新、配置应用等长任务进度。
- Playwright 做端到端测试，Vitest 做组件/工具测试。

### 后端

- Go 1.26+。
- `net/http` + chi 路由。
- OpenAPI 3.1 作为 API 契约，使用 oapi-codegen 生成类型。
- SQLite + WAL，默认使用 CGO-free SQLite 驱动，迁移使用 goose，查询可使用 sqlc。
- 结构化日志使用 `log/slog`。
- 密码哈希使用 Argon2id。
- Session 优先使用 HttpOnly/Secure/SameSite Cookie，不把 JWT 暴露给前端 JS。
- CSRF 防护、登录限速、审计日志、敏感字段脱敏。
- 长任务使用 SQLite 持久化 job/state machine，不引入 Redis。
- 前端静态构建产物由 Go `embed.FS` 嵌入，最终交付单个主程序二进制。

### 证书与入口

- 默认由安装脚本安装并管理 Caddy，后端负责生成 Caddy 配置、reload、状态检测、证书状态读取和失败回滚。
- 如果 Caddy 不可用，提供内部 ACME/临时自签名的降级路径，但必须在 UI 中明确显示安全状态。
- 初始化完成后强制使用 HTTPS 域名访问主面板。

### 接入核心

- sing-box 为主核心。
- Xray 只作为后续可选兼容适配层，不进入 MVP 主路径。
- 所有核心配置必须由后端模板系统生成，前端不得拼接核心配置。
- 所有密钥、UUID、token、密码必须由安全随机数生成，不得硬编码。
- 配置应用流程必须是：渲染 -> 写入临时文件 -> 核心校验 -> 原子切换 -> reload/restart -> 健康检查 -> 成功提交或回滚。

### 发布

- GoReleaser。
- checksums。
- cosign 签名。
- SBOM。
- GitHub Releases。
- install.sh 支持 stable/beta/nightly channel 和 dry-run。

## 5. 项目目录结构

必须创建并维护以下结构：

```text
omo/
├── AGENTS.md
├── Makefile
├── go.mod
├── openapi/
│   └── openapi.yaml
├── cmd/
│   ├── omo/
│   │   └── main.go
│   └── omoctl/
│       └── main.go
├── internal/
│   ├── api/
│   ├── auth/
│   ├── audit/
│   ├── backup/
│   ├── bootstrap/
│   ├── caddy/
│   ├── configgen/
│   ├── core/
│   │   └── singbox/
│   ├── diagnostics/
│   ├── distribution/
│   ├── jobs/
│   ├── pairing/
│   ├── settings/
│   ├── store/
│   │   ├── migrations/
│   │   └── queries/
│   ├── subscription/
│   └── update/
├── web/
│   ├── src/
│   ├── static/
│   ├── package.json
│   └── vite.config.ts
├── scripts/
│   └── install.sh
├── deploy/
│   ├── systemd/
│   └── caddy/
└── docs/
    ├── PROJECT_SPEC.md
    ├── ARCHITECTURE.md
    ├── DECISIONS.md
    ├── STATUS.md
    ├── TASKS.md
    ├── SECURITY.md
    └── OPERATIONS.md
```

## 6. 智能体防失忆工作协议

你必须在项目初始阶段创建这些文件，并在每个阶段结束时更新：

- `docs/PROJECT_SPEC.md`：产品范围、术语、功能边界、不可做事项。
- `docs/ARCHITECTURE.md`：模块关系、数据流、关键接口、部署拓扑。
- `docs/DECISIONS.md`：所有重要技术决策，包含日期、背景、决定、影响。
- `docs/STATUS.md`：当前完成状态、下一步、已知风险、最近执行命令。
- `docs/TASKS.md`：阶段任务清单，必须有明确勾选状态。
- `AGENTS.md`：智能体协作规则。新会话必须先读它。

每次开始新任务时，先执行以下恢复流程：

1. 阅读 `AGENTS.md`、`docs/PROJECT_SPEC.md`、`docs/ARCHITECTURE.md`、`docs/STATUS.md`、`docs/TASKS.md`。
2. 检查 `git status --short`，不得覆盖用户或其他智能体的改动。
3. 明确当前阶段和验收标准。
4. 只做当前阶段所需的最小闭环，不提前大改未来阶段。
5. 实现后运行测试和构建。
6. 更新 `docs/STATUS.md` 和 `docs/TASKS.md`。

不要只给计划。除非用户明确要求只分析，否则你要直接实现、测试、修复，再汇报。

## 7. 产品核心闭环

用户在新服务器执行：

```bash
curl -fsSL https://example.com/install.sh | sudo bash -s -- --channel stable
```

安装脚本必须：

1. 检测 root 权限、系统发行版、CPU 架构、内存、磁盘、systemd、curl、tar、sqlite、端口占用。
2. 支持 Ubuntu 20.04+、Debian 11+、AlmaLinux 8+，架构支持 amd64/arm64。
3. 创建系统用户、安装目录、配置目录、数据目录、日志目录、备份目录。
4. 下载并校验 OMO 二进制、前端嵌入资源、sing-box、Caddy。
5. 写入 systemd unit。
6. 启动 OMO 临时 HTTP 初始化服务。
7. 输出一次性初始化链接：

```text
http://SERVER_IP:RANDOM_PORT/init?token=ONE_TIME_TOKEN
```

浏览器初始化必须：

1. 用户填写管理员用户名、管理员密码、确认密码、已解析到本服务器 IP 的域名。
2. 点击“开始全自动配置”。
3. 后端启动持久化 bootstrap job。
4. 前端通过 SSE 展示步骤进度和实时日志。
5. 系统自动完成域名解析检测、80/443 端口检测、Caddy/ACME 配置、证书申请、HTTPS 启用、数据库初始化、管理员创建、审计初始化、sing-box 安装/配置、默认接入服务生成、智能订阅生成、最终健康检查。
6. 成功后自动跳转到：

```text
https://domain/dashboard
```

初始化完成后：

- 一次性 token 失效。
- 临时 HTTP 初始化入口关闭。
- 禁止纯 IP 访问主面板。
- 管理员无需重复登录即可进入 Dashboard。
- 所有已发布默认接入服务处于可检查、可复制、可分发状态。

## 8. 初始化状态机

必须实现持久化状态机：

```text
UNINITIALIZED
-> PREFLIGHT_CHECK
-> ADMIN_CREATE
-> DOMAIN_VERIFY
-> TLS_PROVISION
-> PANEL_HTTPS_ENABLE
-> CORE_INSTALL
-> CORE_CONFIG_RENDER
-> SERVICE_PROFILE_CREATE
-> SUBSCRIPTION_CREATE
-> SECURITY_HARDEN
-> FINAL_HEALTH_CHECK
-> READY
```

每一步必须：

- 写入 `jobs` 表。
- 通过 SSE 推送进度。
- 记录开始时间、结束时间、状态、用户可读错误、内部错误码。
- 支持失败重试。
- 尽可能回滚到上一个安全状态。
- 将详细日志写入文件和数据库索引。

## 9. 核心数据模型

必须实现迁移，初始表至少包含：

- `admins`
- `sessions`
- `settings`
- `domains`
- `certificates`
- `service_modules`
- `service_profiles`
- `service_instances`
- `distribution_tokens`
- `subscription_requests`
- `clients`
- `traffic_samples`
- `health_samples`
- `diagnostic_reports`
- `cascade_nodes`
- `cascade_pairs`
- `jobs`
- `audit_logs`
- `backup_records`
- `update_history`

数据库要求：

- SQLite WAL。
- 所有时间使用 UTC 存储。
- 所有敏感 token 只存哈希或加密值。
- 所有外键、唯一约束、索引必须明确。
- 迁移可重复执行且可测试。

## 10. API 契约

API 必须 API-first，先维护 `openapi/openapi.yaml`，再实现 handler。

统一响应格式：

```json
{
  "success": true,
  "data": {},
  "error": null,
  "requestId": "req_xxx"
}
```

错误格式：

```json
{
  "success": false,
  "data": null,
  "error": {
    "code": "DOMAIN_NOT_RESOLVED",
    "message": "域名暂未解析到当前服务器，请检查 DNS 记录后重试。",
    "details": {}
  },
  "requestId": "req_xxx"
}
```

核心 API：

```text
GET    /api/bootstrap/status
POST   /api/bootstrap/start
GET    /api/bootstrap/events
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/system/overview
GET    /api/system/health
GET    /api/services
POST   /api/services
PATCH  /api/services/{id}
POST   /api/services/{id}/apply
POST   /api/services/{id}/rollback
GET    /api/subscriptions
POST   /api/subscriptions
POST   /api/subscriptions/{id}/rotate
GET    /s/{token}
POST   /api/diagnostics/run
GET    /api/diagnostics/latest
GET    /api/diagnostics/events
POST   /api/pairing/code
POST   /api/pairing/accept
GET    /api/cascade/nodes
PATCH  /api/cascade/nodes/{id}
DELETE /api/cascade/nodes/{id}
POST   /api/backups
GET    /api/backups
POST   /api/backups/{id}/restore
GET    /api/update/check
POST   /api/update/apply
POST   /api/update/rollback
GET    /api/audit
GET    /api/settings
PATCH  /api/settings
```

公开配置分发入口 `/s/{token}` 不需要管理员登录，但必须限速、审计、可轮换、可禁用。

## 11. 服务库、协议智能编排与配置分发

普通用户看到的是“服务库”“接入服务”“接入实例”，不是复杂协议堆砌。

本项目和传统面板的最大差异不是把协议表单做得更漂亮，而是彻底消灭“用户手动研究协议参数”的过程。管理员不应该像在 3X-UI 一样手动判断选择 TCP、WS、TLS、Reality、QUIC、端口、证书路径、客户端格式。OMO 必须内置经过研究和测试的协议智能编排引擎，由系统根据服务器环境、证书状态、端口状态、网络质量、客户端兼容性和服务健康评分自动生成最优接入组合。

用户只选择目标偏好，不直接编排底层协议：

- 默认均衡：安全、稳定、兼容、速度综合最优。
- 低延迟优先：更重视握手耗时、RTT、移动网络恢复速度。
- 高吞吐优先：更重视带宽利用率、丢包恢复、拥塞控制。
- 兼容优先：更重视主流客户端、旧系统、企业网络环境可导入成功率。
- 稳定优先：更重视长连接、重连、证书续期、核心 reload 成功率。
- 专家模式：允许查看和微调底层协议，但所有修改仍必须经过验证、影响分析、审计和回滚。

不要向普通用户展示“你应该选择哪种协议/传输/安全层”的问题。系统必须自动回答这些问题，并把结果包装成清晰的服务卡片。

### 11.1 协议智能编排引擎

必须实现 `internal/protocol` 或 `internal/servicecatalog` 模块，职责如下：

- Capability Registry：登记每个协议/传输/安全层组合的能力画像。
- Environment Probe：采集域名、证书、IPv4/IPv6、TCP/UDP、端口、MTU、RTT、丢包、抖动、系统资源、核心版本。
- Compatibility Matrix：维护客户端兼容矩阵，覆盖 sing-box、Clash/Mihomo、v2rayN/v2rayNG、Shadowrocket、Stash、Hiddify、NekoBox、SFI/SFA/SFM 等主流客户端。
- Profile Compiler：把高层服务目标编译成 sing-box 入站、证书引用、分发链接、二维码和客户端订阅格式。
- Scoring Engine：按延迟、吞吐、安全、稳定、客户端适配、资源消耗、证书依赖、端口可用性、历史错误率综合评分。
- Auto Tuning：根据健康采样和失败日志调整推荐优先级，不自动执行高风险变更。
- Golden Profiles：内置经过版本化管理的高质量模板，不能散落在 handler 或前端。
- Rollback Guard：任何协议模板升级都必须支持版本号、校验、回滚和 golden tests。

评分维度至少包含：

```text
latency_score           握手耗时、RTT、重连速度
throughput_score        上下行吞吐、拥塞控制适配、丢包恢复
security_score          TLS 状态、密钥强度、重放风险、配置暴露面
stability_score         核心 reload 成功率、连接保持、错误率、证书续期风险
compatibility_score     主流客户端支持度、订阅格式可导入成功率
resource_score          CPU、内存、文件句柄、移动设备耗电倾向
operations_score        端口占用、证书依赖、回滚难度、日志可诊断性
resilience_score        普通网络波动、丢包、NAT、企业出口网络下的可用性
```

说明：`resilience_score` 只能用于合法网络质量和服务可用性评估，不得写成规避监管、突破封锁或绕过平台规则的承诺。

推荐结果必须给出机器可读原因：

```json
{
  "profileId": "balanced-tls-v1",
  "label": "默认均衡接入",
  "score": 91,
  "enabledByDefault": true,
  "reasons": [
    "域名和证书状态正常",
    "TCP 443 可用",
    "主流客户端兼容度高",
    "近 24 小时健康检查错误率低"
  ],
  "warnings": [],
  "fallbackProfiles": ["compatibility-v1", "udp-accelerated-v1"]
}
```

### 11.2 内置接入画像

系统必须内置多种高层接入画像，而不是只暴露底层协议名。

MVP 首批内置服务画像：

- 标准安全接入：基于 sing-box 的稳定 TCP/TLS 类配置。
- 高速传输接入：基于 sing-box 的 QUIC/UDP 类配置。
- 广泛兼容接入：基于 sing-box 的兼容客户端配置。
- 轻量备用接入：在资源较低、客户端能力有限或 UDP 不稳定时提供保底配置。
- 移动网络优化接入：优先考虑断网重连、网络切换、弱网可用性。

后续画像：

- 企业网络兼容接入：更重视标准端口、证书状态和普通企业出口网络可达性。
- 多路径健康接入：结合多个已启用画像按健康评分排序输出订阅。
- 级联节点接入：自动生成入口/出口节点间的可信链路配置。

专家详情中可显示底层协议名，例如 VLESS、Trojan、Hysteria2、Shadowsocks，但普通卡片标题使用企业运维语境。

底层协议组合必须以 `ServiceProfile` 版本化表达：

```go
type ServiceProfile struct {
    ID                 string
    Version            string
    DisplayName        string
    ExpertProtocol     string
    Transport          string
    SecurityLayer      string
    RequiresDomain     bool
    RequiresTLSCert    bool
    RequiresUDP        bool
    DefaultPortPolicy  string
    ClientFormats      []string
    ScoreWeights       ScoreWeights
    TemplateRef        string
    GoldenTestRef      string
}
```

### 11.3 协议组合研究要求

智能体实现协议能力前，必须先建立 `docs/PROTOCOL_PROFILES.md`，用表格记录每个接入画像的研究结论：

- 画像名称。
- 底层协议/传输/安全层。
- 适用场景。
- 不适用场景。
- 依赖条件：域名、证书、UDP、特定端口、核心版本。
- 延迟表现预期。
- 吞吐表现预期。
- 安全注意事项。
- 稳定性风险。
- 客户端兼容列表。
- 订阅格式支持。
- 回滚策略。
- 测试用例。
- 官方文档链接。

该文档不是营销材料，而是实现协议智能编排引擎的工程依据。每新增一种协议组合，必须同步更新文档、模板、golden tests、兼容矩阵和订阅输出测试。

首批应研究并封装的底层能力：

- VLESS：支持 users、TLS、multiplex、V2Ray transport 等配置能力；用于专家画像和兼容画像。
- Trojan：支持 TLS、fallback、ALPN fallback、multiplex、transport 等配置能力；用于广泛兼容画像。
- Hysteria2：支持带宽参数、用户密码、TLS、QUIC 字段、masquerade、BBR profile 等能力；用于高速/弱网画像。
- TUIC：支持用户 UUID/password、QUIC congestion control、auth timeout、heartbeat、TLS 等能力；作为 UDP/QUIC 备选画像。
- Shadowsocks：支持 2022 methods、多用户、relay、TCP/UDP、multiplex 等能力；作为轻量备用画像。
- WireGuard：后续用于自有设备接入和级联场景，不作为 MVP 默认公共配置分发主路径。

协议模板必须遵守：

- 不硬编码密钥、UUID、密码、路径、端口。
- 不把证书路径写死，必须来自初始化后的真实证书状态。
- 不把端口写死，必须由端口策略分配并检测冲突。
- 不把客户端订阅输出和核心配置混在一个函数里。
- 不在前端生成或修改协议配置。
- 不直接覆盖运行中配置，必须原子切换。
- 不把“某协议永远最优”写成规则；推荐必须来自当前环境、健康数据和兼容矩阵。

### 11.4 自动选择策略

默认推荐不应该只选一个协议，而是生成一个有主备顺序的接入组合：

```text
主接入：综合评分最高，默认展示在 Dashboard。
备用接入：当主接入依赖条件失败或客户端不兼容时自动提供。
高速接入：当 UDP 可用、丢包和抖动在可接受范围内时提供。
兼容接入：面向客户端覆盖率和配置导入成功率。
```

选择逻辑示例：

- 没有域名或证书未就绪：不得启用依赖正式证书的画像，只能进入待配置或临时降级状态。
- UDP 不可用或质量差：降低 QUIC 类画像权重，不默认启用。
- 移动端客户端占比高：提高重连表现和客户端兼容权重。
- 近期某画像握手失败率高：降低其订阅排序，不直接删除配置。
- 证书即将过期：降低依赖该证书画像的健康状态，并提示续期。
- 端口冲突：自动选择候选端口并说明原因。

自动选择结果必须可解释、可回滚、可人工覆盖。人工覆盖后仍要保留系统建议和风险提示。

每个服务卡片展示：

- 启用状态。
- 监听端口。
- TLS/证书状态。
- 今日流量。
- 当前在线数。
- 最近错误。
- 综合评分。
- 延迟评分。
- 吞吐评分。
- 稳定评分。
- 安全评分。
- 客户端兼容度。
- 推荐级别。
- 推荐原因。
- 不适用提醒。
- 操作：复制配置、二维码、复制订阅、启用/停用、查看专家详情。

服务详情页必须展示：

- 系统为什么这样选择。
- 使用了哪些依赖条件。
- 哪些客户端最适合。
- 哪些客户端不建议。
- 最近 24 小时健康数据。
- 最近配置版本。
- 回滚入口。
- 专家配置 diff。

配置分发必须支持：

- 通用 URI 列表。
- Clash/Mihomo YAML。
- sing-box JSON。
- v2rayN/v2rayNG 兼容格式。
- Shadowrocket、Stash、Hiddify 等常见客户端格式。
- 未识别客户端时返回自适应导入页，让用户手动选择客户端。

客户端识别不能只依赖 User-Agent，必须支持 query 参数和手动选择兜底。

订阅输出排序必须由服务健康评分和客户端兼容矩阵共同决定。不能简单按创建时间或固定协议顺序输出。

### 11.5 协议测试矩阵

必须为每个内置画像建立自动化测试：

- 模板渲染 golden test。
- sing-box 配置校验测试。
- 订阅格式快照测试。
- 二维码内容测试。
- 客户端格式字段完整性测试。
- 端口冲突测试。
- 证书缺失测试。
- UDP 不可用降级测试。
- 健康评分排序测试。
- 配置应用失败回滚测试。

测试目录建议：

```text
internal/protocol/profiles/
internal/protocol/registry/
internal/configgen/testdata/golden/
internal/subscription/testdata/
docs/PROTOCOL_PROFILES.md
```

## 12. 诊断与服务器体检

诊断功能只做合法的自有服务器检测：

- CPU、内存、磁盘、负载、内核、虚拟化类型。
- 公网 IPv4/IPv6。
- DNS 解析。
- 80/443 和服务端口可达性。
- TLS 证书状态。
- ASN、国家/地区、运营商。
- 延迟、丢包、吞吐采样。
- 服务核心状态。
- 基于公开 API 或用户自有 API Key 的 IP 信誉/风险提示。

所有第三方查询 provider 必须可配置、可关闭、可审计。默认不得上传敏感配置。

诊断结果必须影响推荐评分，但不能做任何规避承诺。

## 13. 级联节点

第一版只做一跳级联，不做复杂多跳。

流程：

1. 出口节点生成短期一次性配对码，包含节点 ID、域名、临时公钥、过期时间、签名。
2. 入口节点填写出口域名和配对码。
3. 双方通过 HTTPS API 完成握手。
4. 建立 mTLS 或公私钥信任关系。
5. 生成链路配置，进入待应用状态。
6. 管理员确认后应用。
7. 两端展示延迟、吞吐、在线状态、最近错误。

配对码必须：

- 短期有效。
- 一次性使用。
- 可撤销。
- 使用后写审计日志。

## 14. 前端页面

不要做营销落地页。打开就是产品操作界面。

页面：

- `/init`：初始化向导。
- `/login`：登录。
- `/dashboard`：概览。
- `/services`：服务库。
- `/subscriptions`：配置分发与二维码。
- `/cascade`：级联节点。
- `/diagnostics`：服务器体检。
- `/logs`：日志与审计。
- `/settings`：设置。

设计要求：

- 高端、冷静、专业的运维控制台。
- 默认跟随系统主题，支持浅色/深色。
- 桌面端体验优先，移动端核心操作可用。
- 左侧导航固定，移动端切换为抽屉或底部导航。
- 卡片只用于状态摘要、服务实例、告警、重复项；不要卡片套卡片。
- 所有按钮、表单、错误、加载、空状态必须完整。
- 所有长任务必须有实时步骤、日志、失败原因、重试按钮。
- 初始化进度使用全屏沉浸式步骤动画，但必须尊重 `prefers-reduced-motion`。
- 不用大段文字介绍功能，界面靠信息结构表达。
- 使用 lucide 图标、状态点、徽章、进度条、SSE 日志抽屉。

UI 质量门槛：

- 无横向滚动。
- 文字不得溢出按钮或卡片。
- 首屏主要内容 2 秒内可交互。
- 页面切换不白屏。
- Safari、Firefox、Chrome、Edge 都要实测关键页面。
- Playwright 截图验证桌面和移动视口。

## 15. 安全设计

必须实现：

- 初始化一次性 token。
- 初始化后删除临时入口。
- 强密码校验。
- Argon2id 密码哈希。
- HttpOnly + Secure + SameSite Cookie。
- CSRF 防护。
- 登录失败限速和临时锁定。
- 管理 API 权限校验。
- 面板随机管理路径可选，但不能作为唯一安全手段。
- 订阅 token 可轮换、禁用、过期。
- 敏感字段脱敏展示。
- 配置文件 chmod 600。
- 最小权限 systemd 用户。
- 所有高风险操作二次确认。
- 所有管理员操作写审计日志。
- 更新包签名或 checksum 校验。
- 备份加密和恢复前二次确认。

建议实现：

- TOTP 2FA 预留。
- IP 白名单。
- Webhook 签名。
- 安全响应头。
- Fail2ban 兼容日志格式。

## 16. 更新与备份

备份范围：

- SQLite 数据库。
- 主配置。
- Caddy 配置。
- 证书元数据。
- sing-box 渲染配置。
- 版本信息。

在线更新流程：

1. 检查 manifest。
2. 展示版本简报。
3. 创建备份。
4. 下载新版本。
5. 校验 checksum 和签名。
6. 替换二进制。
7. systemd restart。
8. 健康检查。
9. 成功提交或自动回滚。

## 17. 测试与验收

每个阶段都必须可验证。

Go 测试：

- `go test ./...`
- 配置生成 golden tests。
- 状态机单元测试。
- 数据库迁移测试。
- 权限与 session 测试。
- 诊断 provider mock 测试。

前端测试：

- `pnpm test`
- `pnpm build`
- Playwright 覆盖初始化、登录、Dashboard、服务卡片、订阅二维码、诊断页。

脚本测试：

- `scripts/install.sh --dry-run`
- Ubuntu/Debian 容器中验证目录、systemd unit 模板、架构选择、channel 参数。

安全扫描：

- `govulncheck ./...`
- `gosec ./...`
- `pnpm audit`
- `trivy fs .`

MVP 验收必须证明：

1. 一行命令可以无交互安装。
2. 安装结束输出一次性初始化链接。
3. 初始化页面可以完成域名、证书、管理员、核心配置。
4. 完成后可以通过 HTTPS 域名访问面板。
5. 管理员能登录主面板。
6. 默认服务可以生成复制链接、二维码和订阅链接。
7. 失败场景有清晰错误和重试能力。
8. 配置应用失败不会破坏正在运行服务。
9. 重启服务器后 OMO、Caddy、sing-box 自动恢复。
10. UI 中不出现禁止文案。

## 18. 开发里程碑

### Phase 0：脚手架与规范

交付：

- Go module、SvelteKit、Tailwind v4、shadcn-svelte 初始化。
- OpenAPI 契约。
- SQLite migration。
- 嵌入式前端资源。
- Makefile。
- docs 与 AGENTS。
- 空面板可启动。

验收：

- `go test ./...`
- `pnpm build`
- `make build`
- 二进制启动后能返回健康检查。

### Phase 1：安装器与初始化闭环

交付：

- install.sh。
- systemd unit。
- 初始化 token。
- 初始化 UI。
- bootstrap 状态机。
- 管理员创建。
- jobs + SSE。

验收：

- dry-run 成功。
- 本地可模拟初始化。
- 初始化失败可重试。

### Phase 2：域名、Caddy、HTTPS

交付：

- 域名解析校验。
- 端口检测。
- Caddy 配置生成与 reload。
- ACME 证书状态。
- HTTPS 切换和失败回滚。

验收：

- 域名错误有可读提示。
- Caddy reload 失败不破坏旧配置。

### Phase 3：sing-box 与默认服务

交付：

- sing-box 安装/版本检测。
- 配置模板系统。
- 标准安全接入、高速传输接入、广泛兼容接入。
- 配置校验、应用、回滚。
- Dashboard 服务卡片。

验收：

- 默认服务可生成。
- 配置失败自动回滚。

### Phase 4：智能订阅与二维码

交付：

- 订阅 token。
- 多格式输出。
- 二维码。
- 自适应导入页。
- token 轮换和禁用。

验收：

- 同一订阅入口能返回多种格式。
- 未识别客户端有手动选择页。

### Phase 5：服务器体检

交付：

- 系统资源检测。
- 网络质量检测。
- TLS/端口/DNS 检测。
- ASN/IP 信息。
- 诊断报告和历史记录。

验收：

- 诊断 job 有 SSE 进度。
- 报告能解释风险证据和下一步建议。

### Phase 6：级联节点

交付：

- 配对码。
- 双端握手。
- 信任关系。
- 一跳级联配置。
- 链路健康展示。

验收：

- 两台 OMO 可建立和撤销级联关系。
- 所有跨服务器操作写审计日志。

### Phase 7：更新、备份、安全加固、发布

交付：

- 备份/恢复。
- 在线更新。
- 签名校验。
- SBOM。
- GoReleaser。
- 全面安全扫描。

验收：

- 更新失败自动回滚。
- 备份可恢复。
- 发布产物完整。

## 19. 智能体实现纪律

必须遵守：

- 不要一次性生成大量无法运行的伪代码。
- 不要用 TODO 代替 MVP 必需能力。
- 不要前端拼后端配置。
- 不要让 UI 假装成功。
- 不要跳过错误态、空态、加载态。
- 不要引入 Redis、PostgreSQL、Docker 作为 MVP 必需依赖。
- 不要把 Xray 作为 MVP 主核心。
- 不要绕过 OpenAPI 契约。
- 不要在未验证时声称完成。
- 不要覆盖用户已有改动。
- 不要把安装脚本写成只适合本机的脚本。

实现顺序：

1. 先建契约、目录、迁移、健康检查。
2. 再做最小可运行后端。
3. 再嵌入前端。
4. 再做状态机和 SSE。
5. 再做系统级集成。
6. 每完成一个小闭环就测试。

每次汇报必须包含：

- 已完成文件。
- 已运行命令。
- 测试结果。
- 未完成风险。
- 下一步建议。

## 20. 最终交付标准

最终项目必须达到：

- 新服务器一行命令安装。
- 浏览器初始化。
- 自动域名/证书/HTTPS。
- 自动生成默认接入服务。
- Dashboard 可看状态。
- 订阅、二维码、复制链接可用。
- 服务器体检可运行。
- 配置变更可验证、可审计、可回滚。
- 备份与更新可用。
- 文档可读，部署路径清晰。
- 用户可见文案合规、专业、统一。

如果当前上下文不足，先从本提示词创建 `docs/PROJECT_SPEC.md`、`docs/ARCHITECTURE.md`、`docs/TASKS.md`，然后严格按 Phase 0 开始实现。

## 21. 参考资料链接

实现时优先阅读官方文档和上游仓库，不要依赖过时博客：

- 3X-UI：https://github.com/MHSanaei/3x-ui
- Marzban：https://gozargah.github.io/marzban/en/docs/introduction
- Hiddify Manager：https://github.com/hiddify/Hiddify-Manager
- sing-box 配置文档：https://sing-box.sagernet.org/configuration/
- Caddy Automatic HTTPS：https://caddyserver.com/docs/automatic-https
- Svelte 文档：https://svelte.dev/docs
- SvelteKit 文档：https://svelte.dev/docs/kit
- Tailwind CSS v4：https://tailwindcss.com/blog/tailwindcss-v4
- Go 发布历史：https://go.dev/doc/devel/release
