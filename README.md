# CFTunnelKit

Cloudflare Tunnel 桌面管理器 —— 基于 **Go + Wails** 的跨平台桌面应用，可视化管理 Cloudflare Tunnel、Ingress 规则（端口映射）与域名绑定，并管理本地 `cloudflared` 进程生命周期。目标用户：NAS / Homelab 用户。

## 当前进度（切片 01–07b 全部完成）

已实现：

- **认证（01）**：粘贴 Cloudflare API Token → 校验（`GET /user/tokens/verify`）→ 账户解析（`/accounts`，403/为空时回退 `/user/memberships`）→ 权限探测（`Tunnel:Edit` / `Zone:Read` / `DNS:Edit`）；配置持久化 `~/.cftunnelkit/config.json`（Windows `%USERPROFILE%\.cftunnelkit\`，`os.UserHomeDir()` 统一，0600）；启动恢复 / 离线"重试"按钮
- **Tunnel 列表（02）**：`ListTunnels`（page/per_page 预留）、状态色块映射（healthy/degraded/down/inactive，未知状态灰色兜底）、401 清配置跳认证页、403/网络错误停留 + 重试、空态/骨架屏/错误态
- **cloudflared 进程管理（03）**：二进制检测/下载（平台映射、temp+rename 原子替换、1MB 校验、重试、并发保护、进度事件）、启动/停止（Setpgid 进程组、SIGTERM→5s→SIGKILL、Windows taskkill 兜底）、多 Tunnel 独立、应用退出全停、意外退出不自动重启、日志逐行推送（ANSI 剥离、5000 行前端上限）
- **Tunnel 创建/删除/详情（04）**：创建（name 校验、409 重名、成功即弹 Token 可复制、Token 获取失败不阻塞）、详情（状态/连接数/Token 显示复制）、删除（输名称确认、活跃连接 1003 拦截、成功跳列表刷新）
- **Ingress 编辑器（05）**：域名→端口规则可视化编辑，末位 404 兜底自动维护不可见，行级校验（重复/通配符/service 格式/Zone 根域名），上/下排序，保存 = PUT + GET 回读归一化比对（不一致保留输入报错），未保存离开确认
- **DNS 联动（06）**：保存 Ingress 时询问创建 CNAME（新增/移除域名分组、默认勾选、可取消）；幂等三态（已指向当前 Tunnel 不报错 / 占用提示不覆盖 / 创建）；删除规则联动删 DNS（失败提示不影响规则删除）；最长 Zone 匹配 + 相对 name 转换
- **NSIS 打包（07a）**：`scripts/nsis-build.sh` 交叉编译 Windows 安装包（`cftunnelkit-amd64-installer.exe`，约 6.4MB）并生成 `checksums.txt`
- **日志落盘轮转（07b）**：`~/.cftunnelkit/logs/cloudflared.log`，单文件 1MB 轮转、保留 3 个、不压缩（lumberjack）；`scripts/smoke.md` 全链路手动冒烟清单

详情见 [SPEC](SPEC.md) 与 GitHub issue（`gh issue list`）。

## 下载

从 [Releases](https://github.com/lsx-xyg/CFTunnelKit/releases) 下载 Windows 可执行文件（`cftunnelkit.exe`）。

**未签名**：Windows SmartScreen 提示时点"更多信息 → 仍要运行"。

## 构建 Linux 包

CI 目前只打 Windows。本地打 Linux：

```bash
# 安装 Linux 依赖
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev

# 交叉编译或本机编译
wails build -platform linux/amd64 -s
# 产物在 build/bin/cftunnelkit
```

## 功能

- 粘贴 Cloudflare API Token 管理 Tunnel
- 可视化编辑 Ingress 规则（域名→本地端口）
- 自动创建/删除 DNS CNAME 记录
- 启停 cloudflared 进程，实时日志
- 关闭窗口隐藏到系统托盘，托盘右键退出
- `Ctrl+` ` 打开浮动终端日志

## 开发

前置：Go ≥ 1.25（`GOTOOLCHAIN=auto` 可自动拉取）、Node ≥ 20、Wails CLI（`go install github.com/wailsapp/wails/v2/cmd/wails@latest`）。

```bash
# 运行单元测试（后端 seam / config / auth / ingress / dns / process）
go test ./...

# 开发模式（热重载）
wails dev

# Linux 构建
wails build -tags webkit2_41   # 使用 webkit2gtk-4.1（源默认）
```

> Linux 构建需要 `libgtk-3-dev`、`libwebkit2gtk-4.1-dev`、`libayatana-appindicator3-dev`、`librsvg2-dev`。

## 打包 Windows 安装包

产物输出到 `build/bin/`：`cftunnelkit.exe`（裸 exe）和 `cftunnelkit-amd64-installer.exe`（NSIS 安装包，约 6MB）。

### 方式 A：Windows 本机构建（推荐）

在 Windows 仓库目录下：

```powershell
# 前置：安装 NSIS 并确保 makensis 在 PATH
#   winget install NSIS.NSIS
#   或从 https://nsis.sourceforge.io/Download 安装，把 C:\Program Files (x86)\NSIS 加入 PATH

wails build -nsis -clean
```

产物直接在 `build/bin/`。

### 方式 B：Linux 交叉编译（CI / 无 Windows 机器时）

```bash
# 前置：nsis + mingw-w64（Debian/Ubuntu）
sudo apt-get install -y nsis mingw-w64
./scripts/nsis-build.sh
```

脚本会：交叉编译 Windows amd64 → 调 NSIS 生成安装包 → 生成 `checksums.txt`（安装包 + exe 的 SHA256）。

### 产物校验

```bash
sha256sum -c build/bin/checksums.txt
```

### 未来：GitHub Actions（计划中）

打 tag 后自动构建 Windows 安装包并上传到 GitHub Release，无需本地打包。

## 目录结构

```
app.go                  # Wails App 与前端绑定（认证 / Tunnel CRUD / Ingress / DNS / 日志）
main.go                 # Wails 入口（OnStartup / OnShutdown）
scripts/
  nsis-build.sh         # 07a Windows 安装包打包（自动生成 checksums.txt）
  smoke.md              # 07b 全链路手动冒烟清单
internal/
  cloudflare/           # CFClient 接口 + HTTP 实现 + 错误映射（httptest seam）
  config/               # 配置存储（JSON，0600）+ bin / logs 路径约定
  auth/                 # 认证状态机 + 全部业务方法（authenticatedClient / clearOnAuth）
  ingress/              # 纯函数：规则校验 / Zone 匹配 / 回读归一化比对
  dns/                  # 纯业务：幂等 CNAME ensure / 按名删除
  process/              # cloudflared 二进制下载 + 进程生命周期 + 日志推送/落盘轮转
frontend/
  src/views/            # AuthView / MainView / IngressEditorView
  src/components/       # PermissionBadge / TunnelStatusBadge / CreateTunnelDialog / TunnelDetailDialog
```

## 已知事项

- API Token 以明文存于本地配置（v1 设计，系统钥匙串列入后续切片）
- 权限探测为只读探测（"该权限族至少可读"），真实写操作权限由操作时报错验证
- **安装包未签名**：
  - Windows：SmartScreen 提示时点击 **"更多信息 → 仍要运行"**
  - macOS：首次打开可能被 Gatekeeper 拦截，需 **右键 → 打开**（未签名应用无官方渠道分发）
  - Linux：无签名需求，正常执行
