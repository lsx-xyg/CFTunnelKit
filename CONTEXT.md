# CFTunnelKit 领域上下文

## 前端 Wails 调用分层

- **现状**：组件统一从 `@/api` 引入业务方法，runtime Events（EventsOn/Off/Emit）例外直接 import `wailsjs/runtime/runtime`。
- **为什么**：集中封装 Wails 调用，便于类型 re-export，组件不直连生成代码。
- **待办**：无。

## Cloudflare SDK 迁移

- **现状**：Tunnel/Ingress/DNS/Zones/Token 已切 cloudflare-go v7 SDK；`VerifyToken` 仍在手写 client。
- **为什么**：SDK 迁移进行到一半，剩余 VerifyToken 需要等 SDK 有对应封装。
- **待办**：完成 VerifyToken 迁移（issue #41），删掉 cfclient.go 手写方法。

## app.go 结构

- **现状**：已拆出 7 个 handler（system/window/auth/tunnel/process/ingress/dns），app.go 只剩组合 + 委托。
- **为什么**：god object 拆分完成，每个 handler 职责单一。
- **待办**：生命周期逻辑（startup/shutdown/RestoreRunning）后续可再拆。

## 版本号与更新检查

- **现状**：版本号在 `internal/version/version.go`，build 时通过 ldflags 注入。
- **为什么**：main 包不能被 internal import，所以版本变量单独放 internal/version。
- **更新检查**：启动后 5 秒静默检查 GitHub releases，6 小时间隔；有新版本时"更多"菜单显示红点。
- **待办**：无。

## git 历史

- **现状**：已完成三批 squash，碎片 commit 合并。
- **为什么**：提升 git log 可读性。
- **待办**：最早 7 个 commit 未处理，可不做。
