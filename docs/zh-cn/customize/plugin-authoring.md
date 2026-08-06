# 编写插件

本文面向要扩展 Wuu 的开发者：插件如何打包、Agent 链路如何扩展、桌面界面如何贡献、
以及本地开发闭环怎么跑。用户侧的安装与管理见[插件总览](plugins.md)。

Wuu 插件平台当前是本地优先的：没有市场、没有中心仓库。插件以目录或 zip 包的形式
在本地安装，开发者通常在自己的 GitHub 仓库中维护，用户 clone 或下载后在 Wuu 中安装。
分发能力只有在这种自然生态出现后才会建设，插件作者现在不需要为任何平台账号或审核
流程做准备。

## 插件能做什么

一个插件包可以同时包含三种贡献，也可以只包含其中一种：

| 贡献 | 运行位置 | 需要代码吗 |
| --- | --- | --- |
| 声明式主题 | Renderer（CSS Token） | 否 |
| 声明式设置 | Renderer + app-server | 否 |
| Agent 插件 | 独立 runtime 进程（Node 等） | 是 |
| 桌面插件 | Wuu Renderer（ESM 模块） | 是 |

插件不能重做整个软件的外观。Wuu 是开源软件：想要完全不同外观的开发者应当 fork
仓库并自己承担合并成本，而不是让插件与宿主的每个版本互相锁定。Surface 替换等
高信任能力是面向可信插件的 escape hatch，区域集合冻结，不再扩张。

## 包结构与 manifest

```text
my-plugin/
├── plugin.json
└── dist/
    ├── runtime.js     # Agent 插件入口（可选）
    └── desktop.js     # 桌面插件入口（可选）
```

`plugin.json` 是清单。Wuu 在安装、每次加载前都会重新读取并校验它。最小示例：

```json
{
  "schema_version": 1,
  "id": "my-plugin",
  "name": "My Plugin",
  "version": "1.0.0",
  "description": "What this plugin does",
  "runtime": {
    "protocol": "wuu-plugin-v1",
    "command": "node",
    "args": ["dist/runtime.js"]
  },
  "desktop": {
    "entry": "dist/desktop.js"
  }
}
```

常用字段：

- `id` 是全局唯一标识，决定安装目录名和所有注册的命名空间前缀；一旦发布不应更改。
- `version` 是语义化版本。包的任何文件变化都会产生新的整包 fingerprint，原审批随之失效。
- `runtime` 声明一个长驻的外部进程，通过标准输入输出与 Wuu 通信（Agent 插件）。
- `desktop.entry` 指向一个自包含的浏览器 ESM 文件，最大 10 MiB（桌面插件）。
- `contributes.themes` 声明式主题；`contributes.settings` 声明式设置。
- `skills`、`hooks`、`mcp_servers`、`commands` 可以让插件直接提供这些能力，与用户
  手动配置的效果一致。
- `minimum_wuu_version` 声明所需的最低 Wuu 版本；不满足时插件不会被激活。

完整字段定义以 [`internal/plugin/manifest.go`](../../../internal/plugin/manifest.go) 和
[`packages/plugin-sdk`](../../../packages/plugin-sdk/) 为准。

## 声明式贡献

### 主题

无需任何代码，在 `contributes.themes` 中声明即可。获批且启用的插件主题会出现在
"设置 → 外观"；禁用插件或切回内置主题时，Wuu 会移除该插件设置的全部 Token。

```json
{
  "contributes": {
    "themes": [
      {
        "id": "my-dark",
        "name": "My Dark",
        "base": "dark",
        "tokens": {
          "--wuu-paper": "#111827",
          "--wuu-ink": "#f9fafb"
        }
      }
    ]
  }
}
```

