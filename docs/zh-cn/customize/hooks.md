# Hooks

Hook 让 Wuu 在工具调用等生命周期事件发生时运行一项检查或自动操作。它适合执行团队
规则、阻止危险命令、记录工具活动，或者把检查结果补充给 Agent。

Hook 会在 Wuu 本机执行命令或调用模型，不是操作系统沙箱。只配置你理解并信任的 Hook，
不要直接启用仓库或第三方提供的未知命令。

## 当前可用范围

当前运行时会触发以下事件：

| 事件 | 触发时机 | 主要输入 | 结果怎样处理 |
| --- | --- | --- | --- |
| `PreToolUse` | 工具执行前 | 工具名和参数 | 可以放行、阻止或替换工具参数 |
| `PermissionRequest` | 工具权限判定前 | 工具名和参数 | 可以阻止工具执行 |
| `PostToolUse` | 工具成功后 | 工具名、参数和结果文本 | 可以向 Agent 补充上下文；不能撤销已经完成的操作 |
| `PostToolUseFailure` | 工具执行失败后 | 工具名、参数和错误 | 用于记录或通知；Hook 错误不会覆盖原工具错误 |
| `PreCompact` | 对话压缩前 | 压缩原因 | 可以阻止本次压缩 |
| `PostCompact` | 对话压缩实现返回后 | 压缩原因和可选错误 | 可以拒绝采用压缩结果 |
| `UserPromptSubmit` | 用户提示进入模型轮次前 | 提示文本 | 可以阻止本轮执行 |
| `SubagentStart` | 子代理轮次开始前 | 子代理 ID | 可以阻止子代理轮次 |
| `SubagentStop` | 子代理轮次结束后 | 子代理 ID | 失败会使该子代理轮次失败 |
| `SessionStart` | 会话绑定完成后 | 会话 ID | 失败会使会话绑定失败 |
| `SessionEnd` | 会话资源关闭前 | 会话 ID | 失败会随清理错误返回 |
| `Stop` | 模型轮次收尾时 | 会话 ID | 可以把本轮标记为失败 |
| `FileChanged` | Wuu 的文件工具成功写入或编辑文件后 | 文件绝对路径 | 用于记录或触发后续动作；输出目前不会改变 Agent 行为 |

`FileChanged` 只跟踪经过 Wuu 文件工具完成的写入。命令、外部程序或用户直接修改文件时，
不保证触发这个事件。

## 配置位置

最直接的方式是在用户配置 `~/.wuu/config.json` 的顶层加入 `hooks`。设置了
`WUU_HOME` 时，用户配置位于 `$WUU_HOME/config.json`。

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "run_shell",
        "type": "command",
        "command": "python3 ~/.wuu/hooks/check-shell.py",
        "timeout": 10
      }
    ]
  }
}
```

这里的配置应合并进现有文件，不要删除原来的模型服务和 Agent 配置。修改后需要重新启动
当前 Wuu runtime；已经运行的会话不会自动重新读取文件。

在自动化场景中，`wuu exec --config <path>` 会把指定文件作为完整配置加载；
`wuu exec --ignore-user-config` 会显式信任项目配置。两者都可能启用配置中的 Hook，
只应对自己控制的文件使用。普通启动下的配置加载和信任边界见[配置模型](../reference/configuration.md)。

启用的插件也可以声明 Hook。第三方插件的 Hook 与直接运行第三方本地命令具有相同风险，
应先检查来源、命令和授权状态。

> Skill frontmatter 中的 `hooks` 当前不会注册 Hook。可用字段见
> [编写与安装 Skill](skill-authoring.md)。

## 配置字段

每个事件对应一个 Hook 数组，按照配置顺序依次执行。遇到第一个阻止或执行失败的
`PreToolUse` Hook 后，后续 Hook 和目标工具都不会继续执行。

| 字段 | 含义 |
| --- | --- |
| `matcher` | 匹配工具名。不填或填 `*` 表示全部；其他值按工具名进行不区分大小写的精确匹配，不支持通配表达式 |
| `type` | `command` 或 `prompt`；不填时使用 `command` |
| `command` | `command` Hook 要执行的 shell 命令 |
| `prompt` | `prompt` Hook 的判断要求；可以用 `$ARGUMENTS` 插入事件输入 JSON |
| `model` | `prompt` Hook 使用的模型；不填时使用当前配置的默认工具模型 |
| `timeout` | 单个 Hook 的超时秒数；不填或小于等于 0 时为 30 秒 |

`matcher` 主要用于三类工具事件。没有工具名的事件只能使用空 matcher 或 `*`。
输入中的真实工具名可以从 Hook 日志查看，不要假设界面名称与内部名称完全相同。

## 编写 command Hook

command Hook 通过 shell 启动。Wuu 把一个 JSON 对象写入命令的标准输入，并从标准输出
读取一个可选 JSON 对象。命令继承 Wuu 进程的环境，但不应依赖进程恰好从工作区启动；
需要工作区路径时，从输入的 `cwd` 读取并显式切换目录。

下面的脚本会阻止包含 `rm -rf` 的 `run_shell` 调用：

```python
#!/usr/bin/env python3
import json
import sys

event = json.load(sys.stdin)
tool_input = event.get("tool_input") or {}
command = tool_input.get("command", "")

