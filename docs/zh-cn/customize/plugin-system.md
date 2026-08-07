# 插件系统架构

本文解释 Wuu 插件系统当前的产品模型和架构边界：功能插件与外观插件如何组合，宿主与插件
分别负责什么，以及开发者应该在什么时候选择设置 Schema、View、UI Kit、Slot、Presenter、
Surface 或 Agent 能力。

如果你只想安装和管理插件，阅读[插件](plugins.md)；如果你准备编写插件，阅读
[编写插件](plugin-authoring.md)。本文不重复完整 API，而是说明这些 API 为什么存在、怎样协作。

## 北极星

> 让功能插件能够自由扩展产品，让外观插件能够统一改变这些扩展的视觉；两者正交、可组合，
> 并且不需要知道彼此的存在。

这意味着：

- 功能插件声明能力、入口和内容，不为某个主题编写专用版本；
- 外观插件作用于公开的语义 Token、UI Kit 和粗粒度界面边界，不逐个追踪私有 DOM；
- Wuu 宿主拥有布局、溢出、滚动、Tab、关闭、键盘、无障碍和系统安全区；
- 一方插件与第三方插件使用同一套机制，不通过产品 ID 特判；
- 允许牺牲任意像素级控制，换取插件组合、宿主升级和长期兼容。

插件系统的目标不是让插件接管 Wuu 的每个像素，而是让它能够形成完整、强烈且一致的视觉
语言，同时不破坏产品结构和恢复路径。

### 更上层的北极星：最小循环，产品由插件组成

Wuu 的长期目标不是在现有 Agent runtime 外围增加一层插件 API，而是主动把核心收敛成一个
很小的通用循环：向模型发出请求，接收回复，执行 Tool，把结果追加回上下文，然后继续或结束。
Plan 是这个执行循环的标准状态和模型可见控制面，继续留在核心；它不负责跨 Turn 自动续跑、
定时唤醒或长期目标管理。Provider 协议不变量、取消、执行租约、持久化完整性、最终权限边界和
崩溃恢复也仍由宿主保证；memory、dream、Cron、Goal、Subagent 等高级产品不应继续作为循环里的
产品分支。HelpMe 直接从产品与代码中删除，不迁移成插件；协作比这些能力更高层，暂不纳入本轮
剥离范围。

这些高级产品应成为一方插件，而且与第三方插件使用同一套公开能力。插件拥有自己的业务模型、
提示、Tool、状态、后台策略和完整界面；Wuu 自己提供的插件只是生态的第一批实现，不是可以调用
私有接口的例外。判断插件平台是否足够的标准，也不是公开了多少接口，而是这些真实产品能否在
核心没有对应产品名和产品分支的情况下完整运行、禁用、升级和卸载。

这不意味着牺牲现有产品体验。比如记忆设置页可以继续提供自动概览、手动重新总结、查看原文，
以及通过对话让 Agent 修改记忆。迁移后，这些交互、提示和数据格式属于 memory 插件；宿主只
负责设置页入口、View 生命周期、模型与执行安全、持久化原语和恢复路径。产品可以保持丰富，
核心不需要知道“记忆概览”是什么。

### 不是工作流 API，而是少量可组合链路

Cron、Memory、Dream、Goal 和 Subagent 看起来是五套产品，实际都可以落在同一个闭环：

```text
注册 Tool / Prompt / View
        ↓
观察 Session / Turn 事件或插件自己的 Timer
        ↓
创建、选择或复用 Session
        ↓
向 Session 投递模型输入
        ↓
宿主排队并执行普通 Turn
        ↓
插件观察结果，更新自己的状态和界面，必要时再次投递
```

这里的 Session 指一个可持续投递 Turn 的对话执行单元，当前代码内部主要称为 Thread。公开合同
最终采用哪个名字可以随 SDK 版本确定，但不能同时保留两套语义重复的执行系统。

插件平台需要的不是 `cron.run`、`goal.continue` 或 `subagent.spawn`，而是四组横向能力：

