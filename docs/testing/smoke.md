# CFTunnelKit 全链路手动冒烟清单（切片 07b）

在真实资源上手动走通：**安装 → 认证 → 建隧道 → 配规则 → DNS → 启停**。
每步记录：操作 / 预期结果 / 实际结果 / 备注。

## 前提

- Cloudflare 账户 + API Token，权限：`Cloudflare Tunnel: Edit`、`DNS: Edit`、`Zone: Read`
- 一个已托管在 Cloudflare 的域名（Zone，用于 DNS 联动）
- 本地 `localhost:8080` 起一个简单 HTTP 服务（如 `python -m http.server 8080`）
- 目标平台运行环境（Windows / macOS / Linux）

## 1. 安装应用

| 项 | 内容 |
|---|---|
| 操作 | 运行安装包 / 解压二进制并启动 CFTunnelKit |
| 预期结果 | 应用窗口打开，进入认证页 |
| 实际结果 | |
| 备注 | Windows 未签名：SmartScreen → "更多信息 → 仍要运行"；macOS：右键 → 打开 |

## 2. 认证

| 项 | 内容 |
|---|---|
| 操作 | 粘贴 API Token → 校验 |
| 预期结果 | 校验通过进入主界面；三项权限（Tunnel:Edit / Zone:Read / DNS:Edit）均显示"ok" |
| 实际结果 | |
| 备注 | 重启应用后认证状态保持，无需重复输入 |

## 3. 建隧道

| 项 | 内容 |
|---|---|
| 操作 | 主界面 → "+ 创建 Tunnel" → 输入名称 → 创建 |
| 预期结果 | Tunnel 出现在列表（状态 inactive）；弹窗展示运行 Token 可复制 |
| 实际结果 | |
| 备注 | Token 获取失败时 Tunnel 仍创建成功，提示在详情页重试 |

## 4. 配规则（Ingress）

| 项 | 内容 |
|---|---|
| 操作 | 行内"Ingress"按钮 → 添加规则 `nas.example.com → http://localhost:8080` → 保存 |
| 预期结果 | 保存成功 toast；回读与输入一致；顶部提示"规则从上到下匹配，第一条生效" |
| 实际结果 | |
| 备注 | hostname 根域名必须属于当前账户 Zone；仅剩兜底时保存按钮置灰 |

## 5. DNS 联动

| 项 | 内容 |
|---|---|
| 操作 | 保存规则后弹窗确认创建 DNS → 确认 |
| 预期结果 | 创建 CNAME `nas → <tunnel-id>.cfargotunnel.com`（结果弹窗显示"已创建 CNAME"） |
| 实际结果 | |
| 备注 | 同名已指向当前 Tunnel → 幂等不报错；指向他处 → 提示占用不覆盖 |

## 6. 启停与日志

| 项 | 内容 |
|---|---|
| 操作 | 行内"启动" → 观察日志面板 → 停止；重复"启动"一次 |
| 预期结果 | 日志实时逐行显示；状态点变绿"运行中"；停止后恢复；重复启动被拒绝提示"该 Tunnel 已在运行" |
| 实际结果 | |
| 备注 | 意外退出推 status=error，不自动重启 |

## 7. 日志滚动（Linux/macOS 可验证；Windows 用相同路径规则）

| 项 | 内容 |
|---|---|
| 操作 | 持续输出日志直到超过 1MB（或临时调小 `MaxSize` 复现），检查 `logs/` 目录 |
| 预期结果 | 单文件到 1MB 轮转，保留 `cloudflared.log` + `.log.1` + `.log.2`（最多 3 个备份），不压缩 |
| 实际结果 | |
| 备注 | 日志路径：`~/.cftunnelkit/logs/cloudflared.log`（Windows `%USERPROFILE%\.cftunnelkit\logs\`） |

## 失败记录模板

| 步骤 | 实际结果 | 错误信息 | 备注 |
|---|---|---|---|
| | | | |

> 记录后请将失败项反馈到仓库 issue，附错误信息与截图（可选）。
