# 插件

插件让 Wuu 在保持宿主安全内核的前提下被重新组装：换主题、加设置、扩展 Agent 的
工具与策略、在桌面界面做有边界的结构贡献。插件平台当前是本地优先的——没有市场或
中心仓库，插件以目录或 zip 包的形式在本地安装。

插件分三类，信任成本不同：

| 类型 | 做什么 | 是否需要代码 |
| --- | --- | --- |
| 声明式主题与设置 | 在 `plugin.json` 中声明，无需执行插件代码 | 否 |
| Agent 插件 | 注册工具、挂钩工具执行、贡献系统提示、替换压缩策略 | 是，独立进程 |
| 桌面插件 | 注册样式、替换或包装稳定 UI Surface | 是，Renderer 代码 |

插件管理、审批、安全模式、崩溃恢复、权限底线和原生窗口生命周期始终由 Wuu 控制，
插件不能替换这些恢复路径。插件的定位是在 Wuu 设计语言骨架内做有边界的贡献和换肤，
而不是重做整个软件的外观；想要完全不同外观的开发者应当 fork 仓库。

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

## 插件能做什么

- **换主题**：获批且启用的插件主题出现在"设置 → 外观"，禁用或切回内置主题时
  Token 被完整移除。主题只需要 `plugin.json` 声明，不执行代码。
- **加设置**：插件可以声明自己的设置项（开关、文本、数字、枚举），在设置界面生成
  控件，按插件命名空间存储，卸载时清除。
- **扩展 Agent**：Agent 插件以独立进程运行，可以注册模型可见的工具、在工具执行
  前后挂钩（拦截、改写、包装）、贡献系统提示片段、参与请求改写与摘要压缩。
- **定制桌面界面**：注册全局样式、在稳定区域放置可持久化的 View、替换或包装语义
  Presenter（消息、工具活动、导航、设置等）、在固定 Slot 插入内容。渲染失败只回退
  当前边界，设置、禁用和默认 UI 恢复始终可用。

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

## 编写插件

要开发自己的插件，阅读[编写插件](plugin-authoring.md)：包结构与 manifest、Agent
插件与桌面插件的 API、本地开发闭环（`wuu plugin create/build/test/dev/pack`）以及
仓库中的两个可直接参考的示例。