| 公共能力 | 最小职责 |
| --- | --- |
| 注册贡献 | 注册模型可见 Tool、提示与请求上下文、命令、设置、View 和产品入口 |
| Session 操作 | 创建、投递、取消以及列出当前插件拥有的 Session |
| 生命周期事件 | 观察 Session 创建/关闭以及 Turn 排队、开始、完成、失败和取消 |
| 状态与资源 | 命名空间存储、受控文件/进程/workspace，以及插件 runtime 自己维护的 Timer 和业务状态机 |

创建 Session 时需要少量通用属性：`owner`、`visibility`、可选 `parent`、上下文来源
`fresh | fork(parent)`、workspace 隔离方式，以及模型/Tool/Profile 选择。`visibility=user` 的
Session 出现在普通会话列表、搜索和历史中；`visibility=plugin` 的 Session 不出现在这些产品入口，
只能由所有者插件管理，但仍由宿主持久化、恢复和审计。Dream 和 Subagent 可以使用插件私有
Session，Cron 可以创建或复用用户可见 Session。

Timer、Cron 表达式、错过触发后的补跑策略和运行记录属于 Cron 插件。插件进程存活时可以自行
计时，重启后依据持久状态重新计算；除非以后明确承诺“Wuu 未运行时也能唤醒”，否则不应先在
宿主建设一个 Cron 产品或万能 scheduler。若未来确实需要操作系统级唤醒，也应作为产品中立的
插件进程唤醒能力单独设计。

### 唤醒主 Agent 与“生成的 query”

Goal 自动续跑和 Subagent 完成回投共享一条尤其重要的公共链路：它们都会向已有主 Session 投递
一次新输入，从而唤醒主 Agent。Cron 向用户可见 Session 投递时也可复用同一语义。产品表现统一
使用现有 query 气泡：用户发送和插件唤醒不需要两套布局、颜色或交互节奏。这里“不把它记录成
用户亲自发送的消息”只约束持久化来源和可编辑性，不要求前端把生成 query 画成另一种组件。

因此通用投递必须区分三件事：

- **模型输入**：真正送给 Agent 的 Prompt 或结构化上下文；
- **展示摘要**：前端 query 气泡里的简短说明，可以与完整模型输入不同；
- **来源**：用户、宿主或具体插件，以及稳定的 cause/request id；该来源不改变 query 气泡的
  基础视觉语义。

概念上的合同类似：

```text
session.send({
  session,
  input,
  presentation: { kind: query_bubble, text, name },
  cause,
  request_id
})
```

`query_bubble` 表示沿用标准 query 气泡展示，不表示作者一定是用户。宿主在持久化记录中盖章
`origin=user | host | plugin`，插件身份由 generation 绑定，不能由请求伪造；系统生成项默认只读、
可审计。Provider 适配层为了启动一次模型回合，应当把这类输入投影成普通 `user` role：对模型
而言它就是驱动下一回合的 query，不需要额外的“系统唤醒消息”协议。这里的 `user` 是 Provider
协议角色，不是声称真人亲手发送；产品数据里的可信来源仍是插件，前端也只能使用经过区分的
展示摘要，不能用完整内部 Prompt 冒充用户原文。宿主负责幂等、持久化、执行租约、排队、取消
和恢复，并保证真实用户工作优先；插件只决定何时投递、模型看什么以及用户看到什么。
Subagent 的完成通知可以把完整交接内容作为模型输入，只把“子任务 A 已更新”显示在气泡里；Goal
可以把目标继续提示作为模型输入，只显示“Goal 持续推进中”。前端只接收经过区分的展示摘要和
来源元数据，不直接拿完整内部 Prompt 生成气泡。这不需要两条产品专用唤醒链路。

这条能力也不是 Goal 或 Automation 的变相专用接口。任何插件只要需要把后台结果、定时触发、
审批恢复、重试结果或外部事件交还给一个持续存在的 Agent，都需要同一条“向 Session 投递普通
query”的链路。核心因此保留的是 `session.send` 的可靠排队和执行语义，而不是“定时唤醒”“目标
续跑”或“子任务交付”等业务含义；是否以及何时调用它完全属于插件。

### 从产品需求提炼公共能力，而不是公开产品内部

一方产品迁移会暴露插件平台的缺口，但不能把当前实现直接翻译成 `host.memory.*`、
`host.automation.*` 或 `host.collaboration.*` 一类低频接口。新增公共能力时遵循以下顺序：

