# Hook 级权限目录设计（安全轨交付，供总架构收敛）

状态：草案 v1。前置：threat-model.md「Permission behavior」节；security-audit.md 缺口 1+2。

## 原则

- 权限目录封闭：manifest 只能请求目录内值，未知值 fail closed（补齐控制 #5 的值域校验，目前只校验字段名）。
- host 独立持有「hook → 所需权限」映射；manifest 声明只是请求，不构成授权。
- 观测与改写分离：能读正文的权限和能改写的权限是不同条目。
- 生命周期 metadata 免费：不含内容的 session.start/stop 不需要权限，但 payload 必须永远不含正文（结构性保证，不靠裁剪）。
- 默认拒绝：未获批权限的 hook 注册被剥离并记录诊断，插件其余能力照常（不因一个越权 hook 整体失败，除非协议违规）。

## 目录（v1）

| 权限 | 语义 | 授予的 hook |
|---|---|---|
| `session.read` | 读会话正文 | `chat.message`（观测） |
| `session.transform` | 读+改写请求 | `chat.request`（含模型/工具/参数改写权） |
| `tools.observe` | 读工具调用与结果 | `tool.execute.after` |
| `tools.transform` | 增删改工具定义、拦截改写调用参数 | `tool.definition`、`tool.execute.before` |
| `shell.env` | 注入 shell 环境 | `shell.env` |
| `network` | 声明性（v1 不执行，UI 诚实标注） | — |
| `fs` | 声明性（同上，同 UID 残余风险） | — |

无需权限：`session.start`、`session.stop`（payload 仅 id/时间/metadata，需在类型上保证无正文字段）。

## hook → 所需权限映射（host 侧常量）

```go
var hookRequiredPermission = map[Hook]string{
    HookChatMessage:       "session.read",
    HookChatRequest:       "session.transform",
    HookToolDefinition:    "tools.transform",
    HookToolExecuteBefore: "tools.transform",
    HookToolExecuteAfter:  "tools.observe",
    HookShellEnv:          "shell.env",
    // session.start/stop: 无条目 = 免费
}
```

## 执行点（两处，纵深防御）

1. **注册剥离（主）**：`Start` 时把已获批权限集合传入 `ProcessConfig`（或包裹为 Policy），initialize 返回的 hook 清单按映射过滤；被剥 hook 记入 `Status.Diagnostics`（用户可见「该插件请求了未获批能力」），插件继续以子集运行。协议违规（未知 hook 名、重复）维持 fail closed。
2. **载荷裁剪（纵深）**：`Host.Run` 按 hook 策略在调用前裁输入、调用后裁输出——例如无 `session.transform` 时 chat.request 的 output 被整体丢弃（等效只读）；`chat.message` input 恒为不可变拷贝。裁剪逻辑集中在 pluginhost，调用点（runtime/plugin_host.go、plugin_tool_executor.go）不感知权限，避免每个新调用点都要记得安全。

## 与现有件的关系

- grant 精确指纹审批（extensions.Grant）不变，审批 UI 把目录权限渲染为人类语言条目；`session.transform` 应标红/单列（等同把完整对话交给插件改写）。
- 包级激活门（activatedPlugins）保留：权限决定「激活后能干什么」，激活门决定「能不能跑」。
- 内置插件（bundled provenance）建议隐式全权限但在 inventory 显式列出，避免隐性特权。

## 测试矩阵（落地后由我补）

| 用例 | 断言 |
|---|---|
| 未获批 `session.transform` 注册 chat.request | 注册被剥 + 诊断记录 + 其余 hook 正常 |
| 无权限插件收到 chat.message | input 不含正文（若观测级也无权则整个 hook 不触达） |
| 有 `session.read` 无 `session.transform` | chat.request output 被丢弃，请求原样继续 |
| 未获批 `shell.env` 注册 shell.env | 剥离，shell 环境无注入 |
| manifest 请求目录外权限值 | fail closed（加载诊断，不激活 runtime） |
| 指纹变化后权限集合扩大 | 重新审批（已有 grant 精确匹配语义覆盖，补端到端用例） |
