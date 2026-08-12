# Agent 插件快速上手

本教程带你用 10 分钟做出第一个可用的 Agent 插件：它注册一个模型可见的工具，
用命名空间 Storage 持久化计数，跑通"生成 → 构建 → 热重载 → 安装 → 运行"
的完整闭环。更完整的参考见[编写插件](plugin-authoring.md)。

想先修改桌面界面？改走[Desktop 插件快速上手](desktop-plugin-quickstart.md)。不确定
Skill、MCP、Hook 和插件怎样选？先看[扩展 Wuu](index.md)。

## 前置条件

- `wuu` CLI（确认 `wuu plugin --help` 可用）；
- Node.js 22+。

## 第 1 步：生成骨架

```bash
wuu plugin create hello-plugin
cd hello-plugin
```

生成的内容：

```text
hello-plugin/
├── plugin.json      # 清单：id、版本、runtime 声明
├── package.json     # build = tsc，依赖 @wuu/plugin-sdk
├── tsconfig.json
└── src/
    └── index.ts     # runtime 入口
```

`plugin.json` 声明了一个由 `node` 启动的长驻 runtime 进程：

```json
{
  "schema_version": 1,
  "id": "hello-plugin",
  "name": "hello-plugin",
  "version": "0.1.0",
  "description": "A Wuu plugin: hello-plugin",
  "runtime": {
    "protocol": "wuu-plugin-v1",
    "command": "node",
    "args": ["dist/index.js"]
  }
}
```

骨架的 `src/index.ts` 只实现了空的 `initialize`，并通过 SDK 的 `runJSONLRuntime`
接好标准输入输出协议：

```ts
import { runJSONLRuntime, type RuntimePlugin } from "@wuu/plugin-sdk";

const plugin: RuntimePlugin = {
  initialize(_params) {
    return { hooks: [], tools: [] };
  },
};

runJSONLRuntime(plugin, { input: process.stdin, output: process.stdout }).catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
```

## 第 2 步：注册一个工具

把 `src/index.ts` 改成下面的内容：注册 `greet` 工具，每次调用时把问候次数
累加进 workspace Storage，并把当前计数写进结果。

```ts
import {
  runJSONLRuntime,
  type RuntimePlugin,
  type ToolExecuteParams,
  type ToolExecuteResult,
} from "@wuu/plugin-sdk";

const plugin: RuntimePlugin = {
  initialize() {
    return {
      protocol_version: 2,
      tools: [
        {
          id: "greet",
          description: "Greet a name and count how many times greetings happened",
          input_schema: {
            type: "object",
            properties: { name: { type: "string" } },
            required: ["name"],
          },
          activity: { read_only: false, concurrency_safe: true, risk: "low" },
        },
      ],
      required_host_services: [
        { id: "host.storage.get", required: true },
        { id: "host.storage.set", required: true },
      ],
    };
  },

  async executeTool(params: ToolExecuteParams, host): Promise<ToolExecuteResult> {
    if (params.tool_id !== "greet") {
      return {
        result: {
          is_error: true,
          content: [{ type: "text", text: `unknown tool: ${params.tool_id}` }],
        },
      };
    }
    const name = (params.arguments as { name?: string }).name ?? "world";

    const current = await host.call("host.storage.get", {
      scope: "workspace",
      key: "greetings",
    });
    const count = Number.parseInt(current.value ?? "0", 10) + 1;
    await host.call("host.storage.set", {
      scope: "workspace",
      key: "greetings",
      value: String(count),
    });

    return {
      result: {
        content: [{ type: "text", text: `Hello, ${name}! (greeted ${count} times)` }],
      },
    };
  },
};

runJSONLRuntime(plugin, { input: process.stdin, output: process.stdout }).catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
```

要点：

- 工具在 `initialize` 返回的 `tools` 中注册（不是某个名为 `agent.tool.register`
  的 capability）；宿主会给工具加上插件命名空间，避免跨插件冲突；
- 模型可见的是 `description` 与 `input_schema`，写清楚参数与用途，模型才用得好；
- 插件进程通过 `host.call` 反向调用宿主服务，这里用了 `host.storage.get/set`；
  用到的服务必须声明在 `required_host_services`，宿主会校验；
- 结果用 `content` 文本返回；出错时设置 `is_error: true`。

## 第 3 步：构建与检查

```bash
npm install
npm run build        # tsc 编译到 dist/
wuu plugin validate .   # 校验 manifest 与包结构
wuu plugin test .       # 启动 runtime 并校验协商后的描述符
```

`wuu plugin test` 会真实启动 runtime，校验初始化、协议协商、capability 描述符和
工具注册；它不会执行 Host Service 调用。检查失败时以
非零退出码结束，可以直接接进 CI。

## 第 4 步：开发模式热重载

```bash
wuu plugin dev .
```

`wuu plugin dev .` 把**参数指定的路径**（这里是 `.`）授权为开发目录：每次保存后自动构建、校验候选并发布原子
generation；构建或激活失败时保留上一代。目录授权是开发专用，不会转移到从下载包
安装的普通插件。

## 第 5 步：安装并运行

```bash
wuu plugin pack .                      # 输出 hello-plugin-0.1.0.zip
wuu extension install ./hello-plugin-0.1.0.zip
```

安装就是信任决定：插件以你的用户权限执行。安装或启用来源表示信任该来源的代码；
同一 npm 包身份或同一 Git remote 的更新延续信任，来源身份改变时重新确认。Wuu
不审核、不认证、也不沙箱插件代码——只安装你信任的代码。

## 第 6 步：使用

在会话中让 Agent "用 greet 工具打个招呼"，观察工具活动卡片。再让它打一次招呼，
结果里的计数会 +1——说明 Storage 持久化生效了。

```bash
wuu extension disable hello-plugin   # 工具从会话中消失
wuu extension remove hello-plugin    # 卸载
```

## 下一步

- 给插件加上[声明式主题与设置](plugin-authoring.md)，不需要任何代码；
- 桌面插件变体：`wuu plugin create --type desktop my-ui` 会生成一个在
  `conversation.header` 注册内容的桌面入口骨架；
- 看仓库示例：`examples/plugins/stateful-runtime`（Storage CAS + 后台调用 +
  Session 创建/投递）、`examples/plugins/developer-loop`（跨 Surface 验收闭环）；
- 遇到激活失败时，在插件目录中查看 `failed/last_error`；用 `WUU_SAFE_MODE=1`
  或 `--safe-mode` 启动可让 Wuu 只发现 manifest、不激活任何插件代码，便于排查。