1. 先把一个产品作为完整纵向切片迁移，明确它自己的领域逻辑、状态和界面；
2. 到真实调用点时，判断缺少的是插件自己可以实现的逻辑，还是必须由宿主仲裁的通用原语；
3. 只有涉及宿主所有权、跨进程安全、共享资源或生命周期完整性时，才增加宿主能力；
4. 用至少另一个合理的一方或第三方场景检验抽象，避免接口只是在隐藏当前产品名；
5. 以最窄的版本化输入、输出和生命周期合同发布，不暴露私有 ThreadItem、React state、
   Agent loop 回调或内部存储结构。

“通用”不等于预先建设一套万能框架。真实迁移尚未触达的 scheduler、后台任务或资源 API 不应
凭想象加入 SDK；目前已经由多个产品证明需要的是“创建/复用 Session”“向 Session 投递输入并
选择展示方式”“订阅生命周期事件”。它们应成为产品中立的能力，而不是分别保留一套 app-server
RPC。

当前代码给出了几个明确的迁移方向：

| 产品 | 如何由公共链路组合 | 插件应拥有 | 不应留在核心的产品接口 |
| --- | --- | --- | --- |
| Cron | Timer → 创建或复用用户可见 Session → 投递 Prompt；注册管理 Tool 和 View | Cron 表达、任务和运行记录、补跑策略、提示、Tool、Timer 与完整界面 | `automation/create`、`AutomationRunID` 和 Turn 中的自动化分支 |
| Memory | 注册提示与文件 Tool → 在合适时机读写文件；管理 View 可调用 Agent 修改文件 | 用户、工作区和会话记忆格式，读写 Tool、安全策略、概览与修改提示、审计和设置界面 | `memory/overview`、`memory/chat`、`session_memory` 核心 Tool 和核心配置字段 |
| Dream | Timer + Memory → 插件私有 Session → 整理后通过 Memory Tool 写回 | 候选选择、整合提示、失败退避、结果状态和管理界面 | `sessionDreamScheduler` 和 StreamRunner 的产品专用 AfterTurn Hook |
| Goal | Turn 完成事件 → 检查目标状态 → 向同一主 Session 投递生成的 query | 目标状态机、预算、提示、Tool、存储和界面 | `agent.turn.continuation` 及 Goal 专用的 probe/prepare 调度 |
| Subagent | 创建私有子 Session → fresh/fork 上下文 → 投递任务 → 完成后向父 Session 回投生成的 query | `spawn_agent` 等 Tool、任务命名、worker 策略、提示、报告和界面 | `host.child_session.request` 的 spawn/send/close/list/await/report 产品动作 |

表中是目标能力模型，不是已经完成的 SDK 承诺。字段和方法必须在纵向迁移到达真实调用点后再定
版本化合同。例如 memory 概览和修改不需要宿主“受约束模型任务”：它可以创建或复用一个 Session
并发送 Prompt。只有这条公共链路确实无法保护宿主不变量时，才继续提炼更底层能力。

### 最小核心与公共能力的边界

最小核心不等于零能力宿主。以下责任如果开放给插件接管，会破坏所有产品共同依赖的不变量，
因此应继续留在 Wuu：

- Provider 消息顺序、Tool call/result 配对、流式响应和上下文窗口等协议正确性；
- Thread/Turn 的持久化、排队、执行租约、取消和崩溃后恢复；
- Plan Tool、当前 Turn 的计划状态和标准事件/展示；Plan 不拥有跨 Turn 调度；
- Tool 执行、最终权限判定、工作区边界和用户审批；
- 插件安装、批准、generation 原子替换、错误隔离、安全模式和默认 UI 逃生路径；
- 原生窗口、系统安全区以及宿主拥有的导航、Tab、滚动、溢出和无障碍。

其他能力默认属于插件。宿主公开的是少量可组合原语：模型可见 Tool、系统提示与请求上下文、
请求变换、压缩决策、Session 创建与投递、生命周期事件、generation 绑定的 runtime 调用与事件、
命名空间设置和存储。公共原语应组合出多个产品，而不是一项原语对应一个 Wuu 功能。

每次新增插件接口都应通过四个问题：