公开 Token 由 [`config/desktop-theme-contract.json`](../../../config/desktop-theme-contract.json)
统一定义，并生成 Manifest、公开 SDK 和 Desktop 校验代码。稳定类别包括语义颜色、字体、
间距、密度、圆角、边框、层级阴影、动效、内容宽度和 `--wuu-syntax-*` 语法色。早期的
`--wuu-paper`、`--wuu-ink`、`--wuu-accent` 与 `--hljs-*` 等名称继续兼容，并在应用时
映射到当前语义 Token；新主题应优先使用 `--wuu-color-*`、`--wuu-font-*` 等当前名称。

### 设置

`contributes.settings` 声明生成式控件，支持 boolean、string、number 和 enum 四种类型。
每个设置都有 `scope`（`user` 或 `workspace`）和 `apply`（`live` 或 `restart`）。用户
在设置界面修改后，插件通过 SDK 的类型化 API 读取；桌面插件还可以通过
`api.settings` 访问。设置按插件命名空间存储，卸载插件时随 generation 一起清除。

```json
{
  "contributes": {
    "settings": {
      "enabled": {
        "type": "boolean",
        "title": "Enable counter",
        "default": true,
        "scope": "user",
        "apply": "live"
      }
    }
  }
}
```

## Agent 插件

Agent 插件是 `runtime` 声明的外部进程。安装或启用插件即授予该进程与 Wuu 相同的
用户权限，因此只能安装你信任的插件，并在启用前检查其来源。

### 进程与协议

runtime 进程由 Wuu 启动，是一个长驻进程，通过标准输入输出上的逐行 JSON 与 Wuu 通信。
它先协商协议与能力，然后持续接收事件和调用。开发者不需要手写协议：

- TypeScript 侧使用公开的 `@wuu/plugin-sdk` 包；
- Go 侧使用 `@wuu/plugin-go`（`packages/plugin-go`）。

进程生命周期由 Wuu 管理：启用时启动，禁用、升级或卸载时终止。插件不能重启自身或
绕过宿主对进程的监督。

### 扩展 Agent 的能力

runtime 插件可以注册工具和挂钩 Agent 生命周期。SDK 提供以下能力（以 SDK 导出为准）：

| 能力 | 作用 | 语义 |
| --- | --- | --- |
| `agent.tool.register` | 注册模型可见的工具 | transform |
| `agent.tool.execute.before` | 工具执行前拦截，可拒绝或改写参数 | guard |
| `agent.tool.execute.after` | 工具成功后包装或改写结果 | transform |
| `agent.system_prompt.section` | 贡献一段系统提示 | transform |
| `agent.request.transform` | 改写发给模型的请求 | transform |
| `agent.compaction` | 替换或参与摘要压缩策略 | decision |
| `session.start` / `session.stop` | 观察会话生命周期 | observe |
| `shell.env` | 提供命令运行时的环境变量 | transform |

每个注册都属于一个 generation，并声明 `kind`（observe / transform / guard / around /
decision）与 `priority`。guard 和 decision 可以短路；transform 按稳定顺序执行；插件
不能把一种 seam 当作任意回调来用。候选激活是原子的：激活失败会回滚整个 generation，
旧 generation 继续工作。

插件不会拿到私有 ThreadItem、协议消息、宿主 React 树或任意回调。快照、输入与输出
都是冻结的公开结构，具体类型以 SDK 的 `index.ts` 为准。

### 工具注册示例

```ts
import { definePlugin, ToolRegistration } from "@wuu/plugin-sdk";

export default definePlugin({
  async initialize(runtime) {
    runtime.registerTool({
      name: "my_search",
      description: "Search a private index",
      inputSchema: { type: "object", properties: { query: { type: "string" } } },
      async execute(args) {
        return { matches: await search(args.query) };
      },
    } satisfies ToolRegistration);
  },
});
```

### 工具策略挂钩示例

在 `tool.execute.before` 挂钩中拒绝访问工作区外的路径，与插件文档中
[权限模型](../reference/permissions.md) 的关系是：这是插件侧的策略，宿主的审批、权限底线和恢复
路径始终由 Wuu 保留，插件不能覆盖。

