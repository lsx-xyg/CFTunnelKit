# CFTunnelKit 领域上下文

## 前端 Wails 调用分层

- **现状**：组件统一从 `@/api` 引入业务方法，runtime Events（EventsOn/Off/Emit）例外直接 import `wailsjs/runtime/runtime`。
- **为什么**：集中封装 Wails 调用，便于类型 re-export，组件不直连生成代码。
- **待办**：无。

## Cloudflare SDK 迁移

- **现状**：`sdkclient.go` 用 cloudflare-go v7 SDK；`cfclient.go` 仍有手写 HTTP 方法，`VerifyToken` 还在手写 client。
- **为什么**：SDK 迁移进行到一半，Tunnel/Ingress/DNS/Zones/Token 已切 SDK。
- **待办**：完成 VerifyToken 迁移（issue #41），删掉 cfclient.go 手写方法。

## app.go 结构

- **现状**：所有 Wails 绑定方法在一个 App struct 上（约 300 行）。
- **为什么**：当前规模可接受，改动频繁时不宜过早定接口。
- **待办**：需求稳定后再按领域拆 handler（Auth/Tunnel/Ingress/DNS）。

## git 历史

- **现状**：已完成三批 squash，碎片 commit 合并。
- **为什么**：提升 git log 可读性。
- **待办**：最早 7 个 commit 未处理，可不做。