1. 去掉 memory、automation、collaboration 等产品名后，这个能力仍然自然吗？
2. 它是否保护了只有宿主才能保护的不变量，还是仅仅替插件省了几行领域代码？
3. 一方插件和外部插件能否在相同权限与生命周期下使用它？
4. 禁用插件后，核心是否仍是一个可用的通用 Agent，而不是留下半个产品状态或循环分支？

如果答案不成立，应继续把逻辑留在插件内部，或重新寻找更底层、更高复用的能力边界。

### 一方高级功能的迁移结果

下面按真实分发中的高级功能记录已经完成的纵向切片。验收依据是业务状态、Prompt、Tool、后台
链路和 Desktop 展示都由插件拥有，而不只是把代码移动到 `plugins/`：

1. **Goal 已迁移到公共 Session 链路。** Goal 的状态机、存储、Tool、提示和 UI 均由插件拥有；
   插件观察 `agent.turn.completed` 后通过 `host.session.send` 向同一 Session 投递只读 query。
   `agent.turn.continuation`、`probe/prepare` 两阶段轮询以及 Turn 主链路里的 Goal 续跑分支已经删除。
2. **Subagent 已迁移到公共 Session 链路。** 插件通过 `host.session.create/send/list/cancel` 创建和
   管理私有子 Session，用插件存储维护任务名与交付状态，观察 owner-scoped Turn lifecycle 后再向
   父 Session 回投只读 query。fresh/fork、共享目录/worktree、模型别名、所有权、取消与最终输出
   都是产品中立的 Session 合同；`host.child_session.request` 及其
   `spawn/send/close/list/await/report` switch 已删除。现有 `agentcontrol` 的租约和恢复代码仍可服务
   核心内部执行，但不再是 Subagent 插件的公开或私有调用入口。
3. **通用 Session create/send 已取代 `host.turn.submit`。** 创建与投递是两个独立调用；创建持久化
   generation 绑定的 owner、`user | plugin` 可见性、parent、`fresh | fork` 和幂等 request id，
   投递则分离模型输入与 query 气泡摘要，并持久化真实插件来源、cause 和只读属性。Provider 仍按
   普通 user role 执行，私有 Session 不进入普通列表与搜索，真实用户排队工作优先于插件唤醒。
4. **HelpMe 已完整删除。** Tool、Schema、Prompt、内部 worker type、历史重写/压缩特判、桌面文案、
   测试和死代码均已移除；没有保留兼容入口，也没有把它重命名成另一种插件工作流。
5. **Plan 的旧迁移设想已经作废。** Plan 留在最小核心链路。插件仍可观察标准计划状态或在自己的
   View 中展示它，但不替换核心 `update_plan` 语义，也不把 Plan 扩张成 Goal。
6. **Automation 已迁移到公共 Session 链路。** 一方插件拥有 Cron 表达式、Timer、补跑、任务与
   运行记录、Prompt、`cron` Tool 和完整桌面 View；触发时只调用 `host.session.create/send`，并通过
   通用 Turn lifecycle 收敛运行状态。核心的 Automation RPC、Manager、scheduler、Turn 特判、
   原生页面、IPC 与 Tool 展示特判已经删除；generation shutdown 会先停止插件后台 Timer。
7. **Memory 已完整迁移到公共插件链路。** 一方插件拥有用户笔记本、工作区 `project_memory`、
   会话 `summary/checkpoint/notes` 的文件布局、安全过滤、`memory_*`/`session_memory` Tool、系统提示、
   概览/管理私有 Session 和完整桌面 View。核心不再读取用户 `MEMORY.md`，不再自动注入会话记忆，
   也不再提供 Memory RPC、原生页面、IPC、核心 Tool/capability、启停配置或专用模型角色。宿主只把
   已解析的 `workspace_state_dir` 作为通用初始化上下文交给插件，并继续拥有普通 Session/Turn
   持久化。旧 `memory` 指令发现配置只在加载边界迁移为产品中立的 `instructions` 配置。
   普通核心文件 Tool 也不再放行用户 Memory 或整个 `WUU_HOME`；命名 Agent 的身份笔记本属于
   暂不迁移的协作域，只在该 Agent 的显式文件范围内开放，不能据此扩张成 `host.memory.*`。
