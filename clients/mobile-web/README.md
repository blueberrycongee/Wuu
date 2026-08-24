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

第一阶段已接通会话加载、完整事件流、发送、排队、打断、归档、重命名和用户问题。
Electron 专属能力（本地文件选择器、PTY、嵌入式 WebContentsView、系统级设置）不能
伪装成浏览器能力，需逐项变成可选 capability 或增加窄的远程宿主 API。Web 扩展模块
加载也暂时关闭，直到存在与 Electron 自定义协议同等的公共加载路径。

浏览器凭据保存在当前 origin 的 `localStorage`，隔离强度低于 Keychain/Keystore。
不要把该页面部署到不受信任的公共 origin；远程链路继续使用端到端加密和既有配对信任。
