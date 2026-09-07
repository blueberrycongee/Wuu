# @wuu/mobile-web — Wuu Web

Wuu Web 不再维护一套独立聊天界面。配对成功后，它直接加载
`desktop/src/renderer` 的工作台，并通过浏览器宿主适配器把 renderer 的
`window.wuu` 调用转发到现有加密远程通道。

```text
shared desktop renderer
          │ WuuDesktopApi
          ▼
RemoteDesktopBridge
          │ app-server line JSON
          ▼
@wuu/remote-core → sealed WebSocket → remote host → Go runtime
```

保留的旧 Web 核心能力是 `@wuu/remote-core`、凭据存储和 Go remote host。旧的
Home/Thread/Drawer 等页面已退出入口，不再作为新 Web 产品的实现基础。

## 运行

```bash
cd clients/mobile-web
npm install
npm run dev       # http://localhost:5174
```

电脑端在 **设置 → 远程** 开启远程访问并生成配对 URI；浏览器粘贴 URI 后，
Agent 和工作区仍运行在电脑上。

```bash
npm run typecheck
npm run build
```

## 当前边界

Web bridge 使用完整的类型检查接口，并通过 `unsupportedMethods` 声明不可用操作。
共享 renderer 据此限制原生窗口、文件管理器、语音、桌宠和本机文件选择器等入口。
会话、模型配置、会话组织、用户问题及 Agent 管理的进程使用已有 app-server RPC。
新建原生 PTY 不可用；Agent 启动的持久进程仍可读取、输入、调整尺寸和停止。

重连会原地重新读取已打开会话、排队消息和用户问题，保留本地草稿与标签选择。
离线操作立即失败，恢复失败可重试。旧连接的请求结果不能覆盖新连接。

文件面板需要目录导航、预览和聊天文件引用解析，因此增加三个只读 RPC：
`workspace/directory/list`、`workspace/file/read`、`workspace/file/resolve`。
它们只访问宿主已知的工作区、会话目录及其子目录，使用 rooted 文件访问阻止路径和符号链接逃逸。
文本预览上限为 512 KiB；PNG、JPEG、GIF、WebP 和 PDF 在 2 MiB 内通过同一加密通道返回。
文件编辑没有现有 renderer 调用方，因此没有增加远程写入接口。

仍需完成注册工作区选择、git 状态和 diff、手机视口布局及真实浏览器端到端验证。
Web 扩展模块加载暂时关闭，直到存在与 Electron 自定义协议同等的公共加载路径。

浏览器凭据保存在当前 origin 的 `localStorage`，隔离强度低于 Keychain/Keystore。
不要把该页面部署到不受信任的公共 origin；远程链路继续使用端到端加密和既有配对信任。