8. **Dream 已迁移到公共 Session 链路。** 一方插件观察标准 Turn 完成事件，用插件存储维护候选、
   Timer、间隔、失败退避和运行状态，再通过 `host.session.create/send` 创建 fork 私有 Session；Prompt
   和设置 View 也由插件拥有，并让该 Session 通过 Memory 插件提供的 `session_memory` Tool 写回。
   核心 `sessionDreamScheduler`、Dream 状态/锁、AfterTurn Hook、配置字段和原生设置已经删除。

### 改造顺序与完成标准

改造应复用已有可靠执行能力，先换公共边界，再删除专用入口：

1. 以现有 Turn submission 为基础定义通用 Session create/send/lifecycle；补齐 owner、visibility、
   parent、fresh/fork、workspace、来源、展示摘要和 request id，统一用户工作优先及幂等规则。
2. 先迁移 Goal：在 Turn 完成事件后由插件主动向同一 Session 投递，验证生成 query、连续唤醒、
   排队、暂停/完成、崩溃恢复和禁用插件；随后删除 `agent.turn.continuation`。
3. Subagent 已使用公共 API 创建和管理私有子 Session；`host.child_session.request` 已删除，任务
   提示、状态、桌面状态条和父 Session 回投由插件拥有。宿主不再识别 `spawn_agent` Tool 名、解析
   `<subagent_notification>`、生成专用 Tool item 或维护原生子任务面板；插件生成的回投只依赖通用
   `display_content/origin/cause/read_only` query 元数据，不改变公共合同。
4. HelpMe 全链路已删除；Plan 仍通过核心 Tool/状态链运行。
5. Cron、Memory 和 Dream 已分别完成“插件 Timer → 用户可见 Session”、“Prompt + Tool + 私有
   Session + View”和“Timer + Memory Tool + 插件私有 Session”的纵向切片，三者没有产品专用
   宿主服务。
6. 每项迁移都必须验证插件禁用、升级和卸载后不再唤醒、不残留 UI、Prompt、Tool、订阅或后台
   generation；核心删除旧协议、死代码和只为旧边界存在的测试。

插件链路的验收不是接口存在，而是：在核心搜索不到 Cron、Memory、Dream、Goal、Subagent 的产品
调度分支时，这五个一方插件仍能仅通过公开合同保持现有体验；外部插件在相同权限和生命周期下也
能组合出同类能力。

## 一个插件包，四类贡献

同一个插件包可以只包含一种贡献，也可以同时包含多种贡献。

| 层 | 贡献内容 | 运行位置 | 典型用途 |
| --- | --- | --- | --- |
| 声明层 | 主题、设置、入口、权限和元数据 | Manifest + 宿主 | 主题、开关、左侧入口、右侧工具、设置页 |
| Agent 层 | Tool、Hook、请求变换、系统提示、压缩策略 | 独立 runtime 进程 | 搜索、策略控制、memory、上下文处理 |
| Workbench 层 | View、命令、状态项、Presenter、Slot、Surface | Electron Renderer | 协作页面、审查工具、消息呈现、复杂设置 |
| 外观层 | 主题 Token、UI Kit 样式、公开语义锚点 | Renderer CSS | 完整换肤、密度、字体、材质和控件视觉 |

这四层不是四种互斥的插件类型。例如未来的协作插件可以同时注册 Agent 工具、左侧入口、
工作区 View、设置项和命令；另一个 Manga 外观插件只提供主题和样式。两者安装后自然叠加。

## 宿主、功能插件与外观插件的责任

| 参与方 | 拥有什么 | 不应该做什么 |
| --- | --- | --- |
| Wuu 宿主 | 窗口安全区、左右侧栏、标题栏、Tab、面板、滚动、溢出、持久化、无障碍和恢复路径 | 为某个插件写产品特判 |
| 功能插件 | 业务能力、内容、命令、设置定义、入口意图和插件内部状态 | 重画宿主导航、自己排列系统 Tab、依赖当前主题 |
| 外观插件 | 颜色、字体、边框、圆角、阴影、材质、有限密度和语义控件状态 | 移动交通灯安全区、改写私有 DOM、按功能插件 ID 逐个适配 |

核心规则是：**谁拥有布局，谁控制间距节奏。** 宿主容器决定页面边距、Tab 尺寸、系统安全
距离和大区域排列；插件在分配给自己的内容区里组织功能。功能插件使用 UI Kit 后，其公共
界面自动继承当前外观。

