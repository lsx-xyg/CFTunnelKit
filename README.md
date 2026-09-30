# CFTunnelKit

<div align="center">

**可视化管理 Cloudflare Tunnel 的 Windows 桌面工具**

无需命令行，点点鼠标就能配好域名 → 本地端口的隧道。

![Build](https://github.com/lsx-xyg/CFTunnelKit/actions/workflows/release.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/lsx-xyg/CFTunnelKit)
![Platform](https://img.shields.io/badge/platform-Windows-blue)
![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)

</div>

---

## 这是什么？

CFTunnelKit 是一个 **Windows 桌面应用**，帮你用图形界面管理 Cloudflare Tunnel：

- 不用敲 `cloudflared` 命令
- 不用手写 `config.yml`
- 自动创建 DNS CNAME 记录
- 关闭窗口后隧道继续跑（托盘常驻）

适合 NAS / Homelab 用户，想把本地服务暴露到公网但不想记命令的人。

## 截图

> _截图待补充_

## 功能

- 🔐 **Token 认证** — 粘贴 Cloudflare API Token，自动校验权限
- 📋 **Tunnel 管理** — 创建、删除、启动、停止、查看运行 Token
- 🌐 **Ingress 规则** — 可视化编辑 域名 → 本地端口，即时保存
- 🚀 **DNS 自动联动** — 添加规则时自动创建 CNAME，删除时可选清理
- 📝 **实时日志** — `Ctrl+` ` 打开浮动终端，支持搜索和级别筛选
- 📦 **系统托盘** — 关闭窗口隐藏到托盘，右键退出
- 📁 **日志落盘** — 所有日志自动存到本地，方便排查问题

## 快速开始

### 1. 下载

去 [Releases](https://github.com/lsx-xyg/CFTunnelKit/releases) 下载最新 `cftunnelkit.exe`，双击即可运行（无需安装）。

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
├── config.json           # API Token（0600 权限）
├── bin/cloudflared       # 自动下载的 cloudflared 二进制
└── logs/
    ├── app.log           # 应用日志
    ├── cloudflared.log   # cloudflared 输出（1MB 轮转 ×3）
    └── operation.log     # 操作记录
```

## 开发

前置：Go ≥ 1.25、Node ≥ 20。

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0

# 克隆
git clone https://github.com/lsx-xyg/CFTunnelKit.git
cd CFTunnelKit

# 安装前端依赖
cd frontend && npm ci && cd ..

# 开发模式（热重载）
wails dev
```

### 打包

```powershell
wails build
```

产物在 `build/bin/cftunnelkit.exe`。

### GitHub Actions

打 tag 自动构建并发布：

```bash
git tag v0.4.0
git push origin v0.4.0
```

## 技术栈

- **后端**：Go 1.26 · Wails v2.16 · energye/systray
- **前端**：Vue 3 · TypeScript · Tailwind CSS · Vite
- **网络**：内置 Mozilla CA bundle + 阿里 DNS resolver

## 路线图

- [ ] cloudflared 作为系统服务运行（关闭 GUI 隧道不掉线）— [#30](https://github.com/lsx-xyg/CFTunnelKit/issues/30)
- [ ] 开机自启
- [ ] Linux / macOS 构建
- [ ] 更好的托盘图标

## 贡献

欢迎提 Issue 和 PR。建议先看 [Issues](https://github.com/lsx-xyg/CFTunnelKit/issues) 里已有的讨论。

## License

[MIT](LICENSE) © lsx-xyg
