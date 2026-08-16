# @wuu/mobile-web — Wuu 手机伴随端（Web 版）

电脑开着的时候，用手机浏览器当遥控器：看会话、发消息、打断、置顶，也可以
**新建对话**（在配对的工作区内，`thread/start`）。会话和 agent 仍然全部跑在
电脑上；本页面只是输入输出界面。

与 `clients/mobile`（RN/Expo 版）的关系：

- 数据层（`src/lib/store.ts`、`threads.ts`、`chatModel.ts`、`handoff.ts`、
  `format.ts`）与 `clients/mobile/src/lib` **镜像**，测试（`test/*.test.ts`）一并
  镜像，保证两个客户端行为一致。改动数据层时两边都要改（或迁移为共享包）。
- 控制层 `src/lib/controller.ts` 从 `clients/mobile/src/lib/connection.ts` 移植，
  删去了原生推送与深链（浏览器没有 OS push token）。
- 连接协议直接复用 `@wuu/remote-core`（配对、端到端加密通道、attach/resume、
  app-server RPC），Go 侧零改动。

## 运行

```bash
cd clients/mobile-web
npm install
npm run dev          # 监听 0.0.0.0，局域网可访问
```

手机（与电脑同一 Wi-Fi）打开 `http://<电脑局域网IP>:5173`。

生产构建：`npm run build` → `dist/`（纯静态文件，可放任何静态托管）。

## 配对与中继

1. 电脑上的 Wuu：**设置 → 远程**，保存中继地址，开启远程访问，点**显示配对码**。
2. 复制二维码下方的配对 URI。
3. 手机页面 → 粘贴 URI → 配对。

中继是自架的（`wuu relay [--addr HOST:PORT]`），电脑主动向外连中继（NAT 友好），
手机同样连中继。中继地址携带在配对 URI 里，页面本身不绑定任何中继。

局域网场景下中继地址形如 `ws://<电脑局域网IP>:8080/v1/connect`；要在外网用手机，
中继需部署在公网（`wss://`，反代加 TLS）。

## 测试

```bash
npm test
```

含一个**按需启用**的端到端冒烟（默认跳过，不依赖真实进程）：

```bash
# 终端 1：中继
wuu relay --addr 127.0.0.1:18123 --state /tmp/relay-state.json

# 终端 2：宿主（HOME 隔离，避免碰真实 remote.json）
HOME=/tmp/wuu-e2e-home wuu remote init --relay ws://127.0.0.1:18123/v1/connect
HOME=/tmp/wuu-e2e-home wuu remote host --workdir /tmp/wuu-e2e-work --pair

# 终端 3：把 host 打印的 pairing URI 传入
WUU_SMOKE_URI="wuu://pair?…" npx vitest run test/smoke.e2e.test.ts
```

冒烟验证的链路：配对握手 → 加密通道 attach → `initialize` → `thread/list`。

## 已知限制

- **无 iOS 推送**：浏览器页收不到 OS 推送（Android Chrome 可选 web push，未接）。
  页面开着时实时流正常；关掉后重新打开会自动 attach/resume。
- **单工作区**：远程协议面上，host 绑定启动时的工作区；页面没有跨工作区切换。
  需要时再给 kernel 加 workspaces 枚举。
- **凭据存 localStorage**：浏览器 origin 隔离弱于 Keychain/Keystore，不要把此
  页面部署到公共站点；局域网自用可接受。
- 消息为纯文本渲染（无 markdown），无扫码（粘贴 URI）。
