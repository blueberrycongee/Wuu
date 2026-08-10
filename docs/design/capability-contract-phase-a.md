# Plugin Capability Contract — Phase A

## 概述

本文档定义 Wuu 插件平台的公共 capability contract。当前实现以 capability RPC 作为唯一的生产 seam 模型，由能力描述符携带调度语义，并保留 host-owned safety kernel 边界。

## Generation 模型

每个插件贡献都属于一个 `Generation`——一个唯一的、原子化的注册 scope，绑定到一个特定的插件版本+激活：

- **原子发布**：激活成功后所有注册同时生效
- **原子撤回**：禁用、删除或升级时，所有注册通过 disposed generation 自动移除
- **失败隔离**：部分激活失败会回滚整个 generation
- **无残留状态**：generation 被 dispose 后，该 generation 的任何注册都不会残留
- **异步清理感知**：disposer 可以异步完成，dispose 调用会等待所有 disposer 完成

## Typed Seam 语义

每个扩展点必须声明其调度语义。插件不能把所有扩展点当作相同的任意回调：

| Seam 类型 | 语义 | 短路 | 并发 | 示例 |
|-----------|------|------|------|------|
| `observe` | 观察不可变事件，不影响结果 | 否 | 是 | session 生命周期、审计日志 |
| `transform` | 按稳定顺序变换输入/输出 | 否 | 否 | 消息预处理、工具结果增强 |
| `guard` | 增加约束或拒绝，后续插件不能静默放宽 | 是 | 否 | 权限策略、路径限制 |
| `around` | 包装实际执行 | 否 | 否 | 超时、重试、度量、沙箱 |
| `decision` | 返回类型化决策控制流程 | 是 | 否 | compaction 策略选择 |

## Capability RPC

插件在初始化响应中声明 `CapabilityDescriptor`。宿主校验能力 id、kind、版本、依赖与冲突，
再由 `PluginGeneration` 原子发布；同一能力的多个贡献按优先级和发现顺序稳定调度。

## Safety Kernel (不可开放)

以下能力始终由 Wuu host 控制，**永不**通过 capability RPC 暴露给插件：

1. **插件检查、批准、启用、禁用、升级和删除** (`host.plugin.*`)
2. **Safe mode、崩溃恢复和紧急重启** (`host.safe_mode`, `host.crash_recovery`)
3. **权限和信任提示的最终边界** (`host.permission.final`)
4. **原生窗口和 app-server 生命周期** (`host.window.lifecycle`, `host.appserver.lifecycle`)
5. **Plugin generation 的错误隔离** (`host.generation.isolate`)
6. **用户逃生路径：设置、禁用插件、恢复默认 UI** (`host.escape.settings`, `host.escape.default_ui`)

安全内核之外的产品能力，应优先使用公开扩展路径。如果一项功能只能通过修改 Agent loop 实现，应先判断缺少的是哪一个公共能力，而不是直接增加产品专用分支。

## 当前生产能力

- `agent.request.transform` (transform)
- `agent.pre_step` (transform, sourced durable append)
- `agent.system_prompt.section` (transform)
- `agent.compaction` (decision)
- `agent.turn.completed` (observe)
- `agent.turn.lifecycle` (observe, owner-scoped)
- `plugin.client.request` (decision)

Goal 迁移过程中曾存在产品专用的 `agent.turn.continuation` seam；它要求宿主按
`probe/prepare` 轮询插件并为它启动内部 Turn。该 seam 已删除。Goal 现在观察
`agent.turn.completed`，再主动通过 `host.session.send` 投递到同一个 Session。

Goal 与 Subagent 共同证明通用投递还必须把模型输入、前端展示摘要和来源分开。前端可以把一次
插件投递显示成只读 query 气泡，但持久数据必须标记为插件生成，不能伪造用户作者。通用 Session
合同现在持久化 owner、`user | plugin` 可见性、父子关系、fresh/fork 上下文和 workspace 身份，
并把模型输入、query 气泡摘要、真实来源和 cause 分开。Subagent 已使用同一合同完成迁移，旧的
`host.child_session.request` 已删除；公共 Session 合同同时补齐 owner-scoped list/cancel、worktree
和最终输出 lifecycle。

Plan 已作为 bundled 一方插件完成纵向迁移：插件 runtime 拥有 Tool、参数校验与结果合同，Desktop
模块拥有 Tool Activity Presenter 与 Inspector section。宿主只持久化普通 Tool call/result 事实，
并按公开 `display.capability = "plan"` 投影版本化事件和只读 snapshot；核心不再注册 `update_plan`、
保存/恢复可变计划状态、注入过期提醒或渲染原生计划 UI。Plan 不承担跨 Turn 自动续跑。
HelpMe 直接删除，不形成 capability 或兼容层。

## 实施状态

- [x] `internal/pluginhost/capability_rpc.go` — capability negotiation、SeamKind、ErrorPolicy 与 safety-kernel 校验
- [x] `internal/runtime/plugin_generation.go` — 生产 generation 的原子发布与撤销
- [x] 真实 Agent 链路通过 capability RPC 注册、排序和调用插件贡献
- [x] Phase C: Desktop workbench registrations 已接入真实 View、Slot、导航、设置、Inspector、Presenter 和 Style 产品路径；六个 bundled 一方插件的真实 `desktop.js` 生命周期测试覆盖 generation 原子激活、替换、禁用与完整撤回
- [x] Phase D: `wuu plugin create/validate/build/test/pack/dev` 已形成独立插件仓闭环；脚手架只依赖公开 `@wuu/plugin-sdk`，runtime 契约测试启动真实子进程，dev generation 在构建或激活失败时保留上一代

这些阶段只有在公共 SDK 的外部插件能够完成注册、调用、卸载和失败恢复，并有产品路径测试证明时才算完成。仅存在接口、registry 或协议类型不代表该能力已经开放。