## 桌面插件

桌面插件是在 Wuu Renderer 中运行的受信任代码，可以注册全局样式，并替换或包装宿主
提供的稳定 UI Surface，用于形成统一视觉体系或做有边界的结构调整。它继续调用 Wuu
提供的会话与导航动作，不需要依赖 DOM monkey patch 或私有 React state。

桌面入口导出 `activate(api)`。不要打包另一份 React；应使用 `api.react` 提供的宿主
React：

```js
export async function activate(api) {
  const React = api.react;

  api.registerStyle({
    id: "visual-language",
    css: `
      :root {
        --wuu-accent: #7c5cff;
        --wuu-accent-press: #6847ed;
      }
    `,
  });

  api.registerSurface("app.sidebar", {
    id: "sidebar-frame",
    mode: "wrap",
    render(_context, fallback) {
      return React.createElement("section", { className: "my-frame" }, fallback);
    },
  });
}
```

`mode: "replace"` 接管完整语义边界；`mode: "wrap"` 包装当前结果。渲染失败只回退当前
边界，宿主始终保留设置、插件禁用和默认 UI 恢复路径。

### 可用 API 概览

- `registerStyle`：注册 CSS；任意 CSS 只提供给受信任的桌面代码插件。
- `registerSurface`：替换或包装稳定 Surface（`app.sidebar` 等）。
- `registerSlot`：在原生 UI 的稳定位置插入内容（`sidebar.primary`、
  `workspace.header`、`composer.toolbar`、`settings.plugin` 等）。
- `registerViewType` + `registerViewPlacement`：注册可持久化的 View，并请求宿主把它
  首次放到 `main`、`sidebar` 或 `auxiliary` 区域。`priority` 只在区域尚无用户选择时
  决定初始激活；用户后续的切换和关闭优先并持久化。落位 API 不暴露宿主 DOM、任意
  父节点、分割树或面板尺寸。旧 `registerLayoutContribution` 仅作兼容保留，其
  `parentId`、`size`、`minSize` 字段从未真正控制布局树，新插件应使用
  `registerViewPlacement`。
- `registerPresenter`：替换具体产品概念而不是宽泛区域。目标包括
  `conversation.item`、`conversation.process`、`conversation.tool-activity`、
  `conversation.composer`、`header.conversation`、`header.workspace`、
  `navigation.primary`、`app.status`、`content.preview`、`settings`。Presenter 收到
  冻结、带版本且经过脱敏的 snapshot、原生 fallback，以及只包含当前边界可用动作的
  host；只有出现在 `host.actions` 中的 Action 才能调用。
  `registerToolActivityPresenter` 继续作为兼容的 Tool 专用入口。
- `registerCommand`、`registerStatusItem`、`registerLocale`：命令、状态项和本地化。
- `api.settings` / `api.storage`：读取声明式设置，读写插件命名空间的持久化存储。

### 声明式 CSS 锚点

常用宿主 Dialog、菜单、Popover、Tooltip、Notice 和浮动导航统一渲染到受保护的
Layer Host，并带有稳定的 `data-wuu-component`、`data-wuu-layer` 和 `data-wuu-state`
属性。拖拽预览、PDF ShadowRoot 内容和插件 View pane 仍是专用渲染边界。

主要界面区域和控件带有公开的 `data-wuu-component` 锚点，让逐元素微调可以走 CSS
snippets 而不是新增主题 Token：`app-shell`、`sidebar`、`conversation-pane`、
`settings-shell`、`skills-catalog`、`automations-catalog`、`workspace-panel`、
`launch-view`、`turn`、`message`（区分 `data-wuu-variant="user" | "agent"`）、
`composer`、`composer-input`、`composer-send`（区分 `data-wuu-state="send" | "stop"`）。
这份清单由 `desktop/src/renderer/plugins/ProductionSemanticAnchors.test.ts` 强制约束；
锚点改名属于破坏性变更。