## 声明式优先，代码是逐级开放的逃生口

开发者应使用能表达需求的最窄接口。越靠下的接口自由度越高，兼容成本和信任成本也越高。

| 需求 | 优先使用 | 用户在哪里看到 | 宿主保留的控制 |
| --- | --- | --- | --- |
| 开关、文本、数字、枚举设置 | `contributes.settings` | 设置页的插件分组 | 表单布局、存储、校验和主题 |
| 左侧主功能入口 | `contributes.navigation` + View | 左侧栏可滚动的插件分组 | 选中态、排序基础和滚动 |
| 右侧上下文工具 | `contributes.workspaceTools` + View | 右侧工具选择器和原生工作区 Tab | 打开、关闭、Tab、面板宽度和持久化 |
| 复杂插件设置 | `contributes.settingsPages` + View | 设置页的插件分组 | 设置导航、页面框架和滚动 |
| 自有工作区页面 | `registerViewType` | 宿主管理的工作区区域 | View 生命周期、位置和持久化 |
| 普通插件界面 | `api.ui` | View 或自定义设置页内部 | 公共组件节奏和主题兼容 |
| 在已有区域追加少量内容 | `registerSlot` | 固定的宿主插槽 | 原生内容和插槽排列 |
| 改变消息、Composer、导航等产品概念 | `registerPresenter` | 对应语义组件 | 版本化快照、允许的动作和失败回退 |
| 包装或替换一个完整大区域 | `registerSurface` | Sidebar、Settings、Catalog 等 Surface | 边界外布局、故障隔离和恢复入口 |
| 强风格化装饰 | 主题 Token，必要时 `registerStyle` | 整个桌面 | 公开语义合同和结构安全 |

不要因为需要一个按钮就替换整个设置页，也不要因为需要全局换肤就给每一层容器发明一个
锚点。接口粒度应对应真实的视觉或功能责任边界。

## 功能入口由宿主管理

插件声明的是放置意图，不是绝对坐标：

- `navigation` 适合高频主功能，例如协作、项目管理或会话类功能；
- `workspaceTools` 适合文件、审查、终端、浏览器等上下文工具；
- `settingsPages` 适合标准 Schema 无法表达的复杂设置；
- 不需要常驻 UI 的插件可以只贡献 Tool、Hook、命令或后台能力。

插件提供标题、图标、排序偏好和 View，Wuu 负责入口选中、Tab、关闭、恢复和面板生命周期。
插件不能为了“多一个入口”替换整条导航或 Tabbar。