if "rm -rf" in command:
    json.dump({
        "decision": "block",
        "reason": "项目规则不允许通过 Agent 运行 rm -rf"
    }, sys.stdout)
else:
    json.dump({}, sys.stdout)
```

例如把它保存到 `~/.wuu/hooks/check-shell.py`，再使用前面的 `PreToolUse` 配置。脚本不需要
可执行位，因为示例通过 `python3` 启动它。

这只是便于理解协议的最小示例，不是完整的命令安全策略。真实规则应解析工具参数，
避免只靠简单字符串匹配判断 shell 语义。

## 输入协议

command Hook 会在标准输入收到以下结构。除了前三个公共字段，Wuu 只填写与当前事件
有关的字段：

```json
{
  "hook_event_name": "PreToolUse",
  "session_id": "...",
  "cwd": "/path/to/workspace",
  "tool_name": "run_shell",
  "tool_input": {
    "command": "go test ./..."
  },
  "tool_response": "...",
  "error": "...",
  "prompt": "...",
  "file_path": "/path/to/workspace/file.go",
  "compact_reason": "proactive",
  "agent_id": "worker-id"
}
```

- `tool_input` 是目标工具的原始 JSON 参数。
- `tool_response` 是成功结果的稳定文本投影，不保证包含富媒体结果的全部内部数据。
- `error` 只用于工具失败事件。
- `file_path` 只用于 `FileChanged`。
- `prompt` 只用于 `UserPromptSubmit`。
- `compact_reason` 用于 `PreCompact` 和 `PostCompact`。
- `agent_id` 用于 `SubagentStart` 和 `SubagentStop`。
- 某些运行路径目前可能不填写 `session_id`；不要把非空会话 ID 当作 Hook 正常运行的前提。

## 输出与退出码

Hook 可以向标准输出写一个 JSON 对象：

```json
{
  "continue": true,
  "decision": "block",
  "reason": "说明为什么阻止",
  "updated_input": {
    "command": "go test ./internal/..."
  },
  "additional_context": "提供给 Agent 的补充信息"
}
```

所有字段都可以省略：

- `decision: "block"` 或 `continue: false` 表示阻止；
- `reason` 解释阻止或判断原因；
- `updated_input` 仅在 `PreToolUse` 中会替换本次工具参数，必须符合目标工具的参数结构；
- `additional_context` 会在成功的 `PostToolUse` 后交给 Agent；
- 只需执行副作用时，可以不输出内容并以状态码 0 退出。

退出码 0 表示继续，退出码 2 表示阻止。使用退出码 2 且未在 JSON 中提供 `reason` 时，
Wuu 会优先使用标准错误作为原因。其他非零退出码视为 Hook 自身执行失败。

只有标准输出整体是有效 JSON 时才会被解析。日志和调试信息应写到标准错误，避免把它们
与 JSON 混在标准输出中。

## 使用 prompt Hook

prompt Hook 把事件交给模型判断，适合难以用确定性脚本表达的软规则：

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "run_shell",
        "type": "prompt",
        "prompt": "判断下面的工具调用是否可能删除用户数据。只有确认安全时才允许：$ARGUMENTS",
        "timeout": 20
      }
    ]
  }
}
```

模型会返回 `ok` 和原因；`ok: false` 会阻止操作，原因也会作为补充上下文。模型请求失败
或返回无法解析的结果时，当前实现会放行操作，因此 prompt Hook **不能作为唯一的安全
边界**。需要强制执行的规则应使用确定性的 command Hook 和 Wuu 权限系统。

prompt Hook 会产生额外模型请求、延迟和费用，事件内容也会发送给所选模型服务。

## 检查和排障

第一次编写 Hook 时，先使用一个无副作用的 command Hook，把标准输入追加到用户控制的
临时日志中，再触发一次范围明确的工具调用。确认字段和工具名后删除日志 Hook，避免长期
记录源代码、命令参数或工具结果。

常见问题：

- **Hook 没有运行：**确认事件名大小写、`matcher` 使用内部工具名，并重启 runtime。
- **配置后所有工具都失败：**检查命令是否存在、Wuu 进程能否读取脚本，以及脚本是否在
  等待标准输入结束之外的交互输入。
- **输出没有生效：**确保标准输出只包含一个有效 JSON 对象，日志写到标准错误。
- **工作区内手动修改没有触发：**`FileChanged` 不是通用文件系统监听器。
- **prompt Hook 总是放行：**检查当前模型服务、模型名称和返回格式；模型或解析失败按
  当前策略不会阻止操作。

## 安全边界

- command Hook 以 Wuu 进程的本地权限运行，可能读取文件、访问网络或修改系统状态；
- Hook 输入可能包含提示词、源代码、命令、路径和工具结果，不要无意上传或长期记录；
- Hook 输出会影响 Agent 或工具参数，应视为不可信输入并防范提示词注入；
- 不要在 Hook 命令、项目文件或日志中写入 API Key；
- Hook 是权限系统之外的扩展执行路径，不会因为 Agent 使用只读模式就自动变成只读。

处理不可信仓库或第三方扩展前，继续阅读[权限模式](../reference/permissions.md)和
[安全模型](../reference/security-model.md)。
