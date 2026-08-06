# 用插件定制桌面界面

Wuu 的桌面代码插件可以注册全局样式，并替换或包装宿主提供的 UI Surface。插件能够
改变整个应用框架、侧边栏和输入区，同时继续调用 Wuu 提供的会话与导航动作。

桌面代码插件与普通主题不同：它是在 Wuu Renderer 中运行的受信任代码。启用前应检查
来源和权限。插件管理、审批、安全模式、崩溃恢复和原生窗口生命周期始终由 Wuu
控制，插件不能替换这些恢复路径。

## 最小包结构

```text
my-layout/
├── plugin.json
└── dist/
    └── desktop.js
```

`plugin.json`：

```json
{
  "schemaVersion": 1,
  "id": "my-layout",
  "name": "My Layout",
  "version": "1.0.0",
  "desktop": {
    "entry": "dist/desktop.js"
  }
}
```

当前桌面入口必须是一个不依赖相对 import 的自包含 ESM 文件，最大 10 MiB。Wuu 会把
整个插件包纳入 fingerprint；文件变化后，原来的批准会失效，用户需要检查并批准新版本。

## 桌面入口

入口导出 `activate(api)`。不要打包另一份 React；应使用 `api.react` 提供的宿主 React：

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
      .app-shell { font-family: Inter, system-ui, sans-serif; }
    `,
  });

  api.registerSurface("app.sidebar", {
    id: "sidebar-frame",
    mode: "wrap",
    render(context, fallback) {
      return React.createElement(
        "section",
        { className: "my-sidebar-frame" },
        fallback,
      );
    },
  });

  api.registerSurface("conversation.composer", {
    id: "compact-composer",
    mode: "replace",
    render(context) {
      const actions = context.actions;
      return React.createElement("textarea", {
        value: String(context.prompt ?? ""),
        placeholder: "输入消息，按 Enter 发送",
        onChange: (event) => actions?.setPrompt?.(event.currentTarget.value),
        onKeyDown: (event) => {
          if (event.key === "Enter" && !event.shiftKey) {
            event.preventDefault();
            actions?.send?.(event.currentTarget.value);
          }
        },
      });
    },
  });
}
```

## Surface 模型

每个 Surface 都有 Wuu 内置的 fallback：

- `mode: "replace"`：用插件界面替换 fallback；同一 Surface 中排序最高的替换项生效。
- `mode: "wrap"`：保留 fallback，并在外层增加布局或行为；多个 wrapper 按稳定顺序组合。
- 插件抛出渲染错误时，Wuu 记录诊断并显示 fallback，不会让整个界面消失。
- 插件被禁用、删除或升级时，旧 generation 的组件、CSS、命令和清理函数会一起卸载。

当前已接入真实产品路径的 Surface：

| Surface | 用途 |
| --- | --- |
| `app.shell` | 替换或包装整个 React 应用界面 |
| `app.sidebar` | 替换或包装左侧导航 |
| `app.main` | 替换或包装主内容区 |
| `app.auxiliary` | 替换或包装辅助 Workspace 区域 |
| `app.status` | 添加应用状态区域 |
| `view.launch` | 替换或包装启动与 Runtime 加载界面 |
| `view.conversation` | 替换或包装当前对话界面 |
| `view.workspace` | 替换或包装 Workspace 侧面板 |
| `conversation.composer` | 替换或包装主输入区 |
| `conversation.timeline` | 替换或包装一组对话消息和 Agent 运行时间线 |
| `conversation.message` | 替换或包装一条脱敏后的对话消息边界 |
| `view.settings` | 替换或包装设置界面 |
| `view.catalog` | 替换或包装 Skills、插件和 Automations 目录 |

`app.shell` context 提供 `openSettings`、`startNewThread`、`openSkills`、
`openAutomations` 和 `toggleSidebar` 等动作。Composer context 还提供当前文本、运行状态、
`setPrompt`、`send` 和 `interrupt`。

