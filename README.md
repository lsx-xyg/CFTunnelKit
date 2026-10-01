# CFTunnelKit

<div align="center">

**可视化管理 Cloudflare Tunnel 的桌面工具**

无需命令行，点点鼠标就能配好域名 → 本地端口的隧道。

![Build](https://github.com/lsx-xyg/CFTunnelKit/actions/workflows/release.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/lsx-xyg/CFTunnelKit)
![Platform](https://img.shields.io/badge/platform-Windows-blue)
![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)

</div>

---

## 这是什么？

CFTunnelKit 是一个跨平台桌面应用（当前支持 Windows），帮你用图形界面管理 Cloudflare Tunnel：

- 不用敲 `cloudflared` 命令
- 不用手写 `config.yml`
- 自动创建 DNS CNAME 记录
- 关闭窗口后隐藏到托盘，下次打开自动恢复上次运行的隧道

适合 NAS / Homelab 用户，想把本地服务暴露到公网但不想记命令的人。

## 功能

- 🔐 **Token 认证** — 粘贴 Cloudflare API Token，自动校验三项权限
- 📋 **Tunnel 管理** — 创建、删除、启动、停止、查看运行 Token
- 🌐 **Ingress 规则** — 可视化编辑 域名 → 本地端口，即时保存
- 🚀 **DNS 自动联动** — 添加规则时自动创建 CNAME，删除时同步清理
- 📝 **浮动终端** — `Ctrl+` ` 打开，可拖拽调高度、全屏、搜索、级别筛选
- 📦 **系统托盘** — 关闭窗口隐藏到托盘，右键开机自启切换
- 🔄 **自动恢复** — 下次打开自动连接上次运行的隧道
- 📁 **日志落盘** — app.log / cloudflared.log / operation.log 三个文件自动轮转

## 快速开始

### 1. 下载

去 [Releases](https://github.com/lsx-xyg/CFTunnelKit/releases) 下载最新 `cftunnelkit.exe`，双击运行（无需安装）。

> ⚠️ **未签名**：Windows SmartScreen 提示时点「更多信息 → 仍要运行」。

### 2. 创建 API Token

打开 [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens) → Create Token → Custom token，勾选：

| 权限 | 级别 |
|---|---|
| Account · Cloudflare Tunnel | **Edit** |
| Zone · DNS | **Edit** |
| Zone · Zone | **Read** |

复制生成的 Token。

### 3. 开始使用

1. 打开 CFTunnelKit，粘贴 Token，点「验证并保存」
2. 点「创建 Tunnel」，输入名称（如 `my-nas`）
3. 选中 Tunnel，点「启动」— cloudflared 会自动下载
4. 点「Ingress」，添加规则：`nas.example.com` → `http://localhost:8080`
5. 保存时弹窗问是否创建 DNS，选「是」
6. 浏览器打开 `https://nas.example.com`，应该能访问到你的本地服务了

## 配置与日志

```
~/.cftunnelkit/
├── config.json           # API Token + 上次运行的 tunnel 列表
├── bin/cloudflared       # 自动下载的 cloudflared 二进制
└── logs/
    ├── app.log           # 应用日志
    ├── cloudflared.log   # cloudflared 输出（1MB 轮转 ×3）
    └── operation.log     # 操作记录
```

## 开发

前置：Go ≥ 1.25、Node ≥ 20。

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
git clone https://github.com/lsx-xyg/CFTunnelKit.git
cd CFTunnelKit
cd frontend && npm ci && cd ..
wails dev
```

### 打包

```bash
wails build
```

产物在 `build/bin/cftunnelkit.exe`。

### Linux 打包

交叉编译到 Windows（CI 使用 windows-latest 原生构建）：

```bash
wails build -platform windows/amd64
```

### GitHub Actions

打 tag 自动构建并发布：

```bash
git tag v0.6.0
git push origin v0.6.0
```

## 技术栈

- **后端**：Go 1.26 · Wails v2.16 · energye/systray
- **前端**：Vue 3 · TypeScript · Tailwind CSS · Vite
- **网络**：内置 Mozilla CA bundle（121 根）+ 阿里 DNS resolver（223.5.5.5）

## 路线图

- [x] 后台轮询 Cloudflare tunnel 状态 — [#37](https://github.com/lsx-xyg/CFTunnelKit/issues/37)
- [x] 迁移到 cloudflare/cloudflare-go 官方 SDK v7 — [#38](https://github.com/lsx-xyg/CFTunnelKit/issues/38)
- [ ] VerifyToken 迁移到 SDK — [#41](https://github.com/lsx-xyg/CFTunnelKit/issues/41)
- [ ] Linux / macOS 构建
- [ ] 代码分层与前端组件重构

## License

[MIT](LICENSE) © lsx-xyg
