<div align="center">

# CFTunnelKit

**可视化管理 Cloudflare Tunnel 的 Windows 桌面工具，无需命令行，自动配置 Ingress 规则和 DNS。**

![Build](https://github.com/lsx-xyg/CFTunnelKit/actions/workflows/release.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/lsx-xyg/CFTunnelKit)
![Platform](https://img.shields.io/badge/platform-Windows-blue)
![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)

[下载](https://github.com/lsx-xyg/CFTunnelKit/releases) · [反馈 Issue](https://github.com/lsx-xyg/CFTunnelKit/issues)

</div>

---

## 目录

- [这是什么](#-这是什么)
- [功能](#-功能)
- [安装](#-安装)
- [使用](#-使用)
- [配置](#-配置)
- [开发](#-开发)
- [项目结构](#-项目结构)
- [贡献](#-贡献)
- [License](#-license)

---

## 💡 这是什么

CFTunnelKit 是一个 Windows 桌面应用，帮你用图形界面管理 Cloudflare Tunnel（原 Argo Tunnel）：不用敲 `cloudflared` 命令，不用手写 `config.yml`，添加规则时自动创建 DNS CNAME 记录。

适合 NAS / Homelab 用户、开发者、运维，想把本地服务暴露到公网但不想记命令的人。

> [!NOTE]
> 和官方 Dashboard 的区别：官方只能管理 Tunnel 本身，Ingress 要手写 YAML；CFTunnelKit 可视化编辑规则，自动联动 DNS，桌面端一键启停。

---

## ✨ 功能

| 功能 | 说明 |
|:---|:---|
| 🔐 Token 认证 | 粘贴 Cloudflare API Token，自动校验 Tunnel:Edit / Zone:Read / DNS:Edit 三项权限 |
| 📋 Tunnel 管理 | 创建、删除、启动、停止、查看运行 Token |
| 🌐 Ingress 规则 | 两段式可视化编辑（子域 + Zone 下拉），即时保存 |
| 🚀 DNS 联动 | 添加规则时自动创建 CNAME，删除时静默清理 |
| 📝 浮动终端 | <kbd>Ctrl</kbd> + <kbd>`</kbd> 打开，可拖拽调高度、全屏、搜索 |
| 📦 系统托盘 | 关闭窗口隐藏到托盘，右键开机自启切换 |
| 🔄 自动恢复 | 下次打开自动连接上次运行的隧道 |
| 🔔 检查更新 | 启动时静默检查，红点提示，一键跳转下载 |
| 📁 日志落盘 | app.log / cloudflared.log / operation.log 自动轮转 |

---

## 🚀 安装

### 下载即用

从 [Releases](https://github.com/lsx-xyg/CFTunnelKit/releases) 下载最新 `cftunnelkit.exe`，双击运行（无需安装）。

> [!WARNING]
> 未签名，Windows SmartScreen 提示时点「更多信息 → 仍要运行」。

### 从源码构建

前置：Go ≥ 1.25、Node ≥ 20。

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
git clone https://github.com/lsx-xyg/CFTunnelKit.git
cd CFTunnelKit
cd frontend && npm ci && cd ..
wails build -clean
```

产物在 `build/bin/cftunnelkit.exe`。

---

## 📖 使用

### 1. 创建 API Token

打开 [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens) → Create Token → Custom token：

| 权限 | 级别 |
|:---|:---|
| Account · Cloudflare Tunnel | Edit |
| Zone · DNS | Edit |
| Zone · Zone | Read |

### 2. 配置应用

1. 打开 CFTunnelKit，粘贴 Token，点「验证并保存」
2. 点「创建 Tunnel」，输入名称（如 `my-nas`）
3. 选中 Tunnel，点「启动」— cloudflared 会自动下载
4. 点「Ingress」，添加规则：选 Zone + 填子域，或选「自定义」输入完整域名
5. 保存时弹窗确认是否创建 DNS CNAME
6. 浏览器打开 `https://your-domain.com` 验证

<!-- 替换为实际截图：docs/screenshot-main.png -->

---

## ⚙️ 配置

配置文件位置：

```text
~/.cftunnelkit/
├── config.json
├── bin/cloudflared
└── logs/
    ├── app.log
    ├── cloudflared.log
    └── operation.log
```

<details>
<summary>config.json 字段说明</summary>

| 字段 | 说明 |
|:---|:---|
| `api_token` | Cloudflare API Token |
| `account_id` | Cloudflare 账户 ID |
| `last_running` | 上次运行的 tunnel ID 列表 |
| `last_update_check` | 上次成功检查更新的 Unix 时间戳 |

</details>

---

## 🛠️ 开发

```bash
wails dev
```

打 tag 触发 CI 自动构建发布：

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```

---

## 📁 项目结构

```text
.
├── app.go / main.go          # Wails 入口
├── internal/
│   ├── auth/                 # 认证 + Cloudflare client
│   ├── cloudflare/           # SDK client + transport
│   ├── config/               # 配置读写
│   ├── process/              # cloudflared 进程管理
│   ├── service/              # Wails 绑定 handler
│   └── version/               # 版本号（ldflags 注入）
├── frontend/
│   ├── src/
│   │   ├── api/              # Wails 调用封装
│   │   ├── components/       # Vue 组件
│   │   ├── composables/       # useEscape / useFocusTrap
│   │   ├── utils/             # friendlyError 等
│   │   └── views/            # AuthView / MainView / IngressEditorView
│   └── wailsjs/              # Wails 生成（勿手动改）
└── build/windows/            # NSIS 安装脚本
```

---

## 🤝 贡献

Issue 和 PR 欢迎。提交前请确保 `wails build` 通过、`vue-tsc --noEmit` 无类型错误。

---

## 📄 License

[MIT](LICENSE) © lsx-xyg