需要在原生 UI 中增加内容而不是替换边界时，使用 `registerSlot`。当前生产 Slot 包括
`sidebar.primary`、`sidebar.footer`、`workspace.header`、`conversation.header`、
`conversation.message.before`、`conversation.message.after`、`composer.above`、
`composer.toolbar` 和 `settings.plugin`。Slot context 只包含冻结的摘要字段，不包含宿主私有
记录；Slot 会与原生 UI 和语义 Presenter 一起组合。

## 语义 Presenter

需要替换具体产品概念，而不是宽泛布局区域时，应使用 `registerPresenter`。Wuu 会传入冻结、
带版本且经过脱敏的 snapshot、原生 fallback，以及只包含当前边界可用动作的 host：

```js
export async function activate(api) {
  api.registerPresenter({
    id: "assistant-card",
    target: "conversation.item",
    key: "assistant-message",
    mode: "wrap",
    render({ host, fallback }) {
      const copy = host.actions.includes("conversation.item.copy")
        ? api.react.createElement("button", {
            onClick: () => host.invoke("conversation.item.copy"),
          }, "复制")
        : null;
      return api.react.createElement("article", null, fallback, copy);
    },
  });
}
```

当前内置 target：

| Presenter target | 稳定匹配 key |
| --- | --- |
| `conversation.item` | `assistant-message`、`reasoning`、`attachment` 等条目类型 |
| `conversation.process` | 完整 Process 形态：`reasoning`、`tool-group` 或 `mixed` |
| `conversation.tool-activity` | Tool 的稳定 capability，而不是改写后的执行名称 |
| `conversation.composer` | 无 |
| `header.conversation`、`header.workspace` | 无 |
| `navigation.primary` | 无 |
| `app.status` | 无 |
| `content.preview` | 完整 MIME 类型 |
| `settings` | 无 |

公开 SDK 定义了每种 V1 snapshot 和点分隔 Action ID。只有出现在 `host.actions` 中的 Action
才能调用；Wuu 会拒绝不支持的动作和非法输入。插件不会拿到私有 ThreadItem、协议消息、宿主
React 树或任意回调。

`mode: "replace"` 接管完整语义边界；`mode: "wrap"` 包装当前结果。Presenter 属于一个
generation：候选激活是原子的，激活失败保留旧 generation，渲染失败只回退当前边界，禁用、
升级或卸载会清除全部注册。`registerToolActivityPresenter` 继续作为兼容的 Tool 专用入口。

仓库中的 [`examples/plugins/deep-ui`](../../../examples/plugins/deep-ui/) 是一个可以直接安装
的自包含示例。它用 wrapper 保留所有宿主 fallback，并同时演示声明式主题。

[`examples/plugins/developer-loop`](../../../examples/plugins/developer-loop/) 是只依赖公开 SDK
的跨 Surface 验收示例，覆盖 Host Actions、generation 替换、失败恢复、disposal 和卸载。

## 声明式主题

无需执行桌面代码也可以在 `contributes.themes` 中声明主题。获批且启用的插件主题会出现在
“设置 → 外观”；禁用插件或切回系统、浅色、深色主题时，Wuu 会移除该插件设置的全部
Token。可用 Token 由 Manifest 白名单控制，包括 `--wuu-paper`、`--wuu-ink`、
`--wuu-ink-soft`、`--wuu-hairline`、`--wuu-surface-muted`、`--wuu-accent`、
`--wuu-accent-press` 和公开的 `--hljs-*` 语法色。

## 加载与安全

Renderer 不会读取插件的绝对路径。Wuu 在每次加载前都会由 app-server 重新加载 Manifest、
重算整个包 fingerprint，并确认插件在当前 workspace 中仍获批且启用。Electron 主进程再次
校验源码摘要，然后通过内容寻址的 `wuu-plugin:` 协议加载模块；CSP 不开放 `unsafe-eval`
或任意本地脚本。

普通声明式主题只能修改公开的语义 Token。`registerStyle` 可以使用任意 CSS，因此只提供给
受信任的桌面代码插件。依赖 Wuu 私有 class 名可以用于本地实验，但不属于兼容性承诺。