当前左侧插件入口位于可滚动分组；右侧工具以宿主工具选择器和原生工作区 Tab 打开。完整的
用户固定、取消固定、重排和“更多”溢出菜单仍是后续能力，见[当前边界](#当前边界与尚未完成的能力)。

## 设置页怎样与外观插件组合

标准设置使用 `contributes.settings`。宿主生成导航和控件，并按插件命名空间存储值。复杂设置
可以注册自定义 View，再通过 `contributes.settingsPages` 进入同一个设置导航。

无论设置来自 Wuu、插件 A 还是插件 B，外观插件都通过同一套设置语义和 UI Kit 作用于它们：

1. 功能插件只描述字段或渲染使用 UI Kit 的内容；
2. Wuu 提供设置页框架、页面宽度、滚动和公共控件；
3. 外观插件提供颜色、字体、边框、圆角和材质；
4. 三方不需要互相知道 ID，也不需要配对发布。

如果一个自定义设置页完全自绘，它仍会继承公开基础 Token，但只有使用 UI Kit 的区域才能获得
完整的公共组件兼容。画布、终端、Webview 和专用预览属于明确的主题边界。

## UI Kit 不是页面 DSL

`api.ui` 当前提供 `Page`、`Panel`、`Card`、`Section`、`Stack`、`Row`、`Button`、
`TextInput` 和 `EmptyState`。它的目的有三个：

- 收敛页面、卡片、行和控件的公共节奏；
- 让功能插件自动继承任意兼容外观插件；
- 避免外观插件追踪每个功能插件的私有 class。

它不是一门描述整个页面的 DSL。复杂 View 仍可使用 React 和自己的组件，只需把 UI Kit 用在
适合统一的公共区域，并明确自绘区域的主题边界。UI Kit 的组件集合会由真实插件需求推动扩展，
不会预先复制一套完整组件库。

## 外观合同：Token、组件和语义锚点

外观能力分三层，优先级从上到下：

1. **主题 Token**：语义颜色、字体、间距密度、圆角、边框、阴影、动效、内容宽度和语法色。
   注册表位于 `config/desktop-theme-contract.json`，并生成 Manifest、SDK 和桌面校验代码。
2. **UI Kit**：功能插件使用的宿主公共组件。外观插件修改 Token 后，这些组件自动变化。
3. **粗粒度语义锚点**：`data-wuu-component`、`data-wuu-layer`、`data-wuu-state` 和变体属性，
   用于 Token 无法表达的结构化装饰，例如消息气泡、面板、Tab 选中态或浮层。

锚点按视觉责任边界划分，通常到页面、面板、卡片、行、复合控件就停止。外观插件可以改变
Tab 的整体材质和选中 indicator，但不能拆开 Tab 与关闭按钮重新排列；可以统一装饰标题栏按钮，
但不能缩小 macOS 交通灯安全区。

可信桌面代码可以通过 `registerStyle` 注册任意 CSS，但私有 class、DOM 层级和偶然结构不属于
兼容合同。Manga Studio 的作用是压力测试公开合同，不是要求宿主为 Manga 增加特判。

## Agent 能力与闭环

Agent 插件运行在 Wuu 管理的独立进程中，通过版本化协议注册能力。当前公共能力覆盖：

- 注册模型可见的 Tool；
- Tool 执行前后的 guard 和 transform；
- 系统提示片段与请求变换；
- 压缩策略；
- 会话生命周期观察；
- Shell 环境贡献。

每个能力声明语义种类，例如 observe、transform、guard、around 或 decision，并按稳定优先级
组合。Guard 和 decision 可以短路；Transform 按顺序执行。工具或能力出错时，宿主按公开错误
策略传播、隔离或回退，不能靠吞掉异常维持表面成功。

增加一个 LLM 可见能力时必须闭合两端：实现注册与执行路径，也要让提示或公开说明告诉模型
何时使用；同时保持消息顺序、Tool call/result 配对等 Provider 协议不变量。

## Generation：安装、激活、替换和卸载的原子单位

插件所有可执行贡献都属于一个 generation：

1. Wuu 读取 Manifest、重算整包 fingerprint，并检查批准和权限；
2. 候选 generation 在外部构建和注册，尚未对用户可见；
3. 校验成功后一次性替换旧 generation；
4. 校验或激活失败时，旧 generation 保持运行；
5. 禁用、升级、卸载或开发热重载时，旧 generation 的 View、样式、命令、事件订阅和 runtime
   一起释放。

React 组件可以在 generation 激活后订阅宿主事件，组件卸载或 generation 被替换时订阅会清理。
重复的同类诊断按 generation 去重；同一 generation 重新成功激活后，陈旧诊断被清除。

桌面 Slot、Presenter 和 Surface 都有局部错误边界。一个贡献渲染失败时，Wuu 隔离当前边界并
保留原生 fallback，不让整个 Renderer 白屏。插件管理、设置、禁用和恢复默认界面始终可达。

## 多插件组合与冲突

正常组合不需要仲裁：多个 Slot 按稳定顺序追加，多个 `wrap` Presenter/Surface 依次包装，
功能插件和外观插件通过语义合同正交叠加。

只有多个插件都要求 `replace` 同一语义边界时才产生互斥冲突。Wuu 使用稳定默认顺序选择一个
生效者，并在插件设置中显示候选，让用户可以明确选择。冲突选择由宿主持久化，插件不能自行
覆盖另一个插件。

一个典型验收组合是：

- 插件 A 提供标准设置；
- 插件 B 提供右侧工具；
- 协作插件提供左侧入口、自己的 Tab 和复杂页面；
- 外观插件提供完整主题。

四者不互相认识，但 A、B 和协作插件的公共 UI 都采用当前主题；禁用外观插件后功能与布局
保持完整，禁用任一功能插件后对应入口、订阅和状态干净卸载。

## 一方插件与第三方插件同构

Goal、Subagent 已经通过与第三方插件相同的 generation 和公共 Session 服务运行：前者观察 Turn
终态并向主 Session 投递普通 query，后者创建插件私有 Session 并在完成后向父 Session 交付结果。
两者的专用 continuation 与 child-session 宿主接口已经删除，是当前“一方/三方同构”的纵向证明。
Cron 也已通过同一公共 Session 链路迁移；memory、dream 继续按该边界迁移。协作暂不纳入当前
改造范围，不应为了它预建接口。

判断标准是：如果一项一方功能只能修改私有循环或私有 UI 才能实现，应先确认缺少哪一个通用
能力；只有真实需求证明公共合同不足时才扩展宿主。Wuu 自己的插件是生态的第一批使用者，不是
绕过规则的例外。

## 信任与恢复边界

插件是本地安装的应用代码，不承诺沙箱：

- 声明式主题和设置不执行插件代码；
- Agent runtime 进程与 Wuu 拥有相同用户权限；
- 桌面代码在 Renderer 中运行，并可注册任意 CSS；
- 插件包任何字节变化都会产生新 fingerprint，旧批准失效；
- runtime 只获得文档化的环境变量白名单，不直接继承全部宿主环境；
- Renderer 不读取插件绝对路径，Electron 校验摘要后通过内容寻址的 `wuu-plugin:` URL 加载；
- CSP 不开放 `unsafe-eval`。

插件管理、审批、安全模式、权限底线、原生窗口、app-server 生命周期、崩溃恢复和默认 UI
永远由 Wuu 控制。只能安装和启用可信来源的可执行插件。

## 当前边界与尚未完成的能力

以下内容不能写成已经完成的兼容承诺：

- 还没有 previous-minor/current-minor 的 SDK 与宿主兼容矩阵；插件发布时应声明
  `minimum_wuu_version`，Wuu 升级后重新验证；
- 左侧入口已有滚动，右侧 Tab 已有溢出，但用户固定、取消固定、重排和统一“更多”菜单尚未完成；
- 右侧工具和设置页已有声明式入口，通用底部面板贡献仍未形成稳定公开合同；
- UI Kit 仍然很小，结构化表单、多行输入、列表等应由真实插件需求继续收敛；
- 部分 Presenter 的 `replace` 快照和 Action 还不足以无损重建完整原生语义，优先使用 `wrap`；
- 画布、终端、Webview、PDF ShadowRoot 和专用预览仍是明确的主题边界；
- Marketplace、远程自动更新、排名、依赖解析和签名分发不属于当前本地优先平台；
- Goal、Subagent、Cron、用户 Memory 和 Dream 已去除专用宿主执行 seam；工作区/会话 memory 尚未完成迁移；
- HelpMe 已从代码和产品中删除；Plan 明确保留在核心链路。

这些边界不是鼓励为未来预留抽象。新能力应由真实插件案例驱动，先确定责任属于宿主、功能插件
还是外观插件，再选择最窄的公开合同。

## 开发和验收原则

开发插件时优先验证真实组合，而不是只验证单个接口：

1. 功能插件和外观插件同时启用；
2. 标准设置、自定义设置、左侧入口和右侧工具都能发现；
3. 禁用外观插件后布局与功能不变；
4. 禁用或更新功能插件后贡献干净卸载；
5. 失败 generation 不替换当前可用版本；
6. 强风格主题不会破坏交通灯安全区、Tab、滚动、关闭和无障碍；
7. 实现主要依赖 Token、UI Kit 和少量语义锚点，而不是几十个私有选择器。

仓库中的示例各自承担不同作用：

- [`examples/plugins/developer-loop`](../../../examples/plugins/developer-loop/)：公开 SDK、runtime、
  View、入口、设置、存储和 generation 开发闭环；
- [`examples/plugins/manga-studio`](../../../examples/plugins/manga-studio/)：强风格外观压力测试，
  验证原生界面与插件界面能否统一换肤；
- [`examples/plugins/deep-ui`](../../../examples/plugins/deep-ui/)：Surface wrapper 和声明式主题的
  最小示例。

最终判断只有一句：**功能可以自由扩展，外观可以整体换肤，多插件可以自然叠加，宿主界面仍然稳定。**
