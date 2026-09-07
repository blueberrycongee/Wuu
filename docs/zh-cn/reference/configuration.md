# Wuu 配置模型

Wuu 把配置分成“用户拥有”和“项目补充”两类。核心原则是：打开一个仓库时，
仓库可以描述自己的工作方式，但不能替用户决定凭据发往哪里、读取哪些工作区外
文件，或扩大本地权限。

## 配置来源与顺序

正常启动先读取用户配置，再按顺序叠加项目来源：

1. `~/.wuu/config.json`，或 `WUU_HOME/config.json`
2. 旧版 `~/.config/wuu/config.json`（仅用于兼容与迁移）
3. 项目 `.wuu.json`，不存在时读取 `wuu.json`
4. 项目 `.wuu/settings.json`
5. 项目 `.wuu/settings.local.json`

对象会递归合并，标量和数组由后面的来源替换。加载成功后返回的可写配置路径始终
是用户配置路径，因此桌面设置和模型切换不会意外改写仓库文件。

## 始终由用户拥有的字段

正常启动会忽略所有项目来源中的以下字段，并在标准错误输出一条说明：

- `default_provider`
- `providers`
- `instructions`（旧版 `memory` 指令发现字段也按这一边界处理）
- `agent.model_roles`
- `agent.model_aliases`
- `agent.permission_mode`

这些字段分别控制默认提供商、端点与凭据来源、工作区外指令发现、后台角色的模型路由，
可供 Agent 显式选择的稳定模型别名，以及 Wuu 的本地权限边界。字段名按 JSON 的大小写匹配规则处理，所以换成
`Providers`、`Memory` 或 `Permission_Mode` 也不会绕过限制。

其他项目行为仍会正常叠加。长期指令应写进 `AGENTS.md`。项目配置必须符合完整
配置结构；未知字段会直接报错，避免拼写错误被静默忽略。

## 指令、Memory 与 Dream

需要团队共同遵守的长期规则应写进仓库的 `AGENTS.md` 或项目文档。`instructions`
只控制核心的通用指令文件发现，因此项目不能把它重定向到工作区外。旧版顶层 `memory`
只在读取边界迁移其中的指令发现字段，不再配置或启停任何核心记忆产品。

用户、工作区和会话记忆由一方 [Memory 插件](../customize/memory.md)管理；后台整理由
[Dream 插件](../customize/dream.md)管理。两者的设置保存在插件自己的命名空间中，不写入
核心配置。禁用插件会同时移除相应 Prompt、Tool、后台 Timer 和界面。

## 显式信任完整配置

自动化场景可以显式选择完整配置：

- `wuu exec --config <path>`：只读取并信任指定文件。
- `wuu exec --ignore-user-config`：忽略用户配置，读取并信任项目 `.wuu.json`
  （或 `wuu.json`）及两个项目 settings 层。

这两种方式会接受文件中的提供商端点、凭据环境变量名、指令路径、hooks 和 MCP
服务器，因此只应对自己控制的文件使用。普通桌面和 CLI 启动不会通过空 `HOME`
等隐式条件获得这项信任。

## 匿名 Worker 并发与主动委派

核心只保存通用执行容量：

```json
{
  "agent": {
    "max_parallel": 5
  }
}
```

| 字段 | 填写方式 | 默认值 | 语义 |
| --- | --- | --- | --- |
| `agent.max_parallel` | 非负整数；`0` 等同省略 | `5` | 控制可同时执行的匿名 worker 数量；超出的异步执行进入 `queued`。 |

`queued` 和 `waiting_children` 状态都不占执行槽。子结果唤醒父 worker 做整合不是新
spawn，因此不经过 spawn 排队闸门；整合开始时，实际运行数可能短暂高于
`max_parallel`。负数配置无效。`initialize`、`config/read`、`config/model/update` 和
`wuu exec --json` 的 `session_configured` 事件都会回读实际生效的 `max_parallel`。

主动委派不是核心配置或 app-server 模式。它由 Subagent 插件在自己的命名空间存储中保存开关，
通过 `agent.pre_step` 为后续模型步骤追加带来源、可持久化的隐藏消息，并在 Composer 工具栏提供
A+ 控件。核心没有
`agent.ultra_mode`、Turn 快照、`ultra` 协议字段或 `wuu exec --ultra`；禁用 Subagent 插件会同时
移除委派 Tool、Prompt、状态和界面入口。

## 从旧项目配置迁移

如果旧项目把提供商放在 `.wuu.json` 中：

1. 运行 `wuu init` 创建用户配置。
2. 把 `default_provider`、`providers`、`instructions`、`agent.model_roles`、
   `agent.model_aliases` 和 `agent.permission_mode` 移到用户配置。
3. 在项目文件中保留真正属于仓库的提示词和其他项目行为。

`WUU_HOME` 可以整体移动用户配置、认证、会话、记忆和日志目录。例如设置
`WUU_HOME=/data/wuu` 后，用户配置路径就是 `/data/wuu/config.json`；即使
`HOME` 环境变量没有设置，这个路径仍然有效。

## Windows 上的 shell

bash 语法在所有平台上都是命令执行的契约。Windows 上 wuu 会解析
Git Bash：优先读取 `WUU_GIT_BASH_PATH` 环境变量；否则依次探测标准
安装位置（`%ProgramFiles%\Git`、`%ProgramFiles(x86)%\Git`、
`%LOCALAPPDATA%\Programs\Git`），再从 PATH 上的 `git.exe` 反推
`bash.exe`。全部失败时命令执行会报错并提示安装 Git for Windows。
TTY 模式的后台进程在 Windows 上不可用，会自动退回管道模式。
