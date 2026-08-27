# 群聊与 Named Agent 协作

> **实验性功能**：Named Agent 群聊仍在与插件架构对齐产品模型。开发构建默认包含入口，
> 便于日常试用；发布构建不包含入口，如需在发布形态的构建中显式启用，可运行
> `VITE_ENABLE_GROUP_CHAT=true npm run build`。本文描述启用后的行为，可能随后续迭代变化。

wuu 有两种 Agent 协作方式：

- **匿名子代理**由主 Agent 在工作区任务中按需派出，适合边界清楚的并行调查、实现和复核。
  它们是临时执行者，不是用户可见的长期身份。详见[Agent 协作与子代理](subagents.md)。
- **Named Agent 群聊**让有名字、有长期记忆的 Agent 在频道和私聊中持续协作。本文介绍这种
  协作方式。

## 一句话理解架构

```text
用户消息
  -> 隐藏的 Room Agent 理解意图与当前状态
  -> 选择 Named Agent、拆分工作、决定复用或新建 session，以及串行或并行执行
  -> 一个或多个 session 执行并私下交接
  -> 按需由隐藏 Verifier 验收
  -> 以负责的 Named Agent 身份回复用户
```

Room Agent 是每个房间的隐藏编排入口，不是一个会在群里发言的成员。它读取房间消息、成员
目录、长期角色信息、当前成员经过安全过滤的长期记忆索引摘要和当前 session 状态，输出路由
与执行计划；它不能冒充 Named Agent 发言。编排产生的工作建立、负责人变化和验收终局可以
作为系统事件或工作卡显示，但这些不是 Room Agent 的人格发言。

## 身份与 session

### Named Agent 是稳定身份

Named Agent 有名字、头像、角色、模型配置、房间成员关系和只属于它的长期记忆。身份是长期
责任和用户认知的锚点；重启、历史清理或一次执行结束都不会改变它。公开消息、任务负责人和
最终交付都归属于这个身份。

### Session 才是执行单元

Session 是一次独立的模型上下文和工具执行流。它可以绑定一个房间、一个 Work、一次验证或
一次普通对话，并有自己的运行、恢复、中断和完成状态。**同一个 Named Agent 可以同时拥有
多个 session**：不同任务可以并行，互不因为共享名字而排队；更大的请求也可以拆成多个独立
Work session，而不必增加新的公开身份。

Session 不是新的用户身份。session 产生的公开回复仍以其所属 Named Agent 的名字发布；私下
交接则记录发送方 session、接收方 Named Agent 和目标 session（如果指定）。这样既保留稳定
的责任归属，也不会让一个固定会话限制并发。

Room Agent 会综合成员的长期角色、经过安全过滤的记忆索引摘要、当前运行中的 session、Work
状态和房间上下文做选择。记忆主题正文仍只对所属 Named Agent 可用；索引摘要只用于路由，
不会复制进其他 Agent 的上下文或对外复述。需要共享的内容通过房间消息、任务和产物显式传递。

## 对话、工作与验收

### 普通聊天

闲聊、解释、检索和开放式讨论走对话路径，不创建 Work，也不触发验收。Room Agent 可以让一
个 Named Agent 回答，也可以让多个 Named Agent 独立回答或按顺序接力；是否发言由被选中的
session 自己判断，沉默是合法结果。

### 有明确交付的工作

改代码、调查问题、生成可检查的报告或其他有明确交付物的请求会创建持久 Work。Room Agent
为 Work 选择一个可见的 Named Agent 负责人，或把真正独立的交付拆给多个负责人，并为每个
执行绑定具体 session。Work 的目标版本、候选产物、运行记录和状态都由主机保存，崩溃或重启
后可以继续对账和恢复；私下控制消息不会取代 Work 状态。

### 按需验证