可信代码插件补充 CSS 时，应只使用这些公开属性和 Token，不应依赖私有 class 名或
DOM 层级。依赖私有 class 名可以用于本地实验，但不属于兼容性承诺。

## 本地开发闭环

### 脚手架与构建

```bash
wuu plugin create my-plugin      # 生成骨架
wuu plugin validate .            # 校验 manifest 与包结构
wuu plugin build .               # 运行包内构建（如果有 package.json）
wuu plugin test .                # 启动可执行 runtime，跑公开 SDK 契约检查
wuu plugin pack .                # 打成可分发 zip
```

`wuu plugin create` 生成 agent、desktop 或 full（两者都有）骨架。`wuu plugin test`
以非零退出码反映检查失败，适合接进 CI。

### 开发模式热重载

```bash
wuu plugin dev .
```

`dev` 授权**当前目录**为开发目录：保存后自动构建、校验候选、发布原子 generation，
并保留活跃 generation 的租约直到切换完成；构建或激活失败时保留上一代。目录授权
是开发专用，绝不转移到从下载包里安装的普通插件。

### 安装、验收与发布

```bash
wuu plugin install .                 # 从目录安装
wuu plugin pack .                    # 或打包后分发
wuu plugin install ./my-plugin-1.0.0.zip
wuu plugin approve my-plugin         # 检查后批准
wuu plugin enable my-plugin
wuu plugin dev .                     # 开发期修改
```

安装后代码必须经用户批准才会激活；文件变化产生新 fingerprint，原批准失效，需要重新
检查并批准。同一 ID 再次安装会被暂存为 pending 更新，已安装 generation 保持活跃，
直到新包被批准。

### 示例

仓库中的 [`examples/plugins/deep-ui`](../../../examples/plugins/deep-ui/) 是一个可以直接
安装的自包含示例：用 wrapper 保留所有宿主 fallback，并同时演示声明式主题。

[`examples/plugins/developer-loop`](../../../examples/plugins/developer-loop/) 是只依赖
公开 SDK 的跨 Surface 验收示例，覆盖 Agent runtime（request transform、工具注册）、
Host Actions、generation 替换、失败恢复、disposal 和卸载，并演示完整开发闭环
（install → build → test → dev → pack）。

## 版本兼容

插件跨 Wuu 小版本继续工作的承诺（开发者不 fork 也能跟上更新）是当前平台的
release gate，尚未完成验证：协议与 manifest 兼容锚点已存在，但还缺少
previous-minor/current-minor 的 SDK 与宿主兼容矩阵。在矩阵验证完成前，不要承诺
插件会跨小版本无条件工作；发布插件时声明 `minimum_wuu_version`，并在 Wuu 升级后
重新验证。

## 信任边界与安全内核

- 插件包是 untrusted 输入：Renderer 不会读取插件的绝对路径，Wuu 在每次加载前由
  app-server 重新加载 Manifest、重算整包 fingerprint，并确认插件在当前 workspace
  中仍获批且启用；Electron 主进程再次校验源码摘要，然后通过内容寻址的
  `wuu-plugin:` 协议加载模块；CSP 不开放 `unsafe-eval` 或任意本地脚本。
- 插件管理、审批、安全模式、崩溃恢复、权限提示的最终边界、原生窗口与 app-server
  生命周期、generation 错误隔离，以及用户逃生路径（设置、禁用插件、恢复默认 UI）
  始终由 Wuu host 控制，**永不**通过公开接口暴露给插件。
- 声明式主题只能修改公开语义 Token；`registerStyle` 可以使用任意 CSS，因此只提供给
  受信任的桌面代码插件。
- runtime 进程与 Wuu 同权限，启用的插件声明的 Hook 与直接运行第三方本地命令具有
  相同风险；安装和启用前检查来源、命令与授权状态。
