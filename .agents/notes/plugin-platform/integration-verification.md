# 集成验证记录（安全/测试轨）

## 2026-08-06 纯提交态（pure HEAD）编译断裂修复

**发现**：`575b53b3` 是部分提交——跨文件依赖被拆开，特征文件进了提交、调用方/被依赖方留在工作区。此前所有「全绿」结论都跑在含 Andy 在途改动的工作区上，掩盖了纯提交态不编译的事实。教训：本分支验证必须定期跑**隔离 worktree 的 pure-HEAD build**。

三处断裂与处置：
1. `host.Clients/Replace` 未提交（runtime reconcile 已提交并调用）→ `7235ae8b` 补提交这两个方法（内容即 Andy 工作区版本，co-authored 注明）。
2. `ApplyExtensionPolicy(cfg, discovered)` 二参签名已提交、appserver 一参调用方迁移未提交 → `63adb873` 改为 variadic 快照参数（省略=运行时自行 rediscover），两种调用约定都合法。
3. 纯 HEAD 下 appserver 两个测试失败（extension_package prompt-kit 记录缺失、goal resume 2 requests）——同为部分提交的悬空期待，工作区版本即通过，**留 Andy 完成提交后自然闭合**，不替他提交。

修复后纯 HEAD `go build ./...` 通过；pluginhost/runtime/extensions/plugin 四包在纯 HEAD 全绿。config 的 settings_layer 失败仍是 Andy 在途重写（见下）。

建议（流程）：feature 提交前从干净 checkout 跑 `go build ./...`；或按调用方+被调方原子切片提交。

## 2026-08-05 全仓 Go 测试（HEAD 83d351c9 + Andy 在途未提交改动）

`go test ./...` 全仓运行：**唯一失败** `internal/config` 的 `TestLoadFrom_LocalSettingsCarriesExtensionGrants`（settings_layer_test.go:124: Extensions is nil）。

### 归因

非已提交代码问题。失败由 Andy 未提交的 `internal/config/settings_layer.go` 改动引入：

- 该改动给 shared 层加 `protectUserSettings: true`，并把 `extensions` 加入 `stripProjectUserSettings` 的保护字段；
- 但失败日志显示 `extensions` 在 **local 层**（`.wuu/settings.local.json`）也被 strip 了（"ignoring user-owned settings from project source .../settings.local.json: extensions"）；
- 按现有语义 local 层是 user-owned、应当允许携带 extension grants（测试名即此意），疑似 protectUserSettings 应用范围过宽（波及 local 层）。

安全方向上该改动符合威胁模型控制 #13（项目/共享配置不得授予用户级包策略），但 local 层豁免必须保留。已留给 Andy 修复，未动其文件。

### 结论

除上述一项外全仓绿，包括 83d351c9 的协议 hook 权限执行对所有下游包（runtime/agent/tools/harness/appserver）无回归。

### 桌面侧定向验证

`vitest run` PluginCommandRegistry/SkillsCatalog/ComposerSlashCommands/AppState 四文件：**158/158 全过**（含 Andy 在途 UI 改动）。