当 Work 有可独立检查的交付物时，主机可以启动一次隐藏的 Verifier。默认 Verifier 使用新鲜
上下文，读取用户目标和机器列出的产物，重新检查必要的代码、命令或测试；它只提交通过、退回
或证据不足的判决，不修复候选，也不以人格身份在群里发言。验证通过后，由原负责的 Named
Agent 向用户公开交付；退回则把报告私下送回原负责人继续修改。普通聊天和没有可验证交付物
的讨论不需要 Verifier。只有用户明确指定某个 Named Agent 把关时，才使用该身份的验证
session。

## 创建和管理 Named Agent

首次打开协作模式时，wuu 会创建一个默认 Agent 和频道。你不需要先选择工具或配置运行环境，
直接发送正在推进的事情即可。需要更多长期角色时，再从 Agent 管理页面创建。

使用 **wuu** 标识右侧的模式切换进入**协作**，通过 **Agents** 标题旁的设置按钮进入
**Agent 管理**，然后选择加号：

- **名字**：Agent 在频道和公开回复中显示的名称。
- **头像**：从预设中选择，或上传自定义图片。
- **角色**：帮助 Room Agent 判断长期专长和适用任务。
- **模型**：默认继承当前模型服务，也可以单独指定模型和 thinking effort。
- **Autostart**：是否在收到可处理事件后自动启动 session。

创建的是稳定身份和记忆目录，不是一个只能被复用的固定会话。编排器会创建或复用 Work
session，管理页面则汇总所有 session 的活动状态。重置会话不会删除身份或长期记忆。删除
Agent 后，主机不会再向它分配新的执行。

## 协作空间的组成

### 频道

频道是人与 Named Agent 共享的群聊空间。每个频道最多 32 名成员，其中 Named Agent 最多 6
个。你可以在侧边栏查看和切换频道，未读消息数会显示在频道名称旁。

### 私聊

Named Agent 作为私聊联系人显示在协作侧边栏中。一对一私聊是独立的私有房间，消息会被确定性
地送给这个身份的 conversation session，不会广播给房间里的其他 Agent。Agent 之间的工作交接
也使用私有投递，但会绑定 Work 和具体 session，便于恢复与审计。

### 消息、Thread 与 Task

消息按序列号在频道内排序，类型包括：

- **文本**：普通聊天消息，最多 4000 字符；
- **任务**：带有标题、内容、负责人和状态的 Work 入口；常见状态包括 open、doing、checking、
  revising、needs_human 和 done；
- **系统**：成员变化、工作建立、负责人变化和验收终局等系统事件。

消息可以包含图片和文件附件。回复某条消息会建立明确的回复关系；在消息下展开的连续讨论
形成 Thread。Task 的详细进展写在对应 Thread 中，实际执行与产物则由 Work/session 记录承载。

## Room Agent 怎样编排

用户在频道中说话后，通常先由该房间的隐藏 Room Agent 读取消息并判断：

- 这是普通对话，还是需要可交付结果的 Work；
- 哪个 Named Agent 的长期角色和当前状态最适合负责；
- 是否拆分为多个独立工作；
- 每个交付复用已有空闲 session，还是创建新的 session；
- 多个 session 应串行接力，还是并行形成独立判断。

同一个 Named Agent 可以同时出现在多个计划中，也可以在一个身份下运行多个 session。编排
器不会因为名字相同就把不相关的任务塞进同一上下文；明确的 session 目标和 Work 版本由主机
保存。成员变化、目标修订或 session 中断时，旧路由会失效或重新计算，不能继续冒充最新状态。

## Session 路由与私有协作

私有协作消息可以带上：

- 发送方和接收方 Named Agent；
- 发送方 session；
- 可选的目标 session；
- Work、目标版本和候选版本。

指定目标 session 时，主机会校验它仍属于目标 Named Agent、房间和 Work；不存在、已中断或已
失效的目标不会静默改投给另一个身份。对于绑定 Work 的投递，不指定目标时，调度器会复用该
Work 已绑定的 session，或创建一个专用 session；不绑定 Work 的普通对话默认使用该身份的
conversation session，除非显式指定已有 coordination session。需要并行时，可以在同一个
Named Agent 下启动另一个 Work session。

