# CFTunnelKit

Cloudflare Tunnel 桌面管理器 —— 基于 **Go + Wails** 的跨平台桌面应用，可视化管理 Cloudflare Tunnel、Ingress 规则（端口映射）与域名绑定，并管理本地 `cloudflared` 进程生命周期。目标用户：NAS / Homelab 用户。

## 当前进度（切片 01 · 项目骨架与认证 v1）

已实现：

- Wails 应用骨架（Go 后端 + Vue 3 + TypeScript + Tailwind CSS 前端）
- 认证页：粘贴 Cloudflare API Token → 校验（`GET /user/tokens/verify`）→ 账户解析（`/accounts`，403/为空时回退 `/user/memberships`）→ 权限探测（`Tunnel:Edit` / `Zone:Read` / `DNS:Edit`）
- 配置持久化：`~/.cftunnelkit/config.json`（Windows `%USERPROFILE%\.cftunnelkit\`，`os.UserHomeDir()` 统一；POSIX 权限 0600）
- 启动恢复：有效 Token 直接进入主界面；失效自动清除配置回认证页；网络不可达显示"离线，未验证"+ 手动重试按钮
- 测试 seam：`CFClient` 接口 + `httptest` 假服务器（合法 / 无效 / 权限缺失 / 网络错误 / 无 Zone 五种场景）

详情见 [SPEC](SPEC.md) 与 GitHub issue（`gh issue list`）。

## 开发

前置：Go ≥ 1.25（`GOTOOLCHAIN=auto` 可自动拉取）、Node ≥ 20、Wails CLI（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）。

```bash
# 运行单元测试（后端 seam / config / auth 状态机）
go test ./...

# 开发模式（热重载）
wails dev

# 构建
wails build          # Linux 默认 webkit2gtk-4.0
wails build -tags webkit2_41   # Linux 使用 webkit2gtk-4.1
```

> Linux 构建需要 `libgtk-3-dev`、`libwebkit2gtk-4.1-dev`（或 4.0）、`libayatana-appindicator3-dev`、`librsvg2-dev`。

## 目录结构

```
app.go                  # Wails App 与前端绑定（GetAuthState / VerifyAndSaveToken / RetryVerify）
main.go                 # Wails 入口
internal/
  cloudflare/           # CFClient 接口 + HTTP 实现 + 错误映射（httptest seam）
  config/               # 配置存储（JSON，0600）
  auth/                 # 认证状态机（验证-保存 / 启动恢复 / 离线重试）
frontend/
  src/views/            # AuthView（认证页）/ MainView（主界面占位）
  src/components/       # PermissionBadge 等
```

## 已知事项

- API Token 以明文存于本地配置（v1 设计，系统钥匙串列入后续切片）
- 权限探测为只读探测（"该权限族至少可读"），真实写操作权限由对应切片在操作时报错验证
- Windows 安装包暂未代码签名（见切片 07b）
