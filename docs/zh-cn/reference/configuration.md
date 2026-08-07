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
- `memory`
- `agent.model_roles`
- `agent.model_aliases`
- `agent.permission_mode`

这些字段分别控制默认提供商、端点与凭据来源、全局记忆发现、后台角色的模型路由，
可供 Agent 显式选择的稳定模型别名，以及 Wuu 的本地权限边界。字段名按 JSON 的大小写匹配规则处理，所以换成
`Providers`、`Memory` 或 `Permission_Mode` 也不会绕过限制。

其他项目行为仍会正常叠加，例如 `agent.append_system_prompt`。项目配置必须符合完整
配置结构；未知字段会直接报错，避免拼写错误被静默忽略。

## 全局记忆与项目规则

全局记忆本来就位于工作区外，默认保存在用户控制的 Wuu 主目录中。项目不能重定向
它的发现目录，但这不会关闭全局记忆，也不会阻止 Wuu 正常读取和更新它。

需要团队共同遵守的长期规则应写进仓库的 `AGENTS.md` 或项目文档。个人偏好、跨项目
经验和自动整理出的记忆则留在用户目录中。

### 记忆整合（Dream）

`memory.dream` 配置后台记忆整合：Wuu 在对话轮次结束后检查已完成会话，把稳定事实
写入工作区记忆。它默认关闭。

```json
{
  "memory": {
    "dream": {
      "enabled": true,
      "interval_days": 7,
      "provider": "openai",
      "model": "gpt-4.1"
    }
  }
}
```

- `enabled`：是否启用，默认 `false`；
- `interval_days`：距上次运行至少间隔的天数，默认 `1`；
- `provider` / `model`：可选专用模型，留空使用当前提供商和默认模型。

通过桌面设置修改会立即生效；直接编辑配置文件需要重新启动 Wuu。旧的
`memory.dream_interval_days` 仍被识别（正数启用、`0` 停用），新配置优先。
运行机制与限制见[后台记忆整合（Dream）](../customize/dream.md)。

## 显式信任完整配置

自动化场景可以显式选择完整配置：

- `wuu exec --config <path>`：只读取并信任指定文件。
- `wuu exec --ignore-user-config`：忽略用户配置，读取并信任项目 `.wuu.json`
  （或 `wuu.json`）及两个项目 settings 层。

这两种方式会接受文件中的提供商端点、凭据环境变量名、记忆路径、hooks 和 MCP
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
通过请求变换为后续模型请求加入委派 Prompt，并在 Composer 工具栏提供 A+ 控件。核心没有
`agent.ultra_mode`、Turn 快照、`ultra` 协议字段或 `wuu exec --ultra`；禁用 Subagent 插件会同时
移除委派 Tool、Prompt、状态和界面入口。

## 从旧项目配置迁移

如果旧项目把提供商放在 `.wuu.json` 中：

1. 运行 `wuu init` 创建用户配置。
2. 把 `default_provider`、`providers`、`memory`、`agent.model_roles`、
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