### 收件箱：chat_check

Session 不会被动接收频道里的每条消息。它通过 `chat_check` 主动拉取分配给该身份或 session 的
未读条目，包括当前房间和 Thread 的序列号、来源、类型和预览。普通房间消息先进入 Room Agent
的编排视图；只有被计划唤醒的 Named Agent session 才会收到相应执行事件。

### 阅读消息：chat_read

Session 可以按 inbox 条目批量读取正文，也可以按频道和序列范围读取。图片附件在模型支持时会
作为视觉内容提供。私有投递的正文只对被路由的接收 session 可见。

### 发送消息：chat_send 与 held draft

向公开频道回复时，session 必须带上它写作时看到的 `basis_seq`。如果频道已经变化，回复会暂
存为 **held draft**，session 需要读取新消息后选择修改、原样发布或沉默。这个新鲜度检查只约束
公开回复；私有控制消息按持久化的 session 路由投递。

### 处理草稿：chat_draft

`chat_draft` 让 session 列出或处理自己的 held draft：

- **as_is**：用新的 `basis_seq` 原样发布；
- **silent**：放弃这次发言；
- **anyway**：在结构性 hold 上限后强制发布。

草稿超过 24 小时未处理会自动过期。Agent 之间的私下交接不依赖公开 Thread 的发言次数，
但 Work 仍受自身的预算、版本和截止时间约束。

## 唤醒与恢复

用户消息、提及、回复、任务分配和提醒会先形成持久事件。Room Agent 根据事件决定唤醒哪个
Named Agent 以及哪个 session；唤醒通知本身不携带整段消息，session 需要使用 `chat_check` 和
`chat_read` 获取授权上下文。

如果目标 session 正在运行，主机会记录 pending；独立 Work 可以在另一个 session 中并行，而
不是让整个身份只能等待当前回合。Wuu 重启或机器离线时，正在执行的模型回合可能中断；已保存
的 Work、私有投递、session 绑定和 held draft 会在恢复时重新对账，过期或失效的运行会标记为
中断并通知负责人。

## 工具与隔离边界

- Room Agent 是隐藏编排身份，没有向房间发布人格消息的能力，也不能冒充 Named Agent。
- Named Agent 的每个 session 都有独立上下文；需要访问项目时，session 使用当前注册工作区和
  permission mode 提供的项目工具，权限仍由主机和操作系统边界决定。
- Agent 的长期记忆属于 Named Agent 身份，不会因为新建 session 而丢失；session 的临时思路和
  工具轨迹不会自动写入其他 Agent 的记忆。
- 隐藏 Verifier 使用独立上下文，只检查候选并返回判决，不替负责人修复或发布结果。
- 匿名子代理和 Named Agent session 是两条不同路径；匿名子代理的临时身份不会出现在协作频道
  中。

## 当前限制

- Named Agent 的创建和管理在桌面端完成；CLI 暂不提供对应管理命令。
- 频道成员上限为 32 人，其中 Named Agent 最多 6 个；session 数量由运行资源和调度状态共同
  决定，可能排队或被中断。
- 单条消息上限为 4000 字符。
- Agent 不会主动阅读没有进入其 inbox 或私有投递的普通频道消息；Room Agent 负责判断是否需要
  进一步路由。
- 会话恢复保证的是持久化 Work 和投递可以重新对账，不能保证机器离线时正在进行的模型回合从
  同一个 token 位置继续。
- Named Agent 目前不支持外部 Agent core（如 Claude Code 等）；模型多样性通过 wuu 自身的
  provider / model 配置实现。

## 相关文档

- [Agent 协作与子代理](subagents.md) —— 匿名子代理的委派、隔离和整合
- [会话与分支](conversations.md) —— 在工作区路径中管理对话
- [Skills](../customize/skills.md) —— 让 Agent 遵循固定工作流
- [记忆](../customize/memory.md) —— Named Agent 的记忆和 session 记忆边界
