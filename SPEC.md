# CFTunnelKit SPEC

## Problem Statement

Homelab / NAS 用户希望把本地服务（自建应用、NAS 服务等）通过 Cloudflare Tunnel 安全地暴露到公网并绑定自有域名。但当前的完整链路完全依赖命令行：手动下载安装 `cloudflared`、用 API 或 YAML 维护 Ingress 规则（且必须遵守"末位无 hostname 的 404 兜底规则、规则顺序匹配、通配符仅限左侧"等易错约束）、创建 Tunnel、再手动配置 DNS CNAME 记录。普通用户在桌面端无法一键完成，配置错误会导致服务不可达或配置被 Cloudflare 拒绝。

## Solution

一个基于 **Go + Wails** 的跨平台桌面应用 **CFTunnelKit**（Windows / macOS / Linux），可视化完成：

1. **cloudflared 生命周期管理**：启动时检测本地二进制；缺失时从 GitHub Releases 下载对应平台二进制到用户目录；一键启动 `cloudflared tunnel run --token <token>`；优雅停止；stdout/stderr 实时经 Wails Events 推送到前端。
2. **Tunnel 管理**：通过 Cloudflare API 展示账户下所有 Tunnel（ID、名称、状态、创建时间）；创建远程管理型 Tunnel（`config_src: "cloudflare"`）；删除（二次确认）；查看详情与运行 Token。
3. **Ingress 规则可视化编辑（核心）**：编辑 **域名 → 本地端口** 映射规则；自动保证最后一条为无 hostname 的 404 兜底规则；按顺序从上到下匹配；支持 `*.example.com` 通配符（仅左侧）。
4. **DNS 联动**：绑定域名时自动创建指向 `<tunnel-id>.cfargotunnel.com` 的 CNAME；删除 Ingress 规则时可选择同步删除对应 DNS 记录。
5. **双模式认证**：v1 API Token 直填（第一版必做）；v2 OAuth Authorization Code with PKCE（第二版，无 client secret，本地回调 `localhost:8877`）。

## User Stories

1. 作为 Homelab 用户，我希望应用启动时检测本地 `cloudflared` 是否存在，以便知道是否需要安装。
2. 作为 Homelab 用户，我希望 `cloudflared` 缺失时应用能自动下载当前平台二进制并保存到用户目录，以便免命令行安装。
3. 作为 Homelab 用户，我希望一键启动 `cloudflared tunnel run --token <token>`，以便快速建立隧道。
4. 作为 Homelab 用户，我希望一键停止 `cloudflared` 并优雅释放进程，以便安全下线隧道。
5. 作为 Homelab 用户，我希望在应用内实时看到 `cloudflared` 的 stdout/stderr 日志，以便排查隧道故障。
6. 作为 Homelab 用户，我希望粘贴 API Token 完成认证并立即校验权限（Tunnel Edit / DNS Edit / Zone Read），以便提前发现权限不足。
7. 作为 Homelab 用户，我希望认证信息持久化，重启应用后无需重复输入。
8. 作为 Homelab 用户，我希望看到账户下所有 Tunnel（ID、名称、状态、创建时间），以便总览隧道。
9. 作为 Homelab 用户，我希望创建远程管理型 Tunnel（`config_src: cloudflare`），以便在 Dashboard 统一管理配置。
10. 作为 Homelab 用户，我希望删除 Tunnel 前有二次确认，以免误删。
11. 作为 Homelab 用户，我希望查看单个 Tunnel 的详情和运行 Token，以便在其他设备或 CLI 场景复用。
12. 作为 Homelab 用户，我希望可视化编辑域名 → 本地端口的 Ingress 规则，以便不用手写 YAML。
13. 作为 Homelab 用户，我希望规则列表自动保证最后一条为无 hostname 的 `http_status:404` 兜底规则，以免配置被 Cloudflare 拒绝。
14. 作为 Homelab 用户，我希望支持 `*.example.com` 通配符（仅左侧），以便一条规则覆盖多个子域名。
15. 作为 Homelab 用户，我希望理解规则按从上到下顺序匹配的语义，以便正确排布规则顺序。
16. 作为 Homelab 用户，我希望绑定域名时自动创建指向 `<tunnel-id>.cfargotunnel.com` 的 CNAME，以便域名立即可用。
17. 作为 Homelab 用户，我希望删除 Ingress 规则时可选择同步删除对应 DNS 记录，以免留下僵尸记录。
18. 作为 Homelab 用户，我希望第二版能用 OAuth（PKCE）免 Token 登录，以便省去手动配置权限。
19. 作为 Homelab 用户，我希望日志本地滚动保存（上限约 1MB），以便事后回溯。
20. 作为 Homelab 用户，我希望安装包体积控制在 15MB 以内，以便快速分发安装。

