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
| `conversation.composer` | 替换或包装主输入区 |

`app.shell` context 提供 `openSettings`、`startNewThread`、`openSkills`、
`openAutomations` 和 `toggleSidebar` 等动作。Composer context 还提供当前文本、运行状态、
`setPrompt`、`send` 和 `interrupt`。

## 加载与安全

Renderer 不会读取插件的绝对路径。Wuu 在每次加载前都会由 app-server 重新加载 Manifest、
重算整个包 fingerprint，并确认插件在当前 workspace 中仍获批且启用。Electron 主进程再次
校验源码摘要，然后通过内容寻址的 `wuu-plugin:` 协议加载模块；CSP 不开放 `unsafe-eval`
或任意本地脚本。

普通声明式主题只能修改公开的语义 Token。`registerStyle` 可以使用任意 CSS，因此只提供给
受信任的桌面代码插件。依赖 Wuu 私有 class 名可以用于本地实验，但不属于兼容性承诺。

