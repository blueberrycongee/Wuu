# 集成验证记录（安全/测试轨）

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
