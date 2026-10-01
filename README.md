# CFTunnelKit

![Build](https://github.com/lsx-xyg/CFTunnelKit/actions/workflows/release.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/lsx-xyg/CFTunnelKit)
![Platform](https://img.shields.io/badge/platform-Windows-blue)
![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)

可视化管理 Cloudflare Tunnel 的 Windows 桌面工具，无需命令行，自动配置 Ingress 规则和 DNS 记录。

## 目录

- [安装](#安装)
- [使用](#使用)
- [配置](#配置)
- [项目结构](#项目结构)
- [开发](#开发)
- [贡献](#贡献)
- [License](#license)

## 安装

### 下载即用

从 [Releases](https://github.com/lsx-xyg/CFTunnelKit/releases) 下载最新 `cftunnelkit.exe`，双击运行（无需安装）。

> Windows SmartScreen 提示时点「更多信息 → 仍要运行」（未签名）。

### 从源码构建

前置依赖：Go ≥ 1.25、Node ≥ 20。

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
git clone https://github.com/lsx-xyg/CFTunnelKit.git
cd CFTunnelKit
cd frontend && npm ci && cd ..
wails build -clean
```

产物在 `build/bin/cftunnelkit.exe`。

## 使用

### 1. 创建 API Token

打开 [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens) → Create Token → Custom token，勾选：

| 权限 | 级别 |
|---|---|
| Account · Cloudflare Tunnel | Edit |
| Zone · DNS | Edit |
| Zone · Zone | Read |

复制生成的 Token。

### 2. 配置应用

1. 打开 CFTunnelKit，粘贴 Token，点「验证并保存」
2. 点「创建 Tunnel」，输入名称（如 `my-nas`）
3. 选中 Tunnel，点「启动」— cloudflared 会自动下载
4. 点「Ingress」，添加规则：子域 + Zone 下拉，或选「自定义」输入完整域名
5. 保存时弹窗确认是否创建 DNS CNAME
6. 浏览器打开 `https://your-domain.com` 验证

### 快捷键

- `Ctrl+` ` — 打开/关闭浮动日志终端

## 配置

配置文件位置：

```
~/.cftunnelkit/
├── config.json           # API Token + 上次运行的 tunnel 列表 + 更新检查时间戳
├── bin/cloudflared      # 自动下载的 cloudflared 二进制
└── logs/
    ├── app.log           # 应用日志
    ├── cloudflared.log   # cloudflared 输出（1MB 轮转 ×3）
    └── operation.log     # 操作记录
```

`config.json` 字段说明：

| 字段 | 说明 |
|---|---|
| `api_token` | Cloudflare API Token |
| `account_id` | Cloudflare 账户 ID |
| `last_running` | 上次运行的 tunnel ID 列表（启动自动恢复） |
| `last_update_check` | 上次成功检查更新的 Unix 时间戳（6 小时间隔） |

## 项目结构

```
.
├── app.go / main.go           # Wails 入口
├── internal/
│   ├── auth/                  # 认证 + Cloudflare client 封装
│   ├── cloudflare/            # SDK client + transport（CA bundle + DNS resolver）
│   ├── config/                # 配置读写
│   ├── process/               # cloudflared 进程管理
│   ├── service/               # Wails 绑定 handler（按领域拆分）
│   └── version/               # 版本号（ldflags 注入）
├── frontend/
│   ├── src/
│   │   ├── api/               # Wails 调用封装层
│   │   ├── components/        # Vue 组件
│   │   ├── composables/       # useEscape / useFocusTrap
│   │   ├── utils/             # friendlyError 等工具函数
│   │   └── views/             # AuthView / MainView / IngressEditorView
│   └── wailsjs/               # Wails 生成的绑定（勿手动修改）
└── build/
    └── windows/               # NSIS 安装脚本
```

## 开发

```bash
wails dev
```

### 发布

打 tag 触发 GitHub Actions 自动构建并发布：

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

CI 通过 ldflags 注入版本号到 `internal/version/version.go`。

## 贡献

Issue 和 PR 欢迎。提交前请确保 `wails build` 通过、`vue-tsc --noEmit` 无类型错误。

## License

[MIT](LICENSE) © lsx-xyg