## Implementation Decisions

- **技术栈**：Go + Wails v2；前端 **Vue 3 + TypeScript + Tailwind CSS**（开发者选定）；目标平台 Windows / macOS / Linux。
- **打包**：Wails NSIS 生成安装包，体积目标 ≤ 15MB。
- **配置存储**：用户配置目录下的本地 JSON 文件，保存 API Token、认证状态与应用设置。
- **日志**：本地滚动日志，单文件上限约 1MB。
- **cloudflared 管理**：`exec.Command` 运行 `cloudflared tunnel run --token <token>`；停止时发送终止信号优雅退出；stdout/stderr 经 Wails Events 推送到前端。
- **Cloudflare API 封装**：`cfapi` 包封装 Tunnel 的 List / Create / Delete / Token、Configurations GET / PUT、Zones 与 DNS records 的 CRUD（参考实现：`cfapi` 的 `PushIngressConfig`、`cfgate` 的 `IngressRule` 结构）。
- **Ingress 规则**：`IngressRule { Hostname, Service }`；强制末位无 hostname 的 404 兜底规则；顺序匹配；通配符仅允许左侧 `*.`；编辑时实时校验。
- **DNS 联动**：绑定域名自动创建 CNAME → `<tunnel-id>.cfargotunnel.com`；删除规则可选同步删除 DNS 记录。
- **认证 v1（必做）**：API Token 直填；权限要求 `Cloudflare Tunnel: Edit`、`DNS: Edit`、`Zone: Read`。
- **认证 v2（第二版）**：Authorization Code with PKCE（S256、无 client secret）；本地回调 `localhost:8877`；scopes：`argotunnel.read/write`、`dns.read/write`、`zone.read`；前置条件：Cloudflare Dashboard → Manage Account → OAuth clients 创建应用。
- **测试 seam**（与用户确认）：主 seam `CFClient` 接口 + `httptest` 假服务器；次 seam `ProcessManager` 接口（假命令记录器）；Ingress 校验器作为纯函数直接单测。

## Testing Decisions

- 只测外部行为，不测实现细节；优先复用既有 seam，seam 数量最小化。
- **主 seam**：`CFClient` 接口 + `httptest` 假服务器 —— Tunnel CRUD、配置推送、DNS 联动、OAuth token 交换均通过该 seam 验证请求路径、载荷与错误映射，不触碰真实网络。
- **次 seam**：`ProcessManager` 接口（检测 / 下载 / 启停 / 日志），测试注入假命令记录器。
- **纯函数单测**：Ingress 规则校验器（末位 404 兜底、通配符仅左侧、顺序匹配）。
- **前端**：组件测试 + Wails 集成冒烟测试。

## Out of Scope

- OAuth 登录（第二版，单独排期）。
- 多账户支持与账户切换、Ingress `originRequest` 高级配置（超时 / TLS 验证 / HTTP2）、按 path 正则分流、Cloudflare Access 策略、Connector 状态展示、自动重启、批量操作、环境标签、Docker 镜像、配置历史与回滚。

## Further Notes

- 参考实现：`cfapi` 包（Tunnel List / Create / Delete + `PushIngressConfig`）；`cfgate` 展示的 `IngressRule` 完整 Go 结构。
- 未来扩展方向（多账户、originRequest、路径匹配、Access、Connector 状态、自动重启、批量、环境标签、Docker、配置历史）见上方 Out of Scope 与原始需求文档的扩展表。
