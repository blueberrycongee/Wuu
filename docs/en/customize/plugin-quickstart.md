# Agent plugin quickstart

This tutorial gets you to a working Agent plugin in about ten minutes: it
registers a model-visible tool, persists a counter in namespaced Storage, and
walks the full loop of generate → build → hot reload → install → run. For the
complete reference, see [Writing plugins](plugin-authoring.md).

Want to change the desktop UI first? Follow the
[Desktop plugin quickstart](desktop-plugin-quickstart.md). Not sure whether to use a
Skill, MCP, Hook, or plugin? Start with [Extend Wuu](index.md).

## Prerequisites

- The `wuu` CLI (confirm `wuu plugin --help` works);
- Node.js 22+.

## Step 1: generate a skeleton

```bash
wuu plugin create hello-plugin
cd hello-plugin
```

This generates:

```text
hello-plugin/
├── plugin.json      # manifest: id, version, runtime declaration
├── package.json     # build = tsc, depends on @wuu/plugin-sdk
├── tsconfig.json
└── src/
    └── index.ts     # runtime entry
```

`plugin.json` declares a long-lived runtime process started by `node`:

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

The skeleton's `src/index.ts` implements an empty `initialize` and wires the
standard input/output protocol through the SDK's `runJSONLRuntime`:

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

## Step 2: register a tool

Replace `src/index.ts` with the following: it registers a `greet` tool that
accumulates a greeting counter in workspace Storage and reports the current
count in its result.

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

Key points:

- Tools are registered through the `tools` field of the `initialize` result
  (not through a capability named `agent.tool.register`); the host adds a
  plugin namespace to each tool so plugins cannot collide.
- The model sees `description` and `input_schema`; write the parameters and
  purpose clearly so the model actually uses the tool well.
- The plugin process calls host services back through `host.call`; here it
  uses `host.storage.get/set`. Every service used must be declared in
  `required_host_services`; the host validates the list.
- Results return `content` text; set `is_error: true` on failure.

## Step 3: build and check

```bash
npm install
npm run build        # tsc compiles to dist/
wuu plugin validate .   # validate manifest and package structure
wuu plugin test .       # start the runtime and validate its negotiated descriptors
```

`wuu plugin test` starts the runtime and validates initialization, protocol
negotiation, capability descriptors, and tool registrations. It does not exercise
Host Service calls. Failures exit with a
non-zero code, so it fits into CI.

## Step 4: development-mode hot reload

```bash
wuu plugin dev .
```

`wuu plugin dev .` authorizes **the supplied path** (`.` here) as a development directory: on
every save it rebuilds, validates the candidate, and publishes an atomic
generation; if the build or activation fails, the previous generation stays.
Directory authorization is development-only and never transfers to ordinary
plugins installed from downloaded packages.

## Step 5: install and run

```bash
wuu plugin pack .                      # outputs hello-plugin-0.1.0.zip
wuu extension install ./hello-plugin-0.1.0.zip
```

Install is the trust decision: the plugin runs with your user authority.
Installing or enabling a source means trusting that source's code. Updates from
the same npm package identity or the same Git remote keep the trust; a change of
source identity asks for confirmation again. Wuu does not review, certify, or
sandbox plugin code — install only code you trust.

## Step 6: use it

In a conversation, ask the agent to "use the greet tool to say hi" and watch
the tool activity card. Ask it to greet again — the count in the result
increments, which shows the Storage persistence working.

```bash
wuu extension disable hello-plugin   # the tool disappears from sessions
wuu extension remove hello-plugin    # uninstall
```

## Next steps

- Add [declarative themes and settings](plugin-authoring.md#declarative-contributions)
  to the plugin; no code required.
- Desktop plugin variant: `wuu plugin create --type desktop my-ui` generates a
  desktop-entry skeleton that registers content in `conversation.header`.
- Study the repository examples: `examples/plugins/stateful-runtime` (Storage
  CAS, background calls, Session create/send) and
  `examples/plugins/developer-loop` (cross-surface acceptance loop).
- If activation fails, check `failed/last_error` in the plugin catalog; start
  with `WUU_SAFE_MODE=1` or `--safe-mode` to make Wuu discover manifests only
  without activating any plugin code, which helps debugging.
