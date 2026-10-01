# AGENTS.md

## Agent skills

### Issue tracker

Issues and specs live as GitHub issues; use the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Five canonical roles mapped to `needs-triage` / `needs-info` / `ready-for-agent` / `ready-for-human` / `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout: one `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.

## 前端 Wails 调用约定

- 新增或修改 Vue 组件时，调用 Wails 后端方法统一从 `@/api` 引入，
  不再直接 import `wailsjs/go/main/App`。
- `@/api` 是唯一的前端业务调用入口，负责 re-export Wails 方法并做必要适配。
- 如果 `@/api` 尚未导出某方法或类型，先在 `src/api/index.ts` 补充导出，
  再在组件里使用；不要在组件里绕过 api 直连 wailsjs。
- `cloudflare`、`auth` 等 Wails 生成的类型，统一通过 `@/api` re-export。
  注意：`auth` 是值导出（`auth.State` 是 class），不是纯类型。
- 例外：`wailsjs/runtime/runtime`（EventsOn / EventsOff / EventsEmit）
  是 Wails 框架级事件 API，允许组件直接 import，不封装进 `@/api`。
- `wailsjs/` 由 Wails 生成，任何情况下不手动修改。
