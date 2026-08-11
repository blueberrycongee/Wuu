# Desktop 插件快速上手

本教程带你用 10 分钟给 Wuu 的 Composer 加一个可交互按钮，并跑通创建、构建、热重载、
打包和安装。Desktop 插件在 Wuu Renderer 中运行受信任的 React 代码，不需要 fork Wuu。

## 前置条件

- 已安装 `wuu` CLI，并确认 `wuu plugin --help` 可用；
- Node.js 22+；
- Wuu Desktop 正在运行。

## 第 1 步：生成 Desktop 骨架

```bash
wuu plugin create --type desktop focus-mode
cd focus-mode
npm install
```

生成的包包含 `plugin.json`、TypeScript 配置和 `src/index.ts`。manifest 的 Desktop 入口指向
构建后的 `dist/index.js`：

```json
{
  "schema_version": 1,
  "id": "focus-mode",
  "name": "focus-mode",
  "version": "0.1.0",
  "desktop": {
    "entry": "dist/index.js"
  }
}
```

Desktop 入口导出 `activate(api)`。`api` 属于当前插件 generation；插件禁用、升级或卸载时，
通过它注册的 UI、样式和清理函数会一起回收。

## 第 2 步：在 Composer 加一个按钮

把 `src/index.ts` 替换为：

```ts
import type { PluginGenerationApi } from "@wuu/plugin-sdk";

export function activate(api: PluginGenerationApi): void {
  const React = api.react;
  const ToolbarToggle = api.ui.ToolbarToggle as unknown as (
    props: Readonly<Record<string, unknown>>,
  ) => unknown;

  function FocusToggle() {
    const [enabled, setEnabled] = React.useState(false);
    return React.createElement(
      ToolbarToggle,
      {
        pressed: enabled,
        "aria-label": "切换专注模式",
        onClick: () => setEnabled((value) => !value),
      },
      enabled ? "专注中" : "专注",
    );
  }

  api.registerSlot("composer.toolbar", {
    id: "focus-toggle",
    order: 20,
    render() {
      return React.createElement(FocusToggle, null);
    },
  });
}
```

这里使用了三个关键能力：

- `api.react`：使用 Wuu 自己的 React，不要把另一份 React 打进插件；
- `api.ui`：使用会自动继承主题、密度和无障碍行为的宿主 UI Kit；
- `composer.toolbar`：把控件加入宿主拥有的 Composer 工具栏，而不是查找私有 DOM。

## 第 3 步：构建和检查

```bash
npm run build
wuu plugin validate .
wuu plugin test .
```

对于只有 Desktop 入口的包，`wuu plugin test` 会校验插件包并报告跳过 runtime 测试；它不会
导入或渲染 Desktop 入口。下一步需要在真实 App 中验证 Renderer 行为。

Desktop 入口必须是包内的自包含 ESM 文件。类型导入会在编译时移除；不要在构建结果中保留
指向插件源码的相对 import，也不要打包另一份 React。

## 第 4 步：在真实 App 中热重载

```bash
wuu plugin dev .
```

`wuu plugin dev .` 会授权命令参数指定的路径（这里是 `.`）。保存后 Wuu 构建并激活新的原子 generation；候选构建或激活失败时，
上一代继续工作。在 Wuu Desktop 的 Composer 工具栏中点击“专注”，确认状态可以切换。

主进程或宿主源码没有变化时不需要 fork 或重编 Wuu。插件只通过公开入口加载。

## 第 5 步：打包和安装

```bash
wuu plugin pack .
wuu plugin inspect ./focus-mode-0.1.0.zip
wuu plugin install ./focus-mode-0.1.0.zip
wuu plugin approve focus-mode
wuu plugin enable focus-mode
```

安装包的任何文件变化都会产生新 fingerprint，并要求用户重新检查和批准。开发目录授权不会
随 zip 一起转移。

## 下一步

- 查看[Desktop UI 扩展地图](desktop-plugins.md)，选择 View、Slot、Presenter 或 Surface；
- 跟着[插件场景教程](plugin-recipes.md)实现选区浮层、草稿写入和独立面板；
- 在[插件开发参考](plugin-authoring.md)中查 manifest、设置、Storage 和完整 API；
- 如果还需要 Agent 工具，创建 `--type full` 插件或阅读
  [Agent 插件快速上手](plugin-quickstart.md)。
