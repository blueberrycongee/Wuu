# 扩展 Wuu

Wuu 提供四种主要扩展方式：Skill、MCP、Hook 和 Wuu Plugin。它们可以一起使用，
但解决的问题、运行位置和信任成本不同。先从你想改变的行为出发，不必先选择技术名词。

## 先选择合适的扩展方式

| 你想做什么 | 首选方式 | 原因 |
| --- | --- | --- |
| 让 Agent 按固定步骤完成一类任务 | [Skill](skills.md) | 只提供可复用说明和资源，最轻量 |
| 接入已有的本地或远程工具服务 | [MCP](mcp.md) | 复用标准 MCP server，不需要改 Wuu |
| 在工具调用、提交 Prompt 等事件前后执行检查 | [Hook](hooks.md) | 适合团队规则、阻止操作和自动检查 |
| 只提供主题或宿主渲染的设置项 | [Wuu Plugin](plugins.md) 的声明式贡献 | 不需要加载 Desktop 代码 |
| 注册新的 Agent 工具、上下文或长期后台行为 | [Wuu Plugin](plugins.md) 的 Agent 部分 | 插件 runtime 可以参与 Agent 生命周期并调用宿主服务 |
| 给桌面端增加按钮、面板、页面或消息展示 | [Wuu Plugin](plugins.md) 的 Desktop 部分 | Desktop 模块可以在稳定 UI 边界中运行 React |
| 同时交付 Skill、MCP、Hook、Agent 能力和界面 | [Wuu Plugin](plugins.md) | 一个插件包可以组合多种贡献并统一安装、审批和升级 |

如果一个需求只靠 Skill 或 MCP 就能完成，优先使用更小的扩展方式。需要代码生命周期、
宿主服务或桌面 UI 时，再使用 Wuu Plugin。

## 它们怎样组合

这些方式不是互斥的。例如，一个代码评审扩展可以同时包含：

1. 一个 Skill，告诉 Agent 评审步骤和交付格式；
2. 一个 MCP server，提供组织内部的代码查询工具；
3. 一个 Hook，在提交评审结果前运行合规检查；
4. 一个 Wuu Plugin，在侧边栏提供评审历史 View，并注册专用 Agent 工具。

只有 Wuu Plugin 是带 `plugin.json`、generation 生命周期和审批状态的 Wuu 插件包。
Skill、MCP 和 Hook 也可以由插件包携带，但它们本身不是 Desktop 插件模块。

## 信任与运行位置

| 方式 | 运行位置 | 主要风险 |
| --- | --- | --- |
| Skill | 作为说明进入 Agent 上下文 | 可能引导 Agent 调用工具；使用前阅读内容 |
| MCP | 本地子进程或远程服务器 | 本地命令、网络访问和第三方工具结果 |
| Hook | 本机命令或模型调用 | 会在生命周期事件中执行；可阻止或改写部分行为 |
| Agent 插件 | Wuu 管理的独立进程 | 与当前用户同权限，可注册工具并调用获准服务 |
| Desktop 插件 | Wuu Renderer 中的受信任代码 | 可运行 React 和注入 CSS，只安装可信来源 |

Wuu 的权限模式、工作区边界和插件审批仍然适用。更小的扩展方式不代表可以跳过来源检查。

## 开始开发

- [使用和编写 Skills](skills.md)
- [编写与安装 Skill](skill-authoring.md)
- [连接 MCP 服务器](mcp.md)
- [配置 Hooks](hooks.md)
- [了解 Wuu Plugin](plugins.md)
- [使用插件主题与设置](themes-settings.md)
- [Agent 插件快速上手](plugin-quickstart.md)
- [Desktop 插件快速上手](desktop-plugin-quickstart.md)
- [Desktop UI 扩展地图](desktop-plugins.md)
- [插件场景教程](plugin-recipes.md)

需要完整字段、生命周期和 API 时阅读[插件开发参考](plugin-authoring.md)；需要理解为什么
这些边界存在时，最后再阅读[插件系统架构](plugin-system.md)。
