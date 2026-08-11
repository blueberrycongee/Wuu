# Wuu Plugin

Wuu Plugin 是一个可安装、可审批、可升级的扩展包。它可以只提供一种能力，也可以把
Agent runtime、Desktop UI、主题、设置、Skills、Hooks、MCP server 和命令一起交付。

如果你还不确定是否需要插件，先看[扩展 Wuu](index.md)。一个只需要任务说明的需求适合
Skill，一个已有工具服务适合 MCP；需要代码生命周期、宿主服务或桌面 UI 时才需要
Wuu Plugin。

插件平台当前是本地优先的：没有市场或中心仓库，插件以目录或 zip 包安装。插件作者
通常在自己的 GitHub 仓库中开发和发布，使用者不需要 fork Wuu。

## 一个插件包可以包含什么

| 类型 | 做什么 | 是否需要代码 |
| --- | --- | --- |
| 声明式贡献 | 主题、设置、Skills、Hooks、MCP servers 和命令 | 视贡献而定 |
| Agent 插件 | 注册工具、贡献上下文、变换请求、观察 Turn、提供或消费服务 | 是，独立进程 |
| Desktop 插件 | 添加 View、Slot、Presenter、Surface、样式和交互卡片 | 是，Renderer 代码 |

同一个包可以同时声明 `runtime` 和 `desktop.entry`。例如，Agent runtime 负责查询私有
服务，Desktop 模块负责展示结果；两部分共享插件 ID、审批状态和 generation 生命周期。

插件管理、审批、安全模式、崩溃恢复、权限底线和原生窗口生命周期始终由 Wuu 控制，
插件不能替换这些恢复路径。强风格外观插件可以通过公开 Token、UI Kit 和语义锚点统一改变
整个产品，但窗口安全区、导航结构、Tab、滚动、溢出和恢复入口仍由宿主管理。

第一次开发时直接选择一条路径：

- [Agent 插件快速上手](plugin-quickstart.md)：注册模型可见工具并调用宿主 Storage；
- [Desktop 插件快速上手](desktop-plugin-quickstart.md)：在 Composer 加入一个真实按钮；
- [Desktop UI 扩展地图](desktop-plugins.md)：按界面位置选择 View、Slot、Presenter 或 Surface；
- [插件场景教程](plugin-recipes.md)：输入框按钮、选区浮层和完整面板等组合方式。

## 获取与安装

插件作者通常在 GitHub 仓库中维护插件。你可以 clone 仓库、下载目录，或下载发布包
（zip），然后用桌面端或 CLI 安装：

```bash
wuu plugin install ./path/to/plugin
wuu plugin install ./my-plugin-1.0.0.zip
```

桌面端在"技能与插件"中点击**安装本地插件**，选择目录或 zip 包。插件文件安装在
`~/.wuu/plugins/`（设置 `WUU_HOME` 时在其下），已安装插件的代码每次加载前都会重新
计算整包 fingerprint 并核对批准状态。

## 审批与启用

安装后代码不会立即激活。Wuu 显示包的来源、内容和 fingerprint，你检查并批准后插件
才启用；启用前不会执行任何插件代码。插件包的任何文件变化都会产生新的 fingerprint，
原来的批准失效，需要重新检查并批准新版本。再次安装同一插件会暂存为待更新状态，
已安装的版本保持运行直到你批准新包。

常见管理命令：

```bash
wuu plugin list                       # 查看已安装插件与状态
wuu plugin approve my-plugin          # 检查后批准
wuu plugin reject my-plugin
wuu plugin enable my-plugin
wuu plugin disable my-plugin
wuu plugin remove my-plugin
wuu plugin inspect ./path/to/plugin   # 安装前检查包内容与 fingerprint
```

`wuu plugin inspect` 适合在安装前查看包会做什么、请求哪些权限。

## 常见能力

- **换主题**：获批且启用的插件主题出现在"设置 → 外观"，禁用或切回内置主题时
  Token 被完整移除。主题只需要 `plugin.json` 声明，不执行代码。
- **加设置**：插件可以声明自己的设置项（开关、文本、数字、枚举），在设置界面生成
  控件，按插件命名空间存储。禁用、升级或卸载插件时默认保留设置和 Storage，便于之后
  恢复；当前不会隐式删除这些数据。
- **扩展 Agent**：Agent 插件以独立进程运行，可以注册模型可见的工具、贡献系统提示
  与模型步骤前的持久上下文、参与请求改写与摘要压缩，并观察 Turn 生命周期。插件包还可
  通过 manifest 声明独立的命令或模型 Hook；它们不属于 runtime capability，具体事件与
  信任边界见 [Hooks](hooks.md)。
- **定制桌面界面**：注册全局样式、在稳定区域放置可持久化的 View、替换或包装语义
  Presenter（消息、工具活动、导航、设置等）、在固定 Slot 插入内容。渲染失败只回退
  当前边界，设置、禁用和默认 UI 恢复始终可用。
- **组合多种扩展**：插件包可以携带 Skills、Hooks 和 MCP server 定义，让安装、审批、
  启用、更新和卸载使用同一条生命周期。

## 信任边界

- 声明式主题只能修改公开的语义 Token，适合直接安装。
- Agent 插件的 runtime 进程与 Wuu 拥有相同用户权限；桌面插件可以注册任意 CSS。
  这两类只安装你信任的来源，启用前检查包内容。
- 插件包被视为不可信输入：Renderer 不读取插件绝对路径，加载前由 app-server 重算
  fingerprint、确认批准与启用状态，Electron 主进程再次校验摘要后通过内容寻址的
  `wuu-plugin:` 协议加载；CSP 不开放 `unsafe-eval`。
- 插件声明的 Hook 与直接运行第三方本地命令具有相同风险。

## 当前边界

跨 Wuu 小版本继续工作的兼容承诺（开发者不 fork 也能跟上更新）是插件平台当前阶段的
完成门槛，但尚未验证完成。在这之前，插件可能随 Wuu 升级而需要调整；发布插件时声明
`minimum_wuu_version` 可以避免不兼容组合被激活。

插件还可以声明简单的包关系：缺少 `requires` 中的插件时不会启动；`breaks` 命中时 Wuu
拒绝同时启用；`conflicts` 只显示潜在冲突提示，不会替你选择或自动停用插件。当前不做
版本范围、SAT 求解或组合评分。

## 开发与发布

CLI 提供完整的本地开发闭环：

```bash
wuu plugin create --type agent my-agent
wuu plugin create --type desktop my-ui
wuu plugin create --type full my-extension

wuu plugin validate ./my-extension
wuu plugin build ./my-extension
wuu plugin test ./my-extension
wuu plugin dev ./my-extension
wuu plugin pack ./my-extension
```

完整 manifest、Agent 协议、Desktop API、generation 和安全边界见
[插件开发参考](plugin-authoring.md)。底层设计和宿主所有权见
[插件系统架构](plugin-system.md)。
