# 路线图

[English](ROADMAP.md)

wuu 仍处于 1.0 之前。当前最重要的事情，是先让已有的编码工作流稳定、可靠、容易检查，
再扩展更大的工作区和多 Agent 能力。

这份路线图只表示方向，不是发版时间表。完整方案和进度以对应 issue 为准；已经发布的
内容见[更新日志](CHANGELOG.md)。

## 长期方向

wuu 的目标是成为开放、可组合的 GUI Agent 平台，而不是只提供一种固定 Agent 工作流。软件仍应
开箱即用，同时允许用户选择 Agent 怎样工作，以及工作台中启用哪些产品能力。

长期架构把小型 **Plugin Kernel** 与可替换的产品行为分开：

- Kernel 负责持久 Session 与事件、可靠输入接纳、执行权、取消、恢复、Provider 与 Tool 协议
  正确性、最终权限判定、插件生命周期和可恢复的应用外壳。
- Agent Loop Driver 决定怎样消费输入、组装 Prompt、调度 Tool，以及 Agent 何时重试、计划、委派、
  继续或停止。随软件分发的默认 Loop 将成为一方默认 Driver，而不是永久规定所有 wuu Agent 的
  运行方式。
- 一方插件与社区插件使用同一套公开合同贡献 Tool、Prompt、Service、Event、Storage、Session、
  View、设置和展示。产品专用行为不应要求在 Kernel 中加入私有分支。
- 功能插件与外观插件彼此独立组合。宿主保留布局、无障碍、溢出、系统安全区和恢复路径；插件
  通过稳定语义界面贡献能力与内容。

这一方向参考了 Pi、Cordis 和 HashiCorp go-plugin 等开源项目。wuu 不复制它们的具体运行时
实现，也不声称协议兼容；我们把小循环、Service 组合、作用域生命周期所有权和受监督进程插件等
通用思想适配到 Go core 与 Electron shell。

## 插件架构迁移

迁移会保证每个阶段都有可用的软件，不代表以下内容会在一个版本内全部交付。

1. **统一插件所有权。** 将 runtime 进程、后台工作、事件订阅、Session 所有权和工作台贡献纳入
   同一个 generation 作用域。禁用或更新插件时先停止新工作，等待在途工作收敛，再可靠移除全部
   贡献。
2. **显式声明依赖。** 为插件的 core 与 workbench 两侧生成一份经过校验的激活图。插件等待必需
   Service，缺失时清楚失败，不依赖偶然加载顺序。
3. **稳定 Service 与 Event 合同。** 为 Session、Provider、Tool、权限、checkpoint、Storage 和 UI
   贡献提供版本化、产品中立的 Service；安全和协议不变量仍由宿主执行。
4. **引入 Loop Driver 合同。** 先让现有 Agent 行为通过新合同运行，不改变用户体验。每个 Session
   绑定 Driver 身份与 checkpoint 版本，恢复时绝不猜测另一 Driver 的状态含义。
5. **把默认 Loop 迁入 bundled 插件。** 生命周期和恢复合同得到验证后，再迁移默认 Loop 及真正
   属于它的行为。当前代码位置不会自动成为永久 Kernel 边界。
6. **证明 Loop 可替换。** 维护第二个结构明显不同的 Driver，在不修改 Kernel 的情况下创建、运行、
   恢复和展示 Session。这是 Loop 生态真实成立的验收标准。
7. **谨慎发展公开生态。** 根据真实外部插件补齐兼容门槛、诊断、开发工具、文档和分发能力。
   Marketplace 与签名方案由实际需求推动，不预先发布猜测性的 API。

迁移遵循以下长期原则：

- 机制留在 Kernel，策略属于 Driver 和产品插件；
- 凡是模型可见的内容，都必须能从持久 Session 事实中重建；
- 插件更新是事务：失败候选不能替换仍然工作的 generation；
- 插件缺失或不兼容时，历史仍然可读；宿主提供安全只读 fallback 和显式迁移选择；
- 可执行插件是可信的本地应用代码。扩展点再强，也必须保留指纹批准、最小宿主 Service、显式
  权限和恢复路径。

## 当前重点

- **把插件 runtime 收敛到统一的作用域生命周期。** 现有扩展点已经可以提供大量 Agent 和工作台
  能力，但生命周期、依赖和激活还没有形成一套统一运行模型。下一阶段先建立这个基础，再让默认
  Loop 真正可替换。

- **让后台工作的生命周期可以预期。** 后台命令与需要跨 app-server 重启存活的进程目前
  使用互相冲突的归属和恢复规则，用户不容易判断任务是否还活着、是否还能控制。我们希望
  把它们收敛成一套清楚的生命周期。
  ([#157](https://github.com/blueberrycongee/wuu/issues/157))

- **让后台命令更容易复查。** 命令输出已经可以在终端工作区重新查看，但环境面板还不能
  展示当前会话仍存活的后台进程，也不能直接跳转到对应终端资源。
  ([#103](https://github.com/blueberrycongee/wuu/issues/103))

- **补齐环境面板中的仓库状态。** 环境面板目前仍看不全 upstream、PR 和 CI 状态。
  ([#57](https://github.com/blueberrycongee/wuu/issues/57))

- **让模型支持保持更新，也让消耗说得明白。** 内置模型目录在构建时固定，新模型或修正
  信息必须等下一个 wuu 版本。模型服务返回的 token 总量也不能说明哪些请求部分产生了
  新输入。我们希望支持运行时更新目录，并在不保存提示内容的前提下解释消耗来源。
  ([#148](https://github.com/blueberrycongee/wuu/issues/148)、
  [#119](https://github.com/blueberrycongee/wuu/issues/119))

## 后续计划

- **减少从其他编码 Agent 迁移过来的重复配置。** 用户目前需要手动寻找并重新建立已有的
  项目说明、偏好和其他实用设置。wuu 应发现兼容设置，讲清来源和导入位置，并让用户逐项
  选择；不会静默复制凭据，也不会自动启用可执行扩展。
  ([#153](https://github.com/blueberrycongee/wuu/issues/153))

- **给生成产物一个能一直放在对话旁边的位置。** 交互结果目前主要留在消息流中，办公
  文档也没有一等预览工作区。我们希望用户通过聊天持续制作网页、DOCX 和 PPTX 时，右侧
  始终能看到当前产物，同时继续以工作区文件为真实来源。
  ([#154](https://github.com/blueberrycongee/wuu/issues/154)、
  [#20](https://github.com/blueberrycongee/wuu/issues/20))

## 探索方向

这个问题值得解决，但方案还没有排期：

- **当前内置 webview 无法复用用户已有的浏览器资料，也限制了更深的 Agent 集成。**探索带有
  明确凭据和权限控制的完整浏览器工作面。
  ([#96](https://github.com/blueberrycongee/wuu/issues/96))

如果核心缺陷、安全问题或用户反馈表明有更重要的事情，优先级会随之调整。欢迎通过
[GitHub Issues](https://github.com/blueberrycongee/wuu/issues) 提交建议。
