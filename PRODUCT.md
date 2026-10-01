# Product

<!-- impeccable:product-schema 1 -->

## Platform

web (Wails 桌面应用，当前 Windows)

## Users

有一定技术但懒得敲命令的开发者 / 运维 / Homelab 用户。熟悉 Cloudflare 基本概念，但不想记 `cloudflared` 命令和手写 `config.yml`。

## Product Purpose

用图形界面管理 Cloudflare Tunnel，把"配置 Token → 创建隧道 → 启动 → 配 Ingress"这条第一次成功路径做到最短。

## Positioning

- 比 Cloudflare dashboard 快：dashboard 要网页登录、多 tab 切换；这个 app 打开就在列表。
- 比命令行简单：不用记参数、不用手写 YAML、自动管 cloudflared 二进制。
- 比系统服务方案轻：app 在线隧道才在线，不需要装 Windows 服务。

## Operating Context

- Windows 桌面应用，关闭窗口隐藏到托盘
- cloudflared 二进制自动下载到 `~/.cftunnelkit/bin/`
- 配置持久化到 `~/.cftunnelkit/config.json`
- 日志分三个文件：app.log / cloudflared.log / operation.log

## Capabilities and Constraints

- Token 认证（Cloudflare API Token）
- Tunnel CRUD（创建/删除/启动/停止/查看详情）
- Ingress 规则可视化编辑
- DNS CNAME 自动联动
- 托盘菜单 + 开机自启
- 全局快捷键 Ctrl+` 呼出终端
- 轮询间隔可调

## Product Principles

1. 技术信息可读——不藏 tunnel ID、状态、权限，有技术的用户要看得到
2. 一次操作解决一个场景——不搞多步向导，点按钮就做一件事
3. 状态可见——运行中/断开/异常一眼看出，不用猜
4. 破坏性操作要确认——删除 Tunnel、删除 DNS 必须二次确认
5. 错误说人话——400/403 翻译成用户能懂的中文提示
6. 组件优先用库——不手写复杂交互，shadcn / lucide 优先

## Accessibility & Inclusion

中文界面，状态用颜色 + 文字双重表达（不靠色盲单通道）。
