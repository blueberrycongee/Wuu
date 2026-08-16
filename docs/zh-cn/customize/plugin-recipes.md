# Desktop 插件场景教程

本页按用户需求组织常见组合方式。先阅读[Desktop UI 扩展地图](desktop-plugins.md)，并从
[Desktop 插件快速上手](desktop-plugin-quickstart.md)生成一个可运行的包。

## 给输入框添加按钮

使用 `composer.toolbar` Slot 和 `api.ui.ToolbarToggle` 或 `api.ui.Button`。宿主负责工具栏
布局、主题和可访问性，插件只负责自己的状态和点击行为。

```ts
api.registerSlot("composer.toolbar", {
  id: "my-action",
  render() {
    return api.react.createElement(
      api.ui.Button,
      { onClick: () => console.log("clicked") },
      "运行",
    );
  },
});
```

如果按钮需要读取或修改草稿，不要查找 textarea；改用 `conversation.composer` Presenter。

## 选中文字后添加到输入框

这个功能由三块组成：

1. 标准 Selection API 读取用户选中的文本和坐标；
2. 公开 `data-wuu-component="message"` 锚点确认选区来自消息流；
3. Composer Presenter 调用宿主 `set-draft` Action。

它不需要替换消息渲染，也不需要修改私有 React state。下面是核心实现：

```ts
import type {
  ComposerSnapshotV1,
  PluginGenerationApi,
  PresenterProps,
} from "@wuu/plugin-sdk";

const SET_DRAFT = "conversation.composer.set-draft";

export function activate(api: PluginGenerationApi): void {
  const React = api.react;

  function SelectionToolbar(input: Readonly<Record<string, unknown>>) {
    const props = input.presenter as PresenterProps;
    const snapshot = props.snapshot as ComposerSnapshotV1;
    const [selection, setSelection] = React.useState<null | {
      text: string;
      left: number;
      top: number;
    }>(null);

    React.useEffect(() => {
      const update = () => {
        const current = window.getSelection();
        if (!current || current.isCollapsed || current.rangeCount === 0) {
          setSelection(null);
          return;
        }

        const node = current.anchorNode;
        const element = node instanceof Element ? node : node?.parentElement;
        const text = current.toString().trim();
        if (!text || !element?.closest('[data-wuu-component="message"]')) {
          setSelection(null);
          return;
        }

        const rect = current.getRangeAt(0).getBoundingClientRect();
        setSelection({ text, left: rect.left, top: rect.top - 40 });
      };

      document.addEventListener("selectionchange", update);
      return () => document.removeEventListener("selectionchange", update);
    }, []);

    const append = async () => {
      if (!selection || !props.host.actions.includes(SET_DRAFT)) return;
      const current = snapshot.draftText?.trimEnd() ?? "";
      const quote = `> ${selection.text.replaceAll("\n", "\n> ")}`;
      await props.host.invoke(SET_DRAFT, current ? `${current}\n\n${quote}\n\n` : `${quote}\n\n`);
      setSelection(null);
    };

    return React.createElement(
      "div",
      { style: { display: "contents" } },
      props.fallback,
      selection && React.createElement(
        "div",
        {
          style: {
            position: "fixed",
            left: selection.left,
            top: selection.top,
            zIndex: 1000,
          },
          onMouseDown: (event: { preventDefault(): void }) => event.preventDefault(),
        },
        React.createElement(api.ui.Button, { onClick: append }, "添加到输入框"),
      ),
    );
  }

  api.registerPresenter({
    id: "selection-toolbar",
    target: "conversation.composer",
    mode: "wrap",
    render: (props) => React.createElement(SelectionToolbar, { presenter: props }),
  });
}
```

`onMouseDown.preventDefault()` 用于避免点击按钮前浏览器先清除选区。插件保存的是普通文本和
坐标，不保存宿主节点或 React 对象。生产实现还应处理窗口边缘、滚动、超长选区和本地化。

“翻译”“总结”“解释”等按钮可以复用同一选区，只需把不同模板和选中文字一起写入草稿。
默认让用户检查并补充 query 后再发送；如果确实要自动发送，先检查 `submit` 是否出现在
`host.actions`，再调用 `conversation.composer.submit`。

## 增加完整工作区工具

长列表、编辑器、图表或管理界面应使用 View，而不是塞进 Slot：

```ts
api.registerViewType({
  id: "my-plugin.dashboard",
  title: "Dashboard",
  persistence: "durable",
  render: Dashboard,
});

api.registerViewPlacement({
  id: "dashboard-default",
  view: "my-plugin.dashboard",
  region: "auxiliary",
});
```

再在 `plugin.json` 的 `contributes.workspaceTools`、`navigation` 或 `settingsPages` 中声明
面向用户的入口。宿主负责打开、关闭、Tab 和持久化。

## 改造消息或工具活动卡片

- 只在消息前后增加内容：使用 `conversation.message.before/after` Slot；
- 包装一条消息：使用 `conversation.item` Presenter 的 `wrap` 模式；
- 包装整个消息边界：使用 `conversation.message` Surface；
- 按工具 capability 改造活动卡片：使用 `conversation.tool-activity` Presenter。

Presenter 应读取公开 snapshot，并始终保留合理 fallback。不要解析私有 ThreadItem、猜测
工具内部状态或依赖宿主 class 名。

## 添加无需代码的主题

只改颜色、圆角或语法高亮时，在 manifest 中声明主题，不要加载 Desktop 代码：

```json
{
  "contributes": {
    "themes": [
      {
        "id": "calm-night",
        "name": "Calm Night",
        "base": "dark",
        "tokens": {
          "--wuu-color-canvas": "#151820",
          "--wuu-color-accent": "#8fa7ff"
        }
      }
    ]
  }
}
```

运行 `wuu plugin validate .` 后，在 Desktop 插件目录中添加本地目录并选择**批准并启用**。
主题会出现在**设置 → 外观**；
切回内置主题即可移除覆盖。完整 Token 见[主题 Token 参考](theme-surface-matrix.md)，用户操作见
[插件主题与设置](themes-settings.md)。

## 后台工作完成后显示结果

Desktop 模块可以注册 Command，也可以在后台事件或异步任务完成时调用
`showConversationCard`。Card 适合短生命周期交互；需要跨会话持久化和复杂导航时改用 View。

完整字段、设置、Storage、Runtime 通信和打包规则见[插件开发参考](plugin-authoring.md)。
