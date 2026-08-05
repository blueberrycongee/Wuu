# Plugin Capability Contract — Phase A

## 概述

本文档定义 Wuu 插件平台的公共 capability contract。它是阶段 A 交付物：scope/effect/dispose 模型、typed seam 语义、registries 和 dependency 规则，以及 host-owned safety kernel 边界。

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
| `decision` | 返回类型化决策控制流程 | 是 | 否 | turn 继续、compaction 触发、subagent 路由 |

## Registries

每个 EffectKind 对应一个 typed Registry：

- `Registry[any]` — 类型擦除的通用注册表
- `Register(key, entry)` — 注册贡献并校验依赖约束
- `Get/List/Keys` — 查询活跃条目
- `DepRequired/DepOptional/DepConflicts` — 依赖规则

## Safety Kernel (不可开放)

以下能力始终由 Wuu host 控制，**永不**通过 registry 或 seam 暴露给插件：

1. **插件检查、批准、启用、禁用、升级和删除** (`host.plugin.*`)
2. **Safe mode、崩溃恢复和紧急重启** (`host.safe_mode`, `host.crash_recovery`)
3. **权限和信任提示的最终边界** (`host.permission.final`)
4. **原生窗口和 app-server 生命周期** (`host.window.lifecycle`, `host.appserver.lifecycle`)
5. **Plugin generation 的错误隔离** (`host.generation.isolate`)
6. **用户逃生路径：设置、禁用插件、恢复默认 UI** (`host.escape.settings`, `host.escape.default_ui`)

安全内核之外的产品能力，应优先使用公开扩展路径。如果一项功能只能通过修改 Agent loop 实现，应先判断缺少的是哪一个公共能力，而不是直接增加产品专用分支。

## 标准 Seam 列表

### Agent Runtime Seams
- `agent.tool.register` (transform)
- `agent.tool.execute.before` (guard)
- `agent.tool.execute.around` (around)
- `agent.tool.execute.after` (transform)
- `agent.system_prompt.section` (transform)
- `agent.context.inject` (transform)
- `agent.request.transform` (transform)
- `agent.response.transform` (transform)
- `agent.compaction` (decision)
- `agent.provider.register` (transform)
- `agent.continuation.policy` (decision)
- `agent.subagent.provider` (decision)
- `agent.permission.policy` (guard)
- `agent.session.lifecycle` (observe)

### Desktop Workbench Seams
- `desktop.view.register` (transform)
- `desktop.layout.apply` (transform)
- `desktop.theme.register` (transform)
- `desktop.renderer.register` (transform)
- `desktop.command.register` (transform)
- `desktop.surface.register` (transform)

## 实施状态

- [x] `internal/plugin/scope.go` — Generation, EffectKind 模型
- [x] `internal/plugin/seam.go` — SeamKind, SeamDispatch, SeamCatalog
- [x] `internal/plugin/registry.go` — Registry, DependencyRule, PluginRegistries
- [x] `internal/plugin/scope_manager.go` — PluginScope, ScopeManager
- [ ] Phase B: 在 agent loop 中使用 registries 替换回调点
- [ ] Phase C: Desktop workbench 开放
- [ ] Phase D: 本地开发闭环
