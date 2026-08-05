# 验收标准追踪 — 2026-08-06 插件可定制化战略

对照策略文档第 248-263 行的 12 条验收标准。

| # | 标准 | 状态 | 证据 |
|---|------|------|------|
| 1 | 从本地目录安装、批准、启用、禁用和删除 | ✅ | `cmd/wuu/plugin.go`: install/approve/enable/disable/remove |
| 2 | 开发模式下保存后自动重载 | 🟡 | `cmd/wuu/plugin_dev.go`: dev + poll watch 已实现，fsnotify 待 @秦始皇 |
| 3 | 注册工具并包装工具策略 | ✅ | `plugin/seam.go`: agent.tool.* seams (register/execute.before/around/after) |
| 4 | 提供 system prompt/context section | ✅ | `agent/system_prompt.go`: SystemPromptAssembler + SystemPromptProvider |
| 5 | 替换 compaction 或注册 model Provider | ✅ | `agent/compaction_registry.go` + `agent/provider_registry.go` |
| 6 | 增加可持久化的自定义 workbench view | 🟡 | `workbench.ts`: ViewTypeDefinition 已定义，renderer 侧 wiring 待 @le |
| 7 | 改变布局和完整主题语言 | 🟡 | `workbench.ts`: LayoutContribution + ThemeTokens，renderer wiring 待 @le |
| 8 | 读写插件设置和 namespaced storage | ✅ | `workbench.ts`: PluginStorageAPI + PluginSettingsAPI，capability_rpc.go: host.storage.* |
| 9 | 升级/失败/渲染错误后保留可恢复宿主 | 🟡 | PluginSlot/Surface ErrorBoundary 已有，recovery 集成待 @秦始皇 |
| 10 | 不导入 Wuu 私有源码 | ✅ | `packages/plugin-sdk/` 自包含，`first-party-migration-proof.md` 验证 |
| 11 | 通过公开 SDK 的 contract tests | ✅ | `pluginhost/contracttest/`: 独立 contract test host |
| 12 | Wuu 小版本升级后按兼容契约工作 | 🟡 | 契约已定义，实际跨版本测试待 @秦始皇 |

## 状态图例

- ✅ 已完成，有代码和测试
- 🟡 架构已定义，集成/测试待其他 agent 完成
- ⬜ 未开始

## 当前遗留工作

1. **@le**: Desktop renderer wiring (标准 6, 7)
2. **@秦始皇**: fsnotify dev watch + failure recovery (标准 2, 9, 12)
3. **@Andy**: 集成审查，交叉 review，补充边界测试
4. **@梁子**: 已完成 Phase A-D + 深化，待协助其他 agent
